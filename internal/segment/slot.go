// 自定义插槽渲染：布局 ID "slot:<name>" 由 compose 层解析到这里。
// 与 text/cmd segment 同一套内容语义，外加每槽独立的 RGB 前景色。
package segment

import (
	"strings"

	"ccpill/internal/config"
	"ccpill/internal/render"
	"ccpill/internal/theme"
)

// RenderSlot 渲染一个自定义插槽；无内容返回 nil。
func RenderSlot(s config.Slot, c *Context) *render.Pill {
	text := strings.TrimSpace(s.Text)
	if text == "" && strings.TrimSpace(s.Command) != "" {
		text = cachedCommand(s.Command)
	}
	if text == "" {
		return nil
	}
	color := c.Theme.Extra
	if rgb, ok := theme.ParseHex(s.Color); ok {
		color = rgb
	}
	return &render.Pill{Text: text, Color: color}
}
