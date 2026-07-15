package theme

import "testing"

func TestParseHex(t *testing.T) {
	if c, ok := ParseHex("#a6e3a1"); !ok || c != (RGB{0xa6, 0xe3, 0xa1}) {
		t.Errorf("ParseHex(#a6e3a1) = %v, %v", c, ok)
	}
	if c, ok := ParseHex("#A6E3A1"); !ok || c != (RGB{0xa6, 0xe3, 0xa1}) {
		t.Errorf("大写应可解析: %v, %v", c, ok)
	}
	for _, bad := range []string{"", "#fff", "a6e3a1", "#zzzzzz", "#a6e3a1ff"} {
		if _, ok := ParseHex(bad); ok {
			t.Errorf("ParseHex(%q) 应失败", bad)
		}
	}
}
