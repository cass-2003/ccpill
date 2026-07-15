package render

import "strings"

// IconSet 是三档图标降级方案（PRD §3.4）。
type IconSet struct {
	Model, Branch, Cost, Clock, Flame, Warn, Dir string
	Dirty, Ahead, Behind                         string
	BarFull, BarEmpty                            string
	// 胶囊圆角端帽（Powerline 半圆字形，需 Nerd Font；空 = 平角矩形）
	CapL, CapR string
}

// nerd 档字形用显式 \u 转义（Nerd Font 打包的 Font Awesome/Powerline 码位），
// 避免源码里出现不可见 PUA 字符。
var iconSets = map[string]IconSet{
	"nerd": {
		Model: "", Branch: "", Cost: "", Clock: "",
		Flame: "", Warn: "", Dir: "",
		Dirty: "✚", Ahead: "↑", Behind: "↓",
		BarFull: "●", BarEmpty: "○",
		CapL: "\ue0b6", CapR: "\ue0b4", // Powerline 左/右半圆（圆角胶囊端帽）
	},
	"unicode": {
		Model: "⚡", Branch: "⎇", Cost: "$", Clock: "⏱",
		Flame: "🔥", Warn: "⚠", Dir: "📁",
		Dirty: "✚", Ahead: "↑", Behind: "↓",
		BarFull: "●", BarEmpty: "○",
	},
	"ascii": {
		Model: "*", Branch: "git:", Cost: "$", Clock: "t:",
		Flame: "~", Warn: "!", Dir: "",
		Dirty: "+", Ahead: "^", Behind: "v",
		BarFull: "#", BarEmpty: "-",
	},
}

// Icons 返回指定档位图标集，未知名字回退 unicode 档。
func Icons(name string) IconSet {
	if s, ok := iconSets[name]; ok {
		return s
	}
	return iconSets["unicode"]
}

// Bar 绘制 width 格的百分比块状条（设计稿 C 的 ●●●●○○ 形态）。
func Bar(percent float64, width int, is IconSet) string {
	if width <= 0 {
		width = 10
	}
	filled := int(percent/100*float64(width) + 0.5)
	if filled > width {
		filled = width
	}
	return strings.Repeat(is.BarFull, filled) + strings.Repeat(is.BarEmpty, width-filled)
}
