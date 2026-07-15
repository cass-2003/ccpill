package render

import "strings"

// IconSet 是三档图标降级方案（PRD §3.4）。
type IconSet struct {
	Model, Branch, Cost, Clock, Flame, Warn, Dir string
	Dirty, Ahead, Behind                         string
	BarFull, BarEmpty                            string
	Pie                                          string // 5 档饼形字形（空 = 该档位不支持，退化为文字）
	Spark                                        string // 8 级迷你柱字形（空 = 不画 sparkline）
}

// nerd 档字形用显式 \u 转义（Nerd Font 打包的 Font Awesome/Powerline 码位），
// 避免源码里出现不可见 PUA 字符。
var iconSets = map[string]IconSet{
	"nerd": {
		Model: "", Branch: "", Cost: "", Clock: "",
		Flame: "", Warn: "", Dir: "",
		Dirty: "✚", Ahead: "↑", Behind: "↓",
		BarFull: "●", BarEmpty: "○",
		Pie: "○◔◑◕●", Spark: "▁▂▃▄▅▆▇█",
	},
	"unicode": {
		Model: "⚡", Branch: "⎇", Cost: "$", Clock: "⏱",
		Flame: "🔥", Warn: "⚠", Dir: "📁",
		Dirty: "✚", Ahead: "↑", Behind: "↓",
		BarFull: "●", BarEmpty: "○",
		Pie: "○◔◑◕●", Spark: "▁▂▃▄▅▆▇█",
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

// Pie 返回百分比对应的饼形字形（○◔◑◕●，每 20% 一档）；档位无 Pie 字形时返回空串。
func Pie(percent float64, is IconSet) string {
	glyphs := []rune(is.Pie)
	if len(glyphs) == 0 {
		return ""
	}
	idx := int(percent / 100 * float64(len(glyphs)))
	if idx >= len(glyphs) {
		idx = len(glyphs) - 1
	}
	if idx < 0 {
		idx = 0
	}
	return string(glyphs[idx])
}

// Spark 把数值序列画成迷你柱状图（▁▂▃▄▅▆▇█，按最大值归一化）；
// 档位无 Spark 字形或序列为空/全零时返回空串。
func Spark(values []int64, is IconSet) string {
	levels := []rune(is.Spark)
	if len(levels) == 0 || len(values) == 0 {
		return ""
	}
	var max int64
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	if max == 0 {
		return ""
	}
	var b strings.Builder
	for _, v := range values {
		idx := int(float64(v) / float64(max) * float64(len(levels)-1))
		b.WriteRune(levels[idx])
	}
	return b.String()
}
