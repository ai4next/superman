[English](./README.md) | [简体中文](./README-zh.md)
# Superman

![Logo](assets/banner.png)

General-purpose autonomous AI agent. Multi-model support, 13 built-in tools, layered memory, expert group.

## Design Philosophy

1. **Files over databases.** JSONL beats any vector store on portability, debuggability, and cost. Complexity is debt.

2. **Inject over RAG.** The prompt is faster, cheaper, and more reliable than any retrieval pipeline. Most RAG exists to compensate for bad prompt design.

3. **Route over bloat.** One agent to rule them all is a fantasy. Experts are narrow sub-agents. The main agent just routes.

4. **Background over blocking.** Archiving, analysis, optimization — nothing should make the user wait.

5. **Simple over complicated.** Break hard problems into small steps. Long context fails not because it's short, but because it's noisy. Clarity always wins.

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
- **13 built-in tools** — code execution, file read/write/patch, web scanning, browser execution, user interaction, working memory, long-term memory, memory search, expert query/create/delegate
- **Layered memory (L0-L4)** — SOP rules, memory index, persistent storage, session archives, historical session compression
- **Expert group** — sub-agent dispatch with lifecycle management (draft → active → mature → archived), automatic extraction from usage patterns
- **Plugin system** — memory sync, token tracking, tool logging, session reaper
- **TUI interface** — Bubble Tea + Lipgloss, dark theme, Emacs-style keybindings
- **Autonomous modes** — idle-triggered reflection and scheduled task execution
- **Pattern analysis** — automatic expert draft generation from repeated tool chains
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
    workspace: ./workspace
  web_scan:
    enabled: true
  # ... each tool can be individually enabled/disabled

plugins:
  - name: memory_sync
    enabled: true
```

Environment variables override config: `SUPERMAN_MODEL_PROVIDER=openai`, `SUPERMAN_MODEL_API_KEY=sk-...`, etc.

## Tools

| Tool | Description |
|------|-------------|
| `code_run` | Execute Python/Shell code in a sandboxed workspace |
| `file_read` | Read files with line offset, limit, and keyword search |
| `file_write` | Create, overwrite, or append files |
| `file_patch` | Precise edits via old_string → new_string replacement |
| `web_scan` | Fetch web pages, strip HTML, return text (SSRF-protected) |
| `web_execute` | Browser JS execution (requires future ChromeDP driver) |
| `ask_user` | Interrupt to ask the user a question |
| `checkpoint` | Save/retrieve working notes during a task |
| `long_term_memory` | Persist important information across sessions |
| `search_memory` | Search past conversations for relevant information |
| `query_experts` | Find expert agents matching the current task |
| `create_expert` | Define new specialized expert agents on the fly |
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
# stdin: {"event":"before_tool","tool_name":"file_write","tool_args":{...}}
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
allowed-tools: [file_read, file_patch, web_scan]
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
│   │   └── tools/                   # 13 tool implementations
│   ├── config/                      # YAML + env config (viper)
│   ├── cli/                         # Cobra CLI commands (run, reflect, configure)
│   ├── tui/                         # Bubble Tea TUI
│   │   ├── app.go                   # Model, cursor writer, event handling
│   │   ├── components/              # Chat, input line, toolbar renderers
│   │   └── styles/                  # Dark theme
│   ├── model/                       # Multi-provider LLM factory
│   ├── memory/                      # L0-L4 layered memory system
│   ├── session/                     # Session manager with JSONL persistence
│   ├── plugin/                      # Plugin registry + built-ins
│   ├── hook/                         # Hook manager + script runner
│   ├── reflect/                     # Autonomous idle watcher + scheduler
│   └── expert/                      # Expert group (registry, delegate,
│                                    #   analyzer, stats, FTS5 index)
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

All runtime data is stored under `cfg.Dir` (`~/.sm/` by default), automatically created on first run:

```
~/.sm/                                    # cfg.Dir (default: $HOME/.sm)
├── config.yaml                           # User configuration (created by `sm configure`)
├── tui.log                               # TUI runtime log (redirected for display safety)
├── hooks/                                # Hook event scripts (11 lifecycle events)
├── skills/                               # Skill definitions (SKILL.md)
├── superman/
│   ├── experts/                          # Expert YAML definitions (auto-managed)
│   └── memory/                           # Superman's isolated layered memory
│       ├── l0/                           # L0 SOP rule templates (*.md)
│       ├── l1/index.txt                  # L1 hot memory index (auto-rebuilt)
│       ├── l2/entries.jsonl              # L2 persistent working memory
│       ├── l3/archive.jsonl              # L3 long-term archived memory
│       ├── l4/                           # L4 compressed session archives
│       └── candidates/                   # Evolution candidates (review-only)
│           ├── sop/
│           └── experts/candidates.jsonl  # Expert extraction candidates
└── experts/
    └── {expert_name}/
        ├── calls.jsonl                   # Expert consult/delegate call log
        └── memory/                       # Expert's isolated layered memory
```

## Build

```bash
go build -o sm .
./sm --help
```

Requires Go 1.25+.

## License

MIT
