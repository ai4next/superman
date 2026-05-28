[English](./README.md) | [简体中文](./README-zh.md)
# Superman

![Logo](assets/banner.png)

Self-evolving autonomous AI agent. Superman routes work across tools and expert sub-agents, persists sessions and flat-file memory, and uses layered agent/meta evolution to turn completed work into durable knowledge.

## 💡 Design Philosophy

- **Collective over monolith.** A giant prompt is not intelligence; it is entropy with a logo. Superman stays small by coordinating tools and domain experts instead of pretending one context window can hold every skill.

- **Background over blocking.** The user should not pay latency for housekeeping. Archiving, memory consolidation, expert cultivation, and meta-evolution happen after the run, where they can improve the system without interrupting the work.

- **Evolution over accumulation.** Logs are not memory. Most history is noise unless it is compressed into facts, procedures, or sharper agents. Agent evolution improves Superman and experts; meta evolution improves only the evolution process. The loop learns, but it has walls.

- **Boundaries over vibes.** Self-improvement without write boundaries is just automated drift. Superman memory, expert memory, expert souls, evolver memory, sessions, and audit logs are separate because ownership is what keeps learning from becoming corruption.

- **Clarity over chaos.** Long context fails less from length than from contamination. The system prefers small files, explicit scopes, persistent sessions, and inspectable diffs because durable autonomy needs a filesystem, not a mystique.

---

## 🚀 Quick Start

Install the latest release on Linux or macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/ai4next/superman/main/install.sh | sh
```

Windows PowerShell:

```powershell
iwr https://raw.githubusercontent.com/ai4next/superman/main/install.ps1 -useb | iex
```

```bash
# Create and edit config
sm init

# Set your API key
export OPENAI_API_KEY=sk-...

# Start the terminal UI
sm

# Or run a single prompt
sm run "What's in this directory?"
```

Install a specific release or use a user-writable directory:

```bash
VERSION=v0.0.1 INSTALL_DIR="$HOME/.local/bin" sh -c "$(curl -fsSL https://raw.githubusercontent.com/ai4next/superman/main/install.sh)"
```

## ✨ Features

- **Multi-model support** — Gemini (Vertex AI), OpenAI, DeepSeek, Claude, Ollama, and any OpenAI-compatible API
- **6 built-in tools** — code execution, file read/write/patch, user interaction, expert delegation
- **MCP server integration** — plug in any MCP-compatible tool server via config (stdin/stdout transport)
- **Instant-messaging integration** — run a long-lived server that connects Superman to Telegram, Feishu/Lark, WeCom, Weixin, QQ, DingTalk, Slack, Discord, LINE, and Weibo
- **Persistent sessions** — SQLite-backed session/message store with compact `U/A/T/O` evolution logs, automatic compaction, file revision tracking, and session export/import
- **Runtime audit** — Events (tool calls, text delta, errors, evolutions) streamed to a queryable JSONL audit log
- **Flat-file memory** — global facts (L1) and SOP files (L2) stored directly in the workspace
- **Expert delegation** — dispatch tasks to expert sub-agents with isolated memory and persistent sessions
- **Layered self-evolution** — agent evolver improves Superman/experts from completed sessions; meta evolver improves only the evolution process from evolver sessions
- **Plugin system** — unified run/model/tool logging and session reaper
- **Terminal UI** — dark theme, Emacs-style keybindings, sidebar, and dialog system
- **Hook system** — 11 lifecycle event hooks (before/after run, tool, model, etc.) with external script execution via JSON stdin/stdout protocol
- **Skill system** — filesystem-based skills auto-loaded via ADK skilltoolset, compatible with Claude Code SKILL.md format, supports multiple skill paths

## ⌨️ Commands

| Command | Description |
|---------|-------------|
| `sm` | Start interactive terminal chat |
| `sm run "prompt"` | Run a single prompt, print response |
| `sm run -f prompt.txt` | Run a prompt from a file |
| `sm run -p "hello"` | Run with `--prompt` flag |
| `sm reflect` | Start autonomous idle-watch + scheduler mode |
| `sm im serve` | Run the instant-messaging integration server |
| `sm init` | Create `config.yaml` from the embedded example template |
| `sm configure` | Show or initialize configuration |
| `sm toolsets` | List configured ADK Skill and MCP toolsets |
| `sm sessions list` | List persistent sessions |
| `sm sessions show <id>` | Show session messages |
| `sm sessions last` | Show the most recently updated session |
| `sm sessions search <query>` | Search persisted session messages |
| `sm sessions files <id>` | Show session working files |
| `sm sessions history <id>` | Show session file revision history |
| `sm sessions diff <id> <path>` | Show file revision diff |
| `sm sessions revert <id> <path>` | Revert a file to its previous revision |
| `sm sessions export <id>` | Export session (markdown/json/jsonl) |
| `sm sessions import <path>` | Import a session export |
| `sm sessions compact <id>` | Compact older session context into a summary |
| `sm sessions delete <id>` | Delete a persistent session |
| `sm sessions rename <id> <title>` | Rename a session |
| `sm sessions queue <id>` | Inspect queued prompts for a session |
| `sm sessions storage` | Inspect persistent session storage stats |
| `sm sessions storage gc` | Remove orphaned file revision snapshots |
| `sm runtime events` | List runtime audit events |
| `sm runtime summary` | Summarize runtime audit events |

## ⚙️ Configuration

See `config.example.yaml` for all options. Key settings:

```yaml
model:
  provider: openai          # gemini | openai | deepseek | claude | ollama
  name: gpt-4o
  base_url: https://api.openai.com/v1
  api_key: ${OPENAI_API_KEY}
  headers:
    X-Request-Source: superman

tools:
  code_run:
    enabled: true
    timeout: 30s

# Skill system — multiple paths supported
skills:
  enabled: true
  paths:
    - ${HOME}/.sm/skills
    - ./skills

# MCP server integration
mcp:
  servers:
    - name: my-server
      enabled: true
      command: npx
      args: [-y, @modelcontextprotocol/server-filesystem, /tmp]
      tools: []                 # empty = all tools; specify names to filter

# Session management
session:
  max_turns: 75
  loop_detection:
    enabled: true
    window_size: 10
    max_repeats: 5

plugins:
  - name: logger
    enabled: true
```

`model.headers` is optional and is forwarded with every model request, which is useful for custom OpenAI-compatible gateways.

Environment variables override config: `SUPERMAN_MODEL_PROVIDER=openai`, `SUPERMAN_MODEL_API_KEY=sk-...`, etc.

### Instant Messaging

Enable one or more `im.platforms` entries, set the required platform credentials, then run:

```bash
sm im serve --config config.yaml
```

`im serve` is a long-lived server process. Run it under your preferred process manager, or in a shell background job:

```bash
nohup sm im serve --config config.yaml > ~/.sm/runtime/im.log 2>&1 &
```

Example:

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

See `config.example.yaml` for Telegram, Feishu/Lark, WeCom, Weixin, QQ, QQ official bot, DingTalk, Slack, Discord, LINE, and Weibo examples.

For Weixin personal accounts, use QR setup first:

```bash
sm im weixin setup
```

The command prints a QR code in the terminal, waits for phone confirmation, prints the token and account values, then exits. Add the printed values to your `weixin` entry in `im.platforms` before running `sm im serve`.

## 🛠️ Tools

| Tool | Description |
|------|-------------|
| `code_run` | Execute Python/Shell code |
| `read` | Read file lines |
| `write` | Write files |
| `patch` | Replace one exact text match in a file |
| `ask_user` | Interrupt to ask the user a question |
| `delegate_to_expert` | Delegate a task to an expert for independent execution |

`delegate_to_expert` is loaded dynamically only when expert delegation is enabled and at least one expert is available. Experts are stored as directories under `experts/{expert_name}`; the directory name is the expert name and `soul.md` is the expert's system prompt.

## 🔌 Hooks & Skills

### Hooks

Place executable scripts in `hooks/<event>/` directories. They receive JSON context via stdin and return JSON via stdout.

```
hooks/
├── before_run/          # Before agent run
├── after_run/           # After agent run
├── before_tool/         # Before tool execution
├── after_tool/          # After tool execution
├── before_model/        # Before LLM call
├── after_model/         # After LLM call
├── before_agent/        # Before agent execution
├── after_agent/         # After agent execution
├── on_user_message/     # On user message
├── on_model_error/      # On model error
└── on_tool_error/       # On tool error
```

Example script (`hooks/before_tool/audit.sh`):

```bash
#!/bin/sh
# stdin: {"event":"before_tool","tool_name":"write","tool_args":{...}}
echo '{"allow": true}'
# Return {"allow": false, "reason": "..."} to block the tool
```

### Skills

Create skill directories under `skills/` or any configured skill path. Each skill is a `SKILL.md` file with YAML frontmatter.

```
skills/
└── code-review/
    ├── SKILL.md           # Required: YAML frontmatter + Markdown instructions
    └── references/        # Optional: reference docs
```

Example (`skills/code-review/SKILL.md`):

```markdown
---
name: code-review
description: Professional code review for PRs and changes
allowed-tools: [read, patch, web_scan]
---

You are a code review expert. Focus on:
1. Security — OWASP Top 10, injection vulnerabilities
2. Correctness — logic errors, edge cases
3. Maintainability — naming, separation of concerns
```

### MCP Servers

Superman supports any MCP-compatible server via stdin/stdout transport. Configure servers in `config.yaml`:

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

Use `sm toolsets` to verify configured servers and their available tools.

## 📁 Project Structure

```
superman/
├── main.go                          # Entry point
├── internal/
│   ├── agent/
│   │   ├── agent.go                 # Agent factory with memory/SOP injection
│   │   ├── context.go               # Context builder for agent runs
│   │   ├── tool.go                  # Dynamic built-in/toolset assembly
│   │   └── evolver.go               # Agent/meta evolution agent factories
│   ├── prompt/                      # Embedded prompt templates
│   │   └── template/                # Markdown prompt templates
│   ├── config/                      # YAML + env config (viper), embedded config.example.yaml
│   ├── cli/                         # Cobra CLI commands (init, run, reflect, im, configure, toolsets, sessions, runtime)
│   ├── tui/                         # Terminal UI
│   │   ├── tui.go                   # Compatibility wrapper
│   │   ├── app/                     # Model, runtime, sessions, commands, dialogs, layout
│   │   ├── components/              # Chat, input line, toolbar, sidebar renderers
│   │   └── styles/                  # Dark theme, icons, color themes
│   ├── model/                       # Multi-provider LLM factory
│   ├── memory/                      # Flat-file memory (L1 global facts, L2 SOP files)
│   ├── session/                     # Persistent session manager with JSONL store, file tracking, references
│   ├── runtime/                     # Event-driven runtime with audit logging
│   ├── im/                          # Instant-messaging integration
│   ├── plugin/                      # Plugin registry + built-ins
│   ├── hook/                        # Hook manager + script runner
│   ├── reflect/                     # Autonomous idle watcher + scheduler
│   └── expert/                      # Directory-backed expert registry (`soul.md`)
├── hooks/                            # Hook scripts (convention-based, 11 event dirs)
├── skills/                           # Skill definitions (ADK skilltoolset)
├── config.example.yaml              # Symlink to internal/config/config.example.yaml
├── config/tasks/                    # Sample scheduler task definitions
├── data/
│   └── sessions/                    # Session history
├── go.mod
└── go.sum
```

## 📂 Runtime Directory

All runtime data is stored under `workspace` in `config.yaml`. If omitted, it defaults to `$HOME/.sm`. The directory is created on first run:

```
~/.sm/                                    # workspace (default: $HOME/.sm)
├── config.yaml                           # User configuration (created by `sm init` or `sm configure`)
├── tui.log                               # Terminal UI runtime log (redirected for display safety)
├── hooks/                                # Hook event scripts (11 lifecycle events)
├── skills/                               # Skill definitions (SKILL.md)
├── memory/                               # Superman's flat-file memory
│   ├── l1.toml                           # L1 global facts
│   ├── l2/                               # L2 SOP files (*.md)
├── state.db                              # SQLite session/message metadata store
├── sessions/                             # Compact session logs and snapshots
│   ├── <id>.log                          # LLM evolution projection
│   └── snapshots/                        # File revision snapshots
├── runtime/
│   └── events.jsonl                      # Runtime audit event log
├── evolution/                            # Agent evolver + meta evolver runtime root
│   ├── memory/                           # Evolver's flat-file memory
│   │   ├── l1.toml
│   │   └── l2/
│   ├── state.db                          # Evolver SQLite session/message metadata store
│   └── sessions/
│       ├── agent-evolution-<...>.log     # Agent evolution session log
│       └── meta-evolution-<...>.log      # Meta evolution session log
└── experts/
    └── {expert_name}/
        ├── soul.md                       # Expert system prompt; directory name is the expert name
        ├── memory/                       # Expert's isolated memory
        ├── state.db                      # Expert SQLite session/message metadata store
        └── sessions/
            └── <id>.log                  # Expert compact session log
```

## 🏗️ Build

Install from GitHub Releases:

```bash
# macOS/Linux
curl -fsSL https://raw.githubusercontent.com/ai4next/superman/main/install.sh | sh

# Windows PowerShell
iwr https://raw.githubusercontent.com/ai4next/superman/main/install.ps1 -useb | iex
```

Build from source:

```bash
go build -o sm .
./sm --help
```

Requires Go 1.26+.

## 📄 License

MIT

---

## 📈 Star History

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
