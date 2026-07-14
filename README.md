# ccpill 💊

> A pill-styled, blazing-fast statusline for Claude Code. Written in Go.
> Claude Code 状态栏工具：胶囊视觉 · Go 原生性能 · 本地 Web 配置中心。

**状态：V0.1 开发中**（2026-07-15 立项）

## 设计签名

- 💊 **薄胶囊 segment**：每个 widget 一颗圆角药丸（可一键关闭背景退化为彩色文字）
- 🎨 **全套流行主题**：Catppuccin（默认）/ Tokyo Night / Nord / Dracula / Gruvbox …整套切换
- ⚡ **Go 单二进制**：无运行时依赖，缓存 TTL 架构，冷启动即热
- 🌐 **本地 Web 配置中心**：`ccpill --config` 拉起 localhost 页面，拖拽排 segment、实时真数据预览（全竞品独家）
- 📊 **16 widgets**：费用/burn rate/5h 窗口 · 上下文 · 模型/思考等级 · Git/PR · token 速度 · 系统资源等
- 🔔 **四类预警**：上下文阈值 / 日预算线 / 5h 窗口耗尽 / Git 未提交堆积
- 🔤 **三档图标集**：Nerd Font / Unicode 安全 / 纯 ASCII

## 文档

- 产品需求：`docs/PRD.md`
- 竞品拆解笔记：`docs/research/`

## 开发

```bash
go build -o ccpill.exe .
go test ./...
```

License: MIT（开源发布时正式添加）
