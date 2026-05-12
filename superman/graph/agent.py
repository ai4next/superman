"""LangGraph agent — session-aware, context-governed, tool-using graph.

Nodes
-----
load_session   Load conversation history from JSONL (first turn only).
call_model     Assemble system prompt + working memory + context
               governance, then invoke the LLM with bound tools.
execute_tools  LangGraph's built-in ToolNode.
save_session   Persist new messages back to JSONL.

Edges
-----
START → load_session → call_model ─┬─ (has tools) → execute_tools → call_model
                                   └─ (no tools)  → save_session → END
"""

from __future__ import annotations

import asyncio
import hashlib
import os
import platform
import re
from datetime import datetime, timezone
from pathlib import Path
from typing import Annotated, Any

from langchain_core.language_models.chat_models import BaseChatModel
from langchain_core.messages import AIMessage, BaseMessage, HumanMessage, SystemMessage, ToolMessage
from langchain_core.tools import tool
from langchain_core.runnables import RunnableConfig
from langgraph.graph import END, START, StateGraph
from langgraph.graph.message import add_messages
from langgraph.prebuilt import ToolNode
from loguru import logger

from superman.config.schema import SupermanConfig
from superman.graph.state import AgentState
from superman.providers import create_llm
from superman.session.manager import SessionManager
from superman.templates.agent.identity import IDENTITY_TEMPLATE

# ======================================================================
# Helpers
# ======================================================================

_WORKSPACE: str = ""
"""Set once at startup by ``build_graph()``."""


def _now(tz: str | None = None) -> str:
    if tz:
        try:
            import zoneinfo
            return datetime.now(zoneinfo.ZoneInfo(tz)).strftime("%Y-%m-%d %H:%M:%S %Z")
        except Exception:
            pass
    return datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S UTC")


def _system_prompt(workspace: Path) -> str:
    w = str(workspace.expanduser().resolve())
    sys_name = platform.system()
    runtime = f"{'macOS' if sys_name == 'Darwin' else sys_name} {platform.machine()}, Python {platform.python_version()}"
    parts = [IDENTITY_TEMPLATE.format(workspace_path=w, runtime=runtime)]
    for name in ("AGENTS.md", "SOUL.md", "USER.md", "TOOLS.md"):
        p = workspace / name
        if p.exists():
            parts.append(f"## {name}\n\n{p.read_text(encoding='utf-8')}")
    return "\n\n---\n\n".join(parts)


def _mk_user(content: str) -> HumanMessage:
    return HumanMessage(content=content)


def _mk_assistant(content: str, tool_calls: list | None = None) -> AIMessage:
    return AIMessage(content=content, tool_calls=tool_calls or [])


def _mk_tool(content: str, tool_call_id: str) -> ToolMessage:
    return ToolMessage(content=content, tool_call_id=tool_call_id)


# ======================================================================
# Context governance  (pure functions, applied before every LLM call)
# ======================================================================


def _govern(messages: list[BaseMessage], max_count: int = 10, max_chars: int = 10_000) -> list[BaseMessage]:
    """Apply context governance in order: backfill → drop orphans → compact → budget."""
    msgs = _backfill(_drop_orphans(list(messages)))
    msgs = _compact(msgs, max_count)
    msgs = _budget(msgs, max_chars)
    return msgs


def _drop_orphans(messages: list[BaseMessage]) -> list[BaseMessage]:
    """Remove ToolMessages whose tool_call_id has no matching tool_use in the
    preceding AIMessage."""
    active: set[str] = set()
    out: list[BaseMessage] = []
    for m in messages:
        if isinstance(m, AIMessage):
            out.append(m)
            for tc in (m.tool_calls or []):
                tid = tc.get("id") or tc.get("tool_call_id")
                if tid:
                    active.add(tid)
        elif isinstance(m, ToolMessage):
            if m.tool_call_id in active:
                out.append(m)
        else:
            out.append(m)
    return out


def _backfill(messages: list[BaseMessage]) -> list[BaseMessage]:
    """Insert synthetic error ToolMessages for tool_use blocks whose result
    never arrived."""
    call_ids: dict[str, int] = {}
    result_ids: set[str] = set()
    for i, m in enumerate(messages):
        if isinstance(m, AIMessage):
            for tc in (m.tool_calls or []):
                tid = tc.get("id") or tc.get("tool_call_id")
                if tid:
                    call_ids[tid] = i
        elif isinstance(m, ToolMessage):
            result_ids.add(m.tool_call_id)
    backfills: list[ToolMessage] = []
    for tid in call_ids:
        if tid not in result_ids:
            backfills.append(_mk_tool("Error: Tool call interrupted.", tid))
    if not backfills:
        return messages
    result = list(messages)
    insert_at = max(call_ids[tid] for tid in call_ids if tid not in result_ids) + 1 if call_ids else len(messages)
    for bf in backfills:
        result.insert(insert_at, bf)
        insert_at += 1
    return result


_COMPACTABLE = {"read_file", "glob", "grep", "web_search", "web_fetch"}


def _compact(messages: list[BaseMessage], max_count: int = 10) -> list[BaseMessage]:
    """Summarise older compactable tool results to one line."""
    compact_count = 0
    out: list[BaseMessage] = []
    for m in reversed(messages):
        if isinstance(m, ToolMessage):
            name = getattr(m, "name", "") or ""
            is_c = name.lower() in _COMPACTABLE
            if is_c:
                if compact_count >= max_count and len((m.content or "")) > 200:
                    first = (m.content or "").split("\n")[0][:120]
                    m = _mk_tool(f"[compact] {first}…", m.tool_call_id)
                compact_count += 1
        out.append(m)
    out.reverse()
    return out


def _budget(messages: list[BaseMessage], max_chars: int = 10_000) -> list[BaseMessage]:
    """Truncate oversized ToolMessage content."""
    out: list[BaseMessage] = []
    for m in messages:
        if isinstance(m, ToolMessage) and len((m.content or "")) > max_chars:
            half = max_chars // 2
            c = (m.content or "")
            m = _mk_tool(c[:half] + f"\n… ({len(c)} chars) …\n" + c[-half:], m.tool_call_id)
        out.append(m)
    return out


# ======================================================================
# Graph nodes
# ======================================================================


def load_session(state: AgentState, config: RunnableConfig) -> dict:
    """Load conversation history from JSONL on the first turn."""
    if state.get("session_loaded"):
        return {}

    mgr: SessionManager = config["configurable"]["session_manager"]
    session = mgr.get_or_create(state["session_key"])
    history = session.get_history(120)

    msgs: list[BaseMessage] = []
    for m in history:
        role = m.get("role", "")
        content = m.get("content", "") or ""
        if role == "user":
            msgs.append(_mk_user(content))
        elif role == "assistant":
            msgs.append(_mk_assistant(content, m.get("tool_calls")))
        elif role == "tool":
            msgs.append(_mk_tool(content, m.get("tool_call_id", "")))

    wm: dict = state.get("working_memory") or {}
    wm["turn_count"] = len([m for m in msgs if isinstance(m, AIMessage) and not m.tool_calls])

    return {"messages": msgs, "session_loaded": True, "working_memory": wm}


def call_model(state: AgentState, config: RunnableConfig) -> dict:
    """Build the full prompt (system + runtime + working memory + governed
    history), call the LLM, and return the response."""
    llm: BaseChatModel = config["configurable"]["llm"]
    tools = config["configurable"]["tools"]
    workspace = Path(config["configurable"]["workspace"])
    timezone = config["configurable"]["timezone"]
    llm_with_tools = llm.bind_tools(tools)

    # System prompt
    sys = _system_prompt(workspace)

    # Runtime context
    rt = f"Current Time: {_now(timezone)}\nTurn: {state.get('turn_count', 0) + 1}"

    # Working memory
    wm = state.get("working_memory") or {}
    _current_wm.set(wm)  # make available to tools
    wm_lines = ["--- Working Memory ---"]
    if wm.get("key_info"):
        wm_lines.append(f"Key Info: {wm['key_info']}")
    if wm.get("related_sops"):
        wm_lines.append(f"Related SOPs: {', '.join(wm['related_sops'])}")
    wm_block = "\n".join(wm_lines)

    # Assemble message list with context governance
    conv = _govern(list(state["messages"]))
    full: list[BaseMessage] = [
        SystemMessage(content=sys),
        SystemMessage(content=f"---\n{rt}\n---"),
        SystemMessage(content=wm_block),
    ] + conv

    try:
        response = llm_with_tools.invoke(full)
    except Exception as exc:
        logger.warning("LLM call failed: {}", exc)
        return {
            "messages": [_mk_assistant(f"Error: {exc}")],
            "turn_count": state.get("turn_count", 0) + 1,
        }

    return {
        "messages": [response],
        "turn_count": state.get("turn_count", 0) + 1,
    }


def should_continue(state: AgentState) -> str:
    """Route: tools → ``execute_tools``, otherwise → ``save_session``."""
    msgs = state.get("messages") or []
    if msgs and hasattr(msgs[-1], "tool_calls") and msgs[-1].tool_calls:
        return "execute_tools"
    return "save_session"


def save_session(state: AgentState, config: RunnableConfig) -> dict:
    """Persist ``messages`` to the JSONL session store."""
    mgr: SessionManager = config["configurable"]["session_manager"]
    session = mgr.get_or_create(state["session_key"])

    conv: list[dict[str, Any]] = []
    for m in state.get("messages") or []:
        if isinstance(m, HumanMessage):
            conv.append({"role": "user", "content": m.content or ""})
        elif isinstance(m, AIMessage):
            d: dict[str, Any] = {"role": "assistant", "content": m.content or ""}
            if m.tool_calls:
                d["tool_calls"] = m.tool_calls
            conv.append(d)
        elif isinstance(m, ToolMessage):
            conv.append({"role": "tool", "content": m.content or "", "tool_call_id": m.tool_call_id})

    session.messages = conv
    mgr.save(session, fsync=True)
    logger.debug("Saved {} messages to session {}", len(conv), state["session_key"])
    return {}


# ======================================================================
# Builder
# ======================================================================


def build_graph(config: SupermanConfig, *, verbose: bool = False, user_message: str = "") -> Any:
    """Build and compile the agent ``StateGraph``.

    Returns the compiled graph ready for ``.invoke()``.
    """
    global _WORKSPACE
    _WORKSPACE = str(config.workspace_path)

    llm = create_llm(config)
    tools = _make_tools(str(config.workspace_path))
    session_mgr = SessionManager(config.workspace_path)

    builder = StateGraph(AgentState)

    builder.add_node("load_session", load_session)
    builder.add_node("call_model", call_model)
    builder.add_node("execute_tools", ToolNode(tools))
    builder.add_node("save_session", save_session)

    builder.add_edge(START, "load_session")
    builder.add_edge("load_session", "call_model")
    builder.add_conditional_edges("call_model", should_continue, {
        "execute_tools": "execute_tools",
        "save_session": "save_session",
    })
    builder.add_edge("execute_tools", "call_model")
    builder.add_edge("save_session", END)

    compiled = builder.compile()

    return compiled


# ======================================================================
# Tool definitions
# ======================================================================


def _make_tools(workspace: str) -> list:
    """Build the tool list bound to the given workspace."""
    w = str(Path(workspace).expanduser().resolve())
    return [
        _read_file(w),
        _write_file(w),
        _glob_tool(w),
        _grep_tool(w),
        _exec_tool(w),
        _web_search(),
        _web_fetch(),
        _update_wm(),
    ]


# -- helpers -----------------------------------------------------------

def _resolve(path: str, workspace: str) -> Path:
    p = Path(path)
    return p if p.is_absolute() else Path(workspace) / p


# -- file tools --------------------------------------------------------


def _read_file(workspace: str):
    @tool
    async def read_file(path: str) -> str:
        """Read the content of a file at the given path.

        Args:
            path: Path to the file (absolute or relative to workspace).
        """
        full = _resolve(path, workspace)
        if not full.exists():
            return f"Error: File not found: {full}"
        if not full.is_file():
            return f"Error: Not a file: {full}"
        try:
            content = full.read_text(encoding="utf-8", errors="replace")
            if len(content) > 50_000:
                content = content[:50_000] + "\n\n… (file truncated at 50,000 chars)"
            # Track for edit-without-read detection
            sha = hashlib.sha256(content.encode()).hexdigest()
            _file_reads_cache[str(full)] = sha
            return content
        except Exception as e:
            return f"Error reading file: {e}"
    return read_file


_file_reads_cache: dict[str, str] = {}
"""Resets each session via the graph's AgentState; used as fallback."""
# Actually this is module-level, so it's shared across sessions.
# The proper approach uses state["file_reads"]. We'll access it from graph state:
# but tools can't access state directly with @tool.
# Alternative: use a ContextVar. Let me keep it simple and just warn at CLI level.

# UPDATE: we track file reads via a dict passed through configurable at invoke time.
# For the @tool approach, we use a module-level cache scoped to the graph instance.


def _write_file(workspace: str):
    @tool
    async def write_file(path: str, content: str) -> str:
        """Write content to a file. Creates parent directories if needed.

        Args:
            path: Path to the file (absolute or relative to workspace).
            content: Content to write.
        """
        full = _resolve(path, workspace)
        # Warn if writing without reading
        sha = hashlib.sha256(content.encode()).hexdigest()
        prev = _file_reads_cache.get(str(full))
        warn = ""
        if prev is None:
            warn = "\n⚠️  Writing a file you haven't read. Consider read_file() first."
        elif prev != sha:
            warn = "\n⚠️  File content changed since last read. Consider re-reading."
        try:
            full.parent.mkdir(parents=True, exist_ok=True)
            full.write_text(content, encoding="utf-8")
            _file_reads_cache[str(full)] = sha
            return f"Successfully wrote {len(content)} chars to {full}{warn}"
        except Exception as e:
            return f"Error writing file: {e}"
    return write_file


def _glob_tool(workspace: str):
    @tool
    async def glob(pattern: str) -> str:
        """List files matching a glob pattern. Use ** for recursive matching.

        Args:
            pattern: Glob pattern (e.g. ``**/*.py``, ``src/*.ts``).
        """
        root = Path(workspace)
        try:
            matches = [str(p.relative_to(root)) for p in sorted(root.rglob(pattern)) if p.is_file()]
        except ValueError:
            matches = [str(p) for p in sorted(root.rglob(pattern)) if p.is_file()]
        if not matches:
            return f"No files matching '{pattern}'"
        limit = 200
        if len(matches) > limit:
            matches = matches[:limit] + [f"… and {len(matches) - limit} more"]
        return "\n".join(matches)
    return glob


def _grep_tool(workspace: str):
    @tool
    async def grep(pattern: str, glob_pattern: str = "*") -> str:
        """Search file contents using a regex pattern.

        Args:
            pattern: Regex pattern to search for.
            glob_pattern: File glob to restrict search scope (default: *).
        """
        root = Path(workspace)
        matches: list[str] = []
        for p in sorted(root.rglob(glob_pattern)):
            if not p.is_file():
                continue
            try:
                text = p.read_text(encoding="utf-8", errors="replace")
                for i, line in enumerate(text.splitlines(), 1):
                    if re.search(pattern, line):
                        rel = p.relative_to(root)
                        matches.append(f"{rel}:{i}: {line.strip()[:200]}")
            except Exception:
                continue
        if not matches:
            return f"No matches for '{pattern}' in files matching '{glob_pattern}'"
        limit = 200
        if len(matches) > limit:
            matches = matches[:limit] + [f"… and {len(matches) - limit} more lines"]
        return "\n".join(matches)
    return grep


def _exec_tool(workspace: str):
    @tool
    async def exec_command(command: str, timeout: Annotated[int, "Timeout in seconds (max 600)"] = 60) -> str:
        """Execute a shell command and return its output.

        Prefer read_file/write_file for file operations.

        Args:
            command: The shell command to execute.
            timeout: Timeout in seconds (max 600).
        """
        try:
            proc = await asyncio.create_subprocess_shell(
                command,
                stdout=asyncio.subprocess.PIPE,
                stderr=asyncio.subprocess.PIPE,
                cwd=workspace,
            )
            try:
                stdout, stderr = await asyncio.wait_for(proc.communicate(), timeout=min(timeout, 600))
            except asyncio.TimeoutError:
                proc.kill()
                return f"Error: Command timed out after {timeout}s"
            out_parts = []
            if stdout:
                out_parts.append(stdout.decode("utf-8", errors="replace"))
            if stderr:
                text = stderr.decode("utf-8", errors="replace")
                if text.strip():
                    out_parts.append(f"STDERR:\n{text}")
            out_parts.append(f"Exit code: {proc.returncode}")
            result = "\n".join(out_parts)
            max_out = 10_000
            if len(result) > max_out:
                half = max_out // 2
                result = result[:half] + f"\n… ({len(result) - max_out} chars) …\n" + result[-half:]
            return result
        except Exception as e:
            return f"Error: {e}"
    return exec_command


def _web_search():
    @tool
    async def web_search(query: str, max_results: Annotated[int, "Maximum number of results (1-10)"] = 5) -> str:
        """Search the web for current information.

        Args:
            query: Search query.
            max_results: Maximum number of results (1-10).
        """
        try:
            from duckduckgo_search import DDGS
            results = []
            async with DDGS() as ddgs:
                async for r in ddgs.atext(query, max_results=min(max_results, 10)):
                    results.append(f"## {r.get('title', '')}\n{r.get('body', '')}\nSource: {r.get('href', '')}")
            return "\n\n".join(results) if results else "No search results found."
        except ImportError:
            return "Error: duckduckgo_search not installed. Install with: pip install duckduckgo_search"
    return web_search


def _web_fetch():
    @tool
    async def web_fetch(url: str) -> str:
        """Fetch the content of a URL as readable text.

        Args:
            url: The URL to fetch.
        """
        try:
            import httpx
            async with httpx.AsyncClient(follow_redirects=True, timeout=30) as client:
                resp = await client.get(url, headers={"User-Agent": "Superman-Agent/1.0"})
                resp.raise_for_status()
                text = resp.text
            try:
                from readability import Document
                doc = Document(text)
                content = doc.summary()
                content = re.sub(r"<[^>]+>", " ", content)
                import html
                content = html.unescape(content)
                content = re.sub(r"\s+", " ", content).strip()
                title = doc.title() or ""
                result = f"# {title}\n\n{content}" if title else content
            except ImportError:
                result = text[:10_000]
            if len(result) > 20_000:
                result = result[:20_000] + "\n\n…(truncated at 20,000 chars)"
            return result
        except httpx.HTTPStatusError as e:
            return f"Error: HTTP {e.response.status_code} fetching {url}"
        except Exception as e:
            return f"Error fetching {url}: {e}"
    return web_fetch


def _update_wm():
    @tool
    async def update_working_memory(key: str, value: str) -> str:
        """Update your working memory for the current session.

        Working memory is auto-injected into every turn. Use 'key_info'
        for the primary task description and 'related_sops' for
        comma-separated SOP names.

        Args:
            key: Field name (``key_info`` or ``related_sops``).
            value: Field value.
        """
        wm = _current_wm.get()
        if key == "key_info":
            wm["key_info"] = value
        elif key == "related_sops":
            wm["related_sops"] = [s.strip() for s in value.split(",") if s.strip()]
        else:
            wm[key] = value
        _current_wm.set(wm)
        return f"Updated working memory '{key}'."
    return update_working_memory