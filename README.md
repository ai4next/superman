# Superman

![Logo](assets/banner.png)

General-purpose autonomous AI agent. Multi-model support, 9 built-in tools, layered memory.

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
- **9 built-in tools** — code execution, file read/write/patch, web scanning, user interaction, working memory, long-term memory
- **Layered memory** (L0-L3) — meta-rules, memory index, long-term storage, session archives
- **Plugin system** — memory sync, token tracking, tool logging, session reaper
- **TUI interface** — Bubble Tea + Lipgloss, dark theme
- **Autonomous modes** — idle-triggered reflection and scheduled task execution

## Commands

| Command | Description |
|---------|-------------|
| `sm` | Start interactive TUI chat |
| `sm run "prompt"` | Run a single prompt, print response |
| `sm run -f prompt.txt` | Run a prompt from a file |
| `sm run -p "hello"` | Run with `--prompt` flag |
| `sm reflect` | Start autonomous idle-watch mode |
| `sm configure` | Interactive config wizard (coming soon) |

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

## Project Structure

```
superman/
├── main.go                        # Entry point
├── internal/
│   ├── agent/
│   │   ├── agent.go               # Agent factory
│   │   ├── prompt/system.txt      # System prompt
│   │   └── tools/                 # 9 tool implementations (code_run, file_*, web_*, etc.)
│   ├── config/                    # YAML + env config (viper, Duration decode hook)
│   ├── cli/                       # Cobra CLI commands (run, reflect, configure)
│   ├── tui/                       # Bubble Tea TUI
│   │   ├── app.go                 # Main TUI model, cursor writer, event handling
│   │   ├── components/            # Chat, input line, toolbar renderers
│   │   └── styles/                # Dark theme (Accent/Warning/Dim/etc.)
│   ├── model/                     # Multi-provider LLM factory (OpenAI, Anthropic, Gemini)
│   ├── memory/                    # L0-L3 memory system
│   ├── session/                   # Session manager with JSONL persistence
│   ├── plugin/                    # Plugin registry + built-ins (memory_sync, token_tracker, etc.)
│   └── reflect/                   # Autonomous idle watcher + scheduled task executor
├── config.example.yaml
├── config/tasks/                  # Sample scheduler task definitions (e.g. health check)
├── go.mod
└── go.sum
```

## Build

```bash
go build -o sm .
./sm --help
```

Requires Go 1.25+.

## License

MIT