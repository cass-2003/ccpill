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

// Span 是胶囊内的一个彩色片段，供同一胶囊内多前景色（如 gitab 的 +绿/−红）。
type Span struct {
	Text  string
	Color theme.RGB
}

// Pill 是一颗待渲染的胶囊。
type Pill struct {
	Text  string
	Color theme.RGB // Normal 时的前景色
	Level Level
	Spans []Span     // 非空时逐段着色渲染；Text 仍需填完整文本（Warn 反色与回退场景用）
	BG    *theme.RGB // 单颗胶囊底色覆盖（nil = 主题统一底色；Warn 反色时不生效）
	Bold  bool
}

// Options 控制渲染形态。
type Options struct {
	Theme    theme.Theme
	PillMode bool   // false = 无胶囊模式：彩色文字 + 细分隔线
	CapL     string // 圆角端帽字形（Nerd Font 半圆；空 = 平角矩形）
	CapR     string
}

const reset = "\x1b[0m"

// CapGlyphs 解析胶囊端帽字形。mode: "round" 强制圆角 / "flat" 强制平角 /
// 其他值(含空)=auto：仅 nerd 图标档默认圆角。圆角用 Powerline 半圆（需字体含
// Powerline 扩展字形，Windows Terminal 默认 Cascadia Mono 即支持）。
func CapGlyphs(mode, iconSet string) (capL, capR string) {
	switch mode {
	case "round":
		return "\ue0b6", "\ue0b4"
	case "flat":
		return "", ""
	default:
		if iconSet == "nerd" {
			return "\ue0b6", "\ue0b4"
		}
		return "", ""
	}
}

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
			renderPill(&b, p, opt)
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

func renderPill(b *strings.Builder, p Pill, opt Options) {
	t := opt.Theme
	pillBG, textFG := t.PillBG, p.Color
	if p.BG != nil {
		pillBG = *p.BG
	}
	if p.Level == Warn {
		// 预警胶囊整体反色：红底深字
		pillBG, textFG = t.Warn, t.WarnFG
	}
	// 圆角端帽：半圆字形以胶囊底色作前景、不带背景，与色块拼成圆角胶囊
	if opt.CapL != "" {
		b.WriteString(fg(pillBG) + opt.CapL + reset)
	}
	b.WriteString(bg(pillBG) + fg(textFG))
	if p.Bold {
		b.WriteString("\x1b[1m")
	}
	b.WriteString(" ")
	if len(p.Spans) > 0 && p.Level != Warn { // Warn 反色时统一用 WarnFG 保证红底可读
		for _, s := range p.Spans {
			b.WriteString(fg(s.Color) + s.Text)
		}
	} else {
		b.WriteString(p.Text)
	}
	b.WriteString(" " + reset)
	if opt.CapR != "" {
		b.WriteString(fg(pillBG) + opt.CapR + reset)
	}
}

func renderPlain(b *strings.Builder, p Pill, t theme.Theme) {
	if p.Bold {
		b.WriteString("\x1b[1m")
	}
	if len(p.Spans) > 0 && p.Level != Warn {
		for _, s := range p.Spans {
			b.WriteString(fg(s.Color) + s.Text)
		}
		b.WriteString(reset)
		return
	}
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
