// Package segment 定义 widget 抽象与注册表。
// 架构结论（拆解 02）：注册表模式替代硬编码 if 链，撑得住 16+ widget 扩展。
package segment

import (
	"ccpill/internal/config"
	"ccpill/internal/gitinfo"
	"ccpill/internal/input"
	"ccpill/internal/render"
	"ccpill/internal/theme"
	"ccpill/internal/usage"
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
}

// Usage 惰性加载跨会话用量聚合（内部带 60s 文件缓存）。
func (c *Context) Usage() usage.Summary {
	if !c.usageOnce {
		c.usageSum = usage.Load()
		c.usageOnce = true
	}
	return c.usageSum
}

// Git 惰性采集 git 信息（仅第一次调用真正跑 git 子进程）。
func (c *Context) Git() gitinfo.Info {
	if !c.gitOnce {
		dir := c.Status.Workspace.CurrentDir
		if dir == "" {
			dir = c.Status.CWD
		}
		c.git = gitinfo.Collect(dir)
		c.gitOnce = true
	}
	return c.git
}

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

// IDs 返回全部已注册 segment ID（供配置校验与 Web 配置页枚举）。
func IDs() []string {
	out := make([]string, 0, len(registry))
	for id := range registry {
		out = append(out, id)
	}
	return out
}
