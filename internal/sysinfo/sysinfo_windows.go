//go:build windows

// Package sysinfo 采集本机 CPU/内存占用。
// Windows 用 kernel32 syscall 直取（零子进程）；CPU 百分比靠「上次调用的采样
// 快照缓存 + 本次差分」——statusline 本身就被反复调用，天然适合差分采样。
package sysinfo

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"ccpill/internal/cache"
)

type cpuSample struct {
	Idle   uint64 `json:"idle"`
	Kernel uint64 `json:"kernel"`
	User   uint64 `json:"user"`
}

var (
	kernel32            = windows.NewLazySystemDLL("kernel32.dll")
	procGetSystemTimes  = kernel32.NewProc("GetSystemTimes")
	procGlobalMemStatus = kernel32.NewProc("GlobalMemoryStatusEx")
)

func getSystemTimes() (cpuSample, bool) {
	var idle, kernel, user windows.Filetime
	r, _, _ := procGetSystemTimes.Call(
		uintptr(unsafe.Pointer(&idle)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)))
	if r == 0 {
		return cpuSample{}, false
	}
	ft := func(f windows.Filetime) uint64 { return uint64(f.HighDateTime)<<32 | uint64(f.LowDateTime) }
	return cpuSample{Idle: ft(idle), Kernel: ft(kernel), User: ft(user)}, true
}

// CPUPercent 返回自上次 statusline 调用以来的 CPU 占用；首次调用无基线返回 false。
func CPUPercent() (float64, bool) {
	cur, ok := getSystemTimes()
	if !ok {
		return 0, false
	}
	var prev cpuSample
	had := cache.Get("cpu-sample", 10*time.Minute, &prev)
	cache.Put("cpu-sample", cur)
	if !had {
		return 0, false
	}
	total := (cur.Kernel - prev.Kernel) + (cur.User - prev.User) // kernel 已含 idle
	idle := cur.Idle - prev.Idle
	if total == 0 {
		return 0, false
	}
	pct := float64(total-idle) / float64(total) * 100
	if pct < 0 || pct > 100 {
		return 0, false
	}
	return pct, true
}

type memStatusEx struct {
	Length               uint32
	MemoryLoad           uint32
	TotalPhys            uint64
	AvailPhys            uint64
	TotalPageFile        uint64
	AvailPageFile        uint64
	TotalVirtual         uint64
	AvailVirtual         uint64
	AvailExtendedVirtual uint64
}

// MemPercent 返回物理内存占用百分比。
func MemPercent() (float64, bool) {
	var m memStatusEx
	m.Length = uint32(unsafe.Sizeof(m))
	r, _, _ := procGlobalMemStatus.Call(uintptr(unsafe.Pointer(&m)))
	if r == 0 {
		return 0, false
	}
	return float64(m.MemoryLoad), true
}
