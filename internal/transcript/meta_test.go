package transcript

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanMeta(t *testing.T) {
	lines := `{"type":"user","timestamp":"2026-07-15T10:00:00Z","message":{"role":"user"}}
{"type":"assistant","timestamp":"2026-07-15T10:00:30Z","message":{"role":"assistant"}}
{"type":"custom-title","customTitle":"修复登录超时"}
{"type":"user","timestamp":"2026-07-15T10:05:00Z","message":{"content":[{"tool_use_id":"tu_1","type":"tool_result"}]}}
{"type":"user","timestamp":"2026-07-15T10:10:00Z","message":{"role":"user"}}
{"type":"assistant","timestamp":"2026-07-15T10:11:30Z","message":{"role":"assistant"}}
{"type":"assistant","timestamp":"2026-07-15T10:12:00Z","message":{"role":"assistant"}}
{"type":"user","timestamp":"2026-07-15T10:20:00Z","isSidechain":true,"message":{"role":"user"}}
`
	p := filepath.Join(t.TempDir(), "t.jsonl")
	if err := os.WriteFile(p, []byte(lines), 0o644); err != nil {
		t.Fatal(err)
	}
	m := ScanMeta(p)
	if m.Title != "修复登录超时" {
		t.Errorf("Title = %q", m.Title)
	}
	// tool_result 回写与 sidechain 不计入
	if m.UserMsgs != 2 {
		t.Errorf("UserMsgs = %d, want 2", m.UserMsgs)
	}
	// 两次配对：30s 与 90s → 平均 60s；第二条 assistant 不重复配对
	if m.RespCount != 2 || m.RespAvg != time.Minute || m.RespLast != 90*time.Second {
		t.Errorf("Resp = count %d avg %v last %v", m.RespCount, m.RespAvg, m.RespLast)
	}
}

func TestScanMetaMissing(t *testing.T) {
	if m := ScanMeta(filepath.Join(t.TempDir(), "nope.jsonl")); m.Title != "" || m.UserMsgs != 0 {
		t.Errorf("缺失文件应返回零值: %+v", m)
	}
}
