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

// Item 是一次渲染中某个已启用 segment 的结果；Pill 为 nil 表示条件不满足而隐藏。
type Item struct {
	ID   string
	Pill *render.Pill
}

// Lines 按配置逐行组装胶囊；未知 segment ID 忽略（向前兼容）。
func Lines(cfg config.Config, status *input.Status) [][]render.Pill {
	var out [][]render.Pill
	for _, row := range Detail(cfg, status) {
		var pills []render.Pill
		for _, it := range row {
			if it.Pill != nil {
				pills = append(pills, *it.Pill)
			}
		}
		out = append(out, pills)
	}
	return out
}

// Detail 保留每个已启用 segment 的位置与渲染结果（含隐藏项），
// 供 Web 配置中心在预览里为隐藏项补示例胶囊。
func Detail(cfg config.Config, status *input.Status) [][]Item {
	ctx := &segment.Context{
		Status: status,
		Icons:  render.Icons(cfg.IconSet),
		Theme:  theme.Get(cfg.Theme),
		Cfg:    cfg,
	}
	var out [][]Item
	for _, lineIDs := range cfg.Lines {
		var row []Item
		for _, id := range lineIDs {
			seg := segment.Get(id)
			if seg == nil {
				continue
			}
			row = append(row, Item{ID: id, Pill: seg.Render(ctx)})
		}
		out = append(out, row)
	}
	return out
}
