package input

import "testing"

func TestParseFlexibleForms(t *testing.T) {
	// 覆盖拆解 01 §1 的全部容错形态：字符串数字、model 对象、usage 对象、effort null
	raw := []byte(`{
		"model": {"id": "claude-fable-5", "display_name": "Fable 5"},
		"cost": {"total_cost_usd": "4.83"},
		"effort": {"level": null},
		"context_window": {
			"context_window_size": 200000,
			"current_usage": {"input_tokens": 100000, "output_tokens": 5000,
				"cache_creation_input_tokens": 14000, "cache_read_input_tokens": 10000}
		},
		"unknown_future_field": {"nested": true}
	}`)
	s, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got := s.ModelName(); got != "Fable 5" {
		t.Errorf("ModelName = %q", got)
	}
	if !s.Cost.TotalCostUSD.Valid || s.Cost.TotalCostUSD.Value != 4.83 {
		t.Errorf("字符串编码数字解析失败: %+v", s.Cost.TotalCostUSD)
	}
	if got := s.EffortLevel(); got != "" {
		t.Errorf("effort.level=null 应视为显式无等级, got %q", got)
	}
	// 分项自算：(100000+14000+10000)/200000 = 62%，不含 output
	pct, ok := s.ContextPercent()
	if !ok || pct != 62 {
		t.Errorf("ContextPercent = %v, %v; want 62, true", pct, ok)
	}
}

func TestParseModelString(t *testing.T) {
	s, err := Parse([]byte(`{"model": "claude-opus-4-8"}`))
	if err != nil || s.ModelName() != "claude-opus-4-8" {
		t.Errorf("model 字符串形态解析失败: %v, %q", err, s.ModelName())
	}
}

func TestContextPercentPriority(t *testing.T) {
	// stdin 官方 used_percentage 优先级最高，即使分项也在
	s, _ := Parse([]byte(`{"context_window": {
		"used_percentage": 45.5,
		"current_usage": {"input_tokens": 190000}
	}}`))
	if pct, ok := s.ContextPercent(); !ok || pct != 45.5 {
		t.Errorf("官方 used_percentage 未被优先: %v, %v", pct, ok)
	}
}

func TestContextPercentNumberForm(t *testing.T) {
	s, _ := Parse([]byte(`{"context_window": {"current_usage": 100000}}`))
	if pct, ok := s.ContextPercent(); !ok || pct != 50 {
		t.Errorf("纯数字 current_usage/默认窗口: %v, %v; want 50", pct, ok)
	}
}

func TestContextPercentMissing(t *testing.T) {
	s, _ := Parse([]byte(`{}`))
	if _, ok := s.ContextPercent(); ok {
		t.Error("无 context_window 应返回 ok=false")
	}
}
