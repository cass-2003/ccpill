// Package config 加载/保存 ccpill 配置（~/.claude/ccpill/config.toml）。
// 契约（拆解 01 §6.2）：读取/解析失败时返回内存态默认值，绝不覆盖用户原文件；
// 保存用 temp file + rename 原子写入。
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Version int        `toml:"version"`
	Theme   string     `toml:"theme"`
	Pills   bool       `toml:"pills"`    // 胶囊背景开关（设计定案：可一键关闭）
	IconSet string     `toml:"icon_set"` // nerd | unicode | ascii
	Lines   [][]string `toml:"lines"`    // 1-3 行，每行有序 segment ID 列表
}

// Default 返回默认配置（设计稿 C 双行默认布局）。
func Default() Config {
	return Config{
		Version: 1,
		Theme:   "catppuccin-mocha",
		Pills:   true,
		IconSet: "unicode",
		Lines: [][]string{
			{"model", "context", "cost"},
			{"dir", "git"},
		},
	}
}

// Dir 返回配置目录（跟随 CLAUDE_CONFIG_DIR，与 Claude Code 同目录树）。
func Dir() string {
	base := os.Getenv("CLAUDE_CONFIG_DIR")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".claude")
	}
	return filepath.Join(base, "ccpill")
}

// Path 返回配置文件路径。
func Path() string { return filepath.Join(Dir(), "config.toml") }

// Load 加载配置；文件不存在或解析失败一律返回默认值（不写盘、不覆盖）。
func Load() Config {
	cfg := Default()
	b, err := os.ReadFile(Path())
	if err != nil {
		return cfg
	}
	var loaded Config
	if err := toml.Unmarshal(b, &loaded); err != nil {
		return cfg // 解析失败：内存态回退默认，原文件保持不动
	}
	if loaded.Theme == "" {
		loaded.Theme = cfg.Theme
	}
	if loaded.IconSet == "" {
		loaded.IconSet = cfg.IconSet
	}
	if len(loaded.Lines) == 0 {
		loaded.Lines = cfg.Lines
	}
	if len(loaded.Lines) > 3 {
		loaded.Lines = loaded.Lines[:3]
	}
	return loaded
}

// Save 原子写入配置。
func Save(cfg Config) error {
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(Dir(), "config-*.toml")
	if err != nil {
		return err
	}
	enc := toml.NewEncoder(tmp)
	if err := enc.Encode(cfg); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), Path())
}
