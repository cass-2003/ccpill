package input

const defaultWindowSize = 200000

// ContextPercent 按优先级链计算上下文占用百分比（拆解 01 §2.1）：
// 1. stdin 官方 used_percentage；2. current_usage 分项自算（不含 output）；
// 3. current_usage 纯数字 / 窗口大小。返回 (百分比, 是否可得)。
// transcript 兜底解析属 V0.2 范围，此处不做。
func (s *Status) ContextPercent() (float64, bool) {
	cw := s.ContextWindow
	if cw == nil {
		return 0, false
	}
	if cw.UsedPercentage.Valid {
		return clamp(cw.UsedPercentage.Value), true
	}
	window := defaultWindowSize
	if cw.Size.Valid && cw.Size.Value > 0 {
		window = int(cw.Size.Value)
	}
	if d := cw.CurrentUsage.Detail; d != nil {
		// output tokens 不占用下一轮输入上下文，故意排除
		used := d.InputTokens.Value + d.CacheCreationTokens.Value + d.CacheReadTokens.Value
		return clamp(used / float64(window) * 100), true
	}
	if cw.CurrentUsage.Total.Valid {
		return clamp(cw.CurrentUsage.Total.Value / float64(window) * 100), true
	}
	return 0, false
}

// EffortLevel 返回思考等级；区分「显式无等级」与「未知」（拆解 01 §2.5）。
// ContextUsedTokens 返回当前上下文占用 token 数（output 不占下一轮输入，故不计）。
func (s *Status) ContextUsedTokens() (float64, bool) {
	cw := s.ContextWindow
	if cw == nil {
		return 0, false
	}
	if d := cw.CurrentUsage.Detail; d != nil {
		return d.InputTokens.Value + d.CacheCreationTokens.Value + d.CacheReadTokens.Value, true
	}
	if cw.CurrentUsage.Total.Valid {
		return cw.CurrentUsage.Total.Value, true
	}
	return 0, false
}

func (s *Status) EffortLevel() string {
	if s.Effort == nil || s.Effort.Level == nil {
		return ""
	}
	return *s.Effort.Level
}

// ModelName 返回展示用模型名。
func (s *Status) ModelName() string {
	if s.Model.DisplayName != "" {
		return s.Model.DisplayName
	}
	return s.Model.ID
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
