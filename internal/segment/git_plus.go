// Git 全家桶：对齐 ccstatusline 28 个 git widget + claude-powerline 独有项
// （stash / tag / 进行中操作 / 距上次 commit 时长）的信息全集。
// 轻数据全部来自 porcelain v2 单次采集；重数据（diff 行数/tag/age/remote）各自惰性。
package segment

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/cass-2003/ccpill/internal/render"
)

func init() {
	Register(gitstatusSeg{})
	Register(gitstagedSeg{})
	Register(gitunstagedSeg{})
	Register(gituntrackedSeg{})
	Register(gitconflictsSeg{})
	Register(gitstashSeg{})
	Register(gitstateSeg{})
	Register(gitrepoSeg{})
	Register(gitdiffSeg{})
	Register(gitinsSeg{})
	Register(gitdelSeg{})
	Register(gittagSeg{})
	Register(gitageSeg{})
	Register(gitremoteSeg{})
}

// ---- gitstatus：工作区状态总览（S 暂存 / U 未暂存 / ? 未跟踪 / ✖ 冲突；干净时 ✓） ----

type gitstatusSeg struct{}

func (gitstatusSeg) ID() string { return "gitstatus" }

func (gitstatusSeg) Render(c *Context) *render.Pill {
	g := c.Git()
	if !g.IsRepo {
		return nil
	}
	if g.Dirty == 0 {
		return &render.Pill{Text: "✓", Color: c.Theme.Cost}
	}
	p := &render.Pill{Color: c.Theme.Git}
	if g.Staged > 0 {
		p.Spans = append(p.Spans, render.Span{Text: fmt.Sprintf("S%d", g.Staged), Color: c.Theme.Cost})
	}
	if g.Unstaged > 0 {
		p.Spans = append(p.Spans, render.Span{Text: spSep(p) + fmt.Sprintf("U%d", g.Unstaged), Color: c.Theme.Rate})
	}
	if g.Untracked > 0 {
		p.Spans = append(p.Spans, render.Span{Text: spSep(p) + fmt.Sprintf("?%d", g.Untracked), Color: c.Theme.Muted})
	}
	if g.Conflicts > 0 {
		p.Spans = append(p.Spans, render.Span{Text: spSep(p) + fmt.Sprintf("✖%d", g.Conflicts), Color: c.Theme.Warn})
	}
	for _, s := range p.Spans {
		p.Text += s.Text
	}
	return p
}

// spSep 在已有片段后补空格分隔。
func spSep(p *render.Pill) string {
	if len(p.Spans) > 0 {
		return " "
	}
	return ""
}

// ---- 单项计数拆分件 ----

type gitstagedSeg struct{}

func (gitstagedSeg) ID() string { return "gitstaged" }

func (gitstagedSeg) Render(c *Context) *render.Pill {
	g := c.Git()
	if !g.IsRepo || g.Staged == 0 {
		return nil
	}
	return &render.Pill{Text: fmt.Sprintf("S:%d", g.Staged), Color: c.Theme.Cost}
}

type gitunstagedSeg struct{}

func (gitunstagedSeg) ID() string { return "gitunstaged" }

func (gitunstagedSeg) Render(c *Context) *render.Pill {
	g := c.Git()
	if !g.IsRepo || g.Unstaged == 0 {
		return nil
	}
	return &render.Pill{Text: fmt.Sprintf("U:%d", g.Unstaged), Color: c.Theme.Rate}
}

type gituntrackedSeg struct{}

func (gituntrackedSeg) ID() string { return "gituntracked" }

func (gituntrackedSeg) Render(c *Context) *render.Pill {
	g := c.Git()
	if !g.IsRepo || g.Untracked == 0 {
		return nil
	}
	return &render.Pill{Text: fmt.Sprintf("?:%d", g.Untracked), Color: c.Theme.Muted}
}

type gitconflictsSeg struct{}

func (gitconflictsSeg) ID() string { return "gitconflicts" }

func (gitconflictsSeg) Render(c *Context) *render.Pill {
	g := c.Git()
	if !g.IsRepo || g.Conflicts == 0 {
		return nil
	}
	return &render.Pill{Text: fmt.Sprintf("✖%d", g.Conflicts), Color: c.Theme.Warn, Level: render.Warn}
}

type gitstashSeg struct{}

func (gitstashSeg) ID() string { return "gitstash" }

func (gitstashSeg) Render(c *Context) *render.Pill {
	g := c.Git()
	if !g.IsRepo || g.Stash == 0 {
		return nil
	}
	return &render.Pill{Text: fmt.Sprintf("⚑%d", g.Stash), Color: c.Theme.Extra}
}

// ---- gitstate：进行中的多步操作（REBASE/MERGE/…，红警提示别忘了收尾） ----

type gitstateSeg struct{}

func (gitstateSeg) ID() string { return "gitstate" }

func (gitstateSeg) Render(c *Context) *render.Pill {
	s := c.GitState()
	if s == "" {
		return nil
	}
	return &render.Pill{Text: s, Color: c.Theme.Warn, Level: render.Warn}
}

// ---- gitrepo：仓库根目录名 ----

type gitrepoSeg struct{}

func (gitrepoSeg) ID() string { return "gitrepo" }

func (gitrepoSeg) Render(c *Context) *render.Pill {
	root := c.GitRepoRoot()
	if root == "" {
		return nil
	}
	return &render.Pill{Text: filepath.Base(root), Color: c.Theme.Dir}
}

// ---- gitdiff / gitins / gitdel：未提交增删行数（+绿 / −红，同 gitab 风格） ----

type gitdiffSeg struct{}

func (gitdiffSeg) ID() string { return "gitdiff" }

func (gitdiffSeg) Render(c *Context) *render.Pill {
	if !c.Git().IsRepo {
		return nil
	}
	d := c.GitDiff()
	if !d.OK || (d.Ins == 0 && d.Del == 0) {
		return nil
	}
	p := &render.Pill{Color: c.Theme.Git}
	if d.Ins > 0 {
		p.Spans = append(p.Spans, render.Span{Text: fmt.Sprintf("+%d", d.Ins), Color: c.Theme.Cost})
	}
	if d.Del > 0 {
		p.Spans = append(p.Spans, render.Span{Text: spSep(p) + fmt.Sprintf("−%d", d.Del), Color: c.Theme.Warn})
	}
	for _, s := range p.Spans {
		p.Text += s.Text
	}
	return p
}

type gitinsSeg struct{}

func (gitinsSeg) ID() string { return "gitins" }

func (gitinsSeg) Render(c *Context) *render.Pill {
	if !c.Git().IsRepo {
		return nil
	}
	d := c.GitDiff()
	if !d.OK || d.Ins == 0 {
		return nil
	}
	return &render.Pill{Text: fmt.Sprintf("+%d", d.Ins), Color: c.Theme.Cost}
}

type gitdelSeg struct{}

func (gitdelSeg) ID() string { return "gitdel" }

func (gitdelSeg) Render(c *Context) *render.Pill {
	if !c.Git().IsRepo {
		return nil
	}
	d := c.GitDiff()
	if !d.OK || d.Del == 0 {
		return nil
	}
	return &render.Pill{Text: fmt.Sprintf("−%d", d.Del), Color: c.Theme.Warn}
}

// ---- gittag：最近 tag ----

type gittagSeg struct{}

func (gittagSeg) ID() string { return "gittag" }

func (gittagSeg) Render(c *Context) *render.Pill {
	if !c.Git().IsRepo {
		return nil
	}
	t := c.GitTag()
	if t == "" {
		return nil
	}
	return &render.Pill{Text: t, Color: c.Theme.Extra}
}

// ---- gitage：距上次 commit 时长 ----

type gitageSeg struct{}

func (gitageSeg) ID() string { return "gitage" }

func (gitageSeg) Render(c *Context) *render.Pill {
	if !c.Git().IsRepo {
		return nil
	}
	age, ok := c.GitAge()
	if !ok {
		return nil
	}
	return &render.Pill{Text: c.L("commit ") + fmtAge(age), Color: c.Theme.Clock}
}

// fmtAge 粗粒度时长：刚刚 / 5m / 3h / 2d。
func fmtAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "刚刚"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours())/24)
	}
}

// ---- gitremote：origin 的 owner/repo ----

type gitremoteSeg struct{}

func (gitremoteSeg) ID() string { return "gitremote" }

func (gitremoteSeg) Render(c *Context) *render.Pill {
	if !c.Git().IsRepo {
		return nil
	}
	r := c.GitRemote()
	if r == "" {
		return nil
	}
	return &render.Pill{Text: r, Color: c.Theme.Muted}
}
