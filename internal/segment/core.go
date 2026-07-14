package segment

import (
	"fmt"
	"path/filepath"
	"strings"

	"ccpill/internal/render"
)

func init() {
	Register(modelSeg{})
	Register(contextSeg{})
	Register(costSeg{})
	Register(gitSeg{})
	Register(dirSeg{})
}

// ---- model：模型名 + 思考等级 ----

type modelSeg struct{}

func (modelSeg) ID() string { return "model" }

func (modelSeg) Render(c *Context) *render.Pill {
	name := c.Status.ModelName()
	if name == "" {
		return nil
	}
	text := c.Icons.Model + " " + name
	if lv := c.Status.EffortLevel(); lv != "" {
		text += " · think:" + shortEffort(lv)
	}
	return &render.Pill{Text: text, Color: c.Theme.Model}
}

func shortEffort(lv string) string {
	switch lv {
	case "low":
		return "lo"
	case "medium":
		return "mid"
	case "high":
		return "hi"
	default:
		return lv // xhigh/max/未知值原样透传，不猜测
	}
}

// ---- context：上下文占用（块状条 + 百分比 + 阈值预警） ----

type contextSeg struct{}

func (contextSeg) ID() string { return "context" }

func (contextSeg) Render(c *Context) *render.Pill {
	pct, ok := c.Status.ContextPercent()
	if !ok {
		return nil
	}
	text := fmt.Sprintf("ctx %s %.0f%%", render.Bar(pct, 10, c.Icons), pct)
	p := &render.Pill{Text: text, Color: c.Theme.Context}
	if pct >= 90 {
		p.Level = render.Warn
		p.Text = c.Icons.Warn + " " + text + " 即将压缩"
	} else if pct >= 80 {
		p.Color = c.Theme.Warn
	}
	return p
}

// ---- cost：会话成本（直接信任 stdin 官方值，不自算） ----

type costSeg struct{}

func (costSeg) ID() string { return "cost" }

func (costSeg) Render(c *Context) *render.Pill {
	cost := c.Status.Cost.TotalCostUSD
	if !cost.Valid {
		return nil
	}
	return &render.Pill{
		Text:  fmt.Sprintf("%s%.2f", c.Icons.Cost, cost.Value),
		Color: c.Theme.Cost,
	}
}

// ---- git：分支 + 脏文件 + ahead/behind ----

type gitSeg struct{}

func (gitSeg) ID() string { return "git" }

func (gitSeg) Render(c *Context) *render.Pill {
	g := c.Git()
	if !g.IsRepo || g.Branch == "" {
		return nil
	}
	var b strings.Builder
	b.WriteString(c.Icons.Branch + " " + g.Branch)
	if g.Dirty > 0 {
		fmt.Fprintf(&b, " %s%d", c.Icons.Dirty, g.Dirty)
	}
	if g.Ahead > 0 {
		fmt.Fprintf(&b, " %s%d", c.Icons.Ahead, g.Ahead)
	}
	if g.Behind > 0 {
		fmt.Fprintf(&b, " %s%d", c.Icons.Behind, g.Behind)
	}
	p := &render.Pill{Text: b.String(), Color: c.Theme.Git}
	if c.Cfg.GitDirtyWarn > 0 && g.Dirty >= c.Cfg.GitDirtyWarn {
		p.Level = render.Warn
		p.Text += " 未提交堆积"
	}
	return p
}

// ---- dir：当前目录名 ----

type dirSeg struct{}

func (dirSeg) ID() string { return "dir" }

func (dirSeg) Render(c *Context) *render.Pill {
	dir := c.Status.Workspace.CurrentDir
	if dir == "" {
		dir = c.Status.CWD
	}
	if dir == "" {
		return nil
	}
	text := filepath.Base(dir)
	if c.Icons.Dir != "" {
		text = c.Icons.Dir + " " + text
	}
	return &render.Pill{Text: text, Color: c.Theme.Dir}
}
