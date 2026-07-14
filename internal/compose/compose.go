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
	lines, _ := LinesDetail(cfg, status)
	return lines
}

// LinesDetail 额外返回「已启用但本次渲染为空」的 segment ID（条件显示类），
// 供 Web 配置中心向用户解释"选了为什么没出现"。
func LinesDetail(cfg config.Config, status *input.Status) ([][]render.Pill, []string) {
	ctx := &segment.Context{
		Status: status,
		Icons:  render.Icons(cfg.IconSet),
		Theme:  theme.Get(cfg.Theme),
		Cfg:    cfg,
	}
	var out [][]render.Pill
	var hidden []string
	for _, lineIDs := range cfg.Lines {
		var pills []render.Pill
		for _, id := range lineIDs {
			seg := segment.Get(id)
			if seg == nil {
				continue
			}
			if p := seg.Render(ctx); p != nil {
				pills = append(pills, *p)
			} else {
				hidden = append(hidden, id)
			}
		}
		out = append(out, pills)
	}
	return out, hidden
}
