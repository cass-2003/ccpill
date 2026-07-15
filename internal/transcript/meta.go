// 会话元信息单趟扫描：会话名（/rename 的 custom-title）、用户消息数、
// 用户提问→首个 assistant 响应的耗时。与用量解析同一字节级预筛策略。
package transcript

import (
	"bytes"
	"encoding/json"
	"os"
	"time"
)

// Meta 是一个会话 transcript 的元信息。
type Meta struct {
	Title     string        // 最近一次 custom-title（/rename），无则空
	UserMsgs  int           // 真实用户消息数（不含 tool_result 回写）
	RespAvg   time.Duration // 用户提问到首个 assistant 响应的平均耗时
	RespLast  time.Duration // 最近一次响应耗时
	RespCount int
}

var (
	titleMarker     = []byte(`"custom-title"`)
	userMarker      = []byte(`"type":"user"`)
	assistantMarker = []byte(`"type":"assistant"`)
	toolUseMarker   = []byte(`"tool_use_id"`)
)

type metaLine struct {
	Type        string `json:"type"`
	CustomTitle string `json:"customTitle"`
	Timestamp   string `json:"timestamp"`
	IsSidechain bool   `json:"isSidechain"`
}

// ScanMeta 单趟扫描会话 transcript 的元信息；任何失败返回零值。
func ScanMeta(path string) Meta {
	var m Meta
	b, err := os.ReadFile(path)
	if err != nil {
		return m
	}
	var lastUser time.Time
	var respSum time.Duration
	for _, line := range bytes.Split(b, []byte("\n")) {
		switch {
		case bytes.Contains(line, titleMarker):
			var l metaLine
			if json.Unmarshal(line, &l) == nil && l.Type == "custom-title" && l.CustomTitle != "" {
				m.Title = l.CustomTitle
			}
		case bytes.Contains(line, userMarker) && !bytes.Contains(line, toolUseMarker):
			var l metaLine
			if json.Unmarshal(line, &l) != nil || l.IsSidechain {
				continue
			}
			m.UserMsgs++
			if ts, err := time.Parse(time.RFC3339, l.Timestamp); err == nil {
				lastUser = ts
			}
		case bytes.Contains(line, assistantMarker):
			if lastUser.IsZero() {
				continue
			}
			var l metaLine
			if json.Unmarshal(line, &l) != nil || l.IsSidechain {
				continue
			}
			ts, err := time.Parse(time.RFC3339, l.Timestamp)
			if err != nil {
				continue
			}
			if d := ts.Sub(lastUser); d > 0 {
				m.RespLast = d
				respSum += d
				m.RespCount++
			}
			lastUser = time.Time{} // 只配对首个响应
		}
	}
	if m.RespCount > 0 {
		m.RespAvg = respSum / time.Duration(m.RespCount)
	}
	return m
}
