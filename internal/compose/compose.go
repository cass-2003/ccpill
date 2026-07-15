// Package compose 把配置 + stdin 状态组装成待渲染的胶囊行。
// 主程序（ANSI 输出）与 Web 配置中心（HTML 预览）共用此层，保证所见即所得。
package compose

import (
	"strings"

	"github.com/cass-2003/ccpill/internal/config"
	"github.com/cass-2003/ccpill/internal/input"
	"github.com/cass-2003/ccpill/internal/render"
	"github.com/cass-2003/ccpill/internal/segment"
	"github.com/cass-2003/ccpill/internal/theme"
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
			ctx.Current = id
			if seg := segment.Get(id); seg != nil {
				pill := seg.Render(ctx)
				ApplyOverride(cfg, id, pill)
				row = append(row, Item{ID: id, Pill: pill})
				continue
			}
			if name, ok := strings.CutPrefix(id, "slot:"); ok {
				if s := cfg.FindSlot(name); s != nil {
					pill := segment.RenderSlot(*s, ctx)
					ApplyOverride(cfg, id, pill)
					row = append(row, Item{ID: id, Pill: pill})
				}
				continue
			}
			// 未知 ID 忽略（向前兼容）
		}
		out = append(out, row)
	}
	return out
}

// ApplyOverride 应用用户的逐 segment 外观覆盖（前缀覆盖在 Context.L 里生效）。
// 预警反色不覆盖（红警可读性优先）；显式指定前景色视为要整颗统一色，多色片段让位。
func ApplyOverride(cfg config.Config, id string, pill *render.Pill) {
	if pill == nil || pill.Level == render.Warn {
		return
	}
	o, ok := cfg.Overrides[id]
	if !ok {
		return
	}
	if rgb, ok := theme.ParseHex(o.Color); ok {
		pill.Color = rgb
		pill.Spans = nil
	}
	if rgb, ok := theme.ParseHex(o.BG); ok {
		pill.BG = &rgb
	}
	if o.Bold {
		pill.Bold = true
	}
}
