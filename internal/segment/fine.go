// 细粒度 segment：把合并型胶囊拆成可自由组合的小件（对齐 ccstatusline 的
// widget 粒度，拆解 01 §3.5）。合并型（model/context/git/tokens…）依旧保留。
package segment

import (
	"fmt"
	"strings"
	"time"

	"ccpill/internal/render"
	"ccpill/internal/sysinfo"
	"ccpill/internal/transcript"
)

func init() {
	Register(modelnameSeg{})
	Register(thinkSeg{})
	Register(ctxbarSeg{})
	Register(ctxpctSeg{})
	Register(ctxlenSeg{})
	Register(tokinSeg{})
	Register(tokoutSeg{})
	Register(tokcacheSeg{})
	Register(toktotalSeg{})
	Register(gitbranchSeg{})
	Register(gitchangesSeg{})
	Register(gitabSeg{})
	Register(blockpctSeg{})
	Register(blocktimeSeg{})
	Register(cpuSeg{})
	Register(memSeg{})
	Register(outstyleSeg{})
	Register(vimSeg{})
}

// sessionTokens 汇总本会话 transcript 的 token 分量（主线，不含 sidechain）。
func sessionTokens(c *Context) (in, out, cacheRead, cacheCreate int64, ok bool) {
	path := c.Status.TranscriptPath
	if path == "" {
		return 0, 0, 0, 0, false
	}
	for _, e := range transcript.ReadFile(path) {
		if e.IsSidechain {
			continue
		}
		in += e.Input
		out += e.Output
		cacheRead += e.CacheRead
		cacheCreate += e.CacheCreate
	}
	return in, out, cacheRead, cacheCreate, in+out+cacheRead+cacheCreate > 0
}

// ---- model 拆分 ----

type modelnameSeg struct{}

func (modelnameSeg) ID() string { return "modelname" }

func (modelnameSeg) Render(c *Context) *render.Pill {
	name := c.Status.ModelName()
	if name == "" {
		return nil
	}
	return &render.Pill{Text: c.Icons.Model + " " + name, Color: c.Theme.Model}
}

type thinkSeg struct{}

func (thinkSeg) ID() string { return "think" }

func (thinkSeg) Render(c *Context) *render.Pill {
	lv := c.Status.EffortLevel()
	if lv == "" {
		return nil
	}
	return &render.Pill{Text: "think:" + shortEffort(lv), Color: c.Theme.Model}
}

// ---- context 拆分 ----

func ctxWarnColor(c *Context, pct float64, p *render.Pill) *render.Pill {
	if pct >= 90 {
		p.Level = render.Warn
	} else if pct >= 80 {
		p.Color = c.Theme.Warn
	}
	return p
}

type ctxbarSeg struct{}

func (ctxbarSeg) ID() string { return "ctxbar" }

func (ctxbarSeg) Render(c *Context) *render.Pill {
	pct, ok := c.Status.ContextPercent()
	if !ok {
		return nil
	}
	return ctxWarnColor(c, pct, &render.Pill{Text: render.Bar(pct, 10, c.Icons), Color: c.Theme.Context})
}

type ctxpctSeg struct{}

func (ctxpctSeg) ID() string { return "ctxpct" }

func (ctxpctSeg) Render(c *Context) *render.Pill {
	pct, ok := c.Status.ContextPercent()
	if !ok {
		return nil
	}
	return ctxWarnColor(c, pct, &render.Pill{Text: fmt.Sprintf("ctx %.0f%%", pct), Color: c.Theme.Context})
}

type ctxlenSeg struct{}

func (ctxlenSeg) ID() string { return "ctxlen" }

func (ctxlenSeg) Render(c *Context) *render.Pill {
	used, ok := c.Status.ContextUsedTokens()
	if !ok || used <= 0 {
		return nil
	}
	return &render.Pill{Text: "ctx " + fmtTok(used), Color: c.Theme.Context}
}

// ---- tokens 拆分（transcript 口径） ----

type tokinSeg struct{}

func (tokinSeg) ID() string { return "tokin" }

func (tokinSeg) Render(c *Context) *render.Pill {
	in, _, _, _, ok := sessionTokens(c)
	if !ok {
		return nil
	}
	return &render.Pill{Text: "in " + fmtTok(float64(in)), Color: c.Theme.Context}
}

type tokoutSeg struct{}

func (tokoutSeg) ID() string { return "tokout" }

func (tokoutSeg) Render(c *Context) *render.Pill {
	_, out, _, _, ok := sessionTokens(c)
	if !ok {
		return nil
	}
	return &render.Pill{Text: "out " + fmtTok(float64(out)), Color: c.Theme.Context}
}

type tokcacheSeg struct{}

func (tokcacheSeg) ID() string { return "tokcache" }

func (tokcacheSeg) Render(c *Context) *render.Pill {
	_, _, cread, _, ok := sessionTokens(c)
	if !ok {
		return nil
	}
	return &render.Pill{Text: "cached " + fmtTok(float64(cread)), Color: c.Theme.Context}
}

type toktotalSeg struct{}

func (toktotalSeg) ID() string { return "toktotal" }

func (toktotalSeg) Render(c *Context) *render.Pill {
	in, out, cread, ccreate, ok := sessionTokens(c)
	if !ok {
		return nil
	}
	return &render.Pill{Text: "tok " + fmtTok(float64(in+out+cread+ccreate)), Color: c.Theme.Context}
}

// ---- git 拆分 ----

type gitbranchSeg struct{}

func (gitbranchSeg) ID() string { return "gitbranch" }

func (gitbranchSeg) Render(c *Context) *render.Pill {
	g := c.Git()
	if !g.IsRepo || g.Branch == "" {
		return nil
	}
	return &render.Pill{Text: c.Icons.Branch + " " + g.Branch, Color: c.Theme.Git}
}

type gitchangesSeg struct{}

func (gitchangesSeg) ID() string { return "gitchanges" }

func (gitchangesSeg) Render(c *Context) *render.Pill {
	g := c.Git()
	if !g.IsRepo || g.Dirty == 0 {
		return nil
	}
	p := &render.Pill{Text: fmt.Sprintf("%s%d", c.Icons.Dirty, g.Dirty), Color: c.Theme.Git}
	if c.Cfg.GitDirtyWarn > 0 && g.Dirty >= c.Cfg.GitDirtyWarn {
		p.Level = render.Warn
	}
	return p
}

type gitabSeg struct{}

func (gitabSeg) ID() string { return "gitab" }

func (gitabSeg) Render(c *Context) *render.Pill {
	g := c.Git()
	if !g.IsRepo || (g.Ahead == 0 && g.Behind == 0) {
		return nil
	}
	var parts []string
	if g.Ahead > 0 {
		parts = append(parts, fmt.Sprintf("%s%d", c.Icons.Ahead, g.Ahead))
	}
	if g.Behind > 0 {
		parts = append(parts, fmt.Sprintf("%s%d", c.Icons.Behind, g.Behind))
	}
	return &render.Pill{Text: strings.Join(parts, " "), Color: c.Theme.Git}
}

// ---- block 拆分（仅 stdin 官方数据） ----

type blockpctSeg struct{}

func (blockpctSeg) ID() string { return "blockpct" }

func (blockpctSeg) Render(c *Context) *render.Pill {
	rl := c.Status.RateLimits
	if rl == nil || rl.FiveHour == nil || !rl.FiveHour.UsedPercentage.Valid {
		return nil
	}
	used := rl.FiveHour.UsedPercentage.Value
	p := &render.Pill{Text: fmt.Sprintf("5h %.0f%%", used), Color: c.Theme.Rate}
	if used >= 90 {
		p.Level = render.Warn
	}
	return p
}

type blocktimeSeg struct{}

func (blocktimeSeg) ID() string { return "blocktime" }

func (blocktimeSeg) Render(c *Context) *render.Pill {
	rl := c.Status.RateLimits
	if rl == nil || rl.FiveHour == nil || !rl.FiveHour.ResetsAt.Valid {
		return nil
	}
	remain := time.Until(time.Unix(int64(rl.FiveHour.ResetsAt.Value), 0))
	if remain <= 0 {
		return nil
	}
	return &render.Pill{Text: "⏳ " + fmtDur(remain), Color: c.Theme.Rate}
}

// ---- cpumem 拆分 ----

type cpuSeg struct{}

func (cpuSeg) ID() string { return "cpu" }

func (cpuSeg) Render(c *Context) *render.Pill {
	v, ok := sysinfo.CPUPercent()
	if !ok {
		return nil
	}
	p := &render.Pill{Text: fmt.Sprintf("CPU %.0f%%", v), Color: c.Theme.Clock}
	if v >= 90 {
		p.Level = render.Warn
	}
	return p
}

type memSeg struct{}

func (memSeg) ID() string { return "mem" }

func (memSeg) Render(c *Context) *render.Pill {
	v, ok := sysinfo.MemPercent()
	if !ok {
		return nil
	}
	p := &render.Pill{Text: fmt.Sprintf("MEM %.0f%%", v), Color: c.Theme.Clock}
	if v >= 90 {
		p.Level = render.Warn
	}
	return p
}

// ---- style 拆分 ----

type outstyleSeg struct{}

func (outstyleSeg) ID() string { return "outstyle" }

func (outstyleSeg) Render(c *Context) *render.Pill {
	n := c.Status.OutputStyle.Name
	if n == "" || n == "default" {
		return nil
	}
	return &render.Pill{Text: n, Color: c.Theme.Extra}
}

type vimSeg struct{}

func (vimSeg) ID() string { return "vim" }

func (vimSeg) Render(c *Context) *render.Pill {
	m := c.Status.Vim.Mode
	if m == nil || *m == "" {
		return nil
	}
	return &render.Pill{Text: "vim:" + *m, Color: c.Theme.Extra}
}
