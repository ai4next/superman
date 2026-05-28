[English](./README.md) | [简体中文](./README-zh.md)
# Superman

![Logo](assets/banner.png)

自进化自治 AI Agent。Superman 负责在工具和专家子 Agent 之间路由任务，持久化会话与扁平文件记忆，并通过分层 agent/meta evolution 将完成的工作沉淀为长期知识。

## 💡 设计哲学

- **群体胜于单体。** 巨型 prompt 不是智能，只是披着产品名的熵。Superman 通过工具和领域专家协同工作，而不是假装一个上下文窗口能容纳所有能力。

- **后台胜于阻塞。** 用户不该为系统打扫卫生付出延迟。归档、记忆沉淀、专家培养、元进化都应该在 run 之后发生，系统变好，但工作不中断。

- **进化胜于堆积。** 日志不是记忆。大多数历史如果没有被压缩成事实、流程或更锋利的 Agent，就只是噪声。Agent evolution 改进 Superman 和专家；meta evolution 只改进进化过程。系统可以学习，但必须有墙。

- **边界胜于感觉。** 没有写入边界的自我改进只是自动漂移。Superman 记忆、专家记忆、专家 soul、evolver 记忆、sessions、audit logs 必须分开，因为所有权边界决定学习会变成能力，还是变成污染。

- **清晰胜于混沌。** 长上下文失败常常不是因为不够长，而是因为被污染。系统选择小文件、显式 scope、持久会话和可检查 diff，因为可靠的自治需要文件系统，而不是神秘感。

---

## 🚀 快速开始

在 Linux 或 macOS 上安装最新发布版：

```bash
curl -fsSL https://raw.githubusercontent.com/ai4next/superman/main/install.sh | sh
```

Windows PowerShell：

```powershell
iwr https://raw.githubusercontent.com/ai4next/superman/main/install.ps1 -useb | iex
```

```bash
# 创建并编辑配置
sm init

# 设置 API Key
export OPENAI_API_KEY=sk-...

# 启动终端界面
sm

# 或单次执行
sm run "这个目录里有什么？"
```

也可以指定版本或安装到用户目录：

```bash
VERSION=v0.0.1 INSTALL_DIR="$HOME/.local/bin" sh -c "$(curl -fsSL https://raw.githubusercontent.com/ai4next/superman/main/install.sh)"
```

## ✨ 功能特性

- **多模型支持** — Gemini (Vertex AI)、OpenAI、DeepSeek、Claude、Ollama，以及任何兼容 OpenAI 的 API
- **6 个内建工具** — 自动适配操作系统的命令执行、文件读写/补丁、用户交互、专家委托
- **MCP Server 集成** — 通过配置接入任意 MCP 兼容工具服务（stdin/stdout transport）
- **即时通信软件接入** — 以常驻 server 方式接入 Telegram、飞书/Lark、企业微信、微信个人号、QQ、钉钉、Slack、Discord、LINE、微博等平台
- **持久会话** — SQLite-backed session/message store，配套精简 `U/A/T/O` 进化日志，支持自动压缩、文件 revision tracking、session 导入导出
- **运行时审计** — 工具调用、文本增量、错误、进化等事件流式写入可查询 JSONL audit log
- **扁平文件记忆** — 全局事实（L1）和 SOP 文件（L2）直接存储在 workspace 中
- **专家委托** — 将任务分派给拥有独立记忆和持久会话的专家子 Agent
- **分层自进化** — agent evolver 从完成会话中改进 Superman/专家；meta evolver 只从 evolver 会话中改进进化过程本身
- **插件系统** — 统一 run/model/tool 日志与会话回收
- **终端界面** — 暗色主题、Emacs 风格键绑定、侧边栏、Dialog 系统
- **Hook 系统** — 11 种生命周期事件钩子（run/tool/model 等前后），通过 JSON stdin/stdout 协议执行外部脚本
- **Skill 系统** — 基于文件系统的技能自动加载（ADK skilltoolset），兼容 Claude Code `SKILL.md` 格式，支持多个 skill path

## ⌨️ 命令

| 命令 | 说明 |
|------|------|
| `sm` | 启动交互式终端聊天 |
| `sm run "提示词"` | 单次执行并打印响应 |
| `sm run -f prompt.txt` | 从文件读取提示词执行 |
| `sm run -p "hello"` | 使用 `--prompt` flag 执行 |
| `sm reflect` | 启动自主空闲监听 + 调度模式 |
| `sm im serve` | 启动即时通信接入 server |
| `sm init` | 从嵌入的示例模板创建 `config.yaml` |
| `sm configure` | 查看或初始化配置 |
| `sm toolsets` | 列出已配置的 ADK Skill 和 MCP toolsets |
| `sm sessions list` | 列出持久会话 |
| `sm sessions show <id>` | 查看会话消息 |
| `sm sessions last` | 查看最近更新的会话 |
| `sm sessions search <query>` | 搜索持久会话消息 |
| `sm sessions files <id>` | 查看会话工作文件 |
| `sm sessions history <id>` | 查看会话文件 revision 历史 |
| `sm sessions diff <id> <path>` | 查看文件 revision diff |
| `sm sessions revert <id> <path>` | 将文件回滚到上一 revision |
| `sm sessions export <id>` | 导出会话（markdown/json/jsonl） |
| `sm sessions import <path>` | 导入会话导出文件 |
| `sm sessions compact <id>` | 将较旧会话上下文压缩为 summary |
| `sm sessions delete <id>` | 删除持久会话 |
| `sm sessions rename <id> <title>` | 重命名会话 |
| `sm sessions queue <id>` | 查看会话中排队的 prompts |
| `sm sessions storage` | 查看持久会话存储统计 |
| `sm sessions storage gc` | 删除孤立的文件 revision snapshots |
| `sm runtime events` | 列出 runtime audit events |
| `sm runtime summary` | 汇总 runtime audit events |

## ⚙️ 配置

完整配置见 `config.example.yaml`。关键配置如下：

```yaml
workspace: ${HOME}/.sm

model:
  provider: openai          # gemini | openai | deepseek | claude | ollama
  name: gpt-4o
  base_url: https://api.openai.com/v1
  api_key: ${OPENAI_API_KEY}
  headers:
    X-Request-Source: superman

server:
  addr: 127.0.0.1:8080

tools:
  exec:
    enabled: true
    timeout: 30s
  read:
    enabled: true
    max_size: 10485760       # 10MB
  write:
    enabled: true
    max_size: 10485760       # 10MB
  patch:
    enabled: true
  ask:
    enabled: true

# Memory 索引限制
memory:
  l1:
    max_index_items: 50
    max_sections: 100
  l2:
    max_index_items: 50

# Skill 系统，支持多个路径
skills:
  enabled: true
  paths:
    - ${HOME}/.sm/skills
    - ./skills

# MCP Server 集成
mcp:
  servers:
    - name: my-server
      enabled: true
      command: npx
      args: [-y, @modelcontextprotocol/server-filesystem, /tmp]
      tools: []                 # 空列表 = 全部工具；也可指定工具名过滤

# 会话管理
session:
  app_name: superman
  max_turns: 75
  archive_interval: 6h
  session_ttl: 48h
  loop_detection:
    enabled: true
    window_size: 10
    max_repeats: 5

# 自主反思
reflect:
  autonomous:
    idle_timeout: 30m
  scheduler:
    tasks_dir: ./config/tasks

# 专家培养/委托
expert:
  enabled: true
  max_count: 10

plugins:
  - name: session_reaper
    enabled: true
```

`model.headers` 是可选配置，会随每次模型请求一起发送，适合接入需要自定义请求头的 OpenAI-compatible 网关。

环境变量可以覆盖配置：`SUPERMAN_MODEL_PROVIDER=openai`、`SUPERMAN_MODEL_API_KEY=sk-...` 等。

### 即时通信接入

在 `im.platforms` 中启用一个或多个平台，配置对应凭据，然后运行：

```bash
sm im serve --config config.yaml
```

`im serve` 是一个常驻 server 进程。可以交给你习惯的进程管理器托管，也可以用 shell 后台方式运行：

```bash
nohup sm im serve --config config.yaml > ~/.sm/runtime/im.log 2>&1 &
```

示例：

```yaml
im:
  platforms:
    - name: feishu
      enabled: true
      options:
        app_id: ${FEISHU_APP_ID}
        app_secret: ${FEISHU_APP_SECRET}
        domain: feishu
        allow_from: ""

    - name: qq
      enabled: false
      options:
        ws_url: ws://127.0.0.1:3001
        token: ${QQ_ONEBOT_TOKEN}
        allow_from: ""
```

Telegram、飞书/Lark、企业微信、微信个人号、QQ、QQ 官方机器人、钉钉、Slack、Discord、LINE、微博等完整示例见 `config.example.yaml`。

微信个人号通常需要先扫码接入：

```bash
sm im weixin setup
```

该命令会在终端打印二维码，等待手机确认登录，打印 token 和账号信息后退出。把打印出的值填到 `im.platforms` 中的 `weixin` 配置后，再运行 `sm im serve`。

## 🛠️ 工具列表

| 工具 | 说明 |
|------|------|
| `exec` | 根据当前操作系统使用 bash、sh 或 PowerShell 执行 shell 命令 |
| `read` | 读取文件行 |
| `write` | 写入文件 |
| `patch` | 替换文件中的一个精确文本匹配 |
| `ask` | 中断执行并向用户提问 |
| `delegate` | 将任务委托给专家独立执行 |

`delegate` 会在每次调用模型前动态判断，只有启用专家委托且至少存在一个专家时才会加载。专家存储在 `experts/{expert_name}` 目录下，目录名就是专家名，`soul.md` 是专家的系统提示词。

## 🔌 Hooks & Skills

### Hooks

将可执行脚本放入 `hooks/<event>/` 目录。脚本通过 stdin 接收 JSON 上下文，并通过 stdout 返回 JSON。

```
hooks/
├── before_run/          # Agent run 前
├── after_run/           # Agent run 后
├── before_tool/         # 工具执行前
├── after_tool/          # 工具执行后
├── before_model/        # LLM 调用前
├── after_model/         # LLM 调用后
├── before_agent/        # Agent 执行前
├── after_agent/         # Agent 执行后
├── on_user_message/     # 用户消息
├── on_model_error/      # 模型错误
└── on_tool_error/       # 工具错误
```

示例脚本（`hooks/before_tool/audit.sh`）：

```bash
#!/bin/sh
# stdin: {"event":"before_tool","tool_name":"write","tool_args":{...}}
echo '{"allow": true}'
# 返回 {"allow": false, "reason": "..."} 可以阻止工具执行
```

### Skills

在 `skills/` 或任意已配置的 skill path 下创建技能目录。每个技能是一个带 YAML frontmatter 的 `SKILL.md` 文件。

```
skills/
└── code-review/
    ├── SKILL.md           # 必需：YAML frontmatter + Markdown 指令
    └── references/        # 可选：参考资料
```

示例（`skills/code-review/SKILL.md`）：

```markdown
---
name: code-review
description: Professional code review for PRs and changes
allowed-tools: [read, patch]
---

你是一个代码审查专家。重点关注：
1. 安全性 —— OWASP Top 10、注入漏洞
2. 正确性 —— 逻辑错误、边界条件
3. 可维护性 —— 命名、职责分离
```

### MCP Servers

Superman 支持通过 stdin/stdout transport 接入任意 MCP-compatible server。在 `config.yaml` 中配置：

```yaml
mcp:
  servers:
    - name: filesystem
      command: npx
      args: [-y, @modelcontextprotocol/server-filesystem, /tmp]
    - name: github
      command: npx
      args: [-y, @modelcontextprotocol/server-github]
      tools: [issues, pulls]
```

使用 `sm toolsets` 可以确认当前配置的 servers 及其可用工具。

## 📁 项目结构

```
superman/
├── main.go                          # 入口
├── internal/
│   ├── agent/
│   │   ├── agent.go                 # Agent 工厂，注入 memory/SOP/context
│   │   ├── context.go               # Agent run 上下文构建器
│   │   ├── tool.go                  # 动态内建工具/toolset 组装
│   │   └── evolver.go               # Agent/meta 进化 Agent 工厂
│   ├── prompt/                      # 内嵌提示词模板
│   │   └── template/                # Markdown 提示词模板
│   ├── config/                      # YAML + 环境变量配置 (viper)，内嵌 config.example.yaml
│   ├── cli/                         # Cobra CLI 命令 (init, run, reflect, im, configure, toolsets, sessions, runtime)
│   ├── tui/                         # Terminal UI
│   │   ├── tui.go                   # 兼容 wrapper
│   │   ├── app/                     # Model、runtime、sessions、commands、dialogs、layout
│   │   ├── components/              # Chat、input、toolbar、sidebar renderers
│   │   └── styles/                  # Dark theme、icons、color themes
│   ├── model/                       # 多 Provider LLM 工厂
│   ├── memory/                      # 扁平文件记忆：全局事实与 SOP
│   ├── session/                     # 持久 SessionService：SQLite + compact log、file tracking、references
│   ├── store/                       # GORM/SQLite 持久化模型与读写
│   ├── runtime/                     # 事件驱动 runtime 与 audit logging
│   ├── im/                          # 即时通信接入
│   ├── plugin/                      # 插件注册中心 + 内建插件
│   ├── hook/                        # Hook 管理器 + 脚本执行器
│   ├── reflect/                     # 自主空闲监听 + 调度器
│   └── expert/                      # 基于目录的专家注册中心（`soul.md`）
├── hooks/                            # Hook 脚本目录（约定式，11 个事件子目录）
├── skills/                           # Skill 定义目录（ADK skilltoolset）
├── config.example.yaml              # 指向 internal/config/config.example.yaml 的符号链接
├── config/tasks/                    # 调度任务定义示例
├── go.mod
└── go.sum
```

## 📂 运行时目录

所有运行时数据都存储在 `workspace`（默认为 `~/.sm/`），首次启动时自动创建：

```
~/.sm/                                    # workspace（默认: $HOME/.sm）
├── config.yaml                           # 用户配置（由 `sm init` 或 `sm configure` 创建）
├── tui.log                               # 终端界面 runtime 日志（重定向，避免干扰界面）
├── state.db                              # SQLite session/message 元数据与完整消息
├── sessions/                             # 精简会话日志与 snapshots
│   ├── <id>.log                          # LLM evolution projection
│   └── snapshots/                        # 文件 revision snapshots
├── runtime/
│   └── events.jsonl                      # runtime audit event log
├── evolution/                            # Agent evolver + meta evolver 运行时根目录
│   ├── memory/                           # Evolver 自己的扁平文件记忆
│   │   ├── l1.toml
│   │   └── l2/
│   ├── state.db                          # Evolver SQLite session/message 元数据
│   └── sessions/
│       ├── agent-evolution-<...>.log     # Agent 进化会话日志
│       └── meta-evolution-<...>.log      # Meta 进化会话日志
├── hooks/                                # Hook 事件脚本（11 种生命周期事件）
├── skills/                               # Skill 定义（SKILL.md）
├── memory/                               # superman 扁平文件记忆
│   ├── l1.toml                           # L1 全局事实
│   ├── l2/                               # L2 SOP 文件（*.md）
└── experts/
    └── {expert_name}/
        ├── soul.md                       # 专家系统提示词；目录名就是专家名
        ├── memory/                       # 专家独立记忆
        ├── state.db                      # 专家 SQLite session/message 元数据
        └── sessions/
            └── <id>.log                  # 专家精简会话日志
```

## 🏗️ 构建

从 GitHub Releases 安装：

```bash
# macOS/Linux
curl -fsSL https://raw.githubusercontent.com/ai4next/superman/main/install.sh | sh

# Windows PowerShell
iwr https://raw.githubusercontent.com/ai4next/superman/main/install.ps1 -useb | iex
```

从源码构建：

```bash
go build -o sm .
./sm --help
```

需要 Go 1.26+。

## 📄 许可证

MIT

---

## 📈 Star 历史

<div align="center">

<a href="https://star-history.com/#ai4next/superman&Date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=ai4next/superman&type=Date&theme=dark" />
    <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=ai4next/superman&type=Date" />
    <img alt="Star History Chart" src="https://api.star-history.com/svg?repos=ai4next/superman&type=Date" />
  </picture>
</a>

<br/><br/>
</div>
