# ccpill 使用指南

> 从零到一个完全自定义的状态栏。所有操作都在 Web 配置中心可视化完成，不需要手改配置文件（想手改也行，文末有对照）。

## 0. 三步上岗

一键脚本（Releases 预编译优先，无则本机 Go 源码直装，装完自动写配置）：

```powershell
# Windows
irm https://raw.githubusercontent.com/cass-2003/ccpill/main/scripts/install.ps1 | iex
```

```bash
# macOS / Linux / Git Bash
curl -fsSL https://raw.githubusercontent.com/cass-2003/ccpill/main/scripts/install.sh | bash
```

或手动：

```bash
go build -o ccpill.exe .
./ccpill.exe --install     # 写入 Claude Code settings.json（自动备份）
./ccpill.exe --config      # 打开 Web 配置中心
```

重启 Claude Code（或开新会话），状态栏生效。之后所有调整都在配置中心里做，点「保存生效」即可，**无需重启 Claude Code**——下次状态栏刷新就是新样子。

## 1. 配置中心页面地图

从上到下六张卡片：

| 卡片 | 干什么 |
|------|--------|
| **实时预览** | 所见即所得，用你最近一次真实会话的数据渲染；右上角就是「保存生效」按钮（吸顶常驻，改完随手点） |
| **外观** | 主题 / 图标集 / 圆角端帽 / 胶囊背景开关 / 紧凑模式 |
| **布局编排** | 1-3 行，拖胶囊排序；**每颗胶囊右侧的小圆点 = 外观自定义入口** |
| **Segment 仓库** | 75 个没启用的 segment 按功能分组躺在这，拖进上面任意一行即启用；拖回来即停用 |
| **自定义** | text/cmd 内容 + **自定义插槽**（想加几个加几个） |
| **功能对照表** | 全部 segment 的中英说明 |

预览里的**虚线胶囊**是「已启用但当前条件不满足」的示例占位（比如没有合并冲突时 `gitconflicts` 不显示）——点预览下方的折叠条能看到每一颗为什么没显示。

## 2. 布局：拖拽就完事

- 仓库 → 行内：启用
- 行内 → 仓库：停用
- 行内拖动：排序（拖动过程中实时显示落点；拖到空白处松手 = 取消）
- 最多 3 行，空行自动回收

## 3. 逐 segment 外观自定义（色点面板）

**每颗胶囊右侧有个小圆点**，点它弹出外观面板，四项全开放：

| 项 | 效果 | 例子 |
|----|------|------|
| 前景色 | 文字颜色，任意 RGB | 把 `today` 调成醒目的橙色 `#ff8800` |
| 底色 | 这一颗胶囊的独立底色 | 把 `git` 底色改深红做出警示块；每颗不同色 = powerline 彩虹效果 |
| 前缀 | 三态：不动=默认；留空=去掉这颗的前缀；填字=改叫法 | `今日 $940` → 填 `Today ` 变 `Today $940`；留空变 `$940` |
| 加粗 | 这一颗文字加粗 | 把 `model` 加粗突出当前模型 |

- 有自定义的胶囊，色点会**发紫光**
- **右键色点 = 一键重置**回主题默认
- 红警状态（红底反色）不吃自定义——保证警告永远醒目可读
- 前缀覆盖比全局「紧凑模式」优先：可以全局留前缀、只去掉某一颗的，反之亦然

## 4. 自定义插槽：把任何东西放上状态栏

「自定义」卡片 →「＋添加插槽」→ 起名 + 选类型 + 填内容 + 挑颜色 → 胶囊出现在仓库区「自定义插槽」分组 → 拖进布局。

### 例 1：静态备注

> 名字 `备注`，类型「文本」，内容 `搬砖中`，颜色粉色

状态栏出现一颗粉色的 `搬砖中`。

### 例 2：公网 IP

> 名字 `ip`，类型「命令」，内容 `curl -s ifconfig.me`，颜色蓝色

命令取输出首行，1 秒超时 + 10 秒缓存——坏命令、断网都不会拖慢状态栏，只是这颗胶囊消失。

### 例 3：Node 版本

> 名字 `node`，类型「命令」，内容 `node --version`

### 例 4：待办数量（配合你自己的脚本）

> 名字 `todo`，类型「命令」，内容 `python C:/scripts/todo_count.py`

任何能在 1 秒内输出一行文字的命令都能上状态栏。

插槽和内置 segment 一样支持拖拽排序、色点面板（底色/加粗）；改名字会自动同步布局引用。

## 5. 外观全局项速查

| 控件 | 说明 |
|------|------|
| 主题 | 整套配色（catppuccin-mocha / tokyo-night / nord / dracula / gruvbox-dark） |
| 图标集 | `nerd`（需 Nerd Font）/ `unicode`（安全符号 ⚡⎇，**推荐**）/ `ascii`（纯文本） |
| 胶囊端帽 | `圆角` 需字体含 Powerline 半圆字形（Windows Terminal 默认的 Cascadia Mono 就有）；显示成方块就换 `平角` |
| 胶囊背景 | 关闭后变成彩色文字 + │ 分隔的轻量样式 |
| 紧凑模式 | 全局去掉文字前缀（`ctx 62%` → `62%`），信息密度党专用 |

## 6. 手改 config.toml 对照

配置文件：`~/.claude/ccpill/config.toml`。Web 端能做的它都能做：

```toml
theme = "catppuccin-mocha"
icon_set = "unicode"
caps = "round"
minimal = false

lines = [
  ["model", "context", "today", "block", "slot:ip"],
  ["dir", "gitbranch", "gitstatus", "gitdiff", "clock"],
]

# 逐 segment 外观覆盖
[overrides.today]
color = "#ff8800"  # 前景色
bg = "#11111b"     # 单颗底色
label = "Today "   # 前缀：删掉此行=默认；"" = 去前缀；其他 = 替换
bold = true

# 自定义插槽（布局里用 slot:<name> 引用）
[[slots]]
name = "ip"
command = "curl -s ifconfig.me"
color = "#89b4fa"

[[slots]]
name = "备注"
text = "搬砖中"
color = "#f5c2e7"
```

## 7. 常见问题

- **胶囊两端是方块/问号** → 字体没有 Powerline 半圆字形，端帽换「平角」，或给终端换 Nerd Font / Cascadia Mono
- **图标显示成 ◆** → 字体缺 Font Awesome 字形，图标集换 `unicode`
- **某个 segment 一直不显示** → 看预览下方折叠条里的原因说明（多数是条件不满足：不在 git 仓库、没有 stash、API 没凭据等）
- **weeklysonnet / weeklyopus / overage 不显示** → 这三个走 claude.ai OAuth 用量接口，走第三方中转渠道时没有官方凭据，拿不到数据属正常
- **改了没生效** → 点了「保存生效」吗？预览只是试穿，保存才落盘

## 8. 从 ccstatusline 迁移

```bash
ccpill --import-ccstatusline            # 默认读 ~/.config/ccstatusline/settings.json
ccpill --import-ccstatusline <path>     # 或指定路径
```

- 布局逐行映射（87 个 widget type 对照表内置），最多 3 行
- `custom-text` / `custom-command` 自动转为 ccpill 插槽（`ccs-*`）
- widget 的 hex 颜色与加粗转为逐 segment overrides（命名色 / gradient 不迁移）
- 不支持项（separator、jj-*、voice 等）逐条列出原因，不中断迁移
- 覆盖前自动备份原 config.toml 到 `.bak-before-import`
- 主题 / 分隔符体系不迁移（两家视觉形态不同），迁移后用 `ccpill --config` 微调
