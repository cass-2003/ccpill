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

type pillJSON struct {
	Text string `json:"text"`
	FG   string `json:"fg"`
	Warn bool   `json:"warn"`
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
	rows, hidden := compose.LinesDetail(cfg, status)
	lines := make([][]pillJSON, 0, 3)
	for _, row := range rows {
		line := make([]pillJSON, 0, len(row))
		for _, p := range row {
			line = append(line, pillJSON{Text: p.Text, FG: p.Color.Hex(), Warn: p.Level == render.Warn})
		}
		lines = append(lines, line)
	}
	if hidden == nil {
		hidden = []string{}
	}
	return map[string]any{
		"lines":  lines,
		"hidden": hidden, // 已启用但当前条件不满足而隐藏的 segment
		"theme":  themeJSON{Name: t.Name, PillBG: t.PillBG.Hex(), Sep: t.Sep.Hex(), Warn: t.Warn.Hex(), WarnFG: t.WarnFG.Hex()},
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
