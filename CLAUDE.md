# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test

```bash
make extensions              # build all → ~/.config/piglet/extensions/
make extensions-<name>       # build one, e.g. make extensions-safeguard
make cli                     # build CLI tools → ~/go/bin/
make cli-<name>              # build one CLI, e.g. make cli-repomap
make clean                   # remove installed extensions
go test ./<name>/...         # test one extension
go test -run TestFoo ./memory/  # single test
```

## CLI Tools

Standalone command-line tools built from `cmd/`. Install with `make cli`.

| Tool | Description |
|------|-------------|
| `repomap` | Token-budgeted repo structure map with symbols |
| `pipeline` | Run multi-step YAML workflows with params, loops, retries, and conditionals |
| `bulk` | Run shell commands across many items (git repos, dirs, files) in parallel |
| `lspq` | Query language servers (go-to-def, refs, hover, rename, symbols) |
| `webfetch` | Fetch URLs as clean markdown or search the web (Jina, Brave, Exa) |
| `memory` | Per-project key-value fact store (get, set, list, delete) |
| `sift` | Pipe filter: collapse blanks, repeated lines, truncate large output |
| `fossil` | Git history queries: blame, changes, ownership, co-change, token-budgeted log |
| `confirm` | Minimal verify: affected package analysis → scoped typecheck + test + lint |
| `depgraph` | Dependency graph queries: deps, rdeps, impact, cycles, shortest path |
| `piglet-cron` | Run scheduled cron tasks (daemon mode for launchd) |

### repomap

```bash
repomap [flags] [directory]
  -tokens int     Token budget (default: 2048)
  -format string  compact|verbose|detail|lines (default: compact)
  -json           Output as JSON
```

### pipeline

```bash
pipeline [flags] <file.yaml>    # run a pipeline
pipeline list [directory]        # list available pipelines
  -dry-run          Preview without executing
  -param key=value  Parameter override (repeatable)
  -json             JSON output
  -q                Quiet mode
```

Pipeline YAML supports: `params` (with defaults/required), `steps` with `run`, `timeout`, `retries`, `retry_delay`, `allow_failure`, `each` (list iteration), `loop` (range/cartesian), `workdir`, `env`, `when` (conditional), `max_output` (byte truncation, 0=unlimited), `output_format` (text|json validation). Pipeline-level: `on_error` (halt|continue), `parallel` (concurrent step groups), `finally` (cleanup steps that always run). Template vars: `{param.<name>}`, `{prev.stdout}`, `{prev.json.<key>}`, `{prev.status}`, `{step.<name>.stdout}`, `{step.<name>.status}`, `{item}`, `{loop.<key>}`, `{cwd}`, `{date}`, `{timestamp}`.

### bulk

```bash
bulk [flags] <command>
  -git              Scan for git repos
  -dirs             Scan for directories
  -files            Scan for files by glob
  -list item1,item2 Explicit paths
  -root string      Root directory (default: .)
  -pattern string   Match pattern
  -filter string    Git filter (dirty/clean/ahead/behind/diverged) or shell predicate
  -depth int        Scan depth (default: 1)
  -j int            Concurrency (default: 8)
  -timeout int      Per-item timeout in seconds (default: 30)
  -dry-run          Collect and filter without executing
  -json             JSON output
```

Template vars in command: `{path}`, `{name}`, `{dir}`, `{basename}`.

### lspq

```bash
lspq <command> [flags] <file> [line] [symbol]
  def      Go to definition
  refs     Find all references
  hover    Get type info and docs
  rename   Rename symbol (-to <new-name>)
  symbols  List all symbols in a file
  -to string   New name (rename only)
  -col int     Column (1-based); auto-detected from symbol name
```

### webfetch

```bash
webfetch [flags] <url>           # fetch URL as markdown
webfetch search [flags] <query>  # search the web
  -raw          Fetch directly without reader provider
  -limit int    Max search results (default: 5)
  -json         Output as JSON
```

### memory

```bash
memory [-dir <path>] [-json] <command> [args]
  get <key>                    Get a fact
  set <key> <value> [category] Set a fact
  list [category]              List all facts
  delete <key>                 Delete a fact
  clear                        Clear all facts
  path                         Print backing file path
```

### sift

```bash
sift [flags] < input
  -threshold int          Minimum size before compression (default: 4096)
  -max-size int           Maximum output size (default: 32768)
  -no-collapse-blanks     Disable blank line collapsing
  -no-collapse-repeats    Disable repeated line collapsing
```

### fossil

```bash
fossil <command> [flags] [args]
  why <file>[:<start>-<end>]    Blame lines with commit messages
  changes [-since 7d] [-path p] Recent changes summary
  owners [-limit 10] [path]     Code ownership by commit frequency
  cochange [-limit 10] <file>   Files that change alongside <file>
  log [-tokens 1024] [path]     Token-budgeted git log
  -json                         Output as JSON (all except log)
```

### confirm

```bash
confirm [flags] [file ...]
  --changes file1,file2   Explicit changed files (comma-separated)
  --no-test               Skip tests
  --no-lint               Skip lint
  --json                  JSON output
```

### depgraph

```bash
depgraph <command> [flags] [args]
  deps <package>           Dependencies (what does this import?)
  rdeps <package>          Reverse deps (what imports this?)
  impact <file|package>    Blast radius of changes
  cycles                   Detect circular dependencies
  path <from> <to>         Shortest dependency path
  -depth int               Max traversal depth (0=unlimited)
  -tokens int              Token budget (0=unlimited)
  -json                    JSON output
```

### piglet-cron

```bash
piglet-cron run [flags]
  --verbose, -v   Enable info-level logging to stderr
  --task NAME     Run a specific task by name (implies force run, skips schedule check)
```

Intended for launchd: acquire a process lock on startup, run due tasks, then exit. Another instance running is not an error (exits 0).

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
