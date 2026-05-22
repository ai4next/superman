[English](./README.md) | [简体中文](./README-zh.md)
# Superman

![Logo](assets/banner.png)

通用自治 AI Agent。多模型支持、13 个内建工具、分层记忆、专家智库。

## 设计哲学

1. **文件胜于数据库。** JSONL 在可移植性、可调试性和成本上完胜任何向量存储。复杂就是负债。

2. **注入胜于 RAG。** Prompt 比任何检索管线都快、都省、都可靠。大多数 RAG 只是在给糟糕的 Prompt 设计擦屁股。

3. **路由胜于膨胀。** "一个 Agent 统治一切"是妄想。专家是窄领域的子 Agent，主 Agent 只负责路由。

4. **后台胜于阻塞。** 归档、分析、优化——都不该让用户等待。

5. **简单胜于复杂。** 难的问题拆成小步走。长 Context 失败不是因为太短，而是因为太吵。清晰永远获胜。

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

- **多模型支持** — Gemini (Vertex AI)、OpenAI、DeepSeek、Claude、Ollama 及任何兼容 OpenAI 的 API
- **13 个内建工具** — 代码执行、文件读写/编辑、网页抓取、浏览器操作、用户交互、工作笔记、长期记忆、记忆检索、专家查询/创建/委托
- **分层记忆 (L0-L4)** — SOP 规则、记忆索引、持久存储、会话归档、历史会话压缩
- **专家智库** — 子 Agent 调度，生命周期管理（草稿 → 活跃 → 成熟 → 归档），从使用模式自动萃取
- **插件系统** — 记忆同步、Token 追踪、工具日志、会话回收
- **TUI 界面** — Bubble Tea + Lipgloss，暗色主题，Emacs 风格键绑定
- **自主模式** — 空闲触发反思 + 定时任务执行
- **模式分析** — 从重复工具链中自动生成专家草稿
- **Hook 系统** — 11 种生命周期事件钩子（run/tool/model 等前后），通过 JSON stdin/stdout 协议执行外部脚本
- **Skill 系统** — 基于文件系统的技能自动加载（ADK skilltoolset），兼容 Claude Code SKILL.md 格式

## 命令

| 命令 | 说明 |
|------|------|
| `sm` | 启动交互式 TUI 聊天 |
| `sm run "提示词"` | 单次执行并打印响应 |
| `sm run -f prompt.txt` | 从文件读取提示词执行 |
| `sm run -p "hello"` | 使用 `--prompt` 标志 |
| `sm reflect` | 启动自主空闲监听 + 调度模式 |
| `sm configure` | 查看或初始化配置 |

## 配置

详见 `config.example.yaml`。关键配置：

```yaml
model:
  provider: openai          # gemini | openai | deepseek | claude | ollama
  name: gpt-4o
  base_url: https://api.openai.com/v1
  api_key: ${OPENAI_API_KEY}

tools:
  code_run:
    enabled: true
    timeout: 30s
    workspace: ./workspace
  web_scan:
    enabled: true
  # ... 每个工具可单独启用/禁用

plugins:
  - name: memory_sync
    enabled: true
```

环境变量可覆盖配置：`SUPERMAN_MODEL_PROVIDER=openai`、`SUPERMAN_MODEL_API_KEY=sk-...` 等。

## 工具列表

| 工具 | 说明 |
|------|------|
| `code_run` | 在沙箱工作区中执行 Python/Shell 代码 |
| `file_read` | 读取文件，支持行偏移、限制和关键字搜索 |
| `file_write` | 创建、覆写或追加文件 |
| `file_patch` | 通过 old_string → new_string 替换实现精确编辑 |
| `web_scan` | 抓取网页，剥离 HTML，返回纯文本（SSRF 防护） |
| `web_execute` | 浏览器 JavaScript 执行（需要未来 ChromeDP 驱动） |
| `ask_user` | 中断并向用户提问 |
| `checkpoint` | 在任务过程中保存/读取工作笔记 |
| `long_term_memory` | 跨会话持久化重要信息 |
| `search_memory` | 搜索历史对话中的相关信息 |
| `query_experts` | 查找匹配当前任务的专家 Agent |
| `create_expert` | 动态定义新的专业 Agent |
| `delegate_to_expert` | 将任务委托给专家独立执行 |

## Hooks & Skills

### Hooks

在 `hooks/<事件名>/` 目录下放入可执行脚本即可。脚本通过 stdin 接收 JSON 上下文，stdout 输出 JSON 结果。

```
hooks/
├── before_run/          # Run 开始前
├── after_run/           # Run 结束后
├── before_tool/         # 工具执行前
├── after_tool/          # 工具执行后
├── before_model/        # LLM 调用前
├── after_model/         # LLM 调用后
├── before_agent/        # Agent 执行前
├── after_agent/         # Agent 执行后
├── on_user_message/     # 用户发送消息
├── on_model_error/      # 模型出错
└── on_tool_error/       # 工具出错
```

示例脚本 (`hooks/before_tool/audit.sh`)：

```bash
#!/bin/sh
# stdin: {"event":"before_tool","tool_name":"file_write","tool_args":{...}}
echo '{"allow": true}'
# 返回 {"allow": false, "reason": "..."} 可阻止工具执行
```

### Skills

在 `skills/` 下创建技能目录，每个技能是一个包含 YAML frontmatter 的 `SKILL.md` 文件。

```
skills/
└── code-review/
    ├── SKILL.md           # 必需：YAML frontmatter + Markdown 指令
    └── references/        # 可选：参考文档
```

示例 (`skills/code-review/SKILL.md`)：

```markdown
---
name: code-review
description: 专业的代码审查技能，用于 review PR 和代码变更
allowed-tools: [file_read, file_patch, web_scan]
---

你是一个代码审查专家。审查时关注：
1. 安全性 —— OWASP Top 10、注入漏洞
2. 正确性 —— 逻辑错误、边界条件
3. 可维护性 —— 命名、职责分离
```

## 项目结构

```
superman/
├── main.go                          # 入口
├── internal/
│   ├── agent/
│   │   ├── agent.go                 # Agent 工厂（注入记忆/SOP）
│   │   ├── prompt/system.txt        # 系统提示词
│   │   └── tools/                   # 13 个工具实现
│   ├── config/                      # YAML + 环境变量配置 (viper)
│   ├── cli/                         # Cobra CLI 命令 (run, reflect, configure)
│   ├── tui/                         # Bubble Tea TUI
│   │   ├── app.go                   # Model、光标、事件处理
│   │   ├── components/              # 聊天、输入栏、工具栏渲染
│   │   └── styles/                  # 暗色主题
│   ├── model/                       # 多 Provider LLM 工厂
│   ├── memory/                      # L0-L4 分层记忆系统
│   ├── session/                     # 会话管理器（JSONL 持久化）
│   ├── plugin/                      # 插件注册中心 + 内建插件
│   ├── hook/                         # Hook 管理器 + 脚本执行器
│   ├── reflect/                     # 自主空闲监听 + 调度器
│   └── expert/                      # 专家智库（注册中心、委托执行、
│                                    #   分析器、统计、FTS5 索引）
├── hooks/                            # Hook 脚本目录（约定式，11 个事件子目录）
├── skills/                           # Skill 定义目录（ADK skilltoolset）
├── config.example.yaml
├── config/tasks/                    # 定时任务定义示例
├── data/
│   ├── memory/                      # 持久化记忆文件
│   └── experts/                     # 专家 YAML 定义
├── go.mod
└── go.sum
```

## 运行时目录

所有运行时数据都存储在 `cfg.Dir`（默认为 `~/.sm/`），首次启动时自动创建：

```
~/.sm/                                    # cfg.Dir（默认: $HOME/.sm）
├── config.yaml                           # 用户配置（由 `sm configure` 创建）
├── tui.log                               # TUI 运行时日志（重定向以防干扰界面显示）
├── hooks/                                # Hook 事件脚本（11 种生命周期事件）
├── skills/                               # Skill 定义（SKILL.md）
├── data/
│   └── experts/                          # 专家 YAML 定义（自动管理）
├── memory/                               # 分层记忆持久化
│   ├── l0/                               # L0 SOP 规则模板（*.md）
│   ├── l1/
│   │   └── index.txt                     # L1 热记忆索引（自动重建）
│   ├── l2/
│   │   └── entries.jsonl                 # L2 持久化工作记忆
│   ├── l3/
│   │   └── archive.jsonl                 # L3 长期归档记忆
│   ├── l4/                               # L4 压缩会话归档
│   └── candidates/                       # 进化候选（仅审查，不自动写入）
│       ├── memory.jsonl
│       ├── sop/
│       └── experts/
```

## 构建

```bash
go build -o sm .
./sm --help
```

需要 Go 1.25+。

## 许可证

MIT