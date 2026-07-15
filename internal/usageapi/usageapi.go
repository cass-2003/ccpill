// Package usageapi 请求 Anthropic OAuth 用量接口（对齐 ccstatusline usage-fetch /
// CCometixLine usage.rs）：拿 stdin 里没有的分模型周限额与超额（overage）数据。
// 凭据来自 Claude Code 自己的 .credentials.json，只读取、不回显、不落日志。
package usageapi

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/cass-2003/ccpill/internal/cache"
)

const (
	url        = "https://api.anthropic.com/api/oauth/usage"
	betaHeader = "oauth-2025-04-20"
	ttl        = 5 * time.Minute // 成功与失败共用：即使失败也 5 分钟内不再打 API
	timeout    = 3 * time.Second
)

// Data 是渲染需要的用量子集。OK=false 表示本轮无可用数据（无凭据/网络失败）。
type Data struct {
	OK             bool    `json:"ok"`
	SevenDaySonnet float64 `json:"seven_day_sonnet"` // 利用率 %，<0 = 接口未返回该桶
	SevenDayOpus   float64 `json:"seven_day_opus"`
	ExtraEnabled   bool    `json:"extra_enabled"` // 超额（overage）是否开启
	ExtraLimit     float64 `json:"extra_limit"`   // 月度上限（美元额度）
	ExtraUsed      float64 `json:"extra_used"`
	ExtraUtil      float64 `json:"extra_util"` // 超额利用率 %
}

type bucket struct {
	Utilization *float64 `json:"utilization"`
}

type apiResp struct {
	SevenDaySonnet *bucket `json:"seven_day_sonnet"`
	SevenDayOpus   *bucket `json:"seven_day_opus"`
	ExtraUsage     *struct {
		IsEnabled    *bool    `json:"is_enabled"`
		MonthlyLimit *float64 `json:"monthly_limit"`
		UsedCredits  *float64 `json:"used_credits"`
		Utilization  *float64 `json:"utilization"`
	} `json:"extra_usage"`
}

// Fetch 返回用量数据（5 分钟缓存，失败静默降级为 OK=false）。
func Fetch() Data {
	var d Data
	if cache.Get("oauth-usage", ttl, &d) {
		return d
	}
	d = fetchLive()
	cache.Put("oauth-usage", d)
	return d
}

func fetchLive() Data {
	token := readToken()
	if token == "" {
		return Data{}
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return Data{}
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", betaHeader)
	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return Data{}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Data{}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Data{}
	}
	return parse(body)
}

func parse(body []byte) Data {
	var r apiResp
	if json.Unmarshal(body, &r) != nil {
		return Data{}
	}
	d := Data{OK: true, SevenDaySonnet: -1, SevenDayOpus: -1}
	if r.SevenDaySonnet != nil && r.SevenDaySonnet.Utilization != nil {
		d.SevenDaySonnet = *r.SevenDaySonnet.Utilization
	}
	if r.SevenDayOpus != nil && r.SevenDayOpus.Utilization != nil {
		d.SevenDayOpus = *r.SevenDayOpus.Utilization
	}
	if e := r.ExtraUsage; e != nil {
		if e.IsEnabled != nil {
			d.ExtraEnabled = *e.IsEnabled
		}
		if e.MonthlyLimit != nil {
			d.ExtraLimit = *e.MonthlyLimit
		}
		if e.UsedCredits != nil {
			d.ExtraUsed = *e.UsedCredits
		}
		if e.Utilization != nil {
			d.ExtraUtil = *e.Utilization
		}
	}
	return d
}

// readToken 从 Claude Code 凭据文件取 OAuth access token（与 ccstatusline 同源）。
func readToken() string {
	base := os.Getenv("CLAUDE_CONFIG_DIR")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".claude")
	}
	b, err := os.ReadFile(filepath.Join(base, ".credentials.json"))
	if err != nil {
		return ""
	}
	var doc struct {
		ClaudeAiOauth struct {
			AccessToken string `json:"accessToken"`
		} `json:"claudeAiOauth"`
	}
	if json.Unmarshal(b, &doc) != nil {
		return ""
	}
	return doc.ClaudeAiOauth.AccessToken
}
