// Package render 把 segment 结果渲染为带 ANSI 的胶囊状态栏文本。
// 关键 hack 来自拆解 01 §4.3：行首强制 \x1b[0m 覆盖 Claude Code 的 dim；
// 普通空格→U+00A0 防 VSCode 终端裁剪。
package render

import (
	"strings"

	"ccpill/internal/theme"
)

// Level 是 segment 的预警等级，决定胶囊配色。
type Level int

const (
	Normal Level = iota
	Warn
)

// Pill 是一颗待渲染的胶囊。
type Pill struct {
	Text  string
	Color theme.RGB // Normal 时的前景色
	Level Level
}

// Options 控制渲染形态。
type Options struct {
	Theme    theme.Theme
	PillMode bool // false = 无胶囊模式：彩色文字 + 细分隔线
}

const reset = "\x1b[0m"

func fg(c theme.RGB) string {
	return "\x1b[38;2;" + itoa(c.R) + ";" + itoa(c.G) + ";" + itoa(c.B) + "m"
}

func bg(c theme.RGB) string {
	return "\x1b[48;2;" + itoa(c.R) + ";" + itoa(c.G) + ";" + itoa(c.B) + "m"
}

// Line 渲染一行胶囊。
func Line(pills []Pill, opt Options) string {
	if len(pills) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(reset) // 覆盖 Claude Code 可能设置的 dim
	for i, p := range pills {
		if p.Text == "" {
			continue
		}
		if opt.PillMode {
			renderPill(&b, p, opt.Theme)
			if i < len(pills)-1 {
				b.WriteString(" ")
			}
		} else {
			if i > 0 {
				b.WriteString(fg(opt.Theme.Sep) + " │ " + reset)
			}
			renderPlain(&b, p, opt.Theme)
		}
	}
	// U+00A0 防 VSCode 集成终端裁剪行首尾空格
	return strings.ReplaceAll(b.String(), " ", " ")
}

func renderPill(b *strings.Builder, p Pill, t theme.Theme) {
	if p.Level == Warn {
		// 预警胶囊整体反色：红底深字
		b.WriteString(bg(t.Warn) + fg(t.WarnFG) + " " + p.Text + " " + reset)
		return
	}
	b.WriteString(bg(t.PillBG) + fg(p.Color) + " " + p.Text + " " + reset)
}

func renderPlain(b *strings.Builder, p Pill, t theme.Theme) {
	c := p.Color
	if p.Level == Warn {
		c = t.Warn
	}
	b.WriteString(fg(c) + p.Text + reset)
}

func itoa(v uint8) string {
	if v == 0 {
		return "0"
	}
	var buf [3]byte
	i := 3
	for v > 0 {
		i--
		buf[i] = '0' + v%10
		v /= 10
	}
	return string(buf[i:])
}
