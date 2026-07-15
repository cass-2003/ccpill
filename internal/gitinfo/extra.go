// 按需采集的 git 扩展信息：只有对应 segment 启用时才付出子进程/文件系统开销。
// 与主采集同一原则：全部带超时 + --no-optional-locks，任何失败静默降级。
package gitinfo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// run 执行一条 git 子命令，失败或超时返回空串。
func run(dir string, args ...string) string {
	if dir == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	full := append([]string{"--no-optional-locks", "-C", dir}, args...)
	out, err := exec.CommandContext(ctx, "git", full...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// ---- 增删行数（对齐 ccstatusline git-changes/insertions/deletions） ----

type DiffStat struct {
	Ins, Del int
	OK       bool // false = 采集失败（区别于"无改动"）
}

var insRe = regexp.MustCompile(`(\d+) insertion`)
var delRe = regexp.MustCompile(`(\d+) deletion`)

// Diff 统计相对 HEAD 的未提交增删行数（暂存 + 未暂存）。
// 初始仓库（无 HEAD）回退到仅工作区 diff。
func Diff(dir string) DiffStat {
	out := run(dir, "diff", "HEAD", "--shortstat")
	if out == "" {
		// 可能是无改动（合法空输出）也可能是无 HEAD；回退一次纯工作区 diff
		out = run(dir, "diff", "--shortstat")
	}
	return parseShortStat(out)
}

func parseShortStat(out string) DiffStat {
	d := DiffStat{OK: true}
	if m := insRe.FindStringSubmatch(out); m != nil {
		d.Ins = atoi(m[1])
	}
	if m := delRe.FindStringSubmatch(out); m != nil {
		d.Del = atoi(m[1])
	}
	return d
}

// ---- 最近 tag（对齐 claude-powerline tag） ----

func Tag(dir string) string {
	return run(dir, "describe", "--tags", "--abbrev=0")
}

// ---- 距上次 commit 时长（对齐 claude-powerline timeSinceCommit） ----

func CommitAge(dir string) (time.Duration, bool) {
	out := run(dir, "log", "-1", "--format=%ct")
	if out == "" {
		return 0, false
	}
	ct := atoi(out)
	if ct == 0 {
		return 0, false
	}
	age := time.Since(time.Unix(int64(ct), 0))
	if age < 0 {
		age = 0
	}
	return age, true
}

// ---- origin 远程 owner/repo（对齐 ccstatusline git-origin-owner-repo） ----

func Remote(dir string) string {
	return ParseRemoteURL(run(dir, "remote", "get-url", "origin"))
}

// ParseRemoteURL 从远程 URL 提取 owner/repo。
// 支持 scheme://host/owner/repo(.git) 与 ssh 简写 git@host:owner/repo(.git)。
func ParseRemoteURL(url string) string {
	if url == "" {
		return ""
	}
	url = strings.TrimSuffix(strings.TrimSuffix(url, "/"), ".git")
	if i := strings.Index(url, "://"); i >= 0 {
		url = url[i+3:] // 去 scheme
		if j := strings.Index(url, "/"); j >= 0 {
			url = url[j+1:] // 去 host
		} else {
			return ""
		}
	} else if i := strings.Index(url, ":"); i >= 0 {
		url = url[i+1:] // ssh 简写：取冒号后
	}
	parts := strings.Split(url, "/")
	if len(parts) < 2 {
		return ""
	}
	owner, repo := parts[len(parts)-2], parts[len(parts)-1]
	if owner == "" || repo == "" {
		return ""
	}
	return owner + "/" + repo
}

// ---- 进行中的操作 + 仓库根目录（纯文件系统，零子进程） ----

// FindRoot 从 dir 向上找包含 .git 的目录，返回 (仓库根路径, gitdir 实际路径)。
// worktree 的 .git 是文件（gitdir: 重定向），操作状态文件在重定向目标里。
func FindRoot(dir string) (root, gitDir string) {
	cur := dir
	for {
		p := filepath.Join(cur, ".git")
		if st, err := os.Stat(p); err == nil {
			if st.IsDir() {
				return cur, p
			}
			// .git 文件：worktree/submodule 重定向
			if b, err := os.ReadFile(p); err == nil {
				line := strings.TrimSpace(string(b))
				if g, ok := strings.CutPrefix(line, "gitdir:"); ok {
					g = strings.TrimSpace(g)
					if !filepath.IsAbs(g) {
						g = filepath.Join(cur, g)
					}
					return cur, g
				}
			}
			return cur, ""
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", ""
		}
		cur = parent
	}
}

// State 返回进行中的多步操作名（MERGE/REBASE/CHERRY-PICK/BISECT/REVERT），无则空串。
func State(gitDir string) string {
	if gitDir == "" {
		return ""
	}
	exists := func(name string) bool {
		_, err := os.Stat(filepath.Join(gitDir, name))
		return err == nil
	}
	switch {
	case exists("rebase-merge") || exists("rebase-apply"):
		return "REBASE"
	case exists("MERGE_HEAD"):
		return "MERGE"
	case exists("CHERRY_PICK_HEAD"):
		return "CHERRY-PICK"
	case exists("REVERT_HEAD"):
		return "REVERT"
	case exists("BISECT_LOG"):
		return "BISECT"
	}
	return ""
}
