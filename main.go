// ccpill — a pill-styled, blazing-fast statusline for Claude Code.
package main

import (
	"fmt"
	"io"
	"os"

	"ccpill/internal/config"
	"ccpill/internal/input"
	"ccpill/internal/render"
	"ccpill/internal/segment"
	"ccpill/internal/theme"
)

const version = "0.1.0-dev"

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--version", "-v":
			fmt.Println("ccpill " + version)
			return
		case "--config":
			fmt.Fprintln(os.Stderr, "Web 配置中心开发中（V0.1 后续工作包）")
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

	cfg := config.Load()
	ctx := &segment.Context{
		Status: status,
		Icons:  render.Icons(cfg.IconSet),
		Theme:  theme.Get(cfg.Theme),
	}
	opt := render.Options{Theme: ctx.Theme, PillMode: cfg.Pills}

	for _, lineIDs := range cfg.Lines {
		var pills []render.Pill
		for _, id := range lineIDs {
			seg := segment.Get(id)
			if seg == nil {
				continue // 未知 segment ID：向前兼容，忽略而非报错
			}
			if p := seg.Render(ctx); p != nil {
				pills = append(pills, *p)
			}
		}
		line := render.Line(pills, opt)
		if line != "" {
			fmt.Println(line) // 空行跳过，避免 Claude Code 渲染多余空隙
		}
	}
	return nil
}
