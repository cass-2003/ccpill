package compose

import (
	"testing"

	"github.com/cass-2003/ccpill/internal/config"
	"github.com/cass-2003/ccpill/internal/input"
	"github.com/cass-2003/ccpill/internal/segment"
	"github.com/cass-2003/ccpill/internal/theme"
)

func strPtr(s string) *string { return &s }

func TestDetailSlotAndOverride(t *testing.T) {
	cfg := config.Default()
	cfg.Lines = [][]string{{"clock", "slot:备注", "slot:未定义", "unknown-id"}}
	cfg.Slots = []config.Slot{{Name: "备注", Text: "搬砖中", Color: "#89b4fa"}}
	cfg.Overrides = map[string]config.Override{
		"clock":   {Color: "#ff8800", BG: "#11111b", Bold: true},
		"slot:备注": {Bold: true},
	}
	rows := Detail(cfg, &input.Status{})
	if len(rows) != 1 || len(rows[0]) != 2 {
		t.Fatalf("应只剩 clock + 已定义插槽两项: %+v", rows)
	}
	clock := rows[0][0]
	if clock.ID != "clock" || clock.Pill == nil {
		t.Fatalf("clock 缺失: %+v", clock)
	}
	if clock.Pill.Color != (theme.RGB{R: 0xff, G: 0x88, B: 0x00}) || clock.Pill.BG == nil || !clock.Pill.Bold {
		t.Errorf("clock 覆盖未生效: %+v", clock.Pill)
	}
	slot := rows[0][1]
	if slot.ID != "slot:备注" || slot.Pill == nil || slot.Pill.Text != "搬砖中" {
		t.Fatalf("插槽渲染错误: %+v", slot)
	}
	if slot.Pill.Color != (theme.RGB{R: 0x89, G: 0xb4, B: 0xfa}) || !slot.Pill.Bold {
		t.Errorf("插槽颜色/加粗错误: %+v", slot.Pill)
	}
}

func TestOverrideLabel(t *testing.T) {
	ctx := &segment.Context{
		Cfg: config.Config{Overrides: map[string]config.Override{
			"x": {Label: strPtr("自定义 ")},
			"z": {Label: strPtr("")},
		}},
		Current: "x",
	}
	if got := ctx.L("默认 "); got != "自定义 " {
		t.Errorf("覆盖前缀 = %q", got)
	}
	ctx.Current = "y"
	if got := ctx.L("默认 "); got != "默认 " {
		t.Errorf("无覆盖应默认 = %q", got)
	}
	ctx.Cfg.Minimal = true
	if got := ctx.L("默认 "); got != "" {
		t.Errorf("minimal 应去前缀 = %q", got)
	}
	ctx.Current = "x"
	if got := ctx.L("默认 "); got != "自定义 " {
		t.Errorf("逐段覆盖应优先于 minimal = %q", got)
	}
	ctx.Current = "z"
	if got := ctx.L("默认 "); got != "" {
		t.Errorf("空串覆盖 = 去前缀，got %q", got)
	}
}
