//go:build !windows

package sysinfo

// 非 Windows 平台占位：开源 V0.3 补 /proc 与 sysctl 实现（Windows 优先决策，PRD §2）。

func CPUPercent() (float64, bool) { return 0, false }

func MemPercent() (float64, bool) { return 0, false }
