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

Extensions are standalone binaries communicating via JSON-RPC over stdin/stdout using the Go SDK (`github.com/dotcommander/piglet/sdk`). Core logic lives in the package root — `cmd/main.go` bridges SDK types to business logic.

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
- **Config from `~/.config/piglet/`**: Read via `e.ConfigReadExtension()` (host protocol) or `os.UserConfigDir()`. Never hardcode behavioral content in Go source.
- **Host protocol methods (v3)**: Extensions can call `e.ConfigGet()`, `e.ConfigReadExtension()`, `e.AuthGetKey()`, `e.Chat()`, and `e.RunAgent()` — the host handles config, auth, LLM calls, and agent loops. No direct piglet imports needed.
- **Prompt section ordering**: Lower `Order` = earlier in system prompt. Skills=25, memory=50, rtk=90.
- **Interceptor priority**: Higher = runs first. Safeguard=2000 (security), RTK=100 (rewriting).
- **Atomic file writes**: Memory store writes temp file then renames.

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

### Pre-Commit Gate

Before EVERY commit to this repo:

1. **`git diff --cached`** — read the full staged diff. Look for hardcoded paths, API keys, user-specific config, or debug print statements.
2. **`git diff --cached --name-only`** — verify no unexpected files (binaries, configs, scratch).
3. **No `~/.config/` paths** — grep the diff for `/Users/`, `~/.config/`, absolute home directories. Zero tolerance.
4. **No test scripts in repo** — `/tmp/` test files stay in `/tmp/`. Never stage them.

### Pre-Tag Gate (before `git tag`)

1. **All tests pass**: `go test -race ./... | tail -30`
2. **Build clean**: `go build ./... 2>&1 | tail -10`
3. **go.mod current**: `go mod tidy && go build ./...` — stale deps break consumers
4. **Smoke test**: Build at least one extension and run it against a real repo
5. **No WIP commits**: `git log v<prev>..HEAD --oneline` — every commit should be shippable

### Pre-Push Gate

1. **Review commit list**: `git log origin/main..HEAD --oneline` — every commit is about to become public
2. **No force push to main** — ever
3. **No amended published commits** — create new commits to fix mistakes
