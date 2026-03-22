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

Requires `../piglet` as a sibling directory (see `replace` directive in go.mod).

## Architecture

### Dual Registration Pattern

Each extension has two entry points: `<name>/register.go` (library, `ext.App` API) and `cmd/main.go` (binary, `sdk/go` over JSON-RPC stdin/stdout). Core logic lives in the package root — both entry points call the same functions.

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
- **Config from `~/.config/piglet/`**: Read via `config.ReadExtensionConfig()` or `config.ConfigDir()`. Never hardcode behavioral content in Go source.
- **Prompt section ordering**: Lower `Order` = earlier in system prompt. Skills=25, memory=50, rtk=90.
- **Interceptor priority**: Higher = runs first. Safeguard=2000 (security), RTK=100 (rewriting).
- **Atomic file writes**: Memory store writes temp file then renames.
