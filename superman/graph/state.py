"""Agent graph state — the single source of truth for every turn."""

from __future__ import annotations

from typing import Annotated, Sequence, TypedDict

from langgraph.graph.message import add_messages
from langchain_core.messages import BaseMessage


class AgentState(TypedDict):
    """State flowing through the LangGraph agent.

    ``messages`` is the conversation history (HumanMessage / AIMessage /
    ToolMessage only — SystemMessage is assembled fresh each turn in
    the ``call_model`` node).

    ``file_reads`` maps resolved absolute paths to their SHA-256 content
    hash, tracked per session so the write tool can warn on
    edit-without-read.
    """

    messages: Annotated[Sequence[BaseMessage], add_messages]
    session_key: str
    working_memory: dict  # {"key_info": …, "related_sops": […]}
    file_reads: dict  # {"path": "sha256"}
    turn_count: int
    session_loaded: bool  # False on first turn, True after load_session