[English](./README.md) | [简体中文](./README-zh.md)
# Superman

![Logo](assets/banner.png)

General-purpose autonomous AI agent. Multi-model support, 8 built-in tools, flat-file memory, expert delegation.

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
- **8 built-in tools** — code execution, file read/write/patch, web scanning, browser execution, user interaction, expert delegation
- **Flat-file memory (L0-L3)** — runtime index (L0), global facts (L1), SOP files (L2), session archive (L3)
- **Expert delegation** — dispatch tasks to expert sub-agents with isolated memory
- **Plugin system** — unified run/model/tool logging and session reaper
- **TUI interface** — Bubble Tea + Lipgloss, dark theme, Emacs-style keybindings
- **Hook system** — 11 lifecycle event hooks (before/after run, tool, model, etc.) with external script execution via JSON stdin/stdout protocol
- **Skill system** — filesystem-based skills auto-loaded via ADK skilltoolset, compatible with Claude Code SKILL.md format

## Commands

| Command | Description |
|---------|-------------|
| `sm` | Start interactive TUI chat |
| `sm run "prompt"` | Run a single prompt, print response |
| `sm run -f prompt.txt` | Run a prompt from a file |
| `sm run -p "hello"` | Run with `--prompt` flag |
| `sm reflect` | Start autonomous idle-watch + scheduler mode |
| `sm configure` | Show or initialize configuration |

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
  web_scan:
    enabled: true
  # ... each tool can be individually enabled/disabled

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
| `web_scan` | Fetch web pages, strip HTML, return text (SSRF-protected) |
| `web_execute` | Browser JavaScript execution through ChromeDP; can reuse a configured Chrome profile or remote debugging endpoint |
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

Create skill directories under `skills/`. Each skill is a `SKILL.md` file with YAML frontmatter.

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

## Project Structure

```
superman/
├── main.go                          # Entry point
├── internal/
│   ├── agent/
│   │   ├── agent.go                 # Agent factory with memory/SOP injection
│   │   ├── prompt/system.txt        # System prompt
│   │   └── tools/                   # 8 tool implementations
│   ├── config/                      # YAML + env config (viper)
│   ├── cli/                         # Cobra CLI commands (run, reflect, configure)
│   ├── tui/                         # Bubble Tea TUI
│   │   ├── app.go                   # Model, cursor writer, event handling
│   │   ├── components/              # Chat, input line, toolbar renderers
│   │   └── styles/                  # Dark theme
│   ├── model/                       # Multi-provider LLM factory
│   ├── memory/                      # L0-L3 flat-file memory (rules, profile, SOP, sessions)
│   ├── session/                     # Session manager with JSONL persistence
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
│   └── l3/raw_sessions/                  # L3 raw session JSONL archives
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

Requires Go 1.25+.

## License

MIT
