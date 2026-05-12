"""Session management for conversation history.

Sessions are stored as JSONL files in the workspace ``sessions/``
directory with atomic writes for safety.
"""

from __future__ import annotations

import json
import os
import shutil
from contextlib import suppress
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Any

from loguru import logger


def _safe_filename(key: str) -> str:
    """Derive a filesystem-safe name from a session key."""
    safe = key.replace(":", "_").replace("/", "_").replace(" ", "_")
    return "".join(c for c in safe if c.isalnum() or c in "_-.")


@dataclass
class Session:
    """A conversation session."""

    key: str
    messages: list[dict[str, Any]] = field(default_factory=list)
    created_at: datetime = field(default_factory=datetime.now)
    updated_at: datetime = field(default_factory=datetime.now)
    metadata: dict[str, Any] = field(default_factory=dict)

    def add_message(self, role: str, content: str, **kwargs: Any) -> None:
        self.messages.append({
            "role": role,
            "content": content,
            "timestamp": datetime.now().isoformat(),
            **kwargs,
        })
        self.updated_at = datetime.now()

    def get_history(self, max_messages: int = 100) -> list[dict[str, Any]]:
        """Return recent messages (slice from tail)."""
        if max_messages <= 0:
            max_messages = 100
        return list(self.messages[-max_messages:])

    def clear(self) -> None:
        self.messages.clear()
        self.updated_at = datetime.now()


class SessionManager:
    """Manages conversation sessions with JSONL file persistence."""

    def __init__(self, workspace: Path) -> None:
        self._sessions_dir = workspace / "sessions"
        self._sessions_dir.mkdir(parents=True, exist_ok=True)
        self._cache: dict[str, Session] = {}

    def _path(self, key: str) -> Path:
        return self._sessions_dir / f"{_safe_filename(key)}.jsonl"

    def get_or_create(self, key: str) -> Session:
        if key in self._cache:
            return self._cache[key]
        session = self._load(key)
        if session is None:
            session = Session(key=key)
        self._cache[key] = session
        return session

    def _load(self, key: str) -> Session | None:
        path = self._path(key)
        if not path.exists():
            return None
        try:
            messages: list[dict[str, Any]] = []
            meta: dict[str, Any] = {}
            created_at = updated_at = None
            for line in path.read_text(encoding="utf-8").splitlines():
                line = line.strip()
                if not line:
                    continue
                data = json.loads(line)
                if data.get("_type") == "metadata":
                    meta = data.get("metadata", {})
                    with suppress(Exception):
                        created_at = datetime.fromisoformat(data["created_at"])
                    with suppress(Exception):
                        updated_at = datetime.fromisoformat(data["updated_at"])
                else:
                    messages.append(data)
            return Session(
                key=key,
                messages=messages,
                created_at=created_at or datetime.now(),
                updated_at=updated_at or datetime.now(),
                metadata=meta,
            )
        except Exception as e:
            logger.warning("Failed to load session {}: {}", key, e)
            return None

    def save(self, session: Session, *, fsync: bool = False) -> None:
        path = self._path(session.key)
        tmp = path.with_suffix(".jsonl.tmp")
        try:
            with open(tmp, "w", encoding="utf-8") as f:
                meta = {
                    "_type": "metadata",
                    "key": session.key,
                    "created_at": session.created_at.isoformat(),
                    "updated_at": session.updated_at.isoformat(),
                    "metadata": session.metadata,
                }
                f.write(json.dumps(meta, ensure_ascii=False) + "\n")
                for msg in session.messages:
                    f.write(json.dumps(msg, ensure_ascii=False) + "\n")
                if fsync:
                    f.flush()
                    os.fsync(f.fileno())
            os.replace(tmp, path)
            if fsync:
                with suppress(PermissionError):
                    fd = os.open(str(path.parent), os.O_RDONLY)
                    try:
                        os.fsync(fd)
                    finally:
                        os.close(fd)
        except BaseException:
            tmp.unlink(missing_ok=True)
            raise
        self._cache[session.key] = session

    def flush_all(self) -> int:
        flushed = 0
        for key, session in list(self._cache.items()):
            try:
                self.save(session, fsync=True)
                flushed += 1
            except Exception:
                logger.warning("Failed to flush session {}", key, exc_info=True)
        return flushed

    def invalidate(self, key: str) -> None:
        self._cache.pop(key, None)

    def delete_session(self, key: str) -> bool:
        path = self._path(key)
        self.invalidate(key)
        if path.exists():
            path.unlink()
            return True
        return False

    def list_sessions(self) -> list[dict[str, Any]]:
        sessions = []
        for path in sorted(self._sessions_dir.glob("*.jsonl")):
            try:
                line = path.read_text(encoding="utf-8").splitlines()[0]
                data = json.loads(line)
                if data.get("_type") == "metadata":
                    sessions.append({
                        "key": data.get("key", path.stem),
                        "created_at": data.get("created_at"),
                        "updated_at": data.get("updated_at"),
                        "title": (data.get("metadata") or {}).get("title", ""),
                    })
            except Exception:
                continue
        return sorted(sessions, key=lambda s: s.get("updated_at", ""), reverse=True)