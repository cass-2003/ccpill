# ccpill — Claude Code Statusline 产品需求文档 (PRD)

> 版本 v1.1 | 2026-07-15 | 来源：九轮角色化需求访谈 | 项目名：**ccpill**（npm 404 已验证可注册）

## 1. 定位与演进

- **定位**：先自用后开源。V1 服务 Q 本人工作流，代码质量按开源标准写，跑顺后发布。
- **差异化卖点**：本地 Web 配置中心（全竞品都没有）+ Go 原生性能 + 信息密度全家桶。
- **对标痛点**（对 ccstatusline）：启动/刷新慢、信息不够、样式不精致、配置麻烦——四项全解。

## 2. 核心技术决策

| 决策点 | 结论 |
|--------|------|
| 语言 | Go（单二进制、启动快、交叉编译方便） |
| 平台 | Windows 优先体验，代码跨平台设计，开源时补 mac/Linux |
| 数据刷新 | 本地缓存文件 + TTL：每次调用先读缓存，过期才重算/重请求并回写；无常驻进程 |
| 数据源架构 | adapter 层抽象，V1 只接 Claude Code，预留 Codex/Gemini CLI 接入 |
| 密钥 | 环境变量优先，配置文件只存引用，不落盘明文 |
| 质量 | 核心逻辑（transcript 解析/费用计算/渲染引擎）单测；不定性能硬指标但用 hyperfine 建基线 |
| 分发 | GitHub Releases 三平台二进制 + npm 包装二进制（学 CCometixLine） |
| 仓库 | `J:\claude-statusline-tools\<name>`（独立 git 仓库） |

## 3. 功能清单

### 3.1 Widget 全量（16 项，V0.2 完成）

**费用类**：会话成本 ·  今日总花费（跨会话聚合）· burn rate（$/h）· 5h 计费窗口剩余（含预计耗尽时间）
**上下文类**：context 百分比（图形化）· compact 预警
**会话类**：模型名+思考等级 · token 生成速度（tokens/s）· 会话时长 · compaction 计数 · 输出风格/Vim 模式
**Git 类**：分支 · 脏文件数+ahead/behind · PR/MR 检测（gh CLI）· worktree 标识 · 目录显示（可缩写）
**系统类**：时钟 · 本机 CPU/内存 · Claude API 健康状态 · MCP 服务器状态

**明确不做**：new-api 渠道余额查询。

### 3.2 图形化表达（四种形式全部实现，按 widget 可选）

1. 渐变色进度条（绿→黄→红），2. sparkline 迷你走势图（▁▃▅▇），3. 圆环/块百分比字符（◔◑◕●），4. 语义色数字（阈值变色）。

### 3.3 预警行为（四项全做，阈值可配）

1. 上下文占用超 80%/90% 变色提醒；2. 日花费预算线（超线变红）；3. 5h 窗口按 burn rate 预计提前耗尽时预警；4. Git 未提交堆积提醒（脏文件超阈值/长时间未 commit）。

### 3.4 布局与主题

- 1-3 行可配布局，默认布局从 3 套设计稿中选定。
- 全套流行主题：catppuccin / tokyo-night / nord / dracula / gruvbox / one-dark / solarized 等（对标 claudia-statusline 11 套）。
- 三档图标集一键切换：Nerd Font 满血版 / Unicode 安全版 / 纯 ASCII 版。

### 3.5 Web 配置中心（差异化核心，V0.1 就要有）

- `<binary> --config` 拉起 localhost 页面（仅监听 127.0.0.1）。
- 能力：拖拽排序 segment、开关数据源、调色盘改色、主题切换/编辑、1-3 行布局编排、阈值/预算设置。
- 实时预览用**真实会话数据**渲染（读 ~/.claude transcript），改完一键保存生效。
- 界面**双语 i18n**（中/英切换）。

### 3.6 迁移与安装

- 一键导入 ccstatusline 配置：读其 settings 自动换算。
- 安装时自动写 Claude Code `settings.json` 的 statusLine 配置。

## 4. 版本规划

| 版本 | 范围 | 验收 |
|------|------|------|
| **V0.1** | 渲染引擎 + 四件套 widget（费用/上下文/模型/Git）+ 主题系统 + 三档图标集 + **Web 配置中心基础版**（排序/开关/主题/预览） | 替换掉本机 ccstatusline 上岗 |
| **V0.2** | 全量 16 widget + 四类图形化 + 四项预警 + 缓存 TTL 层完善 | 全家桶稳定运行一周 |
| **V0.3** | ccstatusline 一键迁移 + i18n 完善 + CI 三平台构建 + npm 包装 + 双语 README | 开源发布就绪 |

## 5. 已定项（2026-07-15 设计评审）

1. **项目名：ccpill**——cc 前缀承接 Claude Code 生态命名（ccusage/ccstatusline），pill 即胶囊视觉签名。
2. **默认气质：稿 C 薄胶囊**（Catppuccin 色盘）；胶囊背景可配置开关（关闭即彩色文字+细分隔）；配色整套可切换；稿 A（Tokyo Night 克制风）进主题库；稿 B 赛博风淘汰。
3. Web 配置页前端：Go embed 静态页 + 原生 JS，零 Node 构建依赖。
4. 仓库：`J:\claude-statusline-tools\ccpill`；竞品参考克隆在 `../references/`。

## 6. 访谈决策溯源

九轮访谈原始答案见本目录访谈记录；关键取舍：不做 new-api 余额（Q 主动砍）、不定性能硬指标（信任 Go 天然性能）、Web 配置优先于全量 widget（差异化先行）。
