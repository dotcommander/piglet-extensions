# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test

```bash
make extensions              # build all → ~/.config/piglet/extensions/
make extensions-<name>       # build one, e.g. make extensions-safeguard
make clean                   # remove installed extensions
go test ./<name>/...         # test one extension
go test -run TestFoo ./memory/  # single test
```


## Architecture

### SDK-Only Architecture

Extensions are standalone binaries communicating via JSON-RPC over FD 3/4 (falls back to stdin/stdout) using the Go SDK (`github.com/dotcommander/piglet/sdk`). Core logic lives in the package root — `cmd/main.go` bridges SDK types to business logic.

### Extension Capabilities

Declared in `cmd/manifest.yaml`:

| Capability | What it registers |
|------------|------------------|
| `interceptors` | Before/after hooks on tool calls |
| `tools` | LLM-callable tools |
| `commands` | Slash commands |
| `prompt` | System prompt sections (ordered by priority) |
| `eventHandlers` | Lifecycle event observers |
| `shortcuts` | Keyboard shortcuts |
| `messageHooks` | Pre/post message processing |

### Key Patterns

- **OnInit for CWD-dependent state**: External binaries use `ext.OnInit(func(e *sdk.Extension) { ... })` to initialize after the host sends CWD. See `safeguard/cmd/main.go` and `memory/cmd/main.go`.
- **No hardcoded strings (BLOCKING)**: Prompts, defaults, templates, and behavioral content must never live in Go source. If the config file is missing, create it with defaults via atomic write (`WriteFile` tmp + `Rename`), then read it back. Go code reads config; it does not contain config.
- **Host protocol methods (v4)**: Extensions can call `e.ConfigGet()`, `e.ConfigReadExtension()`, `e.AuthGetKey()`, `e.Chat()`, and `e.RunAgent()` — the host handles config, auth, LLM calls, and agent loops. No direct piglet imports needed.
- **Prompt section ordering**: Lower `Order` = earlier in system prompt. Skills=25, memory=50, rtk=90.
- **Interceptor priority**: Higher = runs first. Safeguard=2000 (security), RTK=100 (rewriting).
- **Atomic file writes**: Memory store and cache library write temp file then rename via `xdg.WriteFileAtomic`.

## Release Safety (BLOCKING — public repo)

This is a **public repository**. Every commit and tag is visible to the world. Follow these gates strictly.

### Never Commit

| Category | Examples |
|----------|---------|
| API keys / secrets | `.env`, `auth.json`, tokens, passwords |
| User config | `~/.config/piglet/config.yaml`, `models.yaml`, session files |
| Local paths | `/Users/<name>/`, `~/www/`, absolute paths to user directories |
| Scratch / work | `.work/`, `/tmp/` test scripts, journal notes |
| Binary artifacts | Compiled extension binaries, `.so`, `.dylib` |
| `replace` directives | `go.mod` must never contain `replace` — use Go workspace (`go.work`) for local dev |

### Pre-Commit Gate

Before EVERY commit to this repo:

1. **`git diff --cached`** — read the full staged diff. Look for hardcoded paths, API keys, user-specific config, or debug print statements.
2. **`git diff --cached --name-only`** — verify no unexpected files (binaries, configs, scratch).
3. **No `~/.config/` paths** — grep the diff for `/Users/`, `~/.config/`, absolute home directories. Zero tolerance.
4. **No test scripts in repo** — `/tmp/` test files stay in `/tmp/`. Never stage them.

### Pre-Tag Gate (before `git tag`)

0. **SDK version current**: `go list -m github.com/dotcommander/piglet/sdk` — must match the latest `sdk/v*` tag in the piglet repo. If the piglet SDK has unpublished commits that extensions depend on, **stop** — tag a new SDK version in piglet first.
1. **All tests pass**: `go test -race ./... | tail -30`
2. **Build clean**: `go build ./... 2>&1 | tail -10`
3. **go.mod current**: `go mod tidy && go build ./...` — stale deps break consumers
4. **Smoke test**: Build at least one extension and run it against a real repo
5. **No WIP commits**: `git log v<prev>..HEAD --oneline` — every commit should be shippable

### Pre-Push Gate

1. **Review commit list**: `git log origin/main..HEAD --oneline` — every commit is about to become public
2. **No force push to main** — ever
3. **No amended published commits** — create new commits to fix mistakes

### Go Workspace (local dev)

Local multi-module development uses a Go workspace file (`go.work`) in the parent directory (outside both repos). This replaces `replace` directives in `go.mod`.

- **`go.work`** resolves `piglet` and `piglet/sdk` to local source during development
- **`go.mod`** always points to published module versions — safe for `go install` and `piglet update`
- **`GOWORK=off`** disables the workspace for testing against published modules: `GOWORK=off go build ./...`
- **Never commit `replace` directives** — they break every consumer that isn't on your machine

## Violation Log

| Rule | Violations | Last |
|------|-----------|------|
| No replace directives: committed `replace ../piglet` to public repo, broke `piglet update` | 1 | 2026-03-26 |
| SDK version current: tagged extensions v0.5.1 against stale sdk/v1.2.0, all extensions failed to start | 1 | 2026-03-26 |
