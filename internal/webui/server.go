// Package webui 是 ccpill 的本地 Web 配置中心（差异化核心，PRD §3.5）。
// 仅监听 127.0.0.1；页面经 go:embed 内嵌，零外部依赖、无网络请求。
package webui

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"ccpill/internal/compose"
	"ccpill/internal/config"
	"ccpill/internal/input"
	"ccpill/internal/render"
	"ccpill/internal/segment"
	"ccpill/internal/theme"
)

//go:embed index.html
var indexHTML []byte

//go:embed sample.json
var sampleStatus []byte

// LastStatusPath 是主程序缓存 stdin 快照的位置（Web 预览的真实数据源）。
func LastStatusPath() string { return filepath.Join(config.Dir(), "last-status.json") }

func loadStatus() (*input.Status, bool) {
	if b, err := os.ReadFile(LastStatusPath()); err == nil {
		if s, err := input.Parse(b); err == nil {
			return s, true // 真实会话数据
		}
	}
	s, _ := input.Parse(sampleStatus)
	return s, false
}

type spanJSON struct {
	Text string `json:"text"`
	FG   string `json:"fg"`
}

type pillJSON struct {
	Seg    string     `json:"seg"`
	Text   string     `json:"text"`
	FG     string     `json:"fg"`
	Warn   bool       `json:"warn"`
	Sample bool       `json:"sample"`          // true = 条件未满足，以示例数据占位展示
	Spans  []spanJSON `json:"spans,omitempty"` // 胶囊内多色片段（如 gitab 的 +绿/−红）
}

func spansJSON(spans []render.Span) []spanJSON {
	if len(spans) == 0 {
		return nil
	}
	out := make([]spanJSON, len(spans))
	for i, s := range spans {
		out[i] = spanJSON{Text: s.Text, FG: s.Color.Hex()}
	}
	return out
}

// samplePill 为条件显示类 segment 生成示例胶囊（仅 Web 预览用，终端不渲染）。
// 前缀跟随紧凑模式（minimal），与真实渲染保持一致。
func samplePill(id string, cfg config.Config, t theme.Theme, ic render.IconSet) *pillJSON {
	L := func(prefix string) string {
		if cfg.Minimal {
			return ""
		}
		return prefix
	}
	var text string
	var fg theme.RGB
	switch id {
	case "model":
		text, fg = ic.Model+" Fable 5"+L(" · think:hi"), t.Model
	case "context":
		text, fg = L("ctx ")+render.Bar(52, 10, ic)+" 52%", t.Context
	case "cost":
		text, fg = ic.Cost+"1.23", t.Cost
	case "today":
		text, fg = L("今日 ")+ic.Cost+"12.34", t.Cost
	case "burn":
		text, fg = ic.Flame+" "+ic.Cost+"8.6/h", t.Rate
	case "block":
		text, fg = L("5h ")+"34% ⏳ 2h10m", t.Rate
	case "git":
		text, fg = ic.Branch+" main "+ic.Dirty+"3", t.Git
	case "dir":
		text, fg = ic.Dir+" project", t.Dir
	case "worktree":
		text, fg = L("wt:")+"feature-x", t.Extra
	case "speed":
		text, fg = L("tok ")+"42/s", t.Cost
	case "session":
		text, fg = ic.Clock+" 1h02m", t.Clock
	case "compact":
		text, fg = L("compact ")+"×2", t.Extra
	case "style":
		text, fg = "concise · vim:i", t.Extra
	case "cpumem":
		if cfg.Minimal {
			text, fg = "C12% · M45%", t.Clock
		} else {
			text, fg = "CPU 12% · MEM 45%", t.Clock
		}
	case "mcp":
		text, fg = L("MCP ")+"●3", t.Git
	case "pr":
		text, fg = L("PR ")+"#128", t.Extra
	case "api":
		text, fg = L("API ")+"●", t.Cost
	case "tokens":
		text, fg = "⇅ 1.2M/38k", t.Context
	case "cachehit":
		text, fg = L("cache ")+"96%", t.Context
	case "lines":
		text, fg = "+123 −45", t.Git
	case "weekly":
		text, fg = L("7d ")+"62% ⏳ 3d4h", t.Rate
	case "version":
		text, fg = "v2.1.209", t.Muted
	case "gitsha":
		text, fg = "f71cfc4", t.Git
	case "sessionid":
		text, fg = L("sid ")+"ae5a234a", t.Muted
	case "email":
		text, fg = "you@example.com", t.Muted
	case "text":
		text, fg = "我的备注", t.Extra
	case "cmd":
		text, fg = "cmd 输出示例", t.Extra
	case "modelname":
		text, fg = ic.Model+" Fable 5", t.Model
	case "think":
		text, fg = L("think:")+"hi", t.Model
	case "ctxbar":
		text, fg = render.Bar(52, 10, ic), t.Context
	case "ctxpct":
		text, fg = L("ctx ")+"52%", t.Context
	case "ctxlen":
		text, fg = L("ctx ")+"89k", t.Context
	case "tokin":
		text, fg = L("in ")+"1.2M", t.Context
	case "tokout":
		text, fg = L("out ")+"38k", t.Context
	case "tokcache":
		text, fg = L("cached ")+"8.4M", t.Context
	case "toktotal":
		text, fg = L("tok ")+"9.9M", t.Context
	case "gitbranch":
		text, fg = ic.Branch+" main", t.Git
	case "gitchanges":
		text, fg = ic.Dirty+"3", t.Git
	case "gitab":
		return &pillJSON{Seg: id, Text: "+2 −1", FG: t.Git.Hex(), Sample: true,
			Spans: []spanJSON{{Text: "+2", FG: t.Cost.Hex()}, {Text: " −1", FG: t.Warn.Hex()}}}
	case "gitstatus":
		return &pillJSON{Seg: id, Text: "S2 U1 ?3", FG: t.Git.Hex(), Sample: true,
			Spans: []spanJSON{{Text: "S2", FG: t.Cost.Hex()}, {Text: " U1", FG: t.Rate.Hex()}, {Text: " ?3", FG: t.Muted.Hex()}}}
	case "gitstaged":
		text, fg = "S:2", t.Cost
	case "gitunstaged":
		text, fg = "U:1", t.Rate
	case "gituntracked":
		text, fg = "?:3", t.Muted
	case "gitconflicts":
		text, fg = "✖1", t.Warn
	case "gitstash":
		text, fg = "⚑2", t.Extra
	case "gitstate":
		text, fg = "REBASE", t.Warn
	case "gitrepo":
		text, fg = "my-repo", t.Dir
	case "gitdiff":
		return &pillJSON{Seg: id, Text: "+42 −10", FG: t.Git.Hex(), Sample: true,
			Spans: []spanJSON{{Text: "+42", FG: t.Cost.Hex()}, {Text: " −10", FG: t.Warn.Hex()}}}
	case "gitins":
		text, fg = "+42", t.Cost
	case "gitdel":
		text, fg = "−10", t.Warn
	case "gittag":
		text, fg = "v0.2.0", t.Extra
	case "gitage":
		text, fg = L("commit ")+"3h", t.Clock
	case "gitremote":
		text, fg = "owner/repo", t.Muted
	case "blockpct":
		text, fg = L("5h ")+"34%", t.Rate
	case "blocktime":
		text, fg = "⏳ 2h10m", t.Rate
	case "cpu":
		if cfg.Minimal {
			text, fg = "C12%", t.Clock
		} else {
			text, fg = "CPU 12%", t.Clock
		}
	case "mem":
		if cfg.Minimal {
			text, fg = "M45%", t.Clock
		} else {
			text, fg = "MEM 45%", t.Clock
		}
	case "outstyle":
		text, fg = "concise", t.Extra
	case "vim":
		text, fg = "vim:i", t.Extra
	default:
		return nil
	}
	return &pillJSON{Seg: id, Text: text, FG: fg.Hex(), Sample: true}
}

type themeJSON struct {
	Name   string `json:"name"`
	PillBG string `json:"pill_bg"`
	Sep    string `json:"sep"`
	Warn   string `json:"warn"`
	WarnFG string `json:"warn_fg"`
}

func previewPayload(cfg config.Config, status *input.Status, real bool) map[string]any {
	t := theme.Get(cfg.Theme)
	ic := render.Icons(cfg.IconSet)
	capL, _ := render.CapGlyphs(cfg.Caps, cfg.IconSet)
	lines := make([][]pillJSON, 0, 3)
	sample := []string{} // 本次以示例数据渲染的 segment（条件未满足）
	for _, row := range compose.Detail(cfg, status) {
		line := make([]pillJSON, 0, len(row))
		for _, it := range row {
			if it.Pill != nil {
				line = append(line, pillJSON{Seg: it.ID, Text: it.Pill.Text, FG: it.Pill.Color.Hex(), Warn: it.Pill.Level == render.Warn, Spans: spansJSON(it.Pill.Spans)})
				continue
			}
			if sp := samplePill(it.ID, cfg, t, ic); sp != nil {
				line = append(line, *sp)
				sample = append(sample, it.ID)
			}
		}
		lines = append(lines, line)
	}
	return map[string]any{
		"lines":  lines,
		"sample": sample, // 已启用但条件未满足、预览中以示例数据占位的 segment
		"theme":  themeJSON{Name: t.Name, PillBG: t.PillBG.Hex(), Sep: t.Sep.Hex(), Warn: t.Warn.Hex(), WarnFG: t.WarnFG.Hex()},
		"round":  capL != "", // 终端胶囊是否圆角端帽（预览同步形状）
		"real":   real,
	}
}

func decodeConfig(r *http.Request) (config.Config, error) {
	cfg := config.Default()
	err := json.NewDecoder(r.Body).Decode(&cfg)
	if len(cfg.Lines) > 3 {
		cfg.Lines = cfg.Lines[:3]
	}
	return cfg, err
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

// Serve 启动配置中心并（尽力）打开浏览器；阻塞直到进程被终止。
func Serve() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	url := fmt.Sprintf("http://%s/", ln.Addr().String())

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		status, real := loadStatus()
		cfg := config.Load()
		writeJSON(w, map[string]any{
			"config":   cfg,
			"segments": segment.IDs(),
			"themes":   theme.Names(),
			"preview":  previewPayload(cfg, status, real),
		})
	})
	mux.HandleFunc("POST /api/preview", func(w http.ResponseWriter, r *http.Request) {
		cfg, err := decodeConfig(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		status, real := loadStatus()
		writeJSON(w, previewPayload(cfg, status, real))
	})
	mux.HandleFunc("POST /api/save", func(w http.ResponseWriter, r *http.Request) {
		cfg, err := decodeConfig(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := config.Save(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "path": config.Path()})
	})

	fmt.Fprintln(os.Stderr, "ccpill 配置中心: "+url+"  (Ctrl+C 退出)")
	openBrowser(url)
	return http.Serve(ln, mux)
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
