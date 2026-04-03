# LSP

Language server queries for go-to-definition, references, hover, rename, and symbols.

## Quick Start

```
# Ask the LLM to find a definition
"Where is the HandleRequest function defined?"

# The extension calls lsp_definition automatically
# Result: internal/server/handler.go:42
```

The extension contributes a prompt section that instructs the LLM to prefer `lsp_definition` and `lsp_references` over grep for code navigation. No configuration is required — servers are discovered from PATH on first use.

## What It Does

LSP connects to language server processes (gopls, typescript-language-server, etc.) via JSON-RPC 2.0 over stdin/stdout. It manages one server per language, starting them lazily on first use and keeping them alive for the session. The five registered tools expose definition lookup, reference finding, hover documentation, workspace-wide rename previews, and symbol listing.

## Capabilities

| Capability | Detail |
|------------|--------|
| `tools` | `lsp_definition`, `lsp_references`, `lsp_hover`, `lsp_rename`, `lsp_symbols` |
| `prompt` | Injects a "Code Intelligence (LSP)" section at order 40 |

## Configuration

No config file. Language servers are located via `PATH`. The prompt section content lives at `~/.config/piglet/extensions/lsp/prompt.md` — edit it to change the instructions injected into the system prompt.

### Supported Languages

| Extension(s) | Language | Server binary |
|---|---|---|
| `.go` | go | `gopls` |
| `.ts`, `.tsx` | typescript | `typescript-language-server --stdio` |
| `.js`, `.jsx` | javascript | `typescript-language-server --stdio` |
| `.py` | python | `pylsp` or `pyright-langserver --stdio` |
| `.rs` | rust | `rust-analyzer` |
| `.c`, `.h` | c | `clangd` |
| `.cpp`, `.cc`, `.cxx`, `.hpp` | cpp | `clangd` |
| `.java` | java | `jdtls` |
| `.lua` | lua | `lua-language-server` |
| `.zig` | zig | `zls` |

The server binary must be installed and in `PATH`. For languages with multiple candidates (e.g. Python), the first one found in `PATH` wins.

## Tools Reference

### `lsp_definition`

Go to the definition of a symbol. Returns the file and line where the symbol is defined, with surrounding context.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `file` | string | yes | File path (absolute or relative to project root) |
| `line` | integer | yes | Line number (1-based) |
| `column` | integer | no | Column number (1-based). Auto-detected if `symbol` is given. |
| `symbol` | string | no | Symbol name — used to auto-detect column when `column` is omitted |

```json
{
  "file": "cmd/main.go",
  "line": 42,
  "symbol": "HandleRequest"
}
```

---

### `lsp_references`

Find all references to a symbol across the codebase. Returns file paths, line numbers, and one line of context per reference.

Parameters are identical to `lsp_definition`.

```json
{
  "file": "internal/server/handler.go",
  "line": 15,
  "symbol": "Handler"
}
```

---

### `lsp_hover`

Get type information and documentation for the symbol at the given position.

Parameters are identical to `lsp_definition`.

```json
{
  "file": "internal/cache/cache.go",
  "line": 30,
  "symbol": "Set"
}
```

---

### `lsp_rename`

Rename a symbol across the entire workspace. Returns a preview of all changes — does not apply them. Use the LLM's editing tools to apply the edits after reviewing.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `file` | string | yes | File containing the symbol |
| `line` | integer | yes | Line number (1-based) |
| `column` | integer | no | Column number (1-based) |
| `symbol` | string | no | Symbol name for auto-column detection |
| `new_name` | string | yes | New name for the symbol |

```json
{
  "file": "internal/cache/cache.go",
  "line": 30,
  "symbol": "Set",
  "new_name": "Put"
}
```

Output format:
```
internal/cache/cache.go: 3 edit(s)
  line 30: "Put"
  line 85: "Put"
  line 102: "Put"

1 file(s), 3 edit(s) total
```

---

### `lsp_symbols`

List all symbols (functions, types, variables, constants) defined in a file. Hierarchical — struct methods appear nested under their type.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `file` | string | yes | File path (absolute or relative to project root) |

```json
{
  "file": "internal/cache/cache.go"
}
```

Output format:
```
struct Cache (line 12)
  method Get (line 25)
  method Set (line 30)
function New (line 8)
```

## How It Works (Developer Notes)

**Init sequence**: `Register` calls `e.OnInit(...)` to defer all setup until the host sends the CWD. Inside `OnInit`, `NewManager(x.CWD())` creates a `Manager` rooted at the project directory, and `RegisterPromptSection` injects the LSP instructions.

**Manager**: `Manager` holds one `*Client` per language ID. `ForFile(ctx, file)` maps the file extension to a language ID (via `extToLanguage`), then either returns an existing client or starts a new server. A `starting` map prevents duplicate starts when multiple tools are called concurrently on the same language.

**Client**: `Client` wraps an `exec.Cmd` and speaks JSON-RPC 2.0 over the process's stdin/stdout. A `readLoop` goroutine dispatches responses to waiting `callRaw` callers via a `pending` map of channels. The `writeMu` mutex serialises writes; `mu` protects the pending map.

**File open**: LSP servers require `textDocument/didOpen` before any position-based query. `EnsureFileOpen` sends this notification exactly once per file (tracked in `openFiles`).

**Symbol detection**: When `symbol` is supplied instead of `column`, `FindSymbolColumn` reads the file, locates the first occurrence of the symbol string on the target line, and returns the byte-to-rune offset as the column.

**Retry loop**: `ForFile` retries up to three times (200 ms apart) if another goroutine is in the middle of starting the same server.

**Prompt section order**: 40 — after skills (25) and memory (50 is memory; LSP at 40 sits between them so code navigation hints appear before general memory facts).

## Related Extensions

- [repomap](repomap.md) — structural overview of the repo; use before LSP for orientation
- [scaffold](scaffold.md) — creates new extension skeletons with proper boilerplate
