"""CLI commands for Superman."""

from __future__ import annotations

import asyncio
import signal
import sys
from pathlib import Path
from typing import Any

import typer
from loguru import logger
from prompt_toolkit import PromptSession
from prompt_toolkit.formatted_text import HTML
from prompt_toolkit.history import FileHistory
from rich.console import Console
from rich.markdown import Markdown
from rich.table import Table

from superman import __logo__, __version__
from superman.config.schema import SupermanConfig, load_config, save_config
from superman.graph.agent import build_graph

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------

logger.remove()
logger.add(
    sys.stderr,
    format="<green>{time:YYYY-MM-DD HH:mm:ss}</green> | <level>{level: <5}</level> | <level>{message}</level>",
    level="INFO",
    colorize=None,
)

console = Console()
EXIT_COMMANDS = {"exit", "quit", "/exit", "/quit", ":q"}

app = typer.Typer(
    name="superman",
    help=f"{__logo__} Superman - General-Purpose AI Agent",
    no_args_is_help=True,
)


def _version_callback(value: bool) -> None:
    if value:
        console.print(f"{__logo__} Superman v{__version__}")
        raise typer.Exit()


@app.callback()
def main(
    version: bool = typer.Option(None, "--version", "-v", callback=_version_callback, is_eager=True),
) -> None:
    pass


# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------


@app.command()
def config(
    show: bool = typer.Option(False, "--show", "-s", help="Show current configuration"),
    init: bool = typer.Option(False, "--init", help="Initialize default config"),
    workspace: str | None = typer.Option(None, "--workspace", "-w", help="Workspace directory"),
    model: str | None = typer.Option(None, "--model", "-m", help="Model name"),
    provider: str | None = typer.Option(None, "--provider", "-p", help="LLM provider"),
    api_key: str | None = typer.Option(None, "--api-key", help="API key"),
):
    """View or edit Superman configuration."""
    if init:
        cfg = SupermanConfig()
        if workspace:
            cfg.workspace = workspace
        if model:
            cfg.llm.model = model
        if provider:
            cfg.llm.provider = provider
        if api_key:
            cfg.llm.api_key = api_key
        save_config(cfg)
        console.print("[green]Config initialized at ~/.superman/config.json[/green]")
        return

    if show:
        cfg = load_config()
        table = Table(title="Superman Configuration")
        table.add_column("Key", style="cyan")
        table.add_column("Value")
        table.add_row("Provider", cfg.llm.provider)
        table.add_row("Model", cfg.llm.model)
        table.add_row("API Key", cfg.llm.api_key[:8] + "..." if cfg.llm.api_key else "(not set)")
        table.add_row("API Base", cfg.llm.api_base or "(default)")
        table.add_row("Workspace", cfg.workspace)
        table.add_row("Max Iterations", str(cfg.max_tool_iterations))
        table.add_row("Max Session Messages", str(cfg.max_session_messages))
        console.print(table)
        return

    console.print("Use [cyan]--show[/cyan] to view config or [cyan]--init[/cyan] to create default config.")


# ---------------------------------------------------------------------------
# Chat
# ---------------------------------------------------------------------------


@app.command()
def chat(
    message: str = typer.Option(None, "--message", "-m", help="Send a single message (one-shot mode)"),
    session: str = typer.Option("cli:direct", "--session", "-s", help="Session ID"),
    config_path: str | None = typer.Option(None, "--config", "-c", help="Config file path"),
    verbose: bool = typer.Option(False, "--verbose", "-v", help="Show verbose logs"),
):
    """Interactive or one-shot chat with the agent."""
    if verbose:
        logger.enable("superman")
    else:
        logger.disable("superman")

    cfg = _load_cfg(config_path)
    cfg.workspace_path.mkdir(parents=True, exist_ok=True)

    graph = build_graph(cfg, verbose=verbose)

    if message:
        asyncio.run(_run_once(graph, message, session))
    else:
        asyncio.run(_run_interactive(graph, cfg, session))


def _load_cfg(config_path: str | None) -> SupermanConfig:
    path = Path(config_path).expanduser().resolve() if config_path else None
    return load_config(path)


async def _run_once(graph: Any, message: str, session_key: str) -> None:
    initial = {
        "messages": [],
        "session_key": session_key,
        "working_memory": {},
        "file_reads": {},
        "turn_count": 0,
        "session_loaded": False,
    }
    result = await graph.ainvoke(initial)
    if result.get("messages"):
        response = result["messages"][-1].content
        if response:
            console.print(Markdown(str(response)))


async def _run_interactive(graph: Any, cfg: SupermanConfig, session_key: str = "cli:direct") -> None:
    console.print(f"{__logo__} [bold]Superman[/bold] — {cfg.llm.model}")
    console.print("Type [bold]exit[/bold] or [bold]Ctrl+C[/bold] to quit\n")

    history_dir = Path.home() / ".superman"
    history_dir.mkdir(parents=True, exist_ok=True)
    prompt_session = PromptSession(history=FileHistory(str(history_dir / "history.txt")))

    # Graph state persists across turns
    state = {
        "messages": [],
        "session_key": session_key,
        "working_memory": {},
        "file_reads": {},
        "turn_count": 0,
        "session_loaded": False,
    }

    async def _process_input(user_input: str) -> None:
        nonlocal state
        # Inject user message into state
        from langchain_core.messages import HumanMessage

        state["messages"] = list(state.get("messages") or []) + [HumanMessage(content=user_input)]
        result = await graph.ainvoke(state)
        state = result  # persist updated state for next turn

        if result.get("messages"):
            response = result["messages"][-1].content
            if response:
                console.print()
                console.print(f"[bold cyan]{__logo__} Superman[/bold cyan]")
                console.print(Markdown(str(response)))
                console.print()

    def _handle_sigint(signum: int, frame: Any) -> None:
        console.print("\nGoodbye!")
        sys.exit(0)

    signal.signal(signal.SIGINT, _handle_sigint)

    try:
        while True:
            try:
                user_input = await prompt_session.prompt_async(HTML("<b>You: </b>"))
            except EOFError:
                console.print("\nGoodbye!")
                break

            cmd = user_input.strip()
            if not cmd:
                continue
            if cmd.lower() in EXIT_COMMANDS:
                console.print("Goodbye!")
                break
            if cmd.startswith("/"):
                # Simple inline commands
                match cmd.lower():
                    case "/clear" | "/new":
                        state["messages"] = []
                        state["turn_count"] = 0
                        console.print("Session cleared.")
                        continue
                    case "/help" | "/?":
                        console.print("Commands: /clear /new /help /? /status /exit /quit")
                        continue
                    case "/status":
                        console.print(f"Messages: {len(state.get('messages', []))}\nModel: {cfg.llm.model}")
                        continue

            await _process_input(cmd)
    finally:
        from superman.session.manager import SessionManager
        mgr = SessionManager(cfg.workspace_path)
        mgr.flush_all()


if __name__ == "__main__":
    app()