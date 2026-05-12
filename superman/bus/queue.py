"""Async message bus that decouples channels from the agent core."""

import asyncio

from superman.bus.events import InboundMessage, OutboundMessage


class MessageBus:
    """Simple pub/sub message bus for channel-agent communication."""

    def __init__(self) -> None:
        self._inbound: asyncio.Queue[InboundMessage] = asyncio.Queue()
        self._outbound: asyncio.Queue[OutboundMessage] = asyncio.Queue()

    async def publish_inbound(self, msg: InboundMessage) -> None:
        await self._inbound.put(msg)

    async def consume_inbound(self) -> InboundMessage:
        return await self._inbound.get()

    async def publish_outbound(self, msg: OutboundMessage) -> None:
        await self._outbound.put(msg)

    async def consume_outbound(self) -> OutboundMessage:
        return await self._outbound.get()

    @property
    def inbound_size(self) -> int:
        return self._inbound.qsize()

    @property
    def outbound_size(self) -> int:
        return self._outbound.qsize()