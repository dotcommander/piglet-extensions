# Safeguard

Security interceptor that blocks dangerous shell commands and out-of-workspace file operations before execution.

## Quick Start

```yaml
# ~/.config/piglet/extensions/safeguard/safeguard.yaml
profile: balanced
patterns:
  - \brm\s+-(r|f|rf|fr)\b
  - \bgit\s+push\s+.*--force\b
  - \b(DROP|TRUNCATE)\s+(TABLE|DATABASE|SCHEMA)\b
```

Install the extension, then run any shell command through piglet — safeguard intercepts the `bash` tool automatically. A blocked command returns an error like:

```
safeguard: blocked dangerous command matching `\brm\s+-rf\b` — edit /path/to/safeguard.yaml to adjust
```

## What It Does

Safeguard runs as a `Before` interceptor on every tool call, with the highest priority in the stack (2000). For `bash` tool calls it classifies the command as read-only, write, or unknown, checks metacharacter injection patterns, then matches against your configured regex blocklist. In `strict` mode it also prevents `write`, `edit`, and `multi_edit` from touching paths outside the current working directory.

Every allow/block decision is appended to a JSONL audit log at `~/.config/piglet/extensions/safeguard/safeguard-audit.jsonl`.

## Capabilities

| Capability | Name | Priority | Description |
|-----------|------|----------|-------------|
| interceptor | `safeguard` | 2000 | Before-hook on all tool calls |

## Configuration

**File**: `~/.config/piglet/extensions/safeguard/safeguard.yaml`

Safeguard creates this file with defaults on first run if it does not exist. A `safeguard-default.yaml` seed file in the same directory takes precedence over the built-in defaults.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `profile` | string | `balanced` | `strict`, `balanced`, or `off` |
| `patterns` | []string | (see below) | Case-insensitive regex blocklist for `bash` commands |

### Profiles

| Profile | Bash blocking | Workspace scoping |
|---------|--------------|-------------------|
| `strict` | yes | yes — write/edit/multi_edit blocked outside CWD |
| `balanced` | yes | no |
| `off` | no | no (interceptor is registered but is a no-op) |

### Default Patterns

```yaml
patterns:
  - \brm\s+-(r|f|rf|fr)\b
  - \brm\s+-\w*(r|f)\w*\s+/
  - \bsudo\s+rm\b
  - \bmkfs\b
  - \bdd\s+if=
  - \b(DROP|TRUNCATE)\s+(TABLE|DATABASE|SCHEMA)\b
  - \bDELETE\s+FROM\s+\S+\s*;?\s*$
  - \bgit\s+push\s+.*--force\b
  - \bgit\s+reset\s+--hard\b
  - \bgit\s+clean\s+-[dfx]
  - \bgit\s+branch\s+-D\b
  - \bchmod\s+-R\s+777\b
  - \bchown\s+-R\b
  - >\s*/dev/sd[a-z]
  - \b:()\s*\{\s*:\|:\s*&\s*\}\s*;?\s*:
  - \bkill\s+-9\s+-1\b
  - \bshutdown\b
  - \breboot\b
  - \bsystemctl\s+(stop|disable|mask)\b
```

Add or remove patterns freely. Each pattern is compiled as `(?i)<pattern>` (case-insensitive).

### Audit Log

`~/.config/piglet/extensions/safeguard/safeguard-audit.jsonl`

Each line is a JSON object:

```json
{"ts":"2026-04-03T12:00:00Z","tool":"bash","decision":"blocked","reason":"\\brm\\s+-rf\\b","detail":"rm -rf /tmp/foo"}
```

Fields: `ts` (RFC3339 UTC), `tool`, `decision` (`allowed`/`blocked`), `reason`, `detail` (command truncated to 200 chars).

## How It Works (Developer Notes)

### Init Sequence

```
Register(e)
  └─ cfg = LoadConfig()           // reads safeguard.yaml, creates defaults if missing
  └─ compiled = CompilePatterns() // compile patterns to (?i) regexps
  └─ audit = NewAuditLogger()     // open safeguard-audit.jsonl for append
  └─ e.OnInitAppend(...)          // runs after host sends CWD
      └─ fn = BlockerWithConfig() // captures cwd for workspace scoping
      └─ blocker.Store(&fn)       // atomic pointer — safe default is allow-all
  └─ e.RegisterInterceptor(...)   // priority 2000 Before hook
```

The `blocker` is an `atomic.Pointer` so that calls arriving before `OnInit` completes fall through as allow-all (safe default). Only after `OnInit` stores the function do strict-mode workspace checks activate.

### Command Classification

`ClassifyCommand` provides a fast path for read-only commands. Conservative rules: a command is only `ReadOnly` if every pipeline segment is in the known-safe set and there are no output redirects. This avoids running the full pattern match and injection checks against commands like `ls`, `grep`, `git log`, etc.

Known read-only base commands include: `cat`, `head`, `tail`, `ls`, `tree`, `grep`, `rg`, `jq`, `ps`, `git status`, `git log`, `git diff`, `go list`, `docker ps`, `find` (without `-exec`/`-delete`), `sed` (without `-i`), and more.

### Injection Checks

Seven metacharacter checks run against every non-read-only bash command (in order, cheapest first):

| Check | What it catches |
|-------|----------------|
| `control-characters` | Non-printable bytes (0x00–0x1F except tab/newline/CR) |
| `command-substitution` | `$(...)` and backtick substitution outside single quotes |
| `ifs-injection` | `$IFS` variable for word-splitting bypasses |
| `brace-expansion` | `{rm,-rf}` style command-reassembly evasion |
| `process-substitution` | `<(...)` and `>(...)` fd injection |
| `backslash-operators` | `\;`, `\|`, `\&` parser differential attacks |
| `variable-redirect` | `$VAR >` and `> $VAR` config injection |

All checks strip single-quoted segments first (where metacharacters are literal).

### Key Patterns

- Interceptor priority 2000 — runs before all other interceptors, including RTK (100) and sift (50).
- `BlockerWithConfig` is a pure function returning a closure — easy to test in isolation.
- `xdg.WriteFileAtomic` (tmp + rename) protects against partial writes to the config file.
- Config migration: if a flat `~/.config/piglet/safeguard.yaml` exists, it is read and migrated to the namespaced path.

## Related Extensions

- [rtk](rtk.md) — command rewriter at priority 100, runs after safeguard
- [sift](sift.md) — output compressor at priority 50, post-execution
