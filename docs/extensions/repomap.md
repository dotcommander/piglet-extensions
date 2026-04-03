# Repomap

Token-budgeted repository structure map with symbol extraction.

## Quick Start

```
# The map is injected automatically into every session's system prompt.
# To view or refresh it during a session:
"Show me the current repository map"
→ calls repomap_show

# After large refactors:
"Refresh the repository map"
→ calls repomap_refresh
```

On session start, repomap scans the project, extracts symbols from all source files, and injects a compact map into the system prompt. The LLM sees the codebase structure before the first message.

## What It Does

Repomap discovers source files (via `git ls-files` when inside a git repo, filesystem walk otherwise), parses symbols using a native Go parser for `.go` files and ctags/regex for everything else, then ranks files by importance. The ranked output is formatted to fit within a token budget and injected as a prompt section. After each turn where code-modifying tools ran, it checks file mtimes and rebuilds in the background if any file changed.

## Capabilities

| Capability | Detail |
|------------|--------|
| `tools` | `repomap_show`, `repomap_refresh`, `repomap_inventory` |
| `prompt` | Injects a "Repository Map" section at order 95 |
| `events` | Listens to `turn_end` to trigger staleness checks |

## Configuration

Config is read from `~/.config/piglet/config.yaml` under the `repomap` key:

```yaml
repomap:
  maxTokens: 1024       # token budget when files are in the conversation context
  maxTokensNoCtx: 2048  # token budget when no files are open (default prompt injection)
```

| Key | Default | Description |
|-----|---------|-------------|
| `maxTokens` | 1024 | Budget for the `repomap_show` compact format |
| `maxTokensNoCtx` | 2048 | Budget for the lines format injected into the system prompt |

The cache lives in `~/.config/piglet/cache/`. Repomap writes to it after each successful build and reads from it on startup for instant load. The cache is considered stale after 30 seconds if any tracked file's mtime has changed.

### Supported Languages

Go, TypeScript, JavaScript, Python, Rust, C/C++, Java, Lua, Zig, Ruby, Swift, Kotlin, PHP. Files larger than 50 KB are skipped.

### Skipped Directories

`.git`, `vendor`, `node_modules`, `__pycache__`, `.venv`, `build`, `dist`, `target`, `out`, `.next`, `.nuxt`, `coverage`, `.work`, `.tmp`, `testdata`, `.terraform`, and several others. See `scanner.go` for the complete list.

## Tools Reference

### `repomap_show`

Show the current repository structure map. Returns source-line format by default.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `verbose` | boolean | no | Show all symbols grouped by category (default: false) |
| `detail` | boolean | no | Show all symbols with full signatures (default: false) |

```json
{}
```

```json
{ "verbose": true }
```

```json
{ "detail": true }
```

The three output modes:
- **default (lines)**: Actual source definition lines, token-budgeted to `maxTokensNoCtx`. Most useful for LLM code navigation.
- **verbose**: All symbols grouped by kind, no token budget.
- **detail**: All symbols with full signatures and struct fields, no token budget.

---

### `repomap_refresh`

Force rebuild the repository map from disk. Use after creating or deleting files, or after major refactors.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `verbose` | boolean | no | Same as `repomap_show` |
| `detail` | boolean | no | Same as `repomap_show` |

```json
{}
```

Returns the newly built map in the requested format.

---

### `repomap_inventory`

Scan repository files for per-file metrics (line count, imports) and query the inventory.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `action` | string | yes | `"scan"` to rebuild from disk, `"query"` to filter existing data |
| `filter` | string | no | Filter expression for query (e.g. `"lines>100"`, `"path=internal/"`) |

```json
{ "action": "scan" }
```

```json
{ "action": "query", "filter": "lines>200" }
```

The inventory is persisted to the cache directory after each scan. The `query` action reads from the persisted inventory without re-scanning.

## How It Works (Developer Notes)

**Init sequence**: `e.OnInit` runs after the host sends CWD. It attempts a 5-second quick build. If the build finishes in time, the result is injected as a prompt section immediately. If it times out (large repo), a placeholder prompt section is registered and the build continues in a background goroutine. A disk cache avoids the quick-build cost on repeated sessions — if the cache is valid, it loads in milliseconds and the prompt section is populated synchronously.

**Stale detection**: After each `turn_end` event, `turnModifiedCode` inspects the event payload for tool names in `codeChangingTools` (`write_file`, `edit_file`, `bash`, etc.). If a match is found, `rm.Dirty()` flags the map stale regardless of mtime. The event handler then calls `rm.Stale()`, which checks file mtimes after a 30-second debounce. Rebuilds run in a goroutine — they never block the response.

**Parsing pipeline**: `Build` calls `ScanFiles` (git ls-files or walk), then parses Go files with the native `go/ast` parser and non-Go files with ctags (if available) or regex patterns. Both parsers run in parallel bounded by `runtime.NumCPU()`. Output is lazy — format strings are computed on first access and cached.

**Token budgeting**: `FormatMap` (compact/verbose/detail) and `FormatLines` (lines) accept a max-token argument and truncate ranked output to fit. Files are ranked by symbol count, export ratio, and path depth.

**Cache format**: Binary-encoded `RankedFile` slices written via `encoding/gob` to the cache directory, keyed by the absolute project path.

**Prompt section order**: 95 — near the end of the system prompt, after skills and memory, so the map serves as reference context rather than primary instruction.

**ErrNotCodeProject**: Returned by `Build` when no supported source files are found. Callers check for this error to avoid logging spurious warnings in non-code directories.

## Related Extensions

- [lsp](lsp.md) — precise symbol navigation; complements repomap's structural overview
- [depgraph](../../README.md) — dependency graph queries on top of the same file inventory
