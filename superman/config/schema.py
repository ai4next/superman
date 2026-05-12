"""Configuration schema using Pydantic."""

from pathlib import Path
from typing import Literal

from pydantic import BaseModel, Field


class LLMConfig(BaseModel):
    """LLM provider configuration."""

    provider: str = "anthropic"  # "anthropic" | "openai" | "custom"
    model: str = "claude-sonnet-4-20250514"
    api_key: str | None = None
    api_base: str | None = None
    max_tokens: int = 8192
    temperature: float = 0.7


class ToolsConfig(BaseModel):
    """Tool configuration."""

    enable_exec: bool = True
    exec_timeout: int = 60
    restrict_to_workspace: bool = False
    enable_web: bool = True
    web_search_provider: str = "duckduckgo"
    web_search_api_key: str = ""
    max_tool_result_chars: int = 10_000


class SupermanConfig(BaseModel):
    """Root configuration."""

    llm: LLMConfig = Field(default_factory=LLMConfig)
    tools: ToolsConfig = Field(default_factory=ToolsConfig)
    workspace: str = "~/.superman/workspace"
    max_tool_iterations: int = 25
    max_session_messages: int = 120
    timezone: str = "UTC"

    @property
    def workspace_path(self) -> Path:
        return Path(self.workspace).expanduser().resolve()


def load_config(config_path: Path | None = None) -> SupermanConfig:
    """Load config from YAML/JSON file, or return defaults."""
    import json

    path = config_path or _default_config_path()
    if path.exists():
        raw = path.read_text(encoding="utf-8")
        if path.suffix in (".yaml", ".yml"):
            import yaml

            data = yaml.safe_load(raw)
        else:
            data = json.loads(raw)
        return SupermanConfig.model_validate(data)
    return SupermanConfig()


def save_config(config: SupermanConfig, config_path: Path | None = None) -> None:
    """Save config to file."""
    import json

    path = config_path or _default_config_path()
    path.parent.mkdir(parents=True, exist_ok=True)
    data = config.model_dump(mode="json")
    path.write_text(json.dumps(data, indent=2, ensure_ascii=False), encoding="utf-8")


def _default_config_path() -> Path:
    return Path.home() / ".superman" / "config.json"