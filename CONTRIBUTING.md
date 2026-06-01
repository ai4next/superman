# Contributing

Thank you for contributing to Superman. Superman is a Go-based autonomous AI agent CLI with a terminal UI, persistent sessions, tools, expert delegation, runtime audit logs, instant-messaging integrations, and layered self-evolution. Contributions should keep boundaries clear, behavior testable, and configuration compatible.

## Development Environment

Recommended setup:

- Go 1.26 or later
- Git
- macOS, Linux, or Windows

Clone the repository and verify dependencies:

```bash
git clone https://github.com/ai4next/superman.git
cd superman
go mod download
go test ./...
```

Run locally:

```bash
go run . --help
go run . run "What's in this directory?"
```

Build a local binary:

```bash
go build -o sm .
```

## Configuration And Secrets

The example configuration is available at `config.example.yaml`. To create a local config file, run:

```bash
go run . init
```

Do not commit real API keys, tokens, session databases, runtime logs, or locally generated data. Inject secrets through environment variables, for example:

```bash
export OPENAI_API_KEY=sk-...
```

If you add a configuration option, update the relevant files:

- `internal/config/config.go`
- `internal/config/config.example.yaml`
- `README.md`
- `README-zh.md`

## Development Workflow

1. Create a branch from the latest `main`:

```bash
git checkout main
git pull
git checkout -b feat/short-description
```

2. Keep changes focused. One PR should solve one problem or deliver one tightly related set of changes.

3. Before committing, format and test:

```bash
gofmt -w .
go test ./...
```

4. If your change affects CLI behavior, configuration, installation, or user-visible features, update the README or related documentation.

## Code Guidelines

- Follow standard Go style and run `gofmt`.
- Prefer existing package structure and helpers over new abstractions for small local changes.
- Keep error messages clear so CLI users can diagnose problems.
- Preserve backward compatibility for persisted files, sessions, memory, and audit logs.
- Do not make tests depend on real external model services, real messaging platforms, or private local user data.
- For concurrency, queues, session compaction, and file writes, pay close attention to cancellation, timeouts, error propagation, and data consistency.

## Testing

Add or update tests near the package whose behavior changed. Common commands:

```bash
go test ./...
go test ./internal/session
go test ./internal/cli -run TestName
```

If a change cannot be fully covered by automated tests, describe the manual verification steps and remaining risk in the PR.

## Commit Messages

Use short imperative commit messages:

```text
add session queue inspection
fix config env override
update install script
```

If a change is breaking, explain the migration path in the commit message or PR description.

## Pull Requests

Before opening a PR, confirm that:

- `gofmt -w .` has been run
- `go test ./...` passes, or failures are explained
- User-visible behavior is documented
- New configuration options include examples
- No API keys, tokens, databases, logs, or local temporary files are committed

PR descriptions should include:

- Purpose of the change
- Main implementation details
- Test results
- Compatibility or migration notes

## Release

Releases are handled by GitHub Actions when a `v*` tag is pushed. The workflow builds `sm` binaries for Linux, macOS, and Windows. Most contributors do not need to publish releases manually.

Maintainer release example:

```bash
git tag v0.0.1
git push origin v0.0.1
```

## Reporting Issues

When opening an issue, include as much of the following as possible:

- Operating system and terminal environment
- Superman version or commit
- Steps to reproduce
- Expected behavior and actual behavior
- Relevant configuration snippets with secrets removed
- Relevant error logs or command output

Clear reproduction details make issues much easier to diagnose.
