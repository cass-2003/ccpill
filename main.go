// ccpill — a pill-styled, blazing-fast statusline for Claude Code.
package main

import (
	"fmt"
	"io"
	"os"

	"ccpill/internal/compose"
	"ccpill/internal/config"
	"ccpill/internal/input"
	"ccpill/internal/render"
	"ccpill/internal/theme"
	"ccpill/internal/webui"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Println("ccpill " + version)
			return
		case "--config":
			if err := webui.Serve(); err != nil {
				fmt.Fprintln(os.Stderr, "ccpill:", err)
				os.Exit(1)
			}
			return
		case "--install":
			if err := install(); err != nil {
				fmt.Fprintln(os.Stderr, "ccpill:", err)
				os.Exit(1)
			}
			return
		case "--uninstall":
			if err := uninstall(); err != nil {
				fmt.Fprintln(os.Stderr, "ccpill:", err)
				os.Exit(1)
			}
			return
		case "--help", "-h":
			fmt.Println(`ccpill 💊 — Claude Code 胶囊状态栏

用法:
  ccpill --install     一键上岗（写入 Claude Code settings.json，自动备份）
  ccpill --config      打开 Web 配置中心（主题/布局/预警，实时预览）
  ccpill --uninstall   卸载（移除 statusLine 配置）
  ccpill --version     版本

无参数时从 stdin 读 Claude Code 状态 JSON 并渲染状态栏（由 Claude Code 调用）。
配置文件: ~/.claude/ccpill/config.toml`)
			return
		}
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ccpill:", err)
		os.Exit(1)
	}
}

func run() error {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	status, err := input.Parse(raw)
	if err != nil {
		return fmt.Errorf("解析 stdin JSON 失败: %w", err)
	}

	cacheLastStatus(raw)

	cfg := config.Load()
	ic := render.Icons(cfg.IconSet)
	opt := render.Options{Theme: theme.Get(cfg.Theme), PillMode: cfg.Pills, CapL: ic.CapL, CapR: ic.CapR}
	for _, pills := range compose.Lines(cfg, status) {
		line := render.Line(pills, opt)
		if line != "" {
			fmt.Println(line) // 空行跳过，避免 Claude Code 渲染多余空隙
		}
	}
	return nil
}

// cacheLastStatus 缓存本次 stdin 快照，作为 Web 配置中心「真实会话数据预览」的数据源。
// 尽力而为：失败不影响渲染。
func cacheLastStatus(raw []byte) {
	if err := os.MkdirAll(config.Dir(), 0o755); err != nil {
		return
	}
	_ = os.WriteFile(webui.LastStatusPath(), raw, 0o644)
}
