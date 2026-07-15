package segment

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"ccpill/internal/cache"
	"ccpill/internal/render"
	"ccpill/internal/sysinfo"
	"ccpill/internal/transcript"
)

func init() {
	Register(sessionSeg{})
	Register(speedSeg{})
	Register(compactSeg{})
	Register(styleSeg{})
	Register(clockSeg{})
	Register(cpumemSeg{})
	Register(worktreeSeg{})
	Register(mcpSeg{})
	Register(prSeg{})
	Register(apiSeg{})
}

// ---- session：会话时长（直接信任 stdin，零计算） ----

type sessionSeg struct{}

func (sessionSeg) ID() string { return "session" }

func (sessionSeg) Render(c *Context) *render.Pill {
	d := c.Status.Cost.TotalDurationMS
	if !d.Valid || d.Value <= 0 {
		return nil
	}
	return &render.Pill{
		Text:  c.Icons.Clock + " " + fmtDur(time.Duration(d.Value)*time.Millisecond),
		Color: c.Theme.Clock,
	}
}

// ---- speed：token 生成速度（当前 transcript 最近 5 分钟 output/间隔） ----

type speedSeg struct{}

func (speedSeg) ID() string { return "speed" }

func (speedSeg) Render(c *Context) *render.Pill {
	path := c.Status.TranscriptPath
	if path == "" {
		return nil
	}
	entries := transcript.ReadFile(path)
	cutoff := time.Now().Add(-5 * time.Minute)
	var out int64
	var first, last time.Time
	for _, e := range entries {
		if e.Timestamp.Before(cutoff) || e.IsSidechain {
			continue
		}
		if first.IsZero() {
			first = e.Timestamp
		}
		last = e.Timestamp
		out += e.Output
	}
	span := last.Sub(first).Seconds()
	if span <= 0 || out == 0 {
		return nil
	}
	return &render.Pill{
		Text:  fmt.Sprintf("%s%.0f/s", c.L("tok "), float64(out)/span),
		Color: c.Theme.Cost,
	}
}

// ---- compact：compaction 计数（精确协议标记扫描） ----

type compactSeg struct{}

func (compactSeg) ID() string { return "compact" }

func (compactSeg) Render(c *Context) *render.Pill {
	if c.Status.TranscriptPath == "" {
		return nil
	}
	n := transcript.CountCompactions(c.Status.TranscriptPath)
	if n == 0 {
		return nil
	}
	return &render.Pill{Text: fmt.Sprintf("%s×%d", c.L("compact "), n), Color: c.Theme.Extra}
}

// ---- style：输出风格 + Vim 模式 ----

type styleSeg struct{}

func (styleSeg) ID() string { return "style" }

func (styleSeg) Render(c *Context) *render.Pill {
	var parts []string
	if n := c.Status.OutputStyle.Name; n != "" && n != "default" {
		parts = append(parts, n)
	}
	if m := c.Status.Vim.Mode; m != nil && *m != "" {
		parts = append(parts, "vim:"+*m)
	}
	if len(parts) == 0 {
		return nil
	}
	return &render.Pill{Text: strings.Join(parts, " · "), Color: c.Theme.Extra}
}

// ---- clock：时钟 ----

type clockSeg struct{}

func (clockSeg) ID() string { return "clock" }

func (clockSeg) Render(c *Context) *render.Pill {
	return &render.Pill{Text: time.Now().Format("15:04"), Color: c.Theme.Clock}
}

// ---- cpumem：本机资源占用 ----

type cpumemSeg struct{}

func (cpumemSeg) ID() string { return "cpumem" }

func (cpumemSeg) Render(c *Context) *render.Pill {
	cpu, cpuOK := sysinfo.CPUPercent()
	mem, memOK := sysinfo.MemPercent()
	if !cpuOK && !memOK {
		return nil
	}
	var parts []string
	cpuLbl, memLbl := "CPU ", "MEM "
	if c.Cfg.Minimal {
		cpuLbl, memLbl = "C", "M"
	}
	if cpuOK {
		parts = append(parts, fmt.Sprintf("%s%.0f%%", cpuLbl, cpu))
	}
	if memOK {
		parts = append(parts, fmt.Sprintf("%s%.0f%%", memLbl, mem))
	}
	p := &render.Pill{Text: strings.Join(parts, " · "), Color: c.Theme.Clock}
	if (cpuOK && cpu >= 90) || (memOK && mem >= 90) {
		p.Level = render.Warn
	}
	return p
}

// ---- worktree：git worktree 标识（stdin 直读） ----

type worktreeSeg struct{}

func (worktreeSeg) ID() string { return "worktree" }

func (worktreeSeg) Render(c *Context) *render.Pill {
	if c.Status.Worktree.Name == "" {
		return nil
	}
	return &render.Pill{Text: c.L("wt:") + c.Status.Worktree.Name, Color: c.Theme.Extra}
}

// ---- mcp：已配置 MCP server 数（读 ~/.claude.json，5 分钟缓存） ----

type mcpSeg struct{}

func (mcpSeg) ID() string { return "mcp" }

func (mcpSeg) Render(c *Context) *render.Pill {
	var n int
	if !cache.Get("mcp-count", 5*time.Minute, &n) {
		n = countMCPServers()
		cache.Put("mcp-count", n)
	}
	if n <= 0 {
		return nil
	}
	return &render.Pill{Text: fmt.Sprintf("%s●%d", c.L("MCP "), n), Color: c.Theme.Git}
}

func countMCPServers() int {
	home, err := os.UserHomeDir()
	if err != nil {
		return 0
	}
	b, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err != nil {
		return 0
	}
	var doc struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if json.Unmarshal(b, &doc) != nil {
		return 0
	}
	return len(doc.MCPServers)
}

// ---- pr：当前分支关联 PR（gh CLI，5 分钟缓存；无 gh/无 PR 均隐藏） ----

type prSeg struct{}

func (prSeg) ID() string { return "pr" }

func (prSeg) Render(c *Context) *render.Pill {
	g := c.Git()
	if !g.IsRepo || g.Branch == "" || g.Branch == "(detached)" {
		return nil
	}
	key := "pr-" + sanitize(g.Branch)
	var num int
	if !cache.Get(key, 5*time.Minute, &num) {
		num = lookupPR(c)
		cache.Put(key, num)
	}
	if num <= 0 {
		return nil
	}
	return &render.Pill{Text: fmt.Sprintf("%s#%d", c.L("PR "), num), Color: c.Theme.Extra}
}

func lookupPR(c *Context) int {
	dir := c.Status.Workspace.CurrentDir
	if dir == "" {
		dir = c.Status.CWD
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "gh", "pr", "view", "--json", "number", "-q", ".number").Output()
	_ = dir // gh 自动用进程 cwd；statusline 由 Claude Code 在项目目录调用
	if err != nil {
		return -1 // 无 gh / 无 PR：缓存负结果避免反复探测
	}
	var n int
	_, _ = fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n)
	return n
}

// ---- api：Anthropic 服务健康（status.anthropic.com，5 分钟缓存含失败） ----

type apiSeg struct{}

func (apiSeg) ID() string { return "api" }

func (apiSeg) Render(c *Context) *render.Pill {
	var ind string
	if !cache.Get("api-status", 5*time.Minute, &ind) {
		ind = fetchAPIStatus()
		cache.Put("api-status", ind) // 失败也缓存，避免每次渲染都等超时
	}
	switch ind {
	case "none":
		return &render.Pill{Text: c.L("API ") + "●", Color: c.Theme.Cost}
	case "unknown", "":
		return nil
	default: // minor/major/critical
		return &render.Pill{Text: c.Icons.Warn + " API " + ind, Color: c.Theme.Warn, Level: render.Warn}
	}
}

func fetchAPIStatus() string {
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get("https://status.anthropic.com/api/v2/status.json")
	if err != nil {
		return "unknown"
	}
	defer resp.Body.Close()
	var doc struct {
		Status struct {
			Indicator string `json:"indicator"`
		} `json:"status"`
	}
	if json.NewDecoder(resp.Body).Decode(&doc) != nil {
		return "unknown"
	}
	return doc.Status.Indicator
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, s)
}
