package transcript

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func mk(id, req string, side bool, out int64) Entry {
	return Entry{Timestamp: time.Now(), MessageID: id, RequestID: req, IsSidechain: side, Output: out}
}

func TestDedupExactKey(t *testing.T) {
	// 同 (message.id, requestId)：流式写入的重复行只留一条
	got := Dedup([]Entry{mk("m1", "r1", false, 10), mk("m1", "r1", false, 20)})
	if len(got) != 1 || got[0].Output != 20 {
		t.Errorf("精确键去重失败: %+v", got)
	}
}

func TestDedupSidechainReplay(t *testing.T) {
	// sidechain 重放：同 message.id 不同 requestId，且一方是 sidechain → 合并，保留非 sidechain
	got := Dedup([]Entry{mk("m1", "r1", false, 30), mk("m1", "r2", true, 30)})
	if len(got) != 1 || got[0].IsSidechain {
		t.Errorf("sidechain 重放去重失败: %+v", got)
	}
	// 都非 sidechain 的同 id 不同 req：视为不同条目，保守保留
	got = Dedup([]Entry{mk("m2", "r1", false, 10), mk("m2", "r2", false, 10)})
	if len(got) != 2 {
		t.Errorf("非 sidechain 不应二级合并: %+v", got)
	}
}

func TestDedupNoID(t *testing.T) {
	got := Dedup([]Entry{mk("", "r1", false, 1), mk("", "r1", false, 1)})
	if len(got) != 2 {
		t.Errorf("无 message.id 永不去重: %+v", got)
	}
}

func TestReadFilePrefilterAndParse(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "t.jsonl")
	content := `{"type":"user","message":{"role":"user","content":"hi"}}
{"type":"assistant","timestamp":"2026-07-15T08:00:00Z","requestId":"req_1","costUSD":0.5,"message":{"id":"msg_1","usage":{"input_tokens":100,"output_tokens":50,"cache_read_input_tokens":1000}}}
not-json-garbage
{"type":"assistant","timestamp":"2026-07-15T08:01:00Z","message":{"id":"msg_2","usage":{"input_tokens":0,"output_tokens":0}}}
`
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	got := ReadFile(p)
	if len(got) != 1 {
		t.Fatalf("应只解析出 1 条有效条目, got %d", len(got))
	}
	e := got[0]
	if e.MessageID != "msg_1" || !e.HasCost || e.CostUSD != 0.5 || e.CacheRead != 1000 {
		t.Errorf("条目字段错误: %+v", e)
	}
}

func TestCountCompactions(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "t.jsonl")
	content := `{"type":"system","subtype":"compact_boundary"}
{"type":"assistant"}
{"type":"system","subtype":"compact_boundary"}
`
	_ = os.WriteFile(p, []byte(content), 0o644)
	if n := CountCompactions(p); n != 2 {
		t.Errorf("CountCompactions = %d, want 2", n)
	}
}
