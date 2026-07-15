package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// claudeSettingsPath 返回 Claude Code settings.json 路径（CLAUDE_CONFIG_DIR 优先）。
func claudeSettingsPath() (string, error) {
	base := os.Getenv("CLAUDE_CONFIG_DIR")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".claude")
	}
	return filepath.Join(base, "settings.json"), nil
}

// loadSettings 读入 settings.json 为泛型 map（保留全部未知字段）。
func loadSettings(path string) (map[string]any, error) {
	doc := map[string]any{}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return doc, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("settings.json 解析失败（不覆盖，请手工检查）: %w", err)
	}
	return doc, nil
}

// writeSettings 备份原文件后原子写入。
func writeSettings(path string, doc map[string]any) error {
	if old, err := os.ReadFile(path); err == nil {
		backup := path + ".ccpill-backup-" + time.Now().Format("20060102-150405")
		if err := os.WriteFile(backup, old, 0o644); err != nil {
			return fmt.Errorf("备份失败，中止写入: %w", err)
		}
		fmt.Fprintln(os.Stderr, "已备份原配置到:", backup)
	}
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "settings-*.json")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// install 把 ccpill 写入 Claude Code 的 statusLine 配置。
func install() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	// 必须用正斜杠：Claude Code 在 Windows 上经 Git Bash(sh) 执行 statusline 命令，
	// 反斜杠路径会被 sh 当转义符吃掉（J:\a\b.exe → J:ab.exe → command not found）。
	// 正斜杠在 sh 和 cmd 下都能正确执行。
	exe = filepath.ToSlash(exe)
	path, err := claudeSettingsPath()
	if err != nil {
		return err
	}
	doc, err := loadSettings(path)
	if err != nil {
		return err
	}
	doc["statusLine"] = map[string]any{
		"type":    "command",
		"command": exe,
		"padding": 0, // 必须为 0：ccpill 自带间距，避免 Claude Code 双重缩进
	}
	if err := writeSettings(path, doc); err != nil {
		return err
	}
	fmt.Println("✅ ccpill 已上岗:", exe)
	fmt.Println("   重启 Claude Code（或开新会话）即可看到胶囊状态栏")
	fmt.Println("   自定义外观: ccpill --config   卸载: ccpill --uninstall")
	return nil
}

// uninstall 移除 statusLine 配置。
func uninstall() error {
	path, err := claudeSettingsPath()
	if err != nil {
		return err
	}
	doc, err := loadSettings(path)
	if err != nil {
		return err
	}
	if _, ok := doc["statusLine"]; !ok {
		fmt.Println("statusLine 未配置，无需卸载")
		return nil
	}
	delete(doc, "statusLine")
	if err := writeSettings(path, doc); err != nil {
		return err
	}
	fmt.Println("✅ 已移除 ccpill 的 statusLine 配置")
	return nil
}
