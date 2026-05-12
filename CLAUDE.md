# Superman — LangGraph-Powered AI Agent

A lightweight, general-purpose AI agent built on **LangGraph**. The entire agent lifecycle is modelled as a directed graph: `load_session → call_model → (tool loop) → save_session`.

## Architecture

```
superman/
├── graph/
│   ├── state.py        # AgentState TypedDict
│   └── agent.py        # Graph nodes, context governance, tool definitions, builder
├── config/
│   └── schema.py       # SupermanConfig, LLMConfig (Pydantic)
├── providers/
│   └── __init__.py     # create_llm() factory
├── session/
│   └── manager.py      # Session, SessionManager (JSONL persistence)
├── bus/
│   ├── events.py       # InboundMessage / OutboundMessage (for channel integration)
│   └── queue.py        # MessageBus (async queues)
├── cli/
│   └── commands.py     # Typer CLI (chat, config)
└── templates/
    └── agent/
        └── identity.py # System prompt template
```

## Graph

```
START → load_session → call_model ─┬─ (tool_calls) → execute_tools → call_model
                                   └─ (text)       → save_session → END
```

| Node | Responsibility |
|------|---------------|
| `load_session` | Load conversation history from JSONL (first turn only) |
| `call_model` | Assemble system prompt + runtime context + working memory, apply context governance, call LLM |
| `execute_tools` | LangGraph's `ToolNode` — execute tool calls (read-only in parallel, mutating sequenced) |
| `save_session` | Persist messages back to JSONL |

**Context governance** (applied before every LLM call inside `call_model`):
- `_backfill` — inject synthetic errors for orphaned tool_use blocks
- `_drop_orphans` — remove dangling tool results
- `_compact` — collapse old read-only tool outputs to one-line summaries
- `_budget` — truncate oversized tool results

## State

```python
class AgentState(TypedDict):
    messages: Annotated[Sequence[BaseMessage], add_messages]
    session_key: str
    working_memory: dict       # key_info, related_sops
    file_reads: dict            # path -> sha256
    turn_count: int
    session_loaded: bool
```

## Development Commands

```bash
# Install in dev mode
pip install -e .

# Run CLI
superman chat                        # Interactive mode
superman chat -m "Hello!"            # One-shot mode
superman config --init               # Init default config
superman config --show               # Show current config
```

## Dependencies

- langchain, langchain-core, langgraph (framework)
- langchain-anthropic, langchain-openai (LLM providers)
- typer, rich, prompt-toolkit (CLI)
- pydantic, pyyaml (config)
- loguru (logging)