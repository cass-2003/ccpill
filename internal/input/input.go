// Package input 解析 Claude Code 传给 statusline 命令的 stdin JSON。
// Schema 与容错规则来自竞品拆解（docs/research/01 §1）：
// 数字字段可能被序列化为字符串；model 有字符串/对象两种形态；
// current_usage 有纯数字/分项对象两种形态；未知字段必须忽略而非报错。
package input

import (
	"encoding/json"
	"strconv"
	"strings"
)

// FlexFloat 兼容 JSON number 与字符串编码数字两种形态。
type FlexFloat struct {
	Value float64
	Valid bool
}

func (f *FlexFloat) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" || s == `""` {
		return nil
	}
	s = strings.Trim(s, `"`)
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return nil // 容错：解析不了视为缺失，不让整个 stdin 解析失败
	}
	f.Value, f.Valid = v, true
	return nil
}

// Model 兼容 "claude-x" 字符串与 {id, display_name} 对象两种历史形态。
type Model struct {
	ID          string
	DisplayName string
}

func (m *Model) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		m.ID = s
		return nil
	}
	var obj struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	}
	if err := json.Unmarshal(b, &obj); err == nil {
		m.ID, m.DisplayName = obj.ID, obj.DisplayName
	}
	return nil
}

// Usage 是 current_usage 的分项对象形态。
type Usage struct {
	InputTokens         FlexFloat `json:"input_tokens"`
	OutputTokens        FlexFloat `json:"output_tokens"`
	CacheCreationTokens FlexFloat `json:"cache_creation_input_tokens"`
	CacheReadTokens     FlexFloat `json:"cache_read_input_tokens"`
}

// CurrentUsage 兼容纯数字与分项对象两种形态。
type CurrentUsage struct {
	Total  FlexFloat // 纯数字形态
	Detail *Usage    // 对象形态
}

func (c *CurrentUsage) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" {
		return nil
	}
	if strings.HasPrefix(s, "{") {
		var u Usage
		if err := json.Unmarshal(b, &u); err == nil {
			c.Detail = &u
		}
		return nil
	}
	_ = c.Total.UnmarshalJSON(b)
	return nil
}

type ContextWindow struct {
	Size                FlexFloat    `json:"context_window_size"`
	TotalInputTokens    FlexFloat    `json:"total_input_tokens"`
	TotalOutputTokens   FlexFloat    `json:"total_output_tokens"`
	CurrentUsage        CurrentUsage `json:"current_usage"`
	UsedPercentage      FlexFloat    `json:"used_percentage"`
	RemainingPercentage FlexFloat    `json:"remaining_percentage"`
}

type Cost struct {
	TotalCostUSD       FlexFloat `json:"total_cost_usd"`
	TotalDurationMS    FlexFloat `json:"total_duration_ms"`
	TotalAPIDurationMS FlexFloat `json:"total_api_duration_ms"`
	TotalLinesAdded    FlexFloat `json:"total_lines_added"`
	TotalLinesRemoved  FlexFloat `json:"total_lines_removed"`
}

type RateWindow struct {
	UsedPercentage FlexFloat `json:"used_percentage"`
	ResetsAt       FlexFloat `json:"resets_at"` // Unix epoch 秒
}

type RateLimits struct {
	FiveHour *RateWindow `json:"five_hour"`
	SevenDay *RateWindow `json:"seven_day"`
}

type Workspace struct {
	CurrentDir string `json:"current_dir"`
	ProjectDir string `json:"project_dir"`
}

// Effort 的 level=null 与整个键缺失语义不同（拆解 01 §2.5），用指针区分。
type Effort struct {
	Level *string `json:"level"`
}

type Status struct {
	SessionID      string    `json:"session_id"`
	TranscriptPath string    `json:"transcript_path"`
	CWD            string    `json:"cwd"`
	Model          Model     `json:"model"`
	Workspace      Workspace `json:"workspace"`
	Version        string    `json:"version"`
	OutputStyle    struct {
		Name string `json:"name"`
	} `json:"output_style"`
	Effort        *Effort        `json:"effort"`
	Cost          Cost           `json:"cost"`
	ContextWindow *ContextWindow `json:"context_window"`
	Vim           struct {
		Mode *string `json:"mode"`
	} `json:"vim"`
	Worktree struct {
		Name   string `json:"name"`
		Branch string `json:"branch"`
	} `json:"worktree"`
	RateLimits *RateLimits `json:"rate_limits"`
}

// Parse 解析 stdin JSON；对局部字段容错，只有整体非法 JSON 才报错。
func Parse(b []byte) (*Status, error) {
	var s Status
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
