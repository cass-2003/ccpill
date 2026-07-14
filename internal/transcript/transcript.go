// Package transcript 解析 Claude Code 的 JSONL 转录文件，提取用量条目。
// 算法复刻自 ccusage 拆解（docs/research/03）：
// 字节级预筛（不含 "usage" 的行不进 JSON 解析器）、
// (message.id, requestId) 去重 + sidechain 重放二级匹配、costUSD display 模式。
package transcript

import (
	"bytes"
	"encoding/json"
	"os"
	"time"
)

// Entry 是一条参与计费/统计的 assistant 用量记录。
type Entry struct {
	Timestamp   time.Time
	MessageID   string
	RequestID   string
	Model       string
	CostUSD     float64
	HasCost     bool
	IsSidechain bool
	Input       int64
	Output      int64
	CacheCreate int64 // cache_creation_input_tokens 总量
	Cache5m     int64 // ephemeral_5m 明细（写价 1.25×）；明细缺失时 = CacheCreate
	Cache1h     int64 // ephemeral_1h 明细（写价 2×）
	CacheRead   int64
}

func (e Entry) TotalTokens() int64 { return e.Input + e.Output + e.CacheCreate + e.CacheRead }

type rawLine struct {
	Timestamp   string   `json:"timestamp"`
	RequestID   string   `json:"requestId"`
	CostUSD     *float64 `json:"costUSD"`
	IsSidechain bool     `json:"isSidechain"`
	Message     struct {
		ID    string `json:"id"`
		Model string `json:"model"`
		Usage struct {
			InputTokens        int64 `json:"input_tokens"`
			OutputTokens       int64 `json:"output_tokens"`
			CacheCreationInput int64 `json:"cache_creation_input_tokens"`
			CacheReadInput     int64 `json:"cache_read_input_tokens"`
			CacheCreation      *struct {
				Ephemeral5m int64 `json:"ephemeral_5m_input_tokens"`
				Ephemeral1h int64 `json:"ephemeral_1h_input_tokens"`
			} `json:"cache_creation"`
		} `json:"usage"`
	} `json:"message"`
}

var usageMarker = []byte(`"usage"`)

// ReadFile 解析单个 JSONL 文件的用量条目（未去重）。
func ReadFile(path string) []Entry {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var out []Entry
	for len(b) > 0 {
		var line []byte
		if i := bytes.IndexByte(b, '\n'); i >= 0 {
			line, b = b[:i], b[i+1:]
		} else {
			line, b = b, nil
		}
		if !bytes.Contains(line, usageMarker) {
			continue // 预筛：绝大多数行不进 JSON 解析器
		}
		var r rawLine
		if json.Unmarshal(line, &r) != nil {
			continue // 容错：半行写入等损坏行静默跳过
		}
		u := r.Message.Usage
		if u.InputTokens == 0 && u.OutputTokens == 0 && u.CacheCreationInput == 0 && u.CacheReadInput == 0 {
			continue
		}
		ts, err := time.Parse(time.RFC3339, r.Timestamp)
		if err != nil {
			continue
		}
		e := Entry{
			Timestamp: ts, MessageID: r.Message.ID, RequestID: r.RequestID,
			Model:       r.Message.Model,
			IsSidechain: r.IsSidechain,
			Input:       u.InputTokens, Output: u.OutputTokens,
			CacheCreate: u.CacheCreationInput, CacheRead: u.CacheReadInput,
		}
		if cc := u.CacheCreation; cc != nil {
			e.Cache5m, e.Cache1h = cc.Ephemeral5m, cc.Ephemeral1h
		} else {
			e.Cache5m = u.CacheCreationInput // 无明细时保守按 5m（1.25×）计
		}
		if r.CostUSD != nil {
			e.CostUSD, e.HasCost = *r.CostUSD, true
		}
		out = append(out, e)
	}
	return out
}

// Dedup 按 (message.id, requestId) 去重；无 message.id 的条目保守保留。
// sidechain 重放（subagent 用新 requestId 重放父消息）按 message.id 二级匹配，
// 保留优先级：非 sidechain > token 总量大。
func Dedup(entries []Entry) []Entry {
	type slot struct{ idx int }
	byKey := map[string]slot{}
	byMsgID := map[string]slot{}
	var out []Entry
	for _, e := range entries {
		if e.MessageID == "" {
			out = append(out, e)
			continue
		}
		key := e.MessageID + "\x00" + e.RequestID
		if s, ok := byKey[key]; ok {
			out[s.idx] = pick(out[s.idx], e)
			continue
		}
		if s, ok := byMsgID[e.MessageID]; ok && (e.IsSidechain || out[s.idx].IsSidechain) {
			out[s.idx] = pick(out[s.idx], e)
			byKey[key] = s
			continue
		}
		byKey[key] = slot{len(out)}
		byMsgID[e.MessageID] = slot{len(out)}
		out = append(out, e)
	}
	return out
}

// CountCompactions 统计 transcript 里的 compact_boundary 标记（精确协议标记，
// 不做启发式推断——拆解 01 §2.4）。
func CountCompactions(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return bytes.Count(b, []byte(`"compact_boundary"`))
}

func pick(a, b Entry) Entry {
	if a.IsSidechain != b.IsSidechain {
		if a.IsSidechain {
			return b
		}
		return a
	}
	if b.TotalTokens() > a.TotalTokens() {
		return b
	}
	return a
}
