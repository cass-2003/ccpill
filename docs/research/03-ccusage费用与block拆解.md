# ccusage 费用计算与 5h Block 源码拆解

> 学习对象：`J:\claude-statusline-tools\references\ccusage`（v20.0.17）
> 写给 ccpill（Go 状态栏工具）的竞品源码笔记 | 2026-07-15

## 0. 重要前提：v20 已是 Rust 重写

ccusage 原为 TypeScript 项目，但当前仓库 **核心逻辑已全部重写为 Rust**（workspace 在 `rust/crates/ccusage`）。npm 包 `ccusage` 只剩一个薄壳启动器 `apps/ccusage/src/cli.js`，通过 optionalDependencies 分发 6 个平台的原生二进制（`packages/ccusage-{darwin,linux,win32}-{x64,arm64}`）。Rust 版是 TS 版的 1:1 移植——测试名直接叫 `rejects_null_schema_fields_like_typescript_loader`（`rust/crates/ccusage/src/adapter/claude/mod.rs:737`）。本笔记全部引用 Rust 源码，算法语义与经典 TS 版一致。

对 ccpill 的直接含义：**竞品也从脚本语言转向了原生二进制**，理由与我们选 Go 相同——statusline 每次渲染都是冷启动进程，启动开销是第一性能指标。

---

## 1. transcript JSONL 费用计算全流程

### 1.1 读取的字段

反序列化结构在 `rust/crates/ccusage/src/types.rs:9-57`：

```
UsageEntry {
  sessionId, timestamp, version, requestId, costUSD,
  isApiErrorMessage, isSidechain,
  message: { id, model, usage: {
    input_tokens, output_tokens,
    cache_creation_input_tokens, cache_read_input_tokens,
    speed,                       // "standard"|"fast"（fast 模式计价乘数）
    cache_creation: {            // 可选细分，优先于扁平字段
      ephemeral_5m_input_tokens,
      ephemeral_1h_input_tokens
    }
  }}
}
```

关键点：`cache_creation_token_count()`（types.rs:42-48）——如果存在 `cache_creation` 细分对象，用 5m+1h 之和；否则回退到扁平的 `cache_creation_input_tokens`。**1h 缓存写入按 2× input 价计费**（`cost.rs:7` 的 `CACHE_CREATE_1H_INPUT_MULTIPLIER = 2.0`），5m 缓存写入按 `cache_create` 价（默认 input×1.25）。

### 1.2 逐行解析的性能预筛（adapter/claude/mod.rs:333-447）

每个 JSONL 文件的处理流程（`read_usage_file`）：

1. `fs::read` 整文件读入，按字节切行（`byte_lines`，不做 UTF-8 校验）
2. **memmem 预筛**：行内不含子串 `"usage":{` 直接跳过（mod.rs:351-354）——绝大多数 transcript 行（user 消息、tool result、progress）根本不进 JSON 解析器
3. `has_unsupported_null_field`（mod.rs:553-598）：纯字节扫描找 `:null`，若字段名命中 schema 黑名单（`id`/`model`/`costUSD`/`sessionId`/`requestId`/`cache_read_input_tokens` 等 11 个）整行丢弃——复刻 TS 版 zod schema 的 nullable 拒绝语义，但避免了先解析再校验
4. serde 解析失败的行静默跳过（容错：transcript 可能有半行写入）
5. `is_valid_usage_entry`（mod.rs:512-551）：version 必须是 semver 前缀；sessionId/requestId/message.id/model 若存在则不得为空串
6. `model == "<synthetic>"` 的条目 model 置 None（不参与计价/模型列表）；`speed: fast` 的条目模型名加 `-fast` 后缀展示

多文件并行：按 CPU 核数开线程，文件**按大小贪心装箱**分配到各线程（LPT 调度，mod.rs:137-165），避免一个大文件拖尾。

### 1.3 条目去重（经典 message.id + requestId 问题）

去重发生在**所有文件加载完之后、聚合之前**（`push_deduped_entry`，mod.rs:240-291）：

- **去重键**：`(message.id, requestId)` 二元组。用 FxHasher 对二者哈希（mod.rs:293-298），哈希桶里存索引列表，**命中后还要回查真实字符串**防哈希碰撞（mod.rs:250-254）
- **message.id 缺失的条目永不去重**（mod.rs:245 `dedupe_lookup` 为 None 直接 push）——注意与 TS 老版本一致：无法构造键就保守保留
- **sidechain 重放问题**（v20 新增，mod.rs:255-268）：subagent（sidechain）的日志会**用新的 requestId 重放父会话的 assistant 消息**，导致精确键 `(id, reqId)` 不同但实为同一条消息。所以有二级查找：仅按 `message.id` 匹配，且要求候选或已存条目至少一方 `isSidechain == true`
- **保留哪条**（`should_replace_deduped_entry`，mod.rs:224-238）优先级：
  1. 非 sidechain 条目 > sidechain 条目（父会话原始记录为准）
  2. token 总量大的 > 小的
  3. 带 speed 字段的 > 不带的

### 1.4 costUSD 直接用 vs 按定价重算（cost.rs:23-37）

三种 CostMode（CLI `--mode`）：

| 模式 | 行为 |
|---|---|
| `display` | 只用条目里的 `costUSD`，缺失记 0，**完全不加载定价表** |
| `calculate` | 无视 `costUSD`，一律按 token × 单价重算 |
| `auto`（默认） | **有 `costUSD` 直接用；没有才重算**（`cost_usd.unwrap_or_else(calc)`） |

计价公式（`calculate_cost_from_pricing`，cost.rs:103-171）分 5 桶：

```
cost = input×p.input + output×p.output
     + cache_5m×p.cache_create            // 默认 input×1.25
     + cache_1h×(p.input×2.0)             // 1h 缓存 = 2× input
     + cache_read×p.cache_read            // 默认 input×0.1
```

每桶再套长上下文分层（cost.rs:123-170）：
- **LiteLLM `*_above_200k_tokens` 字段**：边际计价——超过 200K 的部分按高价（`tiered_cost`，cost.rs:173-183）
- **OpenAI 两段式**（`long_context_threshold`，如 272K）：整请求切换——input 超阈值则**全部桶整体**按长上下文价（不是边际）。这是 v20 修的一个语义坑，Anthropic 与 OpenAI 分层语义不同

`speed: fast` 条目整条成本再乘 `fast_multiplier`（cost.rs:95-100，来自内嵌 `fast-multiplier-overrides.json`）。

模型定价缺失时成本记 0，但会记录 `missing_pricing_model`（cost.rs:39-66）用于报表尾部警告——**静默记 0 但显式告警**，值得抄。

### 1.5 两个容易漏的数据源

- **advisor 消息**（mod.rs:404-444, 479-502）：一行 usage 里可能带 `usage.iterations[]`，其中 `type == "advisor_message"` 的迭代是**另一个模型**（顾问模型）的用量，会被拆成独立条目，message.id 加后缀 `:advisor:{i}` 参与去重，按顾问模型自己的价计费，`costUSD` 置 None 强制重算
- **agent progress 嵌套**（daily.rs:141-183 `DailyUsageLine::AgentProgress`）：subagent 的用量以 `{"data":{"message":{...}}}` 嵌套形态出现在父 transcript 里，daily 快速路径用 serde untagged enum 兼容两种形态

---

## 2. 模型定价数据来源与离线兜底

四层数据源，优先级从高到低（`pricing.rs:227-277` `load_embedded` / `load_with_overrides`）：

| 层 | 来源 | 时机 |
|---|---|---|
| 1. 用户 override | 配置文件 `pricingOverrides` | 最后应用，最终裁决权 |
| 2. LiteLLM 在线 | `raw.githubusercontent.com/BerriAI/litellm/main/model_prices_and_context_window.json` | 仅非 offline 模式，ureq+rustls，10s 超时，64MB 上限（pricing.rs:18-22, 1463-1484） |
| 3. **内嵌 LiteLLM 快照** | **构建期下载并打包进二进制**（`build.rs`：从 flake.lock 锁定的 LiteLLM 版本拉 json，`compact_pricing_json` 压缩字段名为 `i/o/cc/cr/ia/oa/cca/cra/ctx/fast` 后 `include_str!` 嵌入） | 永远可用，离线兜底主力 |
| 4. 内嵌 models.dev 快照 + 在线 models.dev | `src/models-dev-pricing.json`（嵌入）；在线 `models.dev/api.json`（失败后 60s 冷却重试，pricing.rs:23, 138-178） | 只补 LiteLLM 没有的模型（如新发布的 Anthropic 模型），单位是 $/1M 需 ÷1e6（pricing.rs:365-366） |

另有硬编码内置表 `put_builtin_pricing`（pricing.rs:667+，claude-opus-4-5/4-6/4-7/4-8 等 $5/$25 每 M）和内置 OpenAI 长上下文分层价 `builtin_long_context_rates`（pricing.rs:1323-1346，上游不发布，手工维护，注明来源 URL）。

缺省推导：LiteLLM 条目缺 `cache_creation_input_token_cost` 时取 `input×1.25`，缺 `cache_read` 取 `input×0.1`（pricing.rs:315-318）——正好是 Anthropic 官方比例。

**模糊匹配**（`find`，pricing.rs:399-473）：精确命中 → 别名解析 → 归一化（`.`/`@`→`-`）后子串匹配（要求边界是非字母数字、且不吞版本号数字，`pricing_key_matches` pricing.rs:1243-1298；8 位日期后缀 `-20250514` 视为同模型，其他数字后缀视为不同版本）→ 取最长 key。**查询结果含 None 一起缓存**在 Mutex<FxHashMap>（pricing.rs:404-448），几千条目只有几十个 unique 模型名，避免重复模糊匹配。

statusline 默认 `--offline`（types.rs:169），即纯内嵌快照、零网络——状态栏不能等 10s 超时。

---

## 3. 5 小时 billing block 算法

核心在 `blocks.rs:17-128`（`identify_session_blocks`），默认 `DEFAULT_SESSION_DURATION_HOURS = 5.0`（main.rs:65）。

### 3.1 伪代码

```
identify_session_blocks(entries, dur = 5h):
    sort entries by timestamp asc
    blocks = []; cur_start = None; cur_entries = []
    for e in entries:
        if cur_start != None:
            last = cur_entries.last().timestamp
            if e.ts - cur_start > dur OR e.ts - last > dur:
                blocks.push(make_block(cur_start, cur_entries))
                if e.ts - last > dur:                      # 空闲期插 gap 块
                    blocks.push(gap_block(start=last+dur, end=e.ts))
                cur_start = floor_to_utc_hour(e.ts)        # 新块锚点
                cur_entries = []
        else:
            cur_start = floor_to_utc_hour(e.ts)            # 首条消息锚定
        cur_entries.push(e)
    if cur_entries: blocks.push(make_block(cur_start, cur_entries))

make_block(start, entries):
    end        = start + dur                               # 名义结束 = 起点+5h
    actual_end = entries.last().timestamp
    is_active  = (now - actual_end < dur) AND (now < end)  # 双条件
    cost/token_counts/models = 累加 entries
```

### 3.2 关键设计点

- **块起点 = 该块首条消息时间 floor 到整点**，且是 **UTC epoch 整点**（`TimestampMs::floor_to_hour`，date_utils.rs:58-60，`ms div 3600_000 × 3600_000`）。对整点偏移时区等价于本地整点；对 +5:30 这类半点时区会偏 30 分钟——已知近似
- **换块的两个触发条件**（blocks.rs:39）：距块起点超 5h，**或**距上一条消息超 5h（长时间空闲即使没满 5h 也换新块）。后者与 Anthropic 实际计费窗口并不完全一致，是 ccusage 自己的建模选择
- **gap 块**：空闲超 5h 时插入 `is_gap` 块（start = 上条消息+5h，end = 下条消息），报表里显示 "(Xh gap)"，成本为 0
- **活跃判定**（blocks.rs:81）：`now - 最后一条消息 < 5h && now < 块名义终点`——两者缺一不可，防止旧块因 now 越界仍被判活跃
- **剩余时间**：`remaining_minutes = (end_time - now) / 60_000`（blocks.rs:225, 463），格式化为 `Xh Ym left`（`format_remaining_time`，blocks.rs:578-586）
- 块 id 直接用起点的 RFC3339 字符串（blocks.rs:99）

### 3.3 projection（预测块终值，blocks.rs:554-569）

```
projected_tokens = 当前累计 + burn_rate.tokens_per_minute × remaining_minutes
projected_cost   = 当前成本 + (cost_per_hour/60) × remaining_minutes
```

---

## 4. burn rate 计算（blocks.rs:535-552）

**窗口 = 当前活跃 block 内的全部条目**（不是最近 N 分钟滑窗）：

```
duration_min = (block 内最后一条.ts - 第一条.ts) / 60_000    # ≤0 返回 None
tokens_per_minute               = 全部 token 总量 / duration_min
tokens_per_minute_for_indicator = (input + output) / duration_min   # 不含 cache!
cost_per_hour                   = block.cost_usd / duration_min × 60
```

注意区分两个 tpm：**状态栏的 🟢/⚠️/🚨 指示灯用不含 cache 的 tpm**（cache read 动辄百万级会把指示灯打爆），阈值 <2000 Normal / <5000 Moderate / ≥5000 High（commands/mod.rs:467-473）；而 projection 用含 cache 的全量 tpm。美元口径只有 `cost_per_hour` 一个。

对 ccpill 的启示：burn rate 分母是**块内首末条目间隔**而非"块已过时长"——刚开工 2 分钟就有准确读数，不会被块起点 floor 稀释。

---

## 5. 「今日」聚合的时区处理

- 每个条目的 `date` 字段在**加载时**就按时区格式化好（mod.rs:369 `format_date_tz`）：显式 `--timezone`（IANA 名，jiff 解析）缺省时用 **`JiffTimeZone::system()` 本机时区**（date_utils.rs:227-238）
- statusline 的今日过滤（commands/mod.rs:547-561 `statusline_today_shared`）：用 `args.timezone`（没配则本机时区）把 `now` 格式化成 `YYYYMMDD`，设 `since = until = today`，然后**加载全部条目后在内存里按 `entry.date == today` 过滤求和**（commands/mod.rs:446-456）——不是按文件 mtime 预剪枝，是全量扫
- `--since/--until` 是字符串比较（`"20260715" >= since`），条目 date 去掉 `-` 后直接比（mod.rs:126-135）
- timestamp 解析（date_utils.rs:111-145）：手写 RFC3339 解析器（支持 `Z` 和 `±HH:MM` 偏移），不走通用日期库，为了热路径性能；日历运算用 Howard Hinnant 的 days_from_civil 算法（date_utils.rs:194-217）

坑：**跨天瞬间**状态栏 today 数字会跳零，这是语义正确的；但 block 与 today 用的时区可以不同（block 起点是 UTC 整点 floor，today 是本地日），两个数字不构成包含关系。

---

## 6. statusline 子命令（commands/mod.rs:318-545）

### 6.1 stdin 输入（Claude Code status line hook JSON）

```json
{
  "session_id": "...",
  "transcript_path": "/path/to/current.jsonl",
  "model": { "id": "claude-fable-5", "display_name": "Fable 5" },
  "cost": { "total_cost_usd": 0.25 },              // CC 自报，可缺
  "context_window": { "total_input_tokens": 25000,
                      "context_window_size": 200000 },  // 可缺
  "effort": { "level": "high" }                    // 可缺
}
```

### 6.2 stdout 输出（单行）

```
🤖 Fable 5 (high) | 💰 $0.23 session / $1.23 today / $0.45 block (2h 45m left) | 🔥 $0.12/hr 🟢 | 🧠 25,000 (12%)
```

各段来源（`render_statusline`，commands/mod.rs:418-545）：

- **session 成本**：`--cost-source` 四模式（cc/ccusage/auto/both）。默认 auto：优先 hook 的 `cost.total_cost_usd`，缺失才自算（按 session_id 过滤全量条目求和）。`both` 双显对照
- **today**：见第 5 节
- **block + 剩余时间**：全量加载 → `identify_session_blocks(5h)` → 找第一个 `is_active && !is_gap` 的块
- **burn rate**：活跃块的 `cost_per_hour`；`--visual-burn-rate emoji|text|emoji-text` 附加指示灯
- **context**：优先 hook 的 `context_window`；缺失则**从 transcript 文件倒序找最后一条 assistant 行**（commands/mod.rs:603-662），context = input + cache_creation + cache_read；窗口上限查定价表 `context_limit(model_id)`，查不到用 200_000。百分比按 50%/80% 阈值绿黄红
- 出错时打印上次缓存输出或 `❌ Error generating status`——**statusline 永远输出一行**，不让状态栏空白

### 6.3 性能优化：会话级缓存 + 进程活性锁（commands/mod.rs:664-809）

缓存文件 `{temp_dir}/ccusage-semaphore/{session_id}.lock`，JSON 字段：`lastOutput / lastUpdateTime / transcriptPath / transcriptMtime / isUpdating / pid`。

判定逻辑（`cached_statusline_output`，commands/mod.rs:716-735）：

```
新鲜 = (now - lastUpdateTime < refresh_interval×1000)    # 默认 1s
     AND (transcript 当前 mtime == 缓存的 mtime)
新鲜        → 直接打印缓存，进程秒退（零解析）
过期/文件变 → 若 isUpdating 且 pid 仍存活 → 打印旧输出（stale-while-revalidate 防踩踏）
            → 否则自己算：先写 isUpdating=true+自身pid 占锁，算完写回结果
```

要点：**没有增量解析**——缓存失效就全量重扫所有 JSONL。快靠三件事：Rust 冷启动、memmem 行预筛、多线程按文件大小装箱。一次渲染最多 3 次全量加载（session 成本自算时 + today + blocks），依赖 1s 缓存摊薄。

Windows 坑：`process_is_alive` 非 Unix 实现是 `pid == 自身pid`（commands/mod.rs:745-748），即**无法探测他进程存活**，Windows 上并发防踩踏基本退化。

---

## 7. 数据目录查找顺序（adapter/claude/paths.rs:13-53）

```
1. CLAUDE_CONFIG_DIR 环境变量（支持逗号分隔多路径）
   - 每个路径须含 projects/ 子目录才算有效（支持 ~ 展开；
     若路径本身以 projects 结尾则取其父目录）
   - 设了但一个都无效 → 直接报错（不静默回退！）
2. 未设环境变量 → 同时收集两个目录（都存在则都扫，非二选一）：
   a. $XDG_CONFIG_HOME/claude（XDG 未设则 ~/.config/claude）
   b. ~/.claude
```

home 解析（home.rs:3-36）：`HOME` → `USERPROFILE` → `HOMEDRIVE+HOMEPATH`（Windows 兜底链）。

文件发现：`{dir}/projects/**/*.jsonl` 递归（paths.rs:79-99），路径排序保证确定性。session 归属从路径推断（paths.rs:136-177）：`projects/<proj>/<session>.jsonl` 或嵌套 `projects/<proj>/<session>/*.jsonl`，**`subagents/` 子目录的文件归属父 session**——ccpill 若按目录聚合 session 成本必须处理这一层，否则 subagent 用量丢失。

---

## 8. 值得借鉴的设计与已知问题

### 8.1 值得抄进 ccpill

1. **memmem 预筛 + 延迟 JSON 解析**：`"usage":{` 子串预筛淘汰 90%+ 行。Go 对应 `bytes.Contains`，配合 bufio 按行扫，不必上 simdjson
2. **去重三层语义**：`(message.id, requestId)` 精确键 → 无 id 不去重 → sidechain 重放按 message.id 二级匹配、非 sidechain 优先。ccpill 至少要做第一层，subagents 目录要做第三层
3. **auto 成本模式**：`costUSD` 有就信、没有才重算——兼顾老 transcript（有 costUSD）与新版本（多数无此字段）
4. **statusline 缓存协议**：mtime+间隔双失效、isUpdating+pid 防踩踏、失败回放上次输出。Go 里 pid 探活可用 `OpenProcess`/`FindProcess+Signal(0)`，能比 ccusage 的 Windows 退化实现做得更好
5. **定价四层兜底 + 构建期嵌入快照**：Go 用 `go:embed` 嵌入 LiteLLM 快照即可，statusline 默认 offline
6. **burn rate 分母用首末条目间隔**、指示灯 tpm 剔除 cache token——两个容易做错的口径
7. missing pricing 记 0 但收集告警名单，不静默吞

### 8.2 已知问题 / 局限（ccpill 可差异化处）

1. **block 边界是近似**：floor 到 UTC 整点 + "空闲 5h 即换块" 是对 Anthropic 计费窗口的猜测建模，官方从未公开精确算法；半点时区有 30 分钟偏差。README/docs 也只称其为估算
2. **无增量解析**：缓存过期即全量重扫；transcript 巨大（长期用户 GB 级 projects 目录）时每秒一次全扫仍有感。ccpill 机会点：按文件 mtime 跳过未变文件 + 持久化 per-file 聚合缓存
3. 一次 statusline 渲染最多 3 次独立全量 load（session/today/blocks 不共享一次加载结果）——结构上可一次加载三处复用，ccusage 没做
4. Windows 进程活性检测退化（见 6.3），并发 statusline 调用可能重复计算
5. `costUSD` 在 auto 模式无条件信任，若历史条目是错价写入无法纠正（需手动 `--mode calculate`）
6. 全量条目常驻内存（`Vec<LoadedEntry>` 含完整 UsageEntry clone），Arc<str> 只优化了 project/session 字符串；超大历史内存占用可观

### 8.3 与 ccpill 需求的映射

| ccpill 需求 | ccusage 对应实现 | 引用 |
|---|---|---|
| 会话成本 | hook cost 优先，缺则按 session_id 过滤求和 | commands/mod.rs:423-432, 563-572 |
| 今日总花费 | 条目 date（本地时区）== 今日，内存过滤求和 | commands/mod.rs:446-456, 547-561 |
| burn rate $/h | 活跃块 cost / 首末间隔分钟 × 60 | blocks.rs:535-552 |
| 5h 窗口剩余 | 活跃块 end_time - now | blocks.rs:463; 17-128 |

收工。

