// Package compose 把配置 + stdin 状态组装成待渲染的胶囊行。
// 主程序（ANSI 输出）与 Web 配置中心（HTML 预览）共用此层，保证所见即所得。
package compose

import (
	"ccpill/internal/config"
	"ccpill/internal/input"
	"ccpill/internal/render"
	"ccpill/internal/segment"
	"ccpill/internal/theme"
)

// Lines 按配置逐行组装胶囊；未知 segment ID 忽略（向前兼容）。
func Lines(cfg config.Config, status *input.Status) [][]render.Pill {
	ctx := &segment.Context{
		Status: status,
		Icons:  render.Icons(cfg.IconSet),
		Theme:  theme.Get(cfg.Theme),
	}
	var out [][]render.Pill
	for _, lineIDs := range cfg.Lines {
		var pills []render.Pill
		for _, id := range lineIDs {
			seg := segment.Get(id)
			if seg == nil {
				continue
			}
			if p := seg.Render(ctx); p != nil {
				pills = append(pills, *p)
			}
		}
		out = append(out, pills)
	}
	return out
}
