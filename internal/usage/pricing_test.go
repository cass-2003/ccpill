package usage

import (
	"math"
	"testing"

	"github.com/cass-2003/ccpill/internal/transcript"
)

func almost(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestEntryCostCostUSDPriority(t *testing.T) {
	// 老版本 transcript 自带 costUSD → 直接采用，不重算
	e := transcript.Entry{Model: "claude-opus-4-8", CostUSD: 1.23, HasCost: true, Output: 999999}
	if got := entryCost(e); got != 1.23 {
		t.Errorf("costUSD 优先级失效: %v", got)
	}
}

func TestEntryCostCalculate(t *testing.T) {
	// fable-5: $10 in / $50 out；1h cache 写 2×、读 0.1×
	e := transcript.Entry{
		Model: "claude-fable-5",
		Input: 1_000_000, Output: 100_000,
		Cache1h: 500_000, CacheRead: 2_000_000,
	}
	// 10 + 0.1*50 + 0.5*10*2 + 2*10*0.1 = 10 + 5 + 10 + 2 = 27
	if got := entryCost(e); !almost(got, 27) {
		t.Errorf("fable 计价错误: got %v want 27", got)
	}

	// opus-4-7: $5/$25；5m cache 写 1.25×
	e = transcript.Entry{Model: "claude-opus-4-7", Cache5m: 1_000_000}
	if got := entryCost(e); !almost(got, 6.25) {
		t.Errorf("opus 5m 缓存写计价错误: got %v want 6.25", got)
	}
}

func TestEntryCostUnknownModel(t *testing.T) {
	e := transcript.Entry{Model: "gpt-5.4-mini", Input: 1_000_000}
	if got := entryCost(e); got != 0 {
		t.Errorf("未知模型应返回 0（不虚报）: %v", got)
	}
	if got := entryCost(transcript.Entry{Input: 1_000_000}); got != 0 {
		t.Errorf("空模型应返回 0: %v", got)
	}
}
