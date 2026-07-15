// 内嵌 Claude 模型定价表（calculate 模式）。
// 新版 Claude Code 的 transcript 不再写 costUSD 字段（2026-07 实测 6083 条 0 命中），
// display 模式恒为 0——必须按 token 用量 × 定价重算（ccusage calculate 模式，拆解 03 §2）。
//
// 定价快照 2026-07（$/1M tokens）；cache 写价：5m=1.25× input、1h=2× input，读价=0.1× input。
package usage

import (
	"strings"

	"github.com/cass-2003/ccpill/internal/transcript"
)

type price struct{ in, out float64 } // $/1M tokens

// 匹配按序做子串判断，先长后短（fable-5 先于 sonnet 等无前缀冲突，但 haiku-3 需在 haiku 前）。
var priceTable = []struct {
	match string
	p     price
}{
	{"fable-5", price{10, 50}},
	{"mythos-5", price{10, 50}},
	{"opus-4", price{5, 25}},   // 4.5/4.6/4.7/4.8 同价
	{"opus", price{15, 75}},    // opus 4.1 及更早
	{"sonnet-5", price{3, 15}}, // 按 sticker 价，不追踪限时 intro 价
	{"sonnet", price{3, 15}},
	{"haiku-4", price{1, 5}},
	{"haiku-3-5", price{0.8, 4}},
	{"haiku", price{0.25, 1.25}},
}

// entryCost 计算单条用量的美元成本：优先 transcript 自带 costUSD（老版本 Claude Code），
// 否则按定价表重算；未知模型返回 0（宁可少算不虚报）。
func entryCost(e transcript.Entry) float64 {
	if e.HasCost {
		return e.CostUSD
	}
	model := strings.ToLower(e.Model)
	for _, row := range priceTable {
		if strings.Contains(model, row.match) {
			const m = 1e6
			return float64(e.Input)/m*row.p.in +
				float64(e.Output)/m*row.p.out +
				float64(e.Cache5m)/m*row.p.in*1.25 +
				float64(e.Cache1h)/m*row.p.in*2.0 +
				float64(e.CacheRead)/m*row.p.in*0.1
		}
	}
	return 0
}
