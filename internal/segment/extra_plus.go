package segment

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/cass-2003/ccpill/internal/cache"
	"github.com/cass-2003/ccpill/internal/render"
	"github.com/cass-2003/ccpill/internal/transcript"
)

func init() {
	Register(tokensSeg{})
	Register(cachehitSeg{})
	Register(linesSeg{})
	Register(weeklySeg{})
	Register(versionSeg{})
	Register(gitshaSeg{})
	Register(sessionidSeg{})
	Register(emailSeg{})
	Register(textSeg{})
	Register(cmdSeg{})
}

// fmtTok 人性化 token 数：1.2M / 38k / 512。
func fmtTok(n float64) string {
	switch {
	case n >= 1e6:
		return fmt.Sprintf("%.1fM", n/1e6)
	case n >= 1e3:
		return fmt.Sprintf("%.0fk", n/1e3)
	default:
		return fmt.Sprintf("%.0f", n)
	}
}

// ---- tokens：会话输入/输出 token 总量（stdin 优先，transcript 兜底） ----

type tokensSeg struct{}

func (tokensSeg) ID() string { return "tokens" }

func (tokensSeg) Render(c *Context) *render.Pill {
	var in, out float64
	if cw := c.Status.ContextWindow; cw != nil && (cw.TotalInputTokens.Valid || cw.TotalOutputTokens.Valid) {
		in, out = cw.TotalInputTokens.Value, cw.TotalOutputTokens.Value
	} else if path := c.Status.TranscriptPath; path != "" {
		for _, e := range transcript.ReadFile(path) {
			if e.IsSidechain {
				continue
			}
			in += float64(e.Input + e.CacheCreate + e.CacheRead)
			out += float64(e.Output)
		}
	}
	if in == 0 && out == 0 {
		return nil
	}
	return &render.Pill{
		Text:  fmt.Sprintf("⇅ %s/%s", fmtTok(in), fmtTok(out)),
		Color: c.Theme.Context,
	}
}

// ---- cachehit：本会话 prompt cache 命中率（cacheRead 占非缓存写输入的比例） ----

type cachehitSeg struct{}

func (cachehitSeg) ID() string { return "cachehit" }

func (cachehitSeg) Render(c *Context) *render.Pill {
	path := c.Status.TranscriptPath
	if path == "" {
		return nil
	}
	var in, read int64
	for _, e := range transcript.ReadFile(path) {
		if e.IsSidechain {
			continue
		}
		in += e.Input
		read += e.CacheRead
	}
	denom := in + read
	if denom == 0 {
		return nil
	}
	return &render.Pill{
		Text:  fmt.Sprintf("%s%.0f%%", c.L("cache "), float64(read)/float64(denom)*100),
		Color: c.Theme.Context,
	}
}

// ---- lines：本会话代码行增删（stdin 官方统计） ----

type linesSeg struct{}

func (linesSeg) ID() string { return "lines" }

func (linesSeg) Render(c *Context) *render.Pill {
	a, r := c.Status.Cost.TotalLinesAdded, c.Status.Cost.TotalLinesRemoved
	if (!a.Valid && !r.Valid) || (a.Value == 0 && r.Value == 0) {
		return nil
	}
	return &render.Pill{
		Text:  fmt.Sprintf("+%.0f −%.0f", a.Value, r.Value),
		Color: c.Theme.Git,
	}
}

// ---- weekly：7 天限额窗口（stdin rate_limits.seven_day） ----

type weeklySeg struct{}

func (weeklySeg) ID() string { return "weekly" }

func (weeklySeg) Render(c *Context) *render.Pill {
	rl := c.Status.RateLimits
	if rl == nil || rl.SevenDay == nil || !rl.SevenDay.ResetsAt.Valid {
		return nil
	}
	remain := time.Until(time.Unix(int64(rl.SevenDay.ResetsAt.Value), 0))
	if remain <= 0 {
		return nil
	}
	p := &render.Pill{Text: c.L("7d ") + "⏳ " + fmtDurDays(remain), Color: c.Theme.Rate}
	if used := rl.SevenDay.UsedPercentage; used.Valid {
		p.Text = fmt.Sprintf("%s%.0f%% ⏳ %s", c.L("7d "), used.Value, fmtDurDays(remain))
		if used.Value >= 90 {
			p.Level = render.Warn
			p.Text = c.Icons.Warn + " " + p.Text
		}
	}
	return p
}

// fmtDurDays 长跨度时长：2d14h / 5h02m / 34m。
func fmtDurDays(d time.Duration) string {
	if d >= 48*time.Hour {
		days := int(d.Hours()) / 24
		return fmt.Sprintf("%dd%dh", days, int(d.Hours())%24)
	}
	return fmtDur(d)
}

// ---- version：Claude Code 版本号 ----

type versionSeg struct{}

func (versionSeg) ID() string { return "version" }

func (versionSeg) Render(c *Context) *render.Pill {
	v := c.Status.Version
	if v == "" {
		return nil
	}
	return &render.Pill{Text: "v" + strings.TrimPrefix(v, "v"), Color: c.Theme.Muted}
}

// ---- gitsha：HEAD 短 SHA（复用 gitinfo 单次采集，零额外子进程） ----

type gitshaSeg struct{}

func (gitshaSeg) ID() string { return "gitsha" }

func (gitshaSeg) Render(c *Context) *render.Pill {
	g := c.Git()
	if !g.IsRepo || len(g.SHA) < 7 {
		return nil
	}
	return &render.Pill{Text: g.SHA[:7], Color: c.Theme.Git}
}

// ---- sessionid：当前会话 ID 前 8 位（跨终端定位 transcript 用） ----

type sessionidSeg struct{}

func (sessionidSeg) ID() string { return "sessionid" }

func (sessionidSeg) Render(c *Context) *render.Pill {
	id := c.Status.SessionID
	if len(id) < 8 {
		return nil
	}
	return &render.Pill{Text: c.L("sid ") + id[:8], Color: c.Theme.Muted}
}

// ---- email：登录账号邮箱（~/.claude.json oauthAccount，5 分钟缓存） ----

type emailSeg struct{}

func (emailSeg) ID() string { return "email" }

func (emailSeg) Render(c *Context) *render.Pill {
	var mail string
	if !cache.Get("account-email", 5*time.Minute, &mail) {
		mail = readAccountEmail()
		cache.Put("account-email", mail)
	}
	if mail == "" {
		return nil
	}
	return &render.Pill{Text: mail, Color: c.Theme.Muted}
}

func readAccountEmail() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err != nil {
		return ""
	}
	var doc struct {
		OauthAccount struct {
			EmailAddress string `json:"emailAddress"`
		} `json:"oauthAccount"`
	}
	if json.Unmarshal(b, &doc) != nil {
		return ""
	}
	return doc.OauthAccount.EmailAddress
}

// ---- text：自定义静态文本（config custom_text） ----

type textSeg struct{}

func (textSeg) ID() string { return "text" }

func (textSeg) Render(c *Context) *render.Pill {
	t := strings.TrimSpace(c.Cfg.CustomText)
	if t == "" {
		return nil
	}
	return &render.Pill{Text: t, Color: c.Theme.Extra}
}

// ---- cmd：自定义命令输出首行（config custom_command，1s 超时 + 10s 缓存） ----

type cmdSeg struct{}

func (cmdSeg) ID() string { return "cmd" }

func (cmdSeg) Render(c *Context) *render.Pill {
	command := strings.TrimSpace(c.Cfg.CustomCommand)
	if command == "" {
		return nil
	}
	out := cachedCommand(command)
	if out == "" {
		return nil
	}
	return &render.Pill{Text: out, Color: c.Theme.Extra}
}

// cachedCommand 执行自定义命令并按命令内容做 10s 缓存（cmd segment 与插槽共用）。
func cachedCommand(command string) string {
	key := fmt.Sprintf("cmd-%x", sha1.Sum([]byte(command)))
	var out string
	if !cache.Get(key, 10*time.Second, &out) {
		out = runCustomCommand(command)
		cache.Put(key, out) // 失败也缓存空值，避免坏命令每次渲染都等满超时
	}
	return out
}

func runCustomCommand(command string) string {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	b, err := cmd.Output()
	if err != nil {
		return ""
	}
	line, _, _ := strings.Cut(strings.TrimSpace(string(b)), "\n")
	const maxLen = 60 // 防御异常长输出撑爆状态栏
	if len(line) > maxLen {
		line = line[:maxLen]
	}
	return strings.TrimSpace(line)
}
