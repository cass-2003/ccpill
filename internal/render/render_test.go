package render

import (
	"strings"
	"testing"

	"ccpill/internal/theme"
)

var (
	green = theme.RGB{R: 0xa6, G: 0xe3, B: 0xa1}
	red   = theme.RGB{R: 0xf3, G: 0x8b, B: 0xa8}
)

// Spans：胶囊内逐段着色，两种颜色都要出现在 ANSI 输出里。
func TestLineSpans(t *testing.T) {
	p := Pill{
		Text:  "+2 −1",
		Color: theme.RGB{R: 1, G: 2, B: 3},
		Spans: []Span{{Text: "+2", Color: green}, {Text: " −1", Color: red}},
	}
	for _, pillMode := range []bool{true, false} {
		out := Line([]Pill{p}, Options{Theme: theme.Get(""), PillMode: pillMode})
		for _, want := range []string{"38;2;166;227;161m+2", "38;2;243;139;168m"} {
			if !strings.Contains(out, want) {
				t.Errorf("pillMode=%v: 输出缺少 %q\n%q", pillMode, want, out)
			}
		}
	}
}

// Warn 反色时忽略 Spans 颜色，整体用 WarnFG 保证红底可读。
func TestLineSpansWarnFallback(t *testing.T) {
	th := theme.Get("")
	p := Pill{
		Text:  "+2 −1",
		Level: Warn,
		Spans: []Span{{Text: "+2", Color: green}, {Text: " −1", Color: red}},
	}
	out := Line([]Pill{p}, Options{Theme: th, PillMode: true})
	if strings.Contains(out, "38;2;166;227;161m") {
		t.Errorf("Warn 胶囊不应保留 span 自身颜色\n%q", out)
	}
	// 渲染层会把空格换成 U+00A0（防 VSCode 终端裁剪）
	if !strings.Contains(out, "+2 −1") {
		t.Errorf("Warn 胶囊应回退渲染完整 Text\n%q", out)
	}
}
