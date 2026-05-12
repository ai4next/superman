# Superman — General-Purpose AI Agent

A lightweight, general-purpose AI agent built with LangChain + LangGraph. Features a two-level agent architecture (outer state machine + inner tool loop), a thread-safe tool registry, structured error classification, context governance, and working memory.

## Features

- **Two-level agent architecture** — `AgentLoop` (outer FSM: `RESTORE → COMPACT → COMMAND → BUILD → RUN → SAVE → RESPOND → DONE`) + `AgentRunner` (inner tool-calling loop)
- **ToolRegistry** — central, thread-safe registry with generation-based cache invalidation; tools self-register via `ToolSpec` at module level
- **Multi-provider LLM support** — Anthropic Claude, OpenAI, or any OpenAI-compatible API
- **Context governance** — pre-LLM-call cleanup: orphan tool result removal, backfill, micro-compaction, token-budget trimming
- **Error classification** — structured `ErrorKind` taxonomy with recovery hints (retry, compress, rotate credential, fallback)
- **Runtime checkpoint** — crash recovery for mid-turn interruptions
- **Working memory** — auto-injected context notepad (via `update_working_memory` tool)
- **File state tracking** — read-before-write warnings, content deduplication
- **Concurrent sessions** — per-session `asyncio.Lock` with global concurrency gate
- **Built-in tools** — file read/write, glob, grep, shell execution, web search, web fetch
- **Interactive & one-shot modes** — full REPL with history, or single-message automation
- **Session persistence** — JSONL with atomic writes via `os.replace()` + `fsync`

## Quick Start

```bash
# Install
uv sync

# Initialize default config
superman config --init

# Start interactive chat
superman chat
```

## Installation

Requires Python 3.11+ and [uv](https://docs.astral.sh/uv/).

```bash
git clone <repo-url>
cd superman

# Create virtualenv & install dependencies
uv sync

# Optional: web search & fetch support
uv sync --extra web
```

## Usage

### Chat

```bash
# Interactive mode (REPL with history)
superman chat

# One-shot mode
superman chat -m "What files are in the workspace?"

# Use a specific session
superman chat -s "my-session-id"

# Verbose logging (shows iteration details)
superman chat -v
```

### Inline Commands

Type these during an interactive session:

| Command | Description |
|---------|-------------|
| `/clear`, `/new` | Clear conversation history |
| `/help`, `/?` | Show help |
| `/status` | Show session & model info |
| `/exit`, `/quit` | End session |

### Configuration

```bash
# Create default config at ~/.superman/config.json
superman config --init

# View current config
superman config --show

# Init with overrides
superman config --init --provider openai --model gpt-4o --api-key sk-...
```

Configuration is stored as JSON at `~/.superman/config.json`:

```json
{
  "llm": {
    "provider": "anthropic",
    "model": "claude-sonnet-4-20250514",
    "api_key": null,
    "api_base": null,
    "max_tokens": 8192,
    "temperature": 0.7
  },
  "tools": {
    "enable_exec": true,
    "exec_timeout": 60,
    "restrict_to_workspace": false,
    "enable_web": true,
    "web_search_provider": "duckduckgo"
  },
  "workspace": "~/.superman/workspace",
  "max_tool_iterations": 25,
  "max_session_messages": 120,
  "timezone": "UTC"
}
```

## Built-in Tools

| Tool | Description | Read-only |
|------|-------------|-----------|
| `read_file` | Read file contents (capped at 50K chars) | Yes |
| `write_file` | Write content to a file with read-before-write warnings | No |
| `glob` | List files matching a glob pattern | Yes |
| `grep` | Search file contents with regex | Yes |
| `exec` | Execute shell commands (configurable timeout) | No |
| `web_search` | Search the web via DuckDuckGo | Yes |
| `web_fetch` | Fetch and extract readable content from a URL | Yes |
| `update_working_memory` | Update the session working memory notepad | No |

Read-only tools are executed **in parallel** for efficiency; mutating tools run sequentially.

## Architecture

```
superman/
├── agent/
│   ├── loop.py          # AgentLoop — outer FSM (RESTORE→...→DONE)
│   ├── runner.py        # AgentRunner — inner tool loop with governance
│   ├── context.py       # Prompt building, message conversion, context governance
│   ├── errors.py        # Error classification (ErrorKind + ClassifiedError)
│   ├── checkpoint.py    # Runtime checkpoint for crash recovery
│   └── memory.py        # Working memory (auto-injected context notepad)
├── tools/
│   ├── registry.py      # ToolRegistry — thread-safe, generation-based
│   ├── base.py          # ToolSpec dataclass (immutable)
│   ├── builtin.py       # Tool implementations with self-registration
│   └── file_state.py    # FileStateTracker — read-before-write warnings
├── bus/
│   ├── events.py        # InboundMessage / OutboundMessage dataclasses
│   └── queue.py         # MessageBus with async pub/sub queues
├── cli/
│   └── commands.py      # Typer CLI (chat, config commands)
├── config/
│   └── schema.py        # SupermanConfig, LLMConfig (Pydantic models)
├── providers/
│   └── __init__.py      # create_llm() factory (Anthropic, OpenAI, custom)
├── session/
│   └── manager.py       # Session, SessionManager with JSONL persistence
└── templates/
    └── agent/
        └── identity.py  # System prompt template
```

### Agent Lifecycle (Turn State Machine)

Each incoming message flows through a deterministic state machine:

```
RESTORE → COMPACT → COMMAND → BUILD → RUN → SAVE → RESPOND → DONE
```

| State | Purpose |
|-------|---------|
| `RESTORE` | Recover from a previous interrupted turn (crash recovery) |
| `COMPACT` | Auto-compress idle session history |
| `COMMAND` | Check for meta-commands (/clear, /help, /status) |
| `BUILD` | Assemble message list: system prompt → runtime context → working memory → history |
| `RUN` | Delegate to AgentRunner (LLM calls + tool execution) |
| `SAVE` | Persist all messages to session JSONL |
| `RESPOND` | Final formatting, cleanup, return response |

The inner `AgentRunner` handles the LLM-call → tool-execution cycle with:

1. **Context governance** before each LLM call:
   - `backfill_missing_tool_results` — insert synthetic errors for orphaned tool_use blocks
   - `drop_orphan_tool_results` — remove dangling tool results
   - `microcompact_tool_results` — collapse old read-only tool outputs to one line
   - `apply_tool_result_budget` — truncate oversized tool results
2. **Tool batching** — read-only tools execute in parallel, mutating tools sequentially
3. **Error classification** — structured recovery decisions via `classify_error()`

### Key Design Decisions

- **Two-level loop**: `AgentLoop` (outer: sessions, FSM, concurrency) vs `AgentRunner` (inner: LLM, tools, governance) — clean separation that lets the runner be reused by subagents
- **ToolRegistry with generation counter**: Thread-safe, memoization-friendly, extensible — the `_generation` counter lets downstream consumers cheaply invalidate cached schemas
- **Prefer pure functions**: Context governance functions are stateless and testable — no class state, no side effects
- **Runtime checkpoint**: In-flight turn state is persisted to session metadata; crash recovery materializes the partial assistant message + completed tool results
- **Working memory**: Lightweight alternative to RAG — an auto-injected context notepad that the agent updates via the `update_working_memory` tool
- **File state tracking**: Per-session read/write tracker warns on edit-without-read; content deduplication via SHA-256
- **Fail-open with observability**: Every error is classified and logged; retry decisions are structured rather than blind

## Provider Support

| Provider | Value | Backend |
|----------|-------|---------|
| Anthropic | `anthropic` | `langchain-anthropic` |
| OpenAI | `openai` | `langchain-openai` |
| Custom (OpenAI-compatible) | `custom` | `langchain-openai` with custom `api_base` |

## Development

```bash
# Install in dev mode
uv sync

# Activate virtualenv
source .venv/bin/activate

# The project uses hatchling as build backend
# Code style: ruff with line-length 100
```

### Adding Tools

Tools self-register via the `ToolRegistry`. Add your implementation function and a `ToolSpec` entry to the registration list in `superman/tools/builtin.py`:

```python
from superman.tools.base import ToolSpec
from superman.tools.registry import registry

async def my_tool(param: str) -> str:
    """Tool description for the LLM."""
    return f"Result: {param}"

registry.register(ToolSpec(
    name="my_tool",
    description="Description shown to the LLM",
    parameters={
        "type": "object",
        "properties": {
            "param": {"type": "string", "description": "Parameter description"},
        },
        "required": ["param"],
    },
    handler=my_tool,
    toolset="custom",
    read_only=True,  # False if the tool has side effects
))
```

Read-only tools (`read_only=True`) are executed concurrently for efficiency.

## Dependencies

- **Framework:** langchain, langchain-core, langgraph
- **LLM:** langchain-anthropic, langchain-openai
- **CLI:** typer, rich, prompt-toolkit
- **Config:** pydantic, pyyaml
- **Logging:** loguru
- **Web (optional):** duckduckgo_search, readability-lxml, httpx

