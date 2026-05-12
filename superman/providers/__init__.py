"""LLM provider factory using LangChain's BaseChatModel integrations.

We delegate to langchain-anthropic and langchain-openai rather than
implementing our own provider abstraction.  This keeps the surface small:
simply create the right model from config.
"""

from __future__ import annotations

from typing import Any

from langchain_core.language_models.chat_models import BaseChatModel

from superman.config.schema import SupermanConfig


def create_llm(config: SupermanConfig) -> BaseChatModel:
    """Create a LangChain chat model from the application config.

    Supports ``anthropic``, ``openai``, and ``custom`` (OpenAI-compatible)
    providers.
    """
    llm_cfg = config.llm
    provider = llm_cfg.provider.lower()
    kwargs: dict[str, Any] = {
        "model": llm_cfg.model,
        "temperature": llm_cfg.temperature,
        "max_tokens": llm_cfg.max_tokens,
    }
    if llm_cfg.api_key:
        kwargs["api_key"] = llm_cfg.api_key
    if llm_cfg.api_base:
        kwargs["base_url"] = llm_cfg.api_base

    if provider == "anthropic":
        from langchain_anthropic import ChatAnthropic

        return ChatAnthropic(**kwargs, default_headers={"anthropic": "true"})
    elif provider == "openai":
        from langchain_openai import ChatOpenAI

        return ChatOpenAI(**kwargs)
    elif provider == "custom":
        from langchain_openai import ChatOpenAI

        return ChatOpenAI(**kwargs)
    else:
        raise ValueError(f"Unknown provider: {provider}. Supported: anthropic, openai, custom")