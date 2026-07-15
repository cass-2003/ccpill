// Package gitinfo 用单次 `git status --porcelain=v2 --branch` 采集全部 git 状态。
// 竞品坑规避（拆解 02）：所有 git 调用必须带超时与 --no-optional-locks。
package gitinfo

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

type Info struct {
	IsRepo   bool
	Branch   string
	SHA      string // HEAD 完整 oid（porcelain v2 branch.oid；初始仓库为空）
	Upstream string // 上游分支（如 origin/main；未设上游为空）
	Ahead    int
	Behind   int
	Dirty    int // 未提交变更条目数（含未跟踪）
	// 工作区分类计数（porcelain v2 同一次输出解析，零额外子进程）
	Staged    int // 暂存区有改动的条目（XY 的 X 位）
	Unstaged  int // 工作区有改动的条目（XY 的 Y 位）
	Untracked int // 未跟踪文件
	Conflicts int // 冲突（unmerged）条目
	Stash     int // stash 条数（--show-stash 头部）
}

const timeout = 500 * time.Millisecond

// Collect 在 dir 下采集 git 状态；任何失败都返回 IsRepo=false，绝不阻塞渲染。
func Collect(dir string) Info {
	if dir == "" {
		return Info{}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "--no-optional-locks", "-C", dir,
		"status", "--porcelain=v2", "--branch", "--show-stash")
	out, err := cmd.Output()
	if err != nil {
		return Info{}
	}
	return parsePorcelainV2(string(out))
}

func parsePorcelainV2(out string) Info {
	info := Info{IsRepo: true}
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			info.Branch = strings.TrimPrefix(line, "# branch.head ")
		case strings.HasPrefix(line, "# branch.oid "):
			if oid := strings.TrimPrefix(line, "# branch.oid "); oid != "(initial)" {
				info.SHA = oid
			}
		case strings.HasPrefix(line, "# branch.ab "):
			// 形如 "# branch.ab +2 -1"
			for _, f := range strings.Fields(strings.TrimPrefix(line, "# branch.ab ")) {
				if len(f) < 2 {
					continue
				}
				n := atoi(f[1:])
				if f[0] == '+' {
					info.Ahead = n
				} else if f[0] == '-' {
					info.Behind = n
				}
			}
		case strings.HasPrefix(line, "# branch.upstream "):
			info.Upstream = strings.TrimPrefix(line, "# branch.upstream ")
		case strings.HasPrefix(line, "# stash "):
			info.Stash = atoi(strings.TrimPrefix(line, "# stash "))
		case line == "" || strings.HasPrefix(line, "#"):
			// 其他头部行忽略
		default:
			info.Dirty++
			classifyEntry(line, &info)
		}
	}
	return info
}

// classifyEntry 按 porcelain v2 条目类型归类计数。
// 格式：`1 XY ...` 普通变更 / `2 XY ...` 重命名 / `u ...` 冲突 / `? path` 未跟踪。
func classifyEntry(line string, info *Info) {
	switch line[0] {
	case '1', '2':
		if len(line) >= 4 {
			if line[2] != '.' {
				info.Staged++
			}
			if line[3] != '.' {
				info.Unstaged++
			}
		}
	case 'u':
		info.Conflicts++
	case '?':
		info.Untracked++
	}
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}
