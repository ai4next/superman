[English](./README.md) | [简体中文](./README-zh.md)
# Superman

![Logo](assets/banner.png)

通用自治 AI Agent。支持多模型、6 个内建工具、扁平文件记忆、专家委托、MCP Server 集成、持久会话管理，以及 Bubble Tea v2 TUI。

## 设计哲学

- **路由胜于膨胀。** “一个 Agent 统治一切”是幻想。专家是窄领域子 Agent，主 Agent 负责路由。

- **后台胜于阻塞。** 归档、分析、优化都不该让用户等待。

- **简单胜于复杂。** 难题拆成小步。长上下文失败往往不是因为太短，而是因为太吵。清晰永远更重要。

---

## 快速开始

```bash
# 复制并编辑配置
cp config.example.yaml config.yaml

# 设置 API Key
export OPENAI_API_KEY=sk-...

# 启动 TUI
go run .

# 或单次执行
go run . run "这个目录里有什么？"
```

## 功能特性

- **多模型支持** — Gemini (Vertex AI)、OpenAI、DeepSeek、Claude、Ollama，以及任何兼容 OpenAI 的 API
- **6 个内建工具** — 代码执行、文件读写/补丁、用户交互、专家委托
- **MCP Server 集成** — 通过配置接入任意 MCP 兼容工具服务（stdin/stdout transport）
- **持久会话** — SQLite-backed session/message store，配套精简 `U/A/T/O` 进化日志，支持自动压缩、文件 revision tracking、session 导入导出
- **运行时审计** — 工具调用、文本增量、错误、进化等事件流式写入可查询 JSONL audit log
- **扁平文件记忆** — 运行时索引、全局事实、SOP 文件，以及精简会话日志
- **专家委托** — 将任务分派给拥有独立记忆的专家子 Agent
- **插件系统** — 统一 run/model/tool 日志与会话回收
- **TUI 界面** — Bubble Tea v2 + Lipgloss v2，暗色主题、Emacs 风格键绑定、侧边栏、Dialog 系统
- **Hook 系统** — 11 种生命周期事件钩子（run/tool/model 等前后），通过 JSON stdin/stdout 协议执行外部脚本
- **Skill 系统** — 基于文件系统的技能自动加载（ADK skilltoolset），兼容 Claude Code `SKILL.md` 格式，支持多个 skill path

## 命令

| 命令 | 说明 |
|------|------|
| `sm` | 启动交互式 TUI 聊天 |
| `sm run "提示词"` | 单次执行并打印响应 |
| `sm run -f prompt.txt` | 从文件读取提示词执行 |
| `sm run -p "hello"` | 使用 `--prompt` flag 执行 |
| `sm reflect` | 启动自主空闲监听 + 调度模式 |
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

## 配置

完整配置见 `config.example.yaml`。关键配置如下：

```yaml
workspace: ${HOME}/.sm

model:
  provider: openai          # gemini | openai | deepseek | claude | ollama
  name: gpt-4o
  base_url: https://api.openai.com/v1
  api_key: ${OPENAI_API_KEY}

server:
  addr: 127.0.0.1:8080

tools:
  code_run:
    enabled: true
    timeout: 30s
    allowed_languages: [python, bash, sh]
  read:
    enabled: true
    max_size: 10485760       # 10MB
  write:
    enabled: true
    max_size: 10485760       # 10MB
  patch:
    enabled: true
  ask_user:
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

环境变量可以覆盖配置：`SUPERMAN_MODEL_PROVIDER=openai`、`SUPERMAN_MODEL_API_KEY=sk-...` 等。

## 工具列表

| 工具 | 说明 |
|------|------|
| `code_run` | 执行 Python/Shell 代码 |
| `read` | 读取文件行 |
| `write` | 写入文件 |
| `patch` | 替换文件中的一个精确文本匹配 |
| `ask_user` | 中断执行并向用户提问 |
| `delegate_to_expert` | 将任务委托给专家独立执行 |

## Hooks & Skills

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

## 项目结构

```
superman/
├── main.go                          # 入口
├── internal/
│   ├── agent/
│   │   ├── agent.go                 # Agent 工厂，注入 memory/SOP/context
│   │   ├── context.go               # Agent run 上下文构建器
│   │   ├── prompt/system.txt        # 系统提示词
│   │   └── toolsets.go              # Skill + MCP toolset 构建
│   ├── config/                      # YAML + 环境变量配置 (viper)
│   ├── cli/                         # Cobra CLI 命令 (run, reflect, configure, toolsets, sessions, runtime)
│   ├── tui/                         # Bubble Tea v2 TUI
│   │   ├── tui.go                   # 兼容 wrapper
│   │   ├── app/                     # Model、runtime、sessions、commands、dialogs、layout
│   │   ├── components/              # Chat、input、toolbar、sidebar renderers
│   │   └── styles/                  # Dark theme、icons、color themes
│   ├── model/                       # 多 Provider LLM 工厂
│   ├── memory/                      # 扁平文件记忆：全局事实与 SOP
│   ├── session/                     # 持久 SessionService：SQLite + compact log、file tracking、references
│   ├── store/                       # GORM/SQLite 持久化模型与读写
│   ├── runtime/                     # 事件驱动 runtime 与 audit logging
│   ├── plugin/                      # 插件注册中心 + 内建插件
│   ├── hook/                        # Hook 管理器 + 脚本执行器
│   ├── reflect/                     # 自主空闲监听 + 调度器
│   └── expert/                      # 专家注册中心与 Spec 定义
├── hooks/                            # Hook 脚本目录（约定式，11 个事件子目录）
├── skills/                           # Skill 定义目录（ADK skilltoolset）
├── config.example.yaml
├── config/tasks/                    # 调度任务定义示例
├── go.mod
└── go.sum
```

## 运行时目录

所有运行时数据都存储在 `workspace`（默认为 `~/.sm/`），首次启动时自动创建：

```
~/.sm/                                    # workspace（默认: $HOME/.sm）
├── config.yaml                           # 用户配置（由 `sm configure` 创建）
├── tui.log                               # TUI runtime 日志（重定向，避免干扰界面）
├── state.db                              # SQLite session/message 元数据与完整消息
├── sessions/                             # 精简会话日志与 snapshots
│   ├── <id>.log                          # LLM evolution projection
│   └── snapshots/                        # 文件 revision snapshots
├── runtime/
│   └── events.jsonl                      # runtime audit event log
├── hooks/                                # Hook 事件脚本（11 种生命周期事件）
├── skills/                               # Skill 定义（SKILL.md）
├── memory/                               # superman 扁平文件记忆
│   ├── l1.toml                           # L1 全局事实
│   ├── l2/                               # L2 SOP 文件（*.md）
└── experts/
    └── {expert_name}/
        ├── calls.jsonl                   # 专家 consult/delegate 调用日志
        └── memory/                       # 专家独立记忆
```

## 构建

```bash
go build -o sm .
./sm --help
```

需要 Go 1.26+。

## 许可证

MIT
