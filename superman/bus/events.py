"""Async message queue for decoupled agent communication."""

import asyncio
from dataclasses import dataclass, field
from typing import Any


@dataclass
class InboundMessage:
    """Message received from a channel (CLI, API, etc.)."""

    channel: str
    sender_id: str
    chat_id: str
    content: str
    media: list[str] = field(default_factory=list)
    metadata: dict[str, Any] = field(default_factory=dict)

    @property
    def session_key(self) -> str:
        return f"{self.channel}:{self.chat_id}"


@dataclass
class OutboundMessage:
    """Message to send back to a channel."""

    channel: str
    chat_id: str
    content: str
    media: list[str] = field(default_factory=list)
    metadata: dict[str, Any] = field(default_factory=dict)