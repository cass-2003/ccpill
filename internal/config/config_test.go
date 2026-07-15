package config

import "testing"

// overrides 里 label 的三态（nil=默认 / ""=去前缀 / 文字=替换）必须能安全过 TOML 往返。
func TestSaveLoadOverridesAndSlots(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
	empty := ""
	custom := "M "
	cfg := Default()
	cfg.Overrides = map[string]Override{
		"model": {Color: "#ff8800", Label: &empty, Bold: true},
		"today": {Label: &custom},
		"clock": {BG: "#11111b"}, // Label 缺省 = nil
	}
	cfg.Slots = []Slot{{Name: "ip", Command: "echo hi", Color: "#89b4fa"}, {Name: "备注", Text: "搬砖中"}}
	if err := Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got := Load()
	m := got.Overrides["model"]
	if m.Color != "#ff8800" || !m.Bold || m.Label == nil || *m.Label != "" {
		t.Errorf("model 覆盖往返失真: %+v (label=%v)", m, m.Label)
	}
	if l := got.Overrides["today"].Label; l == nil || *l != "M " {
		t.Errorf("today label 往返失真: %v", l)
	}
	if c := got.Overrides["clock"]; c.BG != "#11111b" || c.Label != nil {
		t.Errorf("clock 往返失真（nil label 应保持 nil）: %+v label=%v", c, c.Label)
	}
	if len(got.Slots) != 2 || got.Slots[0].Command != "echo hi" || got.Slots[1].Text != "搬砖中" {
		t.Errorf("插槽往返失真: %+v", got.Slots)
	}
	if got.FindSlot("ip") == nil || got.FindSlot("nope") != nil {
		t.Error("FindSlot 行为错误")
	}
}
