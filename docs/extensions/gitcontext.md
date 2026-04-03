# Gitcontext

Injects the current repository's uncommitted changes, recent commits, and diff into the system prompt so the model always knows the working-tree state.

## Quick Start

```bash
# No configuration required. Install the extension and start a session
# in any git repository. The system prompt will include a "Recent Changes"
# section containing the output of:
#   git diff --stat
#   git log --oneline -5
#   git diff --no-color  (if ≤ 50 lines)
```

Example prompt injection:

```
Uncommitted changes:
\`\`\`
 pkg/foo.go | 12 +++---
 pkg/bar.go |  3 +--
 2 files changed, 8 insertions(+), 7 deletions(-)
\`\`\`

Recent commits:
\`\`\`
a1b2c3d fix(auth): handle token expiry
9f8e7d6 feat(api): add rate limiting
...
\`\`\`

Diff:
\`\`\`diff
diff --git a/pkg/foo.go b/pkg/foo.go
...
\`\`\`
```

If the repository has no uncommitted changes, the extension registers nothing and the prompt section is omitted entirely.

## What It Does

Gitcontext runs three git commands in parallel during `OnInit`: `git diff --stat`, `git log --oneline -5`, and `git diff --no-color`. It assembles their output into a single markdown block and registers it as a prompt section at order 40. The diff hunk is only included when it is 50 lines or fewer — larger diffs are omitted to avoid bloating the context. The diff-stat is capped at 30 files, with a count of remaining files appended.

All git commands run with a 5-second timeout. If git is not available or the directory is not a repository, the commands return empty strings and no prompt section is registered.

## Capabilities

| Capability | Name | Order | Description |
|-----------|------|-------|-------------|
| prompt section | `Recent Changes` | 40 | Uncommitted changes, recent commits, and small diffs |

## Configuration

There is no configuration file. All limits are compile-time constants:

| Constant | Value | Description |
|----------|-------|-------------|
| `promptOrder` | 40 | System prompt order |
| `defaultMaxDiffStatFiles` | 30 | Max files shown in `git diff --stat` |
| `defaultMaxLogLines` | 5 | Max recent commits from `git log --oneline` |
| `defaultMaxDiffHunkLines` | 50 | Max diff lines before the diff section is omitted |
| `defaultGitCommandTimeout` | 5s | Timeout for each git subprocess |

## How It Works (Developer Notes)

### Init Sequence

```
Register(e)
  └─ e.OnInitAppend(...)
      └─ cwd = ext.CWD()
      └─ buildGitContext(cwd, ext)
          └─ 3 goroutines (sync.WaitGroup):
              ├─ git diff --stat   (timeout 5s)
              ├─ git log --oneline -5  (timeout 5s)
              └─ git diff --no-color  (timeout 5s)
          └─ assemble sections:
              ├─ diffStat  → capDiffStat(stat, 30 files)
              ├─ log       → raw oneline output
              └─ hunks     → only if len(lines) <= 50
      └─ if content == "" { return }  // no changes, no section
      └─ ext.RegisterPromptSection(
             Title:   "Recent Changes",
             Content: content,
             Order:   40,
         )
```

### Parallelism

The three git commands run concurrently in goroutines coordinated by `sync.WaitGroup`. Each command has its own `context.WithTimeout` so one slow command cannot block the others beyond 5 seconds.

`gitRun` sets `cmd.Dir = cwd` rather than `cd`-ing the process, which keeps the extension goroutine-safe if multiple projects were ever initialized concurrently.

### Diff Hunk Cap

The diff hunk is included only when `len(strings.Split(hunks, "\n")) <= defaultMaxDiffHunkLines`. This is a line count on the raw diff text, not the patch header count — a diff with many small hunks across a few files still makes it through; a single large refactor does not. Increase `defaultMaxDiffHunkLines` in the source if you want more coverage.

### Key Patterns

- Uses `OnInitAppend` — appended after any `OnInit` callbacks in the same process.
- No config file: the extension is zero-config for the common case. To customize limits, build from source with modified constants.
- The prompt section is only registered when `content != ""` — clean repositories produce no noise.

## Related Extensions

- [behavior](behavior.md) — guidelines at order 10, appears before gitcontext
- [memory](memory.md) — project facts at order 50, appears after gitcontext
