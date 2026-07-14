package segment

import (
	"fmt"
	"time"

	"ccpill/internal/render"
)

func init() {
	Register(todaySeg{})
	Register(burnSeg{})
	Register(blockSeg{})
}

// ---- today：今日总花费（跨会话聚合）+ 日预算线预警 ----

type todaySeg struct{}

func (todaySeg) ID() string { return "today" }

func (todaySeg) Render(c *Context) *render.Pill {
	u := c.Usage()
	if u.TodayCost <= 0 {
		return nil
	}
	p := &render.Pill{
		Text:  fmt.Sprintf("今日 %s%.2f", c.Icons.Cost, u.TodayCost),
		Color: c.Theme.Cost,
	}
	if b := c.Cfg.DailyBudget; b > 0 && u.TodayCost >= b {
		p.Level = render.Warn
		p.Text += " 超预算"
	}
	return p
}

// ---- burn：烧钱速率（活跃块 $/h） ----

type burnSeg struct{}

func (burnSeg) ID() string { return "burn" }

func (burnSeg) Render(c *Context) *render.Pill {
	u := c.Usage()
	if u.CostPerHour <= 0 {
		return nil
	}
	return &render.Pill{
		Text:  fmt.Sprintf("%s %s%.1f/h", c.Icons.Flame, c.Icons.Cost, u.CostPerHour),
		Color: c.Theme.Rate,
	}
}

// ---- block：5h 计费窗口剩余。优先 stdin rate_limits（官方数据），
// 缺失时退化到 transcript 推断的活跃块（ccusage 算法）。----

type blockSeg struct{}

func (blockSeg) ID() string { return "block" }

func (blockSeg) Render(c *Context) *render.Pill {
	// 一级：stdin 官方 rate_limits.five_hour
	if rl := c.Status.RateLimits; rl != nil && rl.FiveHour != nil && rl.FiveHour.ResetsAt.Valid {
		remain := time.Until(time.Unix(int64(rl.FiveHour.ResetsAt.Value), 0))
		if remain > 0 {
			p := &render.Pill{
				Text:  "5h ⏳ " + fmtDur(remain),
				Color: c.Theme.Rate,
			}
			if used := rl.FiveHour.UsedPercentage; used.Valid {
				p.Text = fmt.Sprintf("5h %.0f%% ⏳ %s", used.Value, fmtDur(remain))
				if used.Value >= 90 {
					p.Level = render.Warn
					p.Text = c.Icons.Warn + " " + p.Text
				}
			}
			return p
		}
	}
	// 二级：transcript 推断的活跃块
	u := c.Usage()
	if u.BlockRemainMin < 0 {
		return nil
	}
	return &render.Pill{
		Text:  "5h ⏳ " + fmtDur(time.Duration(u.BlockRemainMin)*time.Minute),
		Color: c.Theme.Rate,
	}
}

func fmtDur(d time.Duration) string {
	d = d.Round(time.Minute)
	h, m := int(d.Hours()), int(d.Minutes())%60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
