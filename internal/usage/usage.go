// Package usage 聚合跨会话用量：今日总花费、5h billing block、burn rate。
// 算法复刻 ccusage（docs/research/03 §3-5）：块起点=首条消息 floor 到 UTC 整点，
// 换块双条件（距起点>5h 或空闲>5h），burn rate 分母=块内首末条目间隔。
// 全量扫描结果整体缓存 60s（无常驻进程架构）。
package usage

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"ccpill/internal/cache"
	"ccpill/internal/transcript"
)

const (
	blockDur = 5 * time.Hour
	cacheTTL = 60 * time.Second
)

// Summary 是一次全量扫描的聚合结果。
type Summary struct {
	TodayCost      float64 `json:"today_cost"`
	BlockCost      float64 `json:"block_cost"`
	BlockRemainMin int     `json:"block_remain_min"` // 活跃块剩余分钟；无活跃块为 -1
	CostPerHour    float64 `json:"cost_per_hour"`    // burn rate；无法计算为 0
}

// Load 返回聚合结果；命中缓存直接用，否则全量扫描并回写。
func Load() Summary {
	var s Summary
	if cache.Get("usage", cacheTTL, &s) {
		return s
	}
	s = scan()
	cache.Put("usage", s)
	return s
}

func projectsDir() string {
	base := os.Getenv("CLAUDE_CONFIG_DIR")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".claude")
	}
	return filepath.Join(base, "projects")
}

func scan() Summary {
	s := Summary{BlockRemainMin: -1}
	dir := projectsDir()
	if dir == "" {
		return s
	}
	var entries []transcript.Entry
	// projects/<slug>/*.jsonl 两层结构；忽略遍历错误（尽力而为）
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".jsonl" {
			return nil
		}
		entries = append(entries, transcript.ReadFile(path)...)
		return nil
	})
	entries = transcript.Dedup(entries)
	if len(entries) == 0 {
		return s
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Timestamp.Before(entries[j].Timestamp) })

	now := time.Now()
	y, m, d := now.Date()
	dayStart := time.Date(y, m, d, 0, 0, 0, 0, now.Location())

	// 单趟同时算 today 与当前块（块划分只需顺序遍历）
	var blockStart time.Time
	var blockEntries []transcript.Entry
	flushIfStale := func(e transcript.Entry) {
		if !blockStart.IsZero() {
			last := blockEntries[len(blockEntries)-1].Timestamp
			if e.Timestamp.Sub(blockStart) > blockDur || e.Timestamp.Sub(last) > blockDur {
				blockStart, blockEntries = floorHourUTC(e.Timestamp), nil
			}
		} else {
			blockStart = floorHourUTC(e.Timestamp)
		}
	}
	for _, e := range entries {
		if !e.Timestamp.Before(dayStart) {
			s.TodayCost += entryCost(e)
		}
		flushIfStale(e)
		blockEntries = append(blockEntries, e)
	}

	// 活跃判定双条件：距最后一条 <5h 且 now < 块名义终点
	last := blockEntries[len(blockEntries)-1].Timestamp
	blockEnd := blockStart.Add(blockDur)
	if now.Sub(last) < blockDur && now.Before(blockEnd) {
		s.BlockRemainMin = int(blockEnd.Sub(now).Minutes())
		var first = blockEntries[0].Timestamp
		for _, e := range blockEntries {
			s.BlockCost += entryCost(e)
		}
		if durMin := last.Sub(first).Minutes(); durMin > 0 {
			s.CostPerHour = s.BlockCost / durMin * 60
		}
	}
	return s
}

func floorHourUTC(t time.Time) time.Time {
	return t.UTC().Truncate(time.Hour)
}
