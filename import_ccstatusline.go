package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cass-2003/ccpill/internal/config"
)

// ccsWidget 是 ccstatusline settings.json 里 WidgetItem 的宽松映射（未知字段忽略）。
type ccsWidget struct {
	Type        string `json:"type"`
	Color       string `json:"color"`
	Bold        any    `json:"bold"` // boolean | "parens"
	Hide        bool   `json:"hide"`
	CustomText  string `json:"customText"`
	CommandPath string `json:"commandPath"`
}

type ccsSettings struct {
	Lines [][]ccsWidget `json:"lines"`
}

// ccsTypeMap 是 ccstatusline widget type → ccpill segment ID 的映射表
// （87 个内置 type 逐一核对自 widget-manifest.ts，拆解笔记 01 §3.5）。
var ccsTypeMap = map[string]string{
	"model": "modelname", "output-style": "outstyle", "thinking-effort": "think", "vim-mode": "vim",
	"git-branch": "gitbranch", "git-changes": "gitchanges", "git-insertions": "gitins",
	"git-deletions": "gitdel", "git-staged-files": "gitstaged", "git-unstaged-files": "gitunstaged",
	"git-untracked-files": "gituntracked", "git-clean-status": "gitstatus", "git-root-dir": "gitrepo",
	"git-review": "pr", "git-pr": "pr", "git-worktree": "worktree", "git-status": "gitstatus",
	"git-staged": "gitstaged", "git-unstaged": "gitunstaged", "git-untracked": "gituntracked",
	"git-ahead-behind": "gitab", "git-conflicts": "gitconflicts", "git-sha": "gitsha",
	"git-origin-owner": "gitremote", "git-origin-repo": "gitremote", "git-origin-owner-repo": "gitremote",
	"git-upstream-owner": "gitremote", "git-upstream-repo": "gitremote", "git-upstream-owner-repo": "gitremote",
	"current-working-dir": "dir",
	"tokens-input":        "tokin", "tokens-output": "tokout", "tokens-cached": "tokcache",
	"tokens-total": "toktotal", "cache-hit-rate": "cachehit", "cache-read": "tokcache", "cache-write": "tokwrite",
	"input-speed": "speedin", "output-speed": "speed", "total-speed": "speedtotal",
	"context-length": "ctxlen", "context-window": "ctxwin", "context-percentage": "ctxpct",
	"context-percentage-usable": "ctxusable", "context-bar": "ctxbar", "compaction-counter": "compact",
	"session-clock": "session", "session-cost": "cost", "session-name": "sessionname",
	"session-usage": "block", "block-timer": "blocktime", "reset-timer": "blocktime",
	"weekly-usage": "weekly", "weekly-reset-timer": "weekly",
	"weekly-sonnet-usage": "weeklysonnet", "weekly-opus-usage": "weeklyopus",
	"extra-usage-utilization": "overage", "extra-usage-remaining": "overage", "extra-usage-used": "overage",
	"terminal-width": "termwidth", "version": "version",
	"claude-session-id": "sessionid", "claude-account-email": "email", "free-memory": "memfree",
	"worktree-name": "worktree",
}

// ccsSkipTypes 是明确不迁移的 type（胶囊设计自带能力或 ccpill 未对齐项）。
var ccsSkipTypes = map[string]string{
	"separator": "胶囊自带分隔", "flex-separator": "胶囊自带分隔",
	"custom-symbol": "未对齐（用插槽替代）", "link": "未对齐（OSC8 终端支持参差）",
	"skills": "未对齐", "voice-status": "未对齐（私有 hook 生态）", "remote-control-status": "未对齐（私有 hook 生态）",
	"git-is-fork": "未对齐", "worktree-mode": "未对齐", "worktree-branch": "未对齐", "worktree-original-branch": "未对齐",
}

func ccsDefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ccstatusline", "settings.json")
}

// importCCStatusline 把 ccstatusline 布局迁移为 ccpill 配置：
// 布局逐行映射；custom-text/custom-command 转为插槽；hex 颜色与加粗转为 overrides；
// 未支持项打印原因、不中断。主题/分隔符等视觉体系不迁移（两家形态不同）。
func importCCStatusline(path string) error {
	if path == "" {
		path = ccsDefaultPath()
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取 ccstatusline 配置失败（默认路径 %s，可用 --import-ccstatusline <path> 指定）: %w", ccsDefaultPath(), err)
	}
	var s ccsSettings
	if err := json.Unmarshal(b, &s); err != nil {
		return fmt.Errorf("解析 %s 失败: %w", path, err)
	}

	cfg := config.Load() // 保留主题/图标集等既有设置，只替换布局相关
	cfg.Lines = nil
	if cfg.Overrides == nil {
		cfg.Overrides = map[string]config.Override{}
	}
	var skipped []string
	slotN := 0
	for _, line := range s.Lines {
		var row []string
		for _, w := range line {
			if w.Hide {
				continue
			}
			id, ok := ccsTypeMap[w.Type]
			switch {
			case ok:
			case w.Type == "custom-text" && w.CustomText != "":
				slotN++
				id = fmt.Sprintf("ccs-text-%d", slotN)
				cfg.Slots = append(cfg.Slots, config.Slot{Name: id, Text: w.CustomText, Color: hexOrEmpty(w.Color)})
				id = "slot:" + id
			case w.Type == "custom-command" && w.CommandPath != "":
				slotN++
				id = fmt.Sprintf("ccs-cmd-%d", slotN)
				cfg.Slots = append(cfg.Slots, config.Slot{Name: id, Command: w.CommandPath, Color: hexOrEmpty(w.Color)})
				id = "slot:" + id
			default:
				reason := ccsSkipTypes[w.Type]
				if reason == "" && len(w.Type) > 3 && w.Type[:3] == "jj-" {
					reason = "未对齐（Jujutsu 8 件，有需求再加）"
				}
				if reason == "" {
					reason = "未知 type"
				}
				skipped = append(skipped, w.Type+"（"+reason+"）")
				continue
			}
			row = appendUnique(row, id)
			applyCCSAppearance(cfg, id, w)
		}
		if len(row) > 0 && len(cfg.Lines) < 3 {
			cfg.Lines = append(cfg.Lines, row)
		}
	}
	if len(cfg.Lines) == 0 {
		return fmt.Errorf("%s 里没有可迁移的 widget（lines 为空？）", path)
	}

	if err := backupConfig(); err != nil {
		return err
	}
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Printf("✅ 已迁移 %d 行布局到 %s\n", len(cfg.Lines), config.Path())
	if slotN > 0 {
		fmt.Printf("   自定义 text/command 转为 %d 个插槽（ccs-*）\n", slotN)
	}
	if len(skipped) > 0 {
		fmt.Println("⚠ 未迁移（不影响其余项）:")
		for _, s := range skipped {
			fmt.Println("   -", s)
		}
	}
	fmt.Println("   主题/分隔符体系不迁移（形态不同）；跑 ccpill --config 微调外观")
	return nil
}

func hexOrEmpty(c string) string {
	if len(c) == 7 && c[0] == '#' {
		return c
	}
	return ""
}

// applyCCSAppearance 把 widget 的 hex 颜色/加粗转为 ccpill overrides（插槽颜色已随槽配置）。
func applyCCSAppearance(cfg config.Config, id string, w ccsWidget) {
	bold := w.Bold == true // "parens" 形态不支持，忽略
	hex := hexOrEmpty(w.Color)
	isSlot := len(id) > 5 && id[:5] == "slot:"
	if (hex == "" || isSlot) && !bold {
		return
	}
	o := cfg.Overrides[id]
	if hex != "" && !isSlot {
		o.Color = hex
	}
	o.Bold = bold
	cfg.Overrides[id] = o
}

func appendUnique(row []string, id string) []string {
	for _, x := range row {
		if x == id {
			return row
		}
	}
	return append(row, id)
}

// backupConfig 在覆盖前备份现有 config.toml（不存在则跳过）。
func backupConfig() error {
	p := config.Path()
	old, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	backup := p + ".bak-before-import"
	if err := os.WriteFile(backup, old, 0o644); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "已备份原配置到:", backup)
	return nil
}
