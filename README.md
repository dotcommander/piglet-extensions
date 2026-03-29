# piglet-extensions

External extensions for [piglet](https://github.com/dotcommander/piglet), a minimalist TUI coding assistant.

Each extension runs as a standalone binary, communicating with piglet over its extension API. Extensions can register tools, commands, shortcuts, prompt sections, interceptors, and event handlers.

## Extensions

| Extension | Capability | Description |
|-----------|-----------|-------------|
| **safeguard** | interceptor | Blocks dangerous commands (`rm -rf`, `DROP TABLE`, `git push --force`, etc.) with configurable profiles: strict, balanced, off. Audit logs decisions to JSONL. |
| **rtk** | interceptor, prompt | Integrates [RTK](https://github.com/reachingforthejack/rtk) to rewrite bash commands for 60-90% token savings. Auto-detects if `rtk` is in PATH. |
| **autotitle** | event handler | Generates short session titles from the first user/assistant exchange using a lightweight LLM call. |
| **clipboard** | tool, shortcut | Reads images from the macOS clipboard and injects them into the conversation as base64. |
| **skill** | tool, command, prompt, message hook | Manages markdown methodology files in `~/.config/piglet/skills/`. Load skills into conversations with `/skill <name>`. |
| **memory** | tool, command, prompt | Project-scoped fact storage with automatic extraction. Persists key-value facts and injects them into the system prompt. |
| **subagent** | tool | Dispatches independent sub-agents with configurable tool access, model override, and turn limits. |
| **lsp** | tool, prompt | LSP client for code intelligence -- go-to-definition, references, hover, rename, and document symbols via JSON-RPC 2.0. |
| **repomap** | tool, prompt, event | Builds a ranked repository map by scanning, parsing, and ranking symbols. Auto-rebuilds when files change. |
| **plan** | tool, command, prompt | Persistent structured task tracking with step statuses (pending, in_progress, done, skipped, failed) and propose/execute modes. |
| **bulk** | tool | Runs commands across multiple targets (git repos, directories, files, or lists) with parallel execution and filtering. |
| **cache** | library | File-backed TTL cache (`cache.Get`/`cache.Set`) importable by any extension. Entries stored as JSON under `~/.config/piglet/cache/`. Used by webfetch for persistent URL/search caching. |
| **mcp** | tool, command, prompt | Connects to configured MCP servers (stdio or HTTP/SSE) and exposes their tools as piglet tools. Config in `~/.config/piglet/mcp.yaml`. |

## CLI Tools

Standalone command-line tools built from `cmd/`. These are independent binaries with no piglet dependency. Install all to `~/go/bin/`:

```bash
make cli          # build all → ~/go/bin/
make cli-<name>   # build one
```

| Tool | Description |
|------|-------------|
| `repomap` | Token-budgeted repo structure map with symbol extraction |
| `pipeline` | Run multi-step YAML workflows with params, loops, retries |
| `bulk` | Run shell commands across many items in parallel |
| `confirm` | Minimal verify: affected packages → scoped typecheck + test + lint |
| `depgraph` | Dependency graph queries: deps, rdeps, impact, cycles, path |
| `lspq` | Query language servers for code intelligence |
| `webfetch` | Fetch URLs as markdown or search the web |
| `memory` | Per-project key-value fact store |
| `sift` | Pipe filter: collapse blanks, repeated lines, truncate |
| `fossil` | Git history queries: blame, changes, ownership, co-change, log |

### repomap

Token-budgeted repository structure map with symbol extraction.

```bash
repomap [flags] [directory]
  -tokens int     Token budget (default: 2048)
  -format string  compact|verbose|detail|lines (default: compact)
  -json           Output as JSON
```

```bash
repomap                          # current dir, 2048 tokens
repomap -tokens 4096 ./myproject # larger budget
repomap -format verbose -json .  # verbose JSON output
```

### pipeline

Run multi-step YAML workflows with parameters, loops, retries, and conditionals.

```bash
pipeline [flags] <file.yaml>    # run a pipeline
pipeline list [directory]        # list available pipelines
  -dry-run          Preview without executing
  -param key=value  Parameter override (repeatable)
  -json             JSON output
  -q                Quiet mode
```

Pipeline YAML supports `params` (with defaults/required), `steps` with `run`, `timeout`, `retries`, `retry_delay`, `allow_failure`, `each` (list iteration), `loop` (range/cartesian), `workdir`, `env`, and `when` (conditional). Template vars: `{param.<name>}`, `{prev.stdout}`, `{prev.json.<key>}`, `{step.<name>.stdout}`, `{item}`, `{loop.<key>}`, `{cwd}`, `{date}`, `{timestamp}`.

```bash
pipeline deploy.yaml                          # run a pipeline
pipeline -dry-run deploy.yaml                 # preview steps
pipeline -param env=staging deploy.yaml       # override parameter
pipeline list .work/pipelines/                # list available
```

### bulk

Run a shell command across many items (git repos, directories, files) in parallel.

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

```bash
bulk -git git status -s                     # status of all repos
bulk -git -filter dirty git stash           # stash dirty repos
bulk -dirs -pattern go.mod go build ./...   # build all Go projects
bulk -files -pattern "*.go" gofmt -w {path} # format Go files
```

### confirm

Minimal verification after code changes. Identifies affected Go packages from changed files and runs scoped typecheck, tests, and lint.

```bash
confirm [flags] [file ...]
  --changes file1,file2   Explicit changed files (comma-separated)
  --no-test               Skip tests
  --no-lint               Skip lint
  --json                  JSON output
```

```bash
confirm                                          # auto-detect from git diff
confirm --no-lint                                # typecheck + tests only
confirm --changes "webfetch/webfetch.go,cache/cache.go"  # explicit files
confirm --json                                   # structured JSON verdict
confirm pkg/foo.go pkg/bar.go                    # positional args as files
confirm --no-test --no-lint                      # typecheck only (fastest)
```

### depgraph

Dependency graph queries for Go packages. Find what imports what, blast radius of changes, cycles, and shortest paths.

```bash
depgraph <command> [flags] [args]
  deps <package>           Dependency tree (what does this import?)
  rdeps <package>          Reverse deps (what imports this?)
  impact <file|package>    Blast radius of changes
  cycles                   Detect circular dependencies
  path <from> <to>         Shortest dependency path
  -depth int               Max traversal depth (default: unlimited)
  -tokens int              Token budget for output (default: unlimited)
  -json                    JSON output
```

```bash
depgraph deps webfetch                # what does webfetch import? (cache, internal/xdg)
depgraph rdeps cache                  # what imports cache? (webfetch → cmd/webfetch)
depgraph impact cache/cache.go        # blast radius: 4 packages affected
depgraph cycles                       # find circular dependencies
depgraph path webfetch cache          # shortest import path between packages
depgraph deps -depth 1 webfetch       # direct dependencies only
depgraph rdeps -json cache            # JSON output for tooling
```

### lspq

Query language servers for code intelligence.

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

```bash
lspq def main.go 42 HandleRequest     # jump to definition
lspq refs main.go 42 HandleRequest    # find all references
lspq hover main.go 42 HandleRequest   # type info and docs
lspq symbols main.go                  # list all symbols
lspq rename main.go 42 old -to new    # rename symbol
```

### webfetch

Fetch URLs as clean markdown or search the web. Uses Jina (free, no key required) with fallback to Brave, Exa, Perplexity, and Gemini when API keys are configured in `~/.config/piglet/webfetch.yaml`.

```bash
webfetch [flags] <url>           # fetch URL as markdown
webfetch search [flags] <query>  # search the web
  -raw          Fetch directly without reader provider
  -limit int    Max search results (default: 5)
  -json         Output as JSON
```

```bash
webfetch https://example.com                          # fetch as markdown
webfetch -raw https://api.example.com/data.json       # raw fetch
webfetch search "golang error handling best practices" # web search
webfetch search -limit 10 -json "rust vs go"          # JSON results
```

### memory

Per-project key-value memory store. Facts are stored in `~/.config/piglet/memory/` keyed by a hash of the working directory.

```bash
memory [-dir <path>] [-json] <command> [args]
  get <key>                    Get a fact
  set <key> <value> [category] Set a fact
  list [category]              List all facts
  delete <key>                 Delete a fact
  clear                        Clear all facts
  path                         Print backing file path
```

```bash
memory set api_url "https://api.example.com" config  # set with category
memory get api_url                                    # retrieve value
memory list                                           # all facts
memory list config                                    # filter by category
memory -dir /other/project list                       # different project
memory -json list                                     # JSON output
```

### sift

Pipe filter that compresses text by collapsing blank lines, repeated lines, and truncating. Reads stdin, writes stdout.

```bash
sift [flags]
  -threshold int          Minimum size before compression (default: 4096)
  -max-size int           Maximum output size (default: 32768)
  -no-collapse-blanks     Disable blank line collapsing
  -no-collapse-repeats    Disable repeated line collapsing
```

```bash
cat large-output.txt | sift                  # compress with defaults
git diff | sift -threshold 2048              # lower threshold
make build 2>&1 | sift -max-size 16384      # cap output size
```

### fossil

Structured git history queries with token-optimized output for LLM agents.

```bash
fossil <command> [flags] [args]
  why <file>[:<start>-<end>]    Blame lines with commit messages, deduplicated
  changes [-since 7d] [-path p] Recent changes grouped by commit
  owners [-limit 10] [path]     Code ownership ranked by commit frequency
  cochange [-limit 10] <file>   Files that historically change alongside <file>
  log [-tokens 1024] [path]     Token-budgeted git log
  -json                         JSON output (all commands except log)
```

```bash
fossil why main.go:42-58              # blame lines with commit context
fossil why main.go                    # blame entire file
fossil changes -since 30d             # last 30 days of changes
fossil changes -since 7d -path pkg/   # scoped to a directory
fossil owners                         # who owns this repo?
fossil owners -limit 5 pkg/auth/      # top 5 contributors to a path
fossil cochange -limit 10 go.mod      # what changes alongside go.mod?
fossil log -tokens 512                # compact token-budgeted log
fossil log -tokens 2048 pkg/          # scoped log with larger budget
fossil why -json main.go:1-10         # JSON output for tooling
```

## Install

Requires Go 1.26+.

```bash
git clone https://github.com/dotcommander/piglet-extensions
cd piglet-extensions
make extensions
```

This builds each extension and installs it to `~/.config/piglet/extensions/`. Piglet discovers them automatically on next launch.

Build CLI tools:

```bash
make cli
```

This installs all CLI binaries to `~/go/bin/`.

To remove all installed extensions:

```bash
make clean
```

## Configuration

Some extensions read config from `~/.config/piglet/`:

| File | Used by |
|------|---------|
| `safeguard.yaml` | safeguard -- patterns and profile |
| `autotitle.md` | autotitle -- title generation prompt |
| `subagent.md` | subagent -- sub-agent system prompt |
| `skills/` | skill -- methodology files |
| `mcp.yaml` | mcp -- server connections (stdio/HTTP) |

## Development

Each extension follows the same structure:

```
<name>/
  cmd/
    main.go          # entry point (SDK-only, JSON-RPC)
    manifest.yaml    # name, version, runtime, capabilities
  <name>.go          # core logic (pure business logic, no piglet imports)
```

Build a single extension:

```bash
make extensions-<name>    # e.g. make extensions-safeguard
```

Run tests:

```bash
go test ./...
```

## License

MIT
