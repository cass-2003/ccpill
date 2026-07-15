// Package segment 定义 widget 抽象与注册表。
// 架构结论（拆解 02）：注册表模式替代硬编码 if 链，撑得住 16+ widget 扩展。
package segment

import (
	"sort"
	"time"

	"ccpill/internal/config"
	"ccpill/internal/gitinfo"
	"ccpill/internal/input"
	"ccpill/internal/render"
	"ccpill/internal/theme"
	"ccpill/internal/transcript"
	"ccpill/internal/usage"
	"ccpill/internal/usageapi"
)

// Context 是一次渲染的共享数据：stdin 解析结果 + 惰性采集的外部信息。
// 约束（拆解 01 坑 1）：任何外部数据源在一次调用内只采集一次，segment 共享。
type Context struct {
	Status *input.Status
	Icons  render.IconSet
	Theme  theme.Theme
	Cfg    config.Config

	gitOnce bool
	git     gitinfo.Info

	usageOnce bool
	usageSum  usage.Summary

	// git 扩展信息：各自惰性，只有对应 segment 启用才付出采集开销
	gitDiffOnce   bool
	gitDiff       gitinfo.DiffStat
	gitTagOnce    bool
	gitTag        string
	gitAgeOnce    bool
	gitAge        time.Duration
	gitAgeOK      bool
	gitRemoteOnce bool
	gitRemote     string
	gitFSOnce     bool // FindRoot + State（纯文件系统）
	gitRoot       string
	gitState      string

	metaOnce bool
	meta     transcript.Meta

	apiOnce bool
	apiData usageapi.Data
}

// L 返回文字前缀；紧凑模式（config minimal）下为空串，只留数值与图标。
func (c *Context) L(prefix string) string {
	if c.Cfg.Minimal {
		return ""
	}
	return prefix
}

// Usage 惰性加载跨会话用量聚合（内部带 60s 文件缓存）。
func (c *Context) Usage() usage.Summary {
	if !c.usageOnce {
		c.usageSum = usage.Load()
		c.usageOnce = true
	}
	return c.usageSum
}

// gitDir 解析 git 采集的工作目录（stdin workspace 优先）。
func (c *Context) gitDir() string {
	dir := c.Status.Workspace.CurrentDir
	if dir == "" {
		dir = c.Status.CWD
	}
	return dir
}

// Git 惰性采集 git 信息（仅第一次调用真正跑 git 子进程）。
func (c *Context) Git() gitinfo.Info {
	if !c.gitOnce {
		c.git = gitinfo.Collect(c.gitDir())
		c.gitOnce = true
	}
	return c.git
}

// GitDiff 惰性统计相对 HEAD 的未提交增删行数。
func (c *Context) GitDiff() gitinfo.DiffStat {
	if !c.gitDiffOnce {
		c.gitDiff = gitinfo.Diff(c.gitDir())
		c.gitDiffOnce = true
	}
	return c.gitDiff
}

// GitTag 惰性取最近 tag。
func (c *Context) GitTag() string {
	if !c.gitTagOnce {
		c.gitTag = gitinfo.Tag(c.gitDir())
		c.gitTagOnce = true
	}
	return c.gitTag
}

// GitAge 惰性取距上次 commit 的时长。
func (c *Context) GitAge() (time.Duration, bool) {
	if !c.gitAgeOnce {
		c.gitAge, c.gitAgeOK = gitinfo.CommitAge(c.gitDir())
		c.gitAgeOnce = true
	}
	return c.gitAge, c.gitAgeOK
}

// GitRemote 惰性取 origin 的 owner/repo。
func (c *Context) GitRemote() string {
	if !c.gitRemoteOnce {
		c.gitRemote = gitinfo.Remote(c.gitDir())
		c.gitRemoteOnce = true
	}
	return c.gitRemote
}

// gitFS 惰性做一次文件系统探测（仓库根 + 进行中操作，零子进程）。
func (c *Context) gitFS() (root, state string) {
	if !c.gitFSOnce {
		var gd string
		c.gitRoot, gd = gitinfo.FindRoot(c.gitDir())
		c.gitState = gitinfo.State(gd)
		c.gitFSOnce = true
	}
	return c.gitRoot, c.gitState
}

// Meta 惰性扫描本会话 transcript 的元信息（会话名/消息数/响应耗时）。
func (c *Context) Meta() transcript.Meta {
	if !c.metaOnce {
		if path := c.Status.TranscriptPath; path != "" {
			c.meta = transcript.ScanMeta(path)
		}
		c.metaOnce = true
	}
	return c.meta
}

// API 惰性请求 OAuth 用量接口（内部 5 分钟缓存，失败静默 OK=false）。
func (c *Context) API() usageapi.Data {
	if !c.apiOnce {
		c.apiData = usageapi.Fetch()
		c.apiOnce = true
	}
	return c.apiData
}

// GitRepoRoot 返回仓库根目录路径（非仓库为空串）。
func (c *Context) GitRepoRoot() string { root, _ := c.gitFS(); return root }

// GitState 返回进行中的多步操作名（MERGE/REBASE/…），无则空串。
func (c *Context) GitState() string { _, state := c.gitFS(); return state }

// Segment 是一个 widget：返回 nil 表示本次无内容不渲染。
type Segment interface {
	ID() string
	Render(c *Context) *render.Pill
}

var registry = map[string]Segment{}

// Register 注册 segment（各实现文件 init 时调用）。
func Register(s Segment) { registry[s.ID()] = s }

// Get 按 ID 取 segment；未知 ID 返回 nil（配置向前兼容：忽略而非报错）。
func Get(id string) Segment { return registry[id] }

// IDs 返回全部已注册 segment ID（按字典序，供配置校验与 Web 配置页枚举）。
func IDs() []string {
	out := make([]string, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}
