# CLI Tools

Standalone command-line tools built from `cmd/`. These are independent binaries with no piglet dependency.

## Quick Start

```bash
just cli          # build all -> ~/go/bin/
just cli-<name>   # build one, e.g. just cli-repomap
```

## Tool Catalog

| Tool | Description |
|------|-------------|
| [repomap](#repomap) | Token-budgeted repo structure map with symbol extraction |
| [pipeline](#pipeline) | Run multi-step YAML workflows with params, loops, retries |
| [bulk](#bulk) | Run shell commands across many items in parallel |
| [confirm](#confirm) | Minimal verify: affected packages -> scoped typecheck + test + lint |
| [depgraph](#depgraph) | Dependency graph queries: deps, rdeps, impact, cycles, path |
| [lspq](#lspq) | Query language servers for code intelligence |
| [webfetch](#webfetch) | Fetch URLs as markdown or search the web |
| [memory](#memory) | Per-project key-value fact store |
| [sift](#sift) | Pipe filter: collapse blanks, repeated lines, truncate |
| [fossil](#fossil) | Git history queries: blame, changes, ownership, co-change, log |
| [extest](#extest) | Exercise extension binaries via JSON-RPC for testing |
| [piglet-cron](#piglet-cron) | Run scheduled cron tasks (daemon mode for launchd) |

---

## repomap

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

## pipeline

Run multi-step YAML workflows with parameters, loops, retries, and conditionals.

```bash
pipeline [flags] <file.yaml>    # run a pipeline
pipeline list [directory]        # list available pipelines
  -dry-run          Preview without executing
  -param key=value  Parameter override (repeatable)
  -json             JSON output
  -q                Quiet mode
```

Pipeline YAML supports `params` (with defaults/required), `steps` with `run`, `timeout`, `retries`, `retry_delay`, `allow_failure`, `each` (list iteration), `loop` (range/cartesian), `workdir`, `env`, `when` (conditional), `max_output` (byte truncation, 0=unlimited), `output_format` (text|json validation). Pipeline-level: `on_error` (halt|continue), `parallel` (concurrent step groups), `finally` (cleanup steps that always run). Template vars: `{param.<name>}`, `{prev.stdout}`, `{prev.json.<key>}`, `{prev.status}`, `{step.<name>.stdout}`, `{step.<name>.status}`, `{item}`, `{loop.<key>}`, `{cwd}`, `{date}`, `{timestamp}`.

```bash
pipeline deploy.yaml                          # run a pipeline
pipeline -dry-run deploy.yaml                 # preview steps
pipeline -param env=staging deploy.yaml       # override parameter
pipeline list .work/pipelines/                # list available
```

## bulk

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

## confirm

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

## depgraph

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
depgraph deps webfetch                # what does webfetch import?
depgraph rdeps cache                  # what imports cache?
depgraph impact cache/cache.go        # blast radius: 4 packages affected
depgraph cycles                       # find circular dependencies
depgraph path webfetch cache          # shortest import path between packages
depgraph deps -depth 1 webfetch       # direct dependencies only
depgraph rdeps -json cache            # JSON output for tooling
```

## lspq

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

## webfetch

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

## memory

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

## sift

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

## fossil

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

## extest

Exercise extension binaries via JSON-RPC for testing. Launches an extension, performs the init handshake, and sends tool/command/event requests.

```bash
extest [flags] <extension-binary>
  -cwd path    Working directory sent to extension (default: .)
  -e tool      Execute a tool by name
  -a args      Arguments: JSON for tools/events, text for commands
  -c command   Execute a slash command by name
  -E event     Dispatch an event by type (e.g. EventAgentEnd)
  -m mode      Output mode: auto, show, json, trace (default: auto)
  -t seconds   Response timeout (default: 5)
  -mock text   Mock response for host/chat RPC (default: "Test session title")
  -i           Interactive REPL mode
```

```bash
# Init only -- see what an extension registers
extest ~/.config/piglet/extensions/tasklist/tasklist

# Test a tool
extest -m show -e tasklist_add -a '{"title":"Fix bug"}' \
  ~/.config/piglet/extensions/tasklist/tasklist

# Test a slash command
extest -m show -c config -a "setup" \
  ~/.config/piglet/extensions/admin/admin

# Test an event handler
extest -m show -E EventAgentEnd \
  -a '{"Messages":[{"role":"user","content":"Fix login"}]}' \
  ~/.config/piglet/extensions/autotitle/autotitle

# Full protocol trace (raw JSON lines)
extest -m trace -e tasklist_list -a '{}' \
  ~/.config/piglet/extensions/tasklist/tasklist

# Interactive mode
extest -i ~/.config/piglet/extensions/tasklist/tasklist
```

### Host RPC Mocking

extest automatically mocks host RPC methods so extensions that call host APIs work end-to-end:

| Method | Mock Response |
|--------|--------------|
| `host/chat` | Returns `-mock` text (default: "Test session title") |
| `host/runBackground` | Returns success |
| `host/isBackgroundRunning` | Returns `{"running": false}` |
| `host/cancelBackground` | Returns success |

Any other `host/*` method returns a `-32601` error. Pass `-mock ""` to disable chat mocking.

## piglet-cron

Run scheduled cron tasks. Intended for launchd: acquires a process lock on startup, runs due tasks, then exits. Another instance running is not an error (exits 0).

```bash
piglet-cron run [flags]
  --verbose, -v   Enable info-level logging to stderr
  --task NAME     Run a specific task by name (implies force run, skips schedule check)
```
