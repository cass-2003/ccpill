// Package theme 定义 ccpill 的配色主题。
// 设计定案（design-drafts v2）：默认 Catppuccin Mocha 薄胶囊；
// Tokyo Night 为主题库首个成员（≈设计稿 A 的形态）。
package theme

import "fmt"

// RGB 是 truecolor 颜色。
type RGB struct{ R, G, B uint8 }

// Hex 返回 #rrggbb 形式（Web 配置页用）。
func (c RGB) Hex() string { return fmt.Sprintf("#%02x%02x%02x", c.R, c.G, c.B) }

// ParseHex 解析 "#rrggbb"（大小写均可）；非法输入返回 ok=false。
func ParseHex(s string) (RGB, bool) {
	if len(s) != 7 || s[0] != '#' {
		return RGB{}, false
	}
	var c RGB
	if _, err := fmt.Sscanf(s[1:], "%02x%02x%02x", &c.R, &c.G, &c.B); err != nil {
		return RGB{}, false
	}
	return c, true
}

// Theme 定义一套配色：胶囊统一底色 + 按语义类别的前景色 + 预警色。
type Theme struct {
	Name   string
	PillBG RGB // 胶囊统一底色
	Sep    RGB // 无胶囊模式下的分隔符颜色
	Muted  RGB // 次要文字
	Warn   RGB // 预警前景（预警胶囊反色时作底色）
	WarnFG RGB // 预警胶囊反色时的前景
	// 语义类别前景色
	Model, Context, Cost, Rate, Git, Dir, Extra, Clock RGB
}

var themes = map[string]Theme{
	"catppuccin-mocha": {
		Name:    "catppuccin-mocha",
		PillBG:  RGB{0x31, 0x32, 0x44}, // Surface0
		Sep:     RGB{0x45, 0x47, 0x5a}, // Surface1
		Muted:   RGB{0x6c, 0x70, 0x86}, // Overlay0
		Warn:    RGB{0xf3, 0x8b, 0xa8}, // Red
		WarnFG:  RGB{0x1e, 0x1e, 0x2e}, // Base
		Model:   RGB{0xcb, 0xa6, 0xf7}, // Mauve
		Context: RGB{0x89, 0xb4, 0xfa}, // Blue
		Cost:    RGB{0xa6, 0xe3, 0xa1}, // Green
		Rate:    RGB{0xfa, 0xb3, 0x87}, // Peach
		Git:     RGB{0x94, 0xe2, 0xd5}, // Teal
		Dir:     RGB{0x89, 0xb4, 0xfa}, // Blue
		Extra:   RGB{0xf5, 0xc2, 0xe7}, // Pink
		Clock:   RGB{0xba, 0xc2, 0xde}, // Subtext1
	},
	"tokyo-night": {
		Name:    "tokyo-night",
		PillBG:  RGB{0x24, 0x28, 0x3b},
		Sep:     RGB{0x3b, 0x42, 0x61},
		Muted:   RGB{0x56, 0x5f, 0x89},
		Warn:    RGB{0xf7, 0x76, 0x8e},
		WarnFG:  RGB{0x1a, 0x1b, 0x26},
		Model:   RGB{0xbb, 0x9a, 0xf7},
		Context: RGB{0x7a, 0xa2, 0xf7},
		Cost:    RGB{0x9e, 0xce, 0x6a},
		Rate:    RGB{0xe0, 0xaf, 0x68},
		Git:     RGB{0x7d, 0xcf, 0xff},
		Dir:     RGB{0x7a, 0xa2, 0xf7},
		Extra:   RGB{0xbb, 0x9a, 0xf7},
		Clock:   RGB{0xc0, 0xca, 0xf5},
	},
}

// Get 返回指定主题，未知名字回退默认主题。
func Get(name string) Theme {
	if t, ok := themes[name]; ok {
		return t
	}
	return themes["catppuccin-mocha"]
}

// Names 返回全部主题名（供配置校验与 Web 配置页枚举）。
func Names() []string {
	out := make([]string, 0, len(themes))
	for n := range themes {
		out = append(out, n)
	}
	return out
}
