# Bulk

Run shell commands across many items (git repos, directories, files) in parallel.

## Quick Start

```
# Run git status across all repos in the current directory
"Show git status for all repos"
→ bulk tool: source=git_repos, command="git status -s"

# Format all Go files
"Run gofmt on every .go file in the project"
→ bulk tool: source=files, pattern="**/*.go", command="gofmt -w {path}"

# Push only repos that are ahead of origin
"Push all repos that have unpushed commits"
→ bulk tool: source=git_repos, filter=ahead, command="git push"
```

## What It Does

Bulk discovers a set of items (git repos, directories matching a pattern, files matching a glob, or an explicit list), optionally filters them with a shell predicate or git status check, then runs a shell command on each item in parallel. Results are collected into a structured JSON summary with per-item status and output. For commands containing destructive keywords (`push`, `rm`, `delete`, `clean`, `reset`, `--force`), `dry_run` defaults to true to prevent accidents.

## Capabilities

| Capability | Detail |
|------------|--------|
| `tools` | `bulk` |
| `prompt` | Injects a "Bulk Operations" section (order 80) |

## Configuration

No config file. The prompt section content lives at `~/.config/piglet/extensions/bulk/prompt.md`.

## Tools Reference

### `bulk`

Run a shell command across multiple items in parallel.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | yes | Shell command. Template vars: `{path}`, `{name}`, `{dir}`, `{basename}` |
| `source` | string | yes | Item source: `git_repos`, `dirs`, `files`, or `list` |
| `directory` | string | no | Root directory to scan (default: cwd) |
| `pattern` | string | no | For `dirs`: filename/dir that must exist inside a dir (e.g. `go.mod`). For `files`: glob pattern (e.g. `*.go`, `**/*.ts`) |
| `items` | string[] | no | Explicit paths for `list` source |
| `filter` | string | no | Shell predicate (exit 0 = keep) or git keyword: `dirty`, `clean`, `ahead`, `behind`, `diverged` |
| `depth` | integer | no | Scan depth (default: 1) |
| `dry_run` | boolean | no | Collect and filter without executing. Auto-true for mutating commands |
| `concurrency` | integer | no | Max parallel executions (default: 8) |
| `timeout` | integer | no | Per-item timeout in seconds (default: 30) |

### Source Types

| Value | Discovers |
|-------|-----------|
| `git_repos` | Subdirectories containing a `.git/` directory |
| `dirs` | Subdirectories; `pattern` filters to dirs containing that file/dir |
| `files` | Files matching a glob pattern (supports `**` via doublestar) |
| `list` | Explicit paths provided in the `items` array |

### Command Template Variables

| Variable | Value |
|----------|-------|
| `{path}` | Absolute path to the item |
| `{name}` | Directory or filename (basename of path) |
| `{dir}` | Parent directory of the item |
| `{basename}` | Same as `{name}` |

Commands without template variables run in the item's directory (for `git_repos` and `dirs` sources).

### Git Filters

When `source` is `git_repos`, the `filter` field accepts these keywords in addition to shell predicates:

| Keyword | Keeps repos where... |
|---------|---------------------|
| `dirty` | Working tree has uncommitted changes |
| `clean` | Working tree is clean |
| `ahead` | Branch is ahead of upstream |
| `behind` | Branch is behind upstream |
| `diverged` | Branch is both ahead and behind upstream |

Any other value is treated as a shell predicate.

### Examples

```json
{
  "command": "git status -s",
  "source": "git_repos"
}
```

```json
{
  "command": "go build ./...",
  "source": "dirs",
  "pattern": "go.mod"
}
```

```json
{
  "command": "gofmt -w {path}",
  "source": "files",
  "pattern": "**/*.go"
}
```

```json
{
  "command": "git push",
  "source": "git_repos",
  "filter": "ahead",
  "dry_run": false
}
```

```json
{
  "command": "npm test",
  "source": "list",
  "items": ["/projects/app-a", "/projects/app-b"],
  "timeout": 120
}
```

### Result Format

```json
{
  "total_collected": 8,
  "matched_filter": 3,
  "results": [
    { "item": "app-a", "path": "/projects/app-a", "status": "ok", "output": "M src/index.ts" },
    { "item": "app-b", "path": "/projects/app-b", "status": "error", "output": "fatal: not a git repository" }
  ],
  "message": "8 items collected, 3 matched, 2 succeeded, 1 failed."
}
```

When `dry_run` is true, each result has `status: "skipped"` and `output` shows the expanded command that would have run.

## How It Works (Developer Notes)

**Architecture**: The tool handler `executeBulk` wires together three composable parts: a `Scanner` that produces `[]Item`, a `Filter` that keeps a subset, and `Execute` that runs the command on matched items.

**Scanners**:
- `GitRepoScanner` — wraps `DirScanner` with a `.git/` existence check
- `DirScanner` — walks directories up to `depth`, applies an optional `Match` function
- `GlobScanner` — uses `github.com/bmatcuk/doublestar` for `**` glob support
- `ListScanner` — resolves explicit paths to absolute form

**Filters**: `ShellFilter` runs the command in the item's directory; exit 0 = keep. `GitFilter` translates keywords to specific `git status --porcelain` or `git rev-list --count` predicates. Both run in parallel via `errgroup` bounded by `concurrency`.

**Execution**: `Run` launches one goroutine per item via `errgroup.SetLimit(concurrency)`. Each goroutine calls `runOne`, which expands the template, runs `shellExec` with a per-item `context.WithTimeout`, and returns a `Result`. Results are sorted by item name before returning.

**Template expansion**: `expandTemplate` does a simple string replacement of `{path}`, `{name}`, `{dir}`, and `{basename}` against the `Item` fields. No format string parsing — just `strings.ReplaceAll`.

**Dry-run auto-detection**: `isMutating` scans the command string for `push`, `rm `, `delete`, `clean`, `reset`, `--force`. If any match, `dry_run` defaults to true. The caller can override this by explicitly setting `dry_run: false`.

**Prompt section order**: 80 — appears after pipeline (75) and before RTK (90).

## Related Extensions

- [pipeline](pipeline.md) — sequential multi-step workflows with retries and conditionals; use when steps depend on each other's output
