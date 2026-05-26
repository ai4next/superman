[English](./README.md) | [简体中文](./README-zh.md)
# Superman

![Logo](assets/banner.png)

General-purpose autonomous AI agent. Multi-model support, 6 built-in tools, flat-file memory, expert delegation, MCP server integration, persistent session management, and Bubble Tea v2 TUI.

## Design Philosophy

- **Route over bloat.** One agent to rule them all is a fantasy. Experts are narrow sub-agents. The main agent just routes.

- **Background over blocking.** Archiving, analysis, optimization — nothing should make the user wait.

- **Simple over complicated.** Break hard problems into small steps. Long context fails not because it's short, but because it's noisy. Clarity always wins.

---

## Quick Start

```bash
# Copy and edit config
cp config.example.yaml config.yaml

# Set your API key
export OPENAI_API_KEY=sk-...

# Start the TUI
go run .

# Or run a single prompt
go run . run "What's in this directory?"
```

## Features

- **Multi-model support** — Gemini (Vertex AI), OpenAI, DeepSeek, Claude, Ollama, and any OpenAI-compatible API
- **6 built-in tools** — code execution, file read/write/patch, user interaction, expert delegation
- **MCP server integration** — plug in any MCP-compatible tool server via config (stdin/stdout transport)
- **Persistent sessions** — SQLite-backed session/message store with compact `U/A/T/O` evolution logs, automatic compaction, file revision tracking, and session export/import
- **Runtime audit** — Events (tool calls, text delta, errors, evolutions) streamed to a queryable JSONL audit log
- **Flat-file memory (L0-L3)** — runtime index (L0), global facts (L1), SOP files (L2), session archive (L3)
- **Expert delegation** — dispatch tasks to expert sub-agents with isolated memory
- **Plugin system** — unified run/model/tool logging and session reaper
- **TUI interface** — Bubble Tea v2 + Lipgloss v2, dark theme, Emacs-style keybindings, sidebar, dialog system
- **Hook system** — 11 lifecycle event hooks (before/after run, tool, model, etc.) with external script execution via JSON stdin/stdout protocol
- **Skill system** — filesystem-based skills auto-loaded via ADK skilltoolset, compatible with Claude Code SKILL.md format, supports multiple skill paths

## Commands

| Command | Description |
|---------|-------------|
| `sm` | Start interactive TUI chat |
| `sm run "prompt"` | Run a single prompt, print response |
| `sm run -f prompt.txt` | Run a prompt from a file |
| `sm run -p "hello"` | Run with `--prompt` flag |
| `sm reflect` | Start autonomous idle-watch + scheduler mode |
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

## Configuration

See `config.example.yaml` for all options. Key settings:

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

Environment variables override config: `SUPERMAN_MODEL_PROVIDER=openai`, `SUPERMAN_MODEL_API_KEY=sk-...`, etc.

## Tools

| Tool | Description |
|------|-------------|
| `code_run` | Execute Python/Shell code |
| `read` | Read file lines |
| `write` | Write files |
| `patch` | Replace one exact text match in a file |
| `ask_user` | Interrupt to ask the user a question |
| `delegate_to_expert` | Delegate a task to an expert for independent execution |

## Hooks & Skills

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

## Project Structure

```
superman/
├── main.go                          # Entry point
├── internal/
│   ├── agent/
│   │   ├── agent.go                 # Agent factory with memory/SOP injection
│   │   ├── context.go               # Context builder for agent runs
│   │   ├── prompt/system.txt        # System prompt
│   │   └── toolsets.go              # Skill + MCP toolset construction
│   ├── config/                      # YAML + env config (viper)
│   ├── cli/                         # Cobra CLI commands (run, reflect, configure, toolsets, sessions, runtime)
│   ├── tui/                         # Bubble Tea v2 TUI
│   │   ├── tui.go                   # Compatibility wrapper
│   │   ├── app/                     # Model, runtime, sessions, commands, dialogs, layout
│   │   ├── components/              # Chat, input line, toolbar, sidebar renderers
│   │   └── styles/                  # Dark theme, icons, color themes
│   ├── model/                       # Multi-provider LLM factory
│   ├── memory/                      # L0-L3 flat-file memory (rules, profile, SOP, sessions)
│   ├── session/                     # Persistent session manager with JSONL store, file tracking, references
│   ├── runtime/                     # Event-driven runtime with audit logging
│   ├── plugin/                      # Plugin registry + built-ins
│   ├── hook/                        # Hook manager + script runner
│   ├── reflect/                     # Autonomous idle watcher + scheduler
│   └── expert/                      # Expert registry with Spec definitions
├── hooks/                            # Hook scripts (convention-based, 11 event dirs)
├── skills/                           # Skill definitions (ADK skilltoolset)
├── config.example.yaml
├── config/tasks/                    # Sample scheduler task definitions
├── data/
│   └── sessions/                    # Session history
├── go.mod
└── go.sum
```

## Runtime Directory

All runtime data is stored under `workspace` in `config.yaml`. If omitted, it defaults to `$HOME/.sm`. The directory is created on first run:

```
~/.sm/                                    # workspace (default: $HOME/.sm)
├── config.yaml                           # User configuration (created by `sm configure`)
├── tui.log                               # TUI runtime log (redirected for display safety)
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
└── experts/
    └── {expert_name}/
        ├── calls.jsonl                   # Expert consult/delegate call log
        └── memory/                       # Expert's isolated memory
```

## Build

```bash
go build -o sm .
./sm --help
```

Requires Go 1.26+.

## License

MIT
