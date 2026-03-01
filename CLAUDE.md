# Forge — Development Guidelines

## Test-Driven Development

All code changes must follow TDD:

1. **Write tests first** — before implementing any new feature or fixing a bug, write failing tests that define the expected behavior.
2. **Run tests to confirm they fail** — verify the tests fail for the right reason.
3. **Implement the code** — write the minimum code to make the tests pass.
4. **Run all tests** — ensure nothing is broken (`go test ./...`).
5. **Refactor** — clean up with confidence, re-running tests after each change.

For web UI changes, use Playwright scripts to verify behavior after Go/Svelte tests pass.

## Build & Test Commands

```bash
go test ./...              # Run all Go tests
cd web && npm run build    # Build Svelte frontend
go build -o bin/forge ./cmd/forge  # Build binary with embedded assets
make all                   # Build everything (CLI + tool servers)
```

## Project Layout

- `internal/server/` — HTTP handlers, WebSocket, session manager (has tests)
- `internal/agent/` — ReAct loop (has tests)
- `internal/storage/sqlite/` — persistence (has tests)
- `internal/tools/` — MCP registry (has tests)
- `web/src/` — Svelte frontend (Playwright for testing)
