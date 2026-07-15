// 非 Git 品类的友商全量对齐（ccstatusline / claude-powerline / CCometixLine）：
// 会话元信息、token/速度拆分、上下文窗口、系统信息、cache TTL 倒计时、
// OAuth 用量接口（分模型周限额 + 超额）。
package segment

import (
	"fmt"
	"time"

	"github.com/cass-2003/ccpill/internal/render"
	"github.com/cass-2003/ccpill/internal/sysinfo"
	"github.com/cass-2003/ccpill/internal/transcript"
)

func init() {
	Register(sessionnameSeg{})
	Register(msgcountSeg{})
	Register(resptimeSeg{})
	Register(tokwriteSeg{})
	Register(speedinSeg{})
	Register(speedtotalSeg{})
	Register(ctxwinSeg{})
	Register(ctxusableSeg{})
	Register(memfreeSeg{})
	Register(termwidthSeg{})
	Register(cachetimerSeg{})
	Register(weeklysonnetSeg{})
	Register(weeklyopusSeg{})
	Register(overageSeg{})
}

// ---- sessionname：会话名（/rename 写入 transcript 的 custom-title） ----

type sessionnameSeg struct{}

func (sessionnameSeg) ID() string { return "sessionname" }

func (sessionnameSeg) Render(c *Context) *render.Pill {
	t := c.Meta().Title
	if t == "" {
		return nil
	}
	return &render.Pill{Text: t, Color: c.Theme.Extra}
}

// ---- msgcount：本会话真实用户消息数（不含 tool_result 回写） ----

type msgcountSeg struct{}

func (msgcountSeg) ID() string { return "msgcount" }

func (msgcountSeg) Render(c *Context) *render.Pill {
	n := c.Meta().UserMsgs
	if n == 0 {
		return nil
	}
	return &render.Pill{Text: fmt.Sprintf("%s%d", c.L("msg "), n), Color: c.Theme.Muted}
}

// ---- resptime：用户提问到首个响应的平均耗时 ----

type resptimeSeg struct{}

func (resptimeSeg) ID() string { return "resptime" }

func (resptimeSeg) Render(c *Context) *render.Pill {
	m := c.Meta()
	if m.RespCount == 0 {
		return nil
	}
	return &render.Pill{Text: c.L("resp ") + fmtSec(m.RespAvg), Color: c.Theme.Clock}
}

// fmtSec 秒级时长：34s / 2m10s。
func fmtSec(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
}

// ---- tokwrite：会话缓存写 token（tokcache 的姊妹件，对齐 cache-write） ----

type tokwriteSeg struct{}

func (tokwriteSeg) ID() string { return "tokwrite" }

func (tokwriteSeg) Render(c *Context) *render.Pill {
	_, _, _, cacheCreate, ok := sessionTokens(c)
	if !ok || cacheCreate == 0 {
		return nil
	}
	return &render.Pill{Text: c.L("cachew ") + fmtTok(float64(cacheCreate)), Color: c.Theme.Context}
}

// ---- speedin / speedtotal：最近 5 分钟输入/总 token 速度（speed 的姊妹件） ----

// recentSpeed 统计最近 5 分钟的 token 速率；pick 决定统计哪个分量。
func recentSpeed(c *Context, pick func(transcript.Entry) int64) (float64, bool) {
	path := c.Status.TranscriptPath
	if path == "" {
		return 0, false
	}
	cutoff := time.Now().Add(-5 * time.Minute)
	var sum int64
	var first, last time.Time
	for _, e := range transcript.ReadFile(path) {
		if e.Timestamp.Before(cutoff) || e.IsSidechain {
			continue
		}
		if first.IsZero() {
			first = e.Timestamp
		}
		last = e.Timestamp
		sum += pick(e)
	}
	span := last.Sub(first).Seconds()
	if span <= 0 || sum == 0 {
		return 0, false
	}
	return float64(sum) / span, true
}

type speedinSeg struct{}

func (speedinSeg) ID() string { return "speedin" }

func (speedinSeg) Render(c *Context) *render.Pill {
	v, ok := recentSpeed(c, func(e transcript.Entry) int64 { return e.Input })
	if !ok {
		return nil
	}
	// 非缓存输入速率常 <1 tok/s，小值用一位小数避免显示成 0/s
	f := "%s%.0f/s"
	if v < 10 {
		f = "%s%.1f/s"
	}
	return &render.Pill{Text: fmt.Sprintf(f, c.L("in "), v), Color: c.Theme.Cost}
}

type speedtotalSeg struct{}

func (speedtotalSeg) ID() string { return "speedtotal" }

func (speedtotalSeg) Render(c *Context) *render.Pill {
	v, ok := recentSpeed(c, transcript.Entry.TotalTokens)
	if !ok {
		return nil
	}
	return &render.Pill{Text: fmt.Sprintf("%s%s/s", c.L("tok∑ "), fmtTok(v)), Color: c.Theme.Cost}
}

// ---- ctxwin / ctxusable：上下文窗口大小 / 可用区（自动压缩前 80%）占用 ----

type ctxwinSeg struct{}

func (ctxwinSeg) ID() string { return "ctxwin" }

func (ctxwinSeg) Render(c *Context) *render.Pill {
	cw := c.Status.ContextWindow
	if cw == nil || !cw.Size.Valid || cw.Size.Value <= 0 {
		return nil
	}
	return &render.Pill{Text: c.L("win ") + fmtTok(cw.Size.Value), Color: c.Theme.Context}
}

type ctxusableSeg struct{}

func (ctxusableSeg) ID() string { return "ctxusable" }

// usableRatio 对齐 ccstatusline USABLE_CONTEXT_RATIO：自动压缩阈值前的可用区。
const usableRatio = 0.8

func (ctxusableSeg) Render(c *Context) *render.Pill {
	cw := c.Status.ContextWindow
	if cw == nil || !cw.UsedPercentage.Valid {
		return nil
	}
	pct := cw.UsedPercentage.Value / usableRatio
	if pct > 100 {
		pct = 100
	}
	p := &render.Pill{Text: fmt.Sprintf("%s%.0f%%", c.L("ctx可用 "), pct), Color: c.Theme.Context}
	if pct >= 90 {
		p.Level = render.Warn
		p.Text = c.Icons.Warn + " " + p.Text
	}
	return p
}

// ---- memfree：空闲/总物理内存 ----

type memfreeSeg struct{}

func (memfreeSeg) ID() string { return "memfree" }

func (memfreeSeg) Render(c *Context) *render.Pill {
	avail, total, ok := sysinfo.MemBytes()
	if !ok {
		return nil
	}
	const g = 1 << 30
	return &render.Pill{
		Text:  fmt.Sprintf("%s%.1fG/%.0fG", c.L("free "), float64(avail)/g, float64(total)/g),
		Color: c.Theme.Clock,
	}
}

// ---- termwidth：终端列宽 ----

type termwidthSeg struct{}

func (termwidthSeg) ID() string { return "termwidth" }

func (termwidthSeg) Render(c *Context) *render.Pill {
	w, ok := sysinfo.TermWidth()
	if !ok {
		return nil
	}
	return &render.Pill{Text: fmt.Sprintf("%s%d", c.L("term "), w), Color: c.Theme.Muted}
}

// ---- cachetimer：prompt cache TTL 倒计时（对齐 claude-powerline cacheTimer） ----

type cachetimerSeg struct{}

func (cachetimerSeg) ID() string { return "cachetimer" }

func (cachetimerSeg) Render(c *Context) *render.Pill {
	path := c.Status.TranscriptPath
	if path == "" {
		return nil
	}
	// 最后一条带缓存写的主线条目决定 TTL 起点与档位（1h 明细 > 5m 默认）
	var ts time.Time
	ttl := 5 * time.Minute
	for _, e := range transcript.ReadFile(path) {
		if e.IsSidechain || e.CacheCreate == 0 {
			continue
		}
		if e.Timestamp.After(ts) {
			ts = e.Timestamp
			if e.Cache1h > 0 {
				ttl = time.Hour
			} else {
				ttl = 5 * time.Minute
			}
		}
	}
	if ts.IsZero() {
		return nil
	}
	remain := ttl - time.Since(ts)
	if remain <= 0 {
		return nil // 缓存已冷，隐藏
	}
	return &render.Pill{Text: c.L("cache ") + "⏳ " + fmtSec(remain), Color: c.Theme.Context}
}

// ---- weeklysonnet / weeklyopus / overage：OAuth 用量接口独有数据 ----

type weeklysonnetSeg struct{}

func (weeklysonnetSeg) ID() string { return "weeklysonnet" }

func (weeklysonnetSeg) Render(c *Context) *render.Pill {
	d := c.API()
	if !d.OK || d.SevenDaySonnet < 0 {
		return nil
	}
	return usagePill(c, "7d Sonnet ", d.SevenDaySonnet)
}

type weeklyopusSeg struct{}

func (weeklyopusSeg) ID() string { return "weeklyopus" }

func (weeklyopusSeg) Render(c *Context) *render.Pill {
	d := c.API()
	if !d.OK || d.SevenDayOpus < 0 {
		return nil
	}
	return usagePill(c, "7d Opus ", d.SevenDayOpus)
}

func usagePill(c *Context, label string, pct float64) *render.Pill {
	p := &render.Pill{Text: fmt.Sprintf("%s%.0f%%", c.L(label), pct), Color: c.Theme.Rate}
	if pct >= 90 {
		p.Level = render.Warn
		p.Text = c.Icons.Warn + " " + p.Text
	}
	return p
}

type overageSeg struct{}

func (overageSeg) ID() string { return "overage" }

func (overageSeg) Render(c *Context) *render.Pill {
	d := c.API()
	if !d.OK || !d.ExtraEnabled {
		return nil
	}
	p := &render.Pill{
		Text:  fmt.Sprintf("%s%s%.0f/%s%.0f", c.L("超额 "), c.Icons.Cost, d.ExtraUsed, c.Icons.Cost, d.ExtraLimit),
		Color: c.Theme.Cost,
	}
	if d.ExtraUtil >= 90 {
		p.Level = render.Warn
	}
	return p
}
