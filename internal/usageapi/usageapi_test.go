package usageapi

import "testing"

func TestParse(t *testing.T) {
	body := `{
		"five_hour": {"utilization": 34.2, "resets_at": "2026-07-15T12:00:00Z"},
		"seven_day": {"utilization": 62.0},
		"seven_day_sonnet": {"utilization": 8.5},
		"seven_day_opus": {"utilization": 91.0},
		"extra_usage": {"is_enabled": true, "monthly_limit": 3894, "used_credits": 106.5, "utilization": 2.7, "currency": "USD"}
	}`
	d := parse([]byte(body))
	if !d.OK || d.SevenDaySonnet != 8.5 || d.SevenDayOpus != 91.0 {
		t.Errorf("分模型周额 = %+v", d)
	}
	if !d.ExtraEnabled || d.ExtraLimit != 3894 || d.ExtraUsed != 106.5 || d.ExtraUtil != 2.7 {
		t.Errorf("超额 = %+v", d)
	}
}

func TestParseMissingBuckets(t *testing.T) {
	// Enterprise 账号可能没有分模型桶（对齐 ccstatusline #343 的空桶处理）
	d := parse([]byte(`{"five_hour": null, "seven_day_sonnet": null}`))
	if !d.OK || d.SevenDaySonnet >= 0 || d.SevenDayOpus >= 0 || d.ExtraEnabled {
		t.Errorf("空桶应为 <0 且超额关闭: %+v", d)
	}
	if d := parse([]byte(`not json`)); d.OK {
		t.Error("坏 JSON 应 OK=false")
	}
}
