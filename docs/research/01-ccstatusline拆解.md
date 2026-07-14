# ccstatusline 源码拆解报告

> 拆解对象：`J:\claude-statusline-tools\references\ccstatusline`（TypeScript，npm 包 `ccstatusline@2.2.23`，Bun 构建为单文件 `dist/ccstatusline.js`）
> 目的：为 Go 重写版 `ccpill` 提供完整情报，含 stdin schema、配置格式、渲染管线、安装逻辑、可抄设计与性能坑。
> 所有引用均为 `文件路径:行号`，基于本次实际 Read 验证，非训练记忆推测。

---

## 1. Claude Code 传给 statusline 的 stdin JSON 完整 Schema

Schema 定义于 `src/types/StatusJSON.ts:22-78`（Zod `z.looseObject`，未知字段会被保留但不校验，说明 Claude Code 会持续新增字段，ccstatusline 刻意向前兼容）。入口读取逻辑见 `src/ccstatusline.ts:296-318`：非 TTY 时读整个 stdin，`JSON.parse` 后用 `StatusJSONSchema.safeParse` 校验，失败则 `process.exit(1)` 并打印错误到 stderr。

### 顶层字段

| 字段 | 类型 | 说明 |
|---|---|---|
| `hook_event_name` | `string?` | 仅在 `--hook` 模式（技能/斜杠命令跟踪等）下出现，不是标准 statusline 调用字段 |
| `session_id` | `string?` | 会话 ID，用于技能调用统计文件名 `~/.cache/ccstatusline/skills/skills-<id>.jsonl`、remote-control 会话匹配 |
| `transcript_path` | `string?` | 当前会话 JSONL 转录文件绝对路径，是 token/速度/压缩/思考等级等 widget 的数据源 |
| `cwd` | `string?` | 当前工作目录，用于 git/jj widget 定位仓库 |
| `model` | `string \| {id?, display_name?}` | 字符串或对象两种历史形态都要兼容 |
| `workspace` | `{current_dir?, project_dir?}` | |
| `version` | `string?` | Claude Code 自身版本号 |
| `output_style` | `{name?: string}` | |
| `effort` | `{level?: string} \| null` | 思考强度等级（low/medium/high/xhigh/max），`null` 与"键不存在"语义不同（见 §2.5） |
| `cost.total_cost_usd` | `number?`（可从字符串强转） | 会话累计花费 USD，`SessionCostWidget` 直接读取，不做任何计算 |
| `cost.total_duration_ms` | `number?` | 用于 SessionClock 兜底（无需再解析 transcript 时间戳） |
| `cost.total_api_duration_ms` / `total_lines_added` / `total_lines_removed` | `number?` | |
| `context_window.context_window_size` | `number \| null` | 模型上下文窗口总大小（token） |
| `context_window.total_input_tokens` / `total_output_tokens` | `number \| null` | 会话累计 |
| `context_window.current_usage` | `number \| {input_tokens, output_tokens, cache_creation_input_tokens, cache_read_input_tokens} \| null` | **两种形态**：纯数字（总量）或分项对象 |
| `context_window.used_percentage` / `remaining_percentage` | `number \| null` | Claude Code **官方直接给出的上下文占用百分比**，优先级最高 |
| `vim.mode` | `string \| null` | |
| `worktree.{name,path,branch,original_cwd,original_branch}` | | git worktree 场景字段 |
| `rate_limits.five_hour` | `{used_percentage?, resets_at?}` | 5 小时账单窗口（`resets_at` 为 **Unix epoch 秒**） |
| `rate_limits.seven_day` / `seven_day_sonnet` / `seven_day_opus` | 同上，后两者可为 `null`（企业账户无该窗口） | |

**数值字段普遍用 `CoercedNumberSchema`**（`StatusJSON.ts:3-15`）：先 trim 字符串再 `Number()`，兼容 Claude Code 把数字序列化成字符串的情况——这是我们 Go 端反序列化时必须复刻的容错点（不能假设都是 JSON number）。

---

## 2. 各 Widget 数据来源与计算方法

Widget 接口定义于 `src/types/Widget.ts:37-50`：`render(item, context, settings) => string | null`。`RenderContext`（`src/types/RenderContext.ts:33-58`）把 stdin 原始 `data`、预解析的 `tokenMetrics`/`speedMetrics`/`usageData`/`compactionData`/`skillsMetrics` 一起传入——**所有 transcript 解析都在 `ccstatusline.ts` 里按需预取一次**，widget 本身只读缓存好的结果（除了 `ThinkingEffortWidget`，见下文的例外）。

### 2.1 Context 百分比（最核心指标）

优先级链（`src/utils/context-percentage.ts:19-39` + `src/utils/context-window.ts:50-123`）：

1. **优先用 stdin `context_window.used_percentage`**（Claude Code 官方计算好的值），直接 clamp 到 [0,100]。
2. 否则由 `context_window.current_usage`（对象形态）算：`contextLength = input + cache_creation + cache_read`（**故意不含 output_tokens**，因为 output 不占用下一轮的输入上下文）。
3. 否则退回到 `used_tokens / windowSize`。
4. 若 stdin 完全没给 `context_window`，才退回解析 `transcript_path`（见 §2.2 `tokenMetrics.contextLength`）。

分母 `windowSize` 的确定见 `src/utils/model-context.ts`：优先用 stdin 的 `context_window_size`；否则尝试从 `model.id`/`display_name` 字符串里正则提取形如 `(200k)`/`1M context` 的窗口大小提示；最后兜底 **200,000**（可用环境变量 `CCSTATUSLINE_CONTEXT_SIZE_FALLBACK` 覆盖）。"可用上下文"固定按 **80%**（`USABLE_CONTEXT_RATIO = 0.8`）计算，对应 `context-percentage-usable` widget。

### 2.2 Token 计数（`context-length` / `tokens-*` widget，`src/utils/jsonl-metrics.ts:151-233`）

当 stdin 没给 `context_window` 时才会读 transcript。逻辑要点：
- 整个文件一次性读入内存，按行 `JSON.parse`（`readJsonlLines`，`src/utils/jsonl-lines.ts:11-14`，无流式/增量解析）。
- Claude Code 流式写入同一次 API 调用的多条 JSONL（中间态 `stop_reason: null`，最终态有值），必须**去重**：若某行有 `stop_reason` 字段则认为该消息流式化，只统计"有终态 stop_reason 的行"+"最后一条未终态的行"，避免重复计数用量。
- `contextLength`（当前上下文占用）取自 **isSidechain !== true 且非 API 错误消息** 的、时间戳最新的一条主链消息的 `input_tokens + cache_read_input_tokens + cache_creation_input_tokens`（同样不含 output）。
- 子代理（Task 工具生成的 subagent）transcript 存放在 `<transcript_dir>/subagents/agent-<id>.jsonl` 或 `<transcript_dir>/<stem>/subagents/agent-<id>.jsonl`（`src/utils/jsonl-metrics.ts:442-491`），主 transcript 里递归搜索所有 `agentId` 字段来定位需要额外读取哪些子代理文件。

### 2.3 Token 速度（input/output/total speed，`src/utils/jsonl-metrics.ts:298-432`）

不是简单的"tokens/duration_ms"：
- 以 `user` 消息时间戳为区间起点，紧随其后的 `assistant` 消息（带 `usage`）时间戳为区间终点，构成一个"请求耗时区间"。
- 多个区间做**合并（merge overlapping intervals）**后再求和总时长，避免并行工具调用/子代理造成的时间重叠被重复计入。
- 支持"滑动窗口"（如最近 60s）：`windowSeconds` 传入后按 `assistantTimestampMs` 过滤请求，区间按窗口边界裁剪。
- `includeSubagents: true` 时把子代理 transcript 的区间也并入合并计算（子代理请求默认不排除 sidechain，主链请求默认排除 sidechain）。

### 2.4 Compaction 计数（`src/utils/compaction.ts:17-83`）

**不是靠猜测上下文突然下降来推断**，而是精确扫描 transcript 里 Claude Code 写入的 `{type:'system', subtype:'compact_boundary', isSidechain !== true}` 标记行，逐条累加：
- `compactMetadata.trigger` 分类到 `auto`/`manual`/`unknown`（缺失或未知值一律算 `unknown`，绝不猜测）。
- `tokensReclaimed` 累加每条标记的 `preTokens - postTokens`（仅当两者都是有限数字；旧版标记没有 `postTokens` 则贡献 0）。
- 无 transcript / 读取失败时返回全零 `ZERO_COMPACTION_STATS`，而不是报错。

### 2.5 思考强度等级（`src/widgets/ThinkingEffort.ts` + `src/utils/jsonl-metadata.ts`）

优先级链，且**这是唯一一个在 `render()` 内部临时读 transcript 而非提前预取的 widget**（`ThinkingEffort.ts:65-72` 直接调用 `getTranscriptThinkingEffort`），因此启用该 widget 时会产生额外一次全量 transcript 读取：
1. stdin `effort.level`（若 `effort` 键存在但 `level` 为 `null`，视为"显式知道当前无等级"，不再往下查；若整个 `effort` 键都不存在才继续查下一优先级——`level` 为 `null` 与键缺失语义不同，见 `ThinkingEffort.ts:18-25`）。
2. 从 transcript **倒序扫描**，找最近一条 `<local-command-stdout>Set effort level to X` 或 `<local-command-stdout>Set model to ... with X effort` 系统回显（正则匹配，`jsonl-metadata.ts:16-19`）。
3. `~/.claude/settings.json` 的 `effortLevel` 字段。
4. 都没有则显示 `default`。
未知等级字符串（不在 low/medium/high/xhigh/max 白名单）会显示为 `xxx?`（带问号），不是直接丢弃——诚实标注不确定性，值得抄。

### 2.6 成本（`src/widgets/SessionCost.ts`）

纯读 stdin `cost.total_cost_usd`，**不做任何计算**，`toFixed(2)`。字段缺失时返回 `null`（widget 不渲染），不会去解析 transcript 里的 token 单价——ccstatusline 完全信任 Claude Code 给的官方成本数字。

### 2.7 用量/限流（session-usage / weekly-usage / extra-usage-* / block-timer，`src/utils/usage-prefetch.ts`）

三层数据来源，按需短路，**这是最值得抄的设计之一**：
1. **优先**用 stdin `rate_limits`（`extractUsageDataFromRateLimits`，`usage-prefetch.ts:172-200`）：`five_hour.used_percentage` → `sessionUsage`，`resets_at`（epoch 秒）转 ISO 字符串。`seven_day_sonnet`/`seven_day_opus` 为 `null` 时视为 `0`（企业账户无该窗口，语义明确，不是"未知"）。
2. 若某些字段 stdin 没给（典型是 `extra_usage`——rate_limits 里完全不含超额用量数据），才发起网络请求 `GET https://api.anthropic.com/api/oauth/usage`（`src/utils/usage-fetch.ts:500-584`），OAuth token 来源：非 macOS 读 `~/.claude/.credentials.json`；macOS 走 Keychain（`security find-generic-password`/`security dump-keychain`，`usage-fetch.ts:342-408`，还处理多凭据候选按修改时间排序）。
3. 网络结果三级缓存：内存缓存（进程内，180s）→ 磁盘缓存 `~/.cache/ccstatusline/usage.json`（180s TTL，写入时带 token 指纹哈希 `sha256(token).slice(0,16)`，账号切换后自动失效）→ 加锁文件 `~/.cache/ccstatusline/usage.lock`（30s 内只允许发一次请求，429 时按 `Retry-After` 头延长锁定时间，避免多个并发 statusline 调用打爆 API）。

### 2.8 5 小时账单块（`block-timer`/`reset-timer`，`src/utils/jsonl-blocks.ts` + `jsonl-cache.ts`）

**stdin 完全不提供这个数据**，只能靠扫描 `~/.claude/projects/**/*.jsonl`（`globSync`，`jsonl-blocks.ts:44-48`）猜测当前 5 小时块的起点：按文件 mtime 倒序取时间戳，用"渐进式回看"（先看 10 小时，不够再看 20/48 小时）找最近一次 ≥5 小时的活动空隙作为块边界，再把边界向下取整到整点小时（`floorToHour`）。计算结果写入 `~/.cache/ccstatusline/block-cache-<sha256(configDir)前16位>.json`，下次调用先查缓存是否仍在有效期内（`getCachedBlockMetrics`，`jsonl-cache.ts:105-133`），避免每次都重新 glob+扫描全部项目文件。

### 2.9 Git / Jujutsu widget（`src/utils/git.ts`）

自行发现 `.git` 目录（支持 worktree 的 `.git` 文件里 `gitdir:` 重定向，`git.ts:79-116`），持久化缓存 `~/.cache/ccstatusline/git-cache/git-<sha256(gitDir)前16位>.json`，缓存 key 附带 `.git/HEAD` 和 index 文件的 mtime——mtime 未变则直接复用上次 `git status`/`git diff` 的输出，不重新 fork 子进程；TTL 由 `settings.gitCacheTtlSeconds`（默认 5 秒）控制。这是**避免每次渲染都 fork git 子进程**的关键优化，Go 版应等价实现。

---

## 3. 自身配置文件格式 `~/.config/ccstatusline/settings.json`

路径固定为 `path.join(os.homedir(), '.config', 'ccstatusline', 'settings.json')`（`src/utils/config.ts:29`），**在所有平台（含 Windows）都用这个路径**，没有区分 `%APPDATA%`——Windows 上实际落在 `C:\Users\<user>\.config\ccstatusline\settings.json`。可用 `--config <path>` 覆盖（`ccstatusline.ts:261-272`），会追加到装进 Claude `settings.json` 的 command 里（`claude-settings.ts:307-312`）。

### 3.1 顶层字段（`src/types/Settings.ts:45-88`，当前 `CURRENT_VERSION = 3`）

| 字段 | 类型 | 默认值 | 说明 |
|---|---|---|---|
| `version` | `number` | `3` | schema 版本，驱动迁移 |
| `lines` | `WidgetItem[][]` | 见下 | **最多 3 行**（数组硬编码 3 个空数组占位），每行是有序 widget 列表 |
| `flexMode` | `'full' \| 'full-minus-40' \| 'full-until-compact'` | `'full-minus-40'` | 决定 flex-separator 撑开到多宽（见 §4.3） |
| `compactThreshold` | `number` (1-99) | `60` | `full-until-compact` 模式下，上下文占用 ≥ 此百分比时切换到 `full-minus-40`（给 auto-compact 提示留空间） |
| `colorLevel` | `0\|1\|2\|3` | `2` | 0=无色 1=16色 2=256色 3=truecolor |
| `defaultSeparator` / `defaultPadding` | `string?` | — | |
| `inheritSeparatorColors` | `boolean` | `false` | 分隔符是否继承前一个 widget 的颜色 |
| `overrideBackgroundColor` / `overrideForegroundColor` | `string?` | — | 全局强制颜色，优先级高于单个 widget 颜色；支持 `gradient:` 前缀（见 §4.1） |
| `globalBold` | `boolean` | `false` | |
| `gitCacheTtlSeconds` | `number` (0-60) | `5` | |
| `minimalistMode` | `boolean` | `false` | 强制所有 widget 走 `rawValue` 模式（不带 label 前缀） |
| `powerline` | `PowerlineConfig` | 见 §3.3 | |
| `updatemessage` | `{message?, remaining?}` | — | 迁移后给用户看的一次性提示，每次渲染 `remaining--`，归零后自动删除该字段（`ccstatusline.ts:231-258`） |
| `installation` | `InstallationMetadata?` | — | 记录安装方式，见 §5 |

`WidgetItem`（`src/types/Widget.ts:7-26`）字段：`id`（GUID）、`type`（**字符串而非枚举**，故意允许未知类型透传以保证向前兼容）、`color`/`backgroundColor`/`bold`/`dim`（`boolean | 'parens'`，`'parens'` 表示只对 `(...)` 括号内文本做暗淡）、`character`（分隔符字符）、`rawValue`、`customText`/`customSymbol`/`commandPath`（自定义 widget 用）、`maxWidth`、`preserveColors`（自定义命令保留原始 ANSI）、`timeout`、`merge`（`boolean | 'no-padding'`，与下一个 widget 合并不加分隔符/不加内边距）、`hide`、`excludeFromAutoAlign`、`metadata`（`Record<string,string>`，各 widget 自定义子配置存这里，如 CompactionCounter 的 `format`/`metric`/`hideZero`）。

### 3.2 默认布局

`SettingsSchema.default()`（`Settings.ts:49-61`）：第一行 `model(cyan) | context-length(brightBlack) | git-branch(magenta) | git-changes(yellow)`，第二、三行为空。

### 3.3 Powerline 配置（`src/types/PowerlineConfig.ts`）

```
{ enabled: bool, separators: string[]（默认 ['']）, separatorInvertBackground: bool[],
  startCaps: string[], endCaps: string[], theme?: string, autoAlign: bool, continueThemeAcrossLines: bool }
```
内置主题（`src/utils/colors.ts:325-491`）：`custom`（用 widget 自身背景色）、`nord`、`nord-aurora`（默认主题 `getDefaultPowerlineTheme()` 返回 `'nord-aurora'`）、`monokai`、`solarized`、`minimal`、`dracula`、`catppuccin`、`gruvbox`、`onedark`、`tokyonight`。每个主题对 3 种 colorLevel（1/2/3）各存一套 `fg[]`/`bg[]` 颜色数组，按 widget 顺序循环取色。

### 3.4 版本迁移（`src/utils/migrations.ts`）

v1（无 `version` 字段）→v2：给每个 widget 补 GUID、若原来用 `defaultSeparator` 则从 lines 中剥离旧式 `separator` 类型项、附加一次性更新提示。v2→v3：仅打版本号+更新提示（引入 block-timer widget）。**迁移结果先用当前 schema 校验通过才落盘**，校验失败保留原文件不覆盖（`config.ts:154-228` 的"recovery contract"注释非常明确：读取/校验失败绝不覆盖用户文件，只在内存里退回默认值）。

### 3.5 全部 87 个内置 widget type（`src/utils/widget-manifest.ts:19-103`）

按类别：Core（model / output-style / thinking-effort / vim-mode / voice-status / remote-control-status）、Git（27 个，git-branch/git-changes/git-status/git-sha/git-origin-*/git-upstream-*/git-worktree-*/git-review 等）、Jujutsu（8 个 jj-*）、Context（context-length/context-window/context-percentage/context-percentage-usable/context-bar/compaction-counter）、Token（tokens-input/output/cached/total、cache-hit-rate、cache-read、cache-write）、Speed（input-speed/output-speed/total-speed）、Session（session-clock/session-cost/session-name/session-usage）、Usage（weekly-usage/weekly-sonnet-usage/weekly-opus-usage/extra-usage-*/reset-timer/weekly-reset-timer/block-timer）、其它（current-working-dir/terminal-width/version/custom-text/custom-symbol/custom-command/link/claude-session-id/claude-account-email/free-memory/skills）+ Layout 2 个（separator/flex-separator）。`git-pr` 是 `git-review` 的历史别名（`LEGACY_WIDGET_TYPE_ALIASES`，`widgets.ts:25`），加载配置时自动升级。

---

## 4. 渲染管线

### 4.1 ANSI 颜色系统（`src/utils/colors.ts` + `src/utils/ansi.ts`）

- 三档 `colorLevel`：`ansi16`（用 chalk 命名色如 `chalk.red`）、`ansi256`（**不是** 0-15 主题色，而是固定映射到 16-231 范围的调色板色号，例如 `red→160`，避免依赖终端主题）、`truecolor`（`chalk.hex('#cc0000')` 等固定 hex）。三档色值在 `COLOR_MAP`（`colors.ts:16-58`）里逐色硬编码，不是运行时换算。
- 颜色也接受两种"逃生舱"格式：`ansi256:<0-255>` 和 `hex:<RRGGBB>`，绕过命名色表直接指定。
- **渐变色**：`gradient:<preset名|RRGGBB,RRGGBB,...|RRGGBB-RRGGBB-...>`（`src/utils/gradient.ts`），13 个内置预设（复刻自 `gradient-string` 库，用 **OKLab** 色彩空间插值而非 RGB 线性插值或 HSV，色彩过渡更均匀；`rainbow`/`pastel` 因为原版是 HSV 环形插值，改用多段 hex 显式采样来逼近）。渐变可作用于整行文本（`applyLineGradient`）或 powerline 模式下单个 widget 内部按列位置着色（`applyLineGradientSegment`），且是在**截断之后**才应用（`renderer.ts:1240-1248` 有详细注释：截断会把结尾 reset 码切掉，渐变必须在那之后叠加，否则会有颜色溢出到行尾）。
- ANSI 转义序列解析自己实现了一套（不依赖第三方 strip-ansi 做核心逻辑），能正确处理 SGR (`\x1b[...m`)、OSC-8 超链接（`\x1b]8;;url\x07...\x1b]8;;\x07`，且**按 Unicode 字位簇（grapheme cluster）+ Emoji_Presentation/Extended_Pictographic Unicode 属性**计算可见宽度（`ansi.ts:64-201`），正确处理 ZWJ 组合 emoji、变体选择符、组合附加符号、regional indicator（国旗）、emoji 修饰符肤色——这一层复杂度是很多简易 statusline 工具会踩的坑（中日韩字符/emoji 宽度算错导致行错位截断）。
- 截断 `truncateStyledText`（`ansi.ts:407-486`）保留已打开的 OSC-8 链接会在截断处补上正确的 close 序列，不会截出一个"半开"的超链接转义。

### 4.2 Powerline 实现（`src/utils/renderer.ts:98-707` `renderPowerlineStatusLine`）

核心思路：先把每行 widget 过滤掉 `separator`/`flex-separator`，逐个渲染出内容+前景色+背景色，然后：
- **分隔符箭头颜色规则**：`separatorInvertBackground` 未开启时，箭头前景=前一个 widget 背景色（转前景色，`bgToFg`），箭头背景=下一个 widget 背景色；开启后互换。相邻背景色相同时用对方前景色代替箭头颜色（避免箭头"消失"在同色块里）。
- **start/end cap**（线段头尾封口字符）：按"渲染段"（被 flex-separator 分隔的连续 widget 组）计数，循环从 `startCaps`/`endCaps` 数组取值，颜色取该段第一个/最后一个 widget 的背景色。
- **flex-separator 撑开**：先在待渲染字符串里插入哨兵 `\x01FLEX_SEP\x01`（一个几乎不可能出现在真实文本里的 SOH 控制字符），整行渲染完后统一按 `terminalWidth - 内容总宽度` 均分空格数（余数依次分配给前面的 flex 槽位），再替换哨兵。
- **auto-align**（多行对齐）：预渲染全部行后，按"列位置"（用同一 merge 分组规则划分）计算每列在所有行里的最大宽度，回填 padding，使多行输出竖直对齐（类似表格），`excludeFromAutoAlign` 的 widget 及其后的同行 widget 跳过对齐。
- Powerline 与非 Powerline 是两套完全独立的渲染函数（`renderStatusLine` 内 `if (isPowerlineMode) return renderPowerlineStatusLine(...)`），非 Powerline 模式下 flex-separator 逻辑更简单（按段落 join，直接均分空格，无哨兵/无 cap）。

### 4.3 多行输出如何传给 Claude Code（`src/ccstatusline.ts:98-259`）

- 最多渲染 3 行（`settings.lines` 数组长度上限），每行独立调用 `renderStatusLine`，**空行（strip ANSI 后为空白）直接跳过不输出**（`ccstatusline.ts:198-201`），避免 Claude Code 把空白行也当成一行渲染出多余空隙。
- 每行输出前统一做两个 hack：① `line.replace(/ /g, ' ')`——所有普通空格替换成 **不换行空格 U+00A0**，防止 VSCode 集成终端把行首/行尾空格裁剪掉；② 行首强制加 `\x1b[0m`（完整 reset），覆盖 Claude Code 自身可能设置的 dim 样式。
- 多行之间通过多次 `console.log()` 换行分隔——**Claude Code 的 statusLine hook 就是读 stdout 的多行文本**，没有特殊分隔协议。
- 配置读取失败（`getConfigLoadError()` 非空）时，会在**第一条有内容的输出行前面**插入一个红色 `⚠ invalid config` 徽章（`buildConfigWarningBadge`），而不是静默降级，方便用户第一时间发现配置坏了。
- `updatemessage`（迁移后的一次性提示）在所有 statusline 行输出完后再单独 `console.log`，并做"倒计时递减+归零删除"的状态机（persists 在 settings.json 里，靠下次调用时的 `saveSettings` 更新）。

### 4.4 终端宽度探测（`src/utils/terminal.ts`）

**Windows 上直接返回 `null`（完全不支持宽度探测）**（`terminal.ts:50-54` 显式短路），这意味着 flex-separator 撑开、powerline 自动截断在 Windows 上默认失效，除非用户手动设置 `CCSTATUSLINE_WIDTH` 环境变量。这对我们在 Windows 环境开发 `ccpill` 是关键提示：**必须做得比 ccstatusline 好**（Windows 下用 `GetConsoleScreenBufferInfo` 之类的 Win32 API 或 `golang.org/x/term` 是可行的，不必像它一样放弃）。

Unix 上的探测链条（`terminal.ts:35-169`，见 §6 性能坑）：环境变量覆盖 → 向上遍历最多 8 层父进程找到拥有真实 TTY 的祖先（`ps -o ppid=`/`ps -o tty=` 各一次 `execSync`）→ 对该 TTY 设备跑 `stty -F/-f/size < ...`（最多 3 种变体依次尝试）→ 最终兜底 `tput cols`。

---

## 5. 如何写入/移除 Claude Code 的 `settings.json`

路径解析（`src/utils/claude-settings.ts:90-140`）：优先读环境变量 `CLAUDE_CONFIG_DIR`，否则 `~/.claude`；`settings.json` 就在该目录下（不区分平台，Windows 同样是 `~/.claude/settings.json`）。

### 5.1 安装（`installStatusLine`，`claude-settings.ts:400-439`）

1. 先备份现有 `~/.claude/settings.json` 到 `.orig` 后缀（`backupClaudeSettings('.orig')`），读取失败也继续（用空对象兜底），并在 stderr 提示备份路径。
2. 写入 `settings.statusLine = { type: 'command', command: <构造的命令>, padding: 0 }`。`padding: 0` 是**判断"是否已安装"的必要条件之一**（`isInstalled()`，`claude-settings.ts:216-231`：命令匹配已知模式 **且** `padding` 为 0 或未定义）——这是因为 Claude Code 默认给 statusLine 输出加左侧 padding，ccstatusline 自己在渲染时已经处理了间距，必须关掉 Claude Code 的默认 padding 避免双重缩进。
3. 若 Claude Code 版本 ≥ 2.1.97（`getClaudeCodeVersion()` 靠 `execSync('claude --version')` 探测），额外写 `statusLine.refreshInterval`（默认 10 秒，保留用户已设置的值）。
4. **三种命令模式**（`getBaseCommandForMode`，`claude-settings.ts:314-323`）：
   - `auto-npx` → `npx -y ccstatusline@latest`（每次调用都用 npx 解析 `@latest`）
   - `auto-bunx` → `bunx -y ccstatusline@latest`
   - `global` → `ccstatusline`（假设用户已 `npm i -g` 或 `bun add -g` 固定版本）
   - 若使用了 `--config <path>` 自定义配置路径，会拼接 `--config <quoted-path>` 到命令末尾（Windows 下按 cmd.exe 规则判断是否需要双引号转义，`needsQuoting`/`quotePathIfNeeded`，`claude-settings.ts:65-84`）。
5. 安装完成后调用 `syncWidgetHooks`（见 §5.3）把当前布局里用到的 widget 所需的 hook 一并注册进 `settings.json` 的 `hooks` 字段。

### 5.2 卸载（`uninstallStatusLine`，`claude-settings.ts:441-464`）

删除 `settings.statusLine` 整个字段、清空安装元数据（`installation`）、调用 `removeManagedHooks()` 清理所有打了 `_tag: 'ccstatusline-managed'` 标记的 hook 条目。**任何写回都会先备份 `.bak` 后缀**（`saveClaudeSettings` → `backupClaudeSettings()`，默认后缀 `.bak`，与安装时的 `.orig` 是两份不同备份）。

### 5.3 Hook 自动同步（`src/utils/hooks.ts`）

目前唯一用到 hook 的 widget 是 `skills`（技能调用统计，`--hook` 模式处理 `PreToolUse`(Skill 工具) 和 `UserPromptSubmit`(斜杠命令) 事件，写入 `~/.cache/ccstatusline/skills/skills-<session_id>.jsonl`，见 `src/utils/hook-handler.ts`）。同步逻辑：
- 每个托管 hook 条目打标记 `_tag: 'ccstatusline-managed'`，`syncWidgetHooks` 每次都先移除所有旧的托管条目（以及正则匹配 `ccstatusline.* --hook` 的"遗留未打标记"条目，兼容老版本升级），再按当前 `settings.lines` 里实际用到的 widget 重新生成一遍——**保证 hook 集合始终和当前布局同步，不会残留孤儿 hook**。
- hook 命令固定是 `<当前 statusLine 命令> --hook`，即和 statusline 渲染是**同一个可执行文件**，只是加了 `--hook` flag 走另一条分支（`ccstatusline.ts:289-293`），复用同一份安装/自动更新逻辑，不需要单独分发一个 hook 二进制。

---

## 6. 值得抄的设计

1. **stdin 优先、transcript 兜底、network 最后**的三级数据获取优先级（§2.7 用量、§2.1 上下文百分比、§2.5 思考等级都是这个模式）——本质是"信任 Claude Code 官方给的实时数据，只在缺失时才自己算/查"，既省性能又保证准确性。Go 版应把这个优先级链做成通用抽象，而不是每个 widget 各写一套。
2. **配置文件"读取失败绝不覆盖"契约**（`config.ts:154-163` 注释即"recovery contract"）：校验失败/JSON 损坏时返回内存态默认值渲染，但**原文件保持不动**，留给用户手动修复；只有全新安装（文件不存在）或迁移后**新格式先验证通过**才会落盘。这是对用户数据的强保护，比"读不出来就用默认值覆盖"安全得多。
3. **原子写入**（temp file + rename，`config.ts:113-134`，还处理了目标是 symlink 的情况——先 resolve 真实目标路径再在同目录写临时文件）。Go 版直接用 `os.CreateTemp` + `os.Rename` 复刻即可。
4. **用量 API 的锁文件 + token 指纹缓存**（§2.7）：30 秒级联请求锁防止多个并发 statusline 调用（多开终端/多个 Claude Code 实例）同时打爆 `api.anthropic.com`；429 时读 `Retry-After` 头动态延长退避时间；token 变化（换账号）通过哈希指纹立即使旧缓存失效，而不是等 TTL 过期。
5. **git/5小时块缓存都基于文件 mtime 判断是否需要重新计算**，而不是固定 TTL 轮询——只要 `.git/HEAD` 没变就直接复用缓存，比"每隔 N 秒重新查"更精确也更省。
6. **Compaction 计数靠精确扫描协议标记而非启发式推断**（§2.4）——上下文占用百分比会因为很多原因波动（用户手动清理、工具调用产生的 system 消息等），只有专门的 `compact_boundary` 系统事件才是压缩发生的可靠信号。
7. **未知 widget type / 未知 schema 字段透传**（`WidgetItemSchema.type: z.string()` 而非枚举，`StatusJSONSchema` 用 `looseObject`）：新版本 Claude Code 加字段、旧版 ccstatusline 配置文件仍可加载，不会因为一个新字段就整个解析失败——这是长期维护 CLI 工具对"上游持续加字段"的正确应对姿态，Go 版用 `map[string]any` 兜底未知字段也能做到。
8. **原子化的类型安全渐变色实现**（OKLab 插值而非 RGB/HSV），视觉效果明显更顺滑，色号/hex 表可以直接照抄（`GRADIENT_PRESETS`，13 组）。
9. **Unicode 宽度计算的正确性**（emoji/ZWJ/变体选择符/CJK 宽字符）是多数简易 clone 会踩的坑，`getVisibleWidth`/`truncateStyledText` 这套逻辑建议直接对齐（Go 生态对应可用 `github.com/mattn/go-runewidth` + 手写宽字符簇合并，或参考 `uniseg` 库做字位簇分割）。

---

## 7. 明显的坑 / 性能瓶颈（启动慢的根因）

1. **同一份 transcript 文件在单次调用里最多被完整读取+逐行 JSON.parse 4~5 遍**：`getTokenMetrics`（`jsonl-metrics.ts:151`）、`getCompactionStats`（`compaction.ts:73`，仅当有 compaction-counter widget）、`getSpeedMetricsCollection`（`jsonl-metrics.ts:493`，仅当有速度 widget）各自独立调用 `readJsonlLines` 完整读一遍文件；`ThinkingEffortWidget.render` 更过分——它**不走 `ccstatusline.ts` 里统一的预取阶段**，而是在渲染阶段同步调用 `getTranscriptThinkingEffort`（`ThinkingEffort.ts:44`），是第 4/5 次独立读取。长会话 transcript 可以到几十 MB，多 widget 组合布局时这是最大的单点性能浪费。**Go 版必须把"读一次 transcript、多个 pass 复用同一份已解析行数组"作为架构级约束**，而不是每个功能各自读文件。
2. **`readJsonlLines` 是一次性把整个文件读进内存再 `content.split('\n')`**（`jsonl-lines.ts:7-14`），没有流式/分块处理，也没有"只读最后 N 行"的快路径（哪怕 `getTranscriptThinkingEffort`、`getSessionDuration` 只需要文件末尾的少数行，也要整文件读完再倒序遍历数组）。Go 版可以对"只需要尾部数据"的场景做反向读取（从文件末尾按块回溯找换行符），大文件下收益明显。
3. **终端宽度探测在 Unix 上最坏情况要 fork 最多 8×2=16 次 `ps` 子进程 + 3 次 `stty` 尝试 + 1 次 `tput`**（`terminal.ts:59-93`，每层父进程走两次 `execSync`）。子进程 spawn 在类 Unix 上通常几毫秒到十几毫秒，但 Claude Code 的 statusline 在**每次渲染/每个 prompt**都会调用一次，这是稳定可复现的延迟来源，且没有跨调用缓存（每次都重新探测一遍父进程链）。Windows 上直接放弃（返回 `null`），此路径不适用但也说明"作者自己也认为这套探测不值得在 Windows 上做"。
4. **默认推荐/常见安装方式是 `npx -y ccstatusline@latest`**（auto-update 模式）：作者自己在 TUI 文案里承认"a small startup cost when the package runner checks or resolves the package"（`InstallMenu.tsx:44`），意味着**每次 statusline 渲染都要走一次 npx 的包解析流程**（校验本地缓存、必要时打 npm registry），这是比 transcript 重复读取更宏观的启动延迟来源，用户若选了默认的"自动更新"模式会持续付出这个代价。`ccpill` 作为 Go 单二进制天然没有这个问题，但如果要模拟"自动更新"体验，也不应该用类似"每次调用都联网检查版本"的模式，而应该是独立的后台/低频检查。
5. **5 小时账单块推断（`findMostRecentBlockStartTime`）在无缓存/缓存失效时要 `globSync` 扫描 `~/.claude/projects/**/*.jsonl`**（不限项目、不限当前会话），对每个候选文件都要整份读入解析时间戳，长期使用 Claude Code 的用户这个目录可能有成百上千个 jsonl 文件——虽然有 5 小时级别的缓存兜底，但**缓存首次冷启动或跨设备同步 configDir 后**这一步可能明显拖慢首次渲染。
6. **`getWidget`/`getChalkColor` 等热路径函数使用 `Array.find`/`Map.get` 在每个 widget 每次渲染都查一遍完整颜色表**（`colors.ts:114, 248`），量级很小（32 色）可以忽略，但如果 Go 版颜色表设计成 slice 线性查找而不是数组直接索引，多行多 widget 场景下会有轻微的可避免开销——建议用固定枚举+数组下标，不用查找。
7. **Windows 下 flex-separator / powerline 自动撑满宽度功能实质性缺失**（§4.4），仅靠用户手动设置环境变量补救，不是真正的自适应——这是一个"清晰的产品缺口"而非单纯性能问题，`ccpill` 若能在 Windows 原生做出可靠的终端宽度探测（Win32 Console API），本身就是相对 ccstatusline 的功能优势。
8. **TUI 交互模式使用 React + Ink**（`ink@6.2.0`、`react@19.2.7`，`package.json`），仅在无 stdin 管道（TTY 直接运行）时才走这条路径，不影响 statusline 渲染的热路径性能，但说明配置工具本身体积较大（bundle 里包含完整 React 运行时）——`ccpill` 若做交互式配置 TUI，可以更轻量（如 Go 的 `bubbletea`/`tview` 通常比 Ink+React 启动更快、产物更小）。

---

## 8. 对 ccpill（Go）的启示小结

- **stdin schema 复刻优先级最高**：完整照抄 §1 的字段与容错规则（数字/字符串双形态、`current_usage` 双形态、`effort.level` 的 `null` vs 缺失语义、`rate_limits.seven_day_*` 的 `null` vs 缺失语义）。用 Go 建议用自定义 `UnmarshalJSON` 或 `json.RawMessage` + 手工探测类型处理"字段可能是数字也可能是字符串/对象"的情况，而不是依赖强类型 struct 直接失败。
- **一次读取、多路复用 transcript**：设计一个 `TranscriptCache`/`TranscriptReader` 结构，在进程内对同一 `transcript_path` 只做一次 I/O + 逐行解析，所有 widget（token/speed/compaction/thinking-effort/session-duration）都从这个共享的已解析结果取数，直接消除坑 1。
- **"一键导入 ccstatusline 配置"** 需要做的字段映射：`~/.config/ccstatusline/settings.json` → ccpill 配置。核心可映射项：`lines`（widget 列表+颜色+分隔符逐项对应，`type` 字符串大部分可以 1:1 映射到 ccpill 自己的 widget 枚举，映射不到的记录为"未支持，已忽略"而不是报错中断）、`colorLevel`、`flexMode`/`compactThreshold`、`powerline.*`（含内置主题色表，§3.3 的 11 个主题可直接照抄色值）、`overrideForegroundColor`/`overrideBackgroundColor`（含 `gradient:` 语法）、`defaultSeparator`/`defaultPadding`/`inheritSeparatorColors`。**不必**照抄的：`installation`/`updatemessage`（ccstatusline 自身安装状态跟踪，与 ccpill 无关）。
- **git/usage/block 缓存策略直接复刻**（mtime 判断 + TTL + 文件锁防并发打爆网络请求），但用 Go 的文件锁（`flock`）替代 JS 的"锁文件+读写时间戳"手搓实现，更可靠。
- **性能基线目标**：单次调用应控制在个位数毫秒级（无网络请求路径下），关键是不要重复读同一文件、不要为宽度探测做多次子进程 spawn（Windows 下用 Win32 API，Unix 下用 `golang.org/x/term.GetSize` 一次系统调用即可，完全不需要 ccstatusline 那套"向上找父进程 TTY"的迂回方案——那是因为 Node.js 生态在管道场景下拿不到 `process.stdout` 的 TTY 信息，Go 程序若直接持有终端 fd 或用系统调用可以更直接地拿到宽度）。

---

*文档生成时间：基于 2026-07 时点 `references/ccstatusline` 快照（`package.json` version `2.2.23`）。所有结论均来自本次实际 Read 工具输出，未依赖训练记忆臆测。*

