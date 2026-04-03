# Sift

Output compression interceptor that reduces large tool results to a fraction of their original size before they reach the model's context window.

## Quick Start

```yaml
# ~/.config/piglet/extensions/sift/sift.yaml
enabled: true
size_threshold: 4096       # bytes — results smaller than this pass through untouched
max_size: 32768            # bytes — hard cap after compression
tools:
  - Read
  - Grep
  - Bash
compression:
  collapse_blank_lines: 3
  collapse_repeated_lines: 5
  strip_trailing_whitespace: true
  truncation_marker: "\n[SIFT: truncated — {kept}/{total} bytes shown]"
```

A compressed result begins with a sift header:

```
[SIFT: 48321 -> 12048 bytes (75% reduction)]
... compressed content ...
```

If content matches a structured rule (e.g. golangci-lint output), sift converts it to a markdown table instead:

```
[SIFT: structured — 12 findings extracted from 24830 bytes]
| file | line | linter | message |
|------|------|--------|---------|
| pkg/foo.go | 42 | errcheck | Error return value not checked |
```

## What It Does

Sift registers an `After` interceptor at priority 50. After every tool call, if the result text exceeds `size_threshold`, sift applies compression: it strips trailing whitespace, collapses runs of blank lines, collapses runs of identical lines, and truncates to `max_size`. For tool outputs that look like linter results, it parses them into a structured table instead. The compressed text replaces the original in the tool result before the model sees it.

The extension also registers a prompt section (injected in `OnInit`) that informs the model about the compression so it can request the raw content if needed.

## Capabilities

| Capability | Name | Priority | Description |
|-----------|------|----------|-------------|
| interceptor | `sift` | 50 | After-hook; compresses tool results above the threshold |
| prompt section | `Sift Result Compression` | order 91 | System prompt note about compression |

## Configuration

**File**: `~/.config/piglet/extensions/sift/sift.yaml`

Created with defaults on first run if it does not exist.

### Top-Level Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable or disable sift entirely |
| `size_threshold` | int | `4096` | Minimum result size in bytes before compression applies |
| `max_size` | int | `32768` | Maximum output size in bytes after compression |
| `tools` | []string | `["Read","Grep","Bash"]` | Tool names to compress; empty list means all tools |

### Compression Sub-Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `collapse_blank_lines` | int | `3` | Collapse runs of N or more blank lines to 1 |
| `collapse_repeated_lines` | int | `5` | Collapse runs of N or more identical lines |
| `strip_trailing_whitespace` | bool | `true` | Remove trailing spaces and tabs from each line |
| `truncation_marker` | string | `\n[SIFT: truncated — {kept}/{total} bytes shown]` | Appended at cut point; `{kept}` and `{total}` are replaced |

### Structured Rules

Structured rules convert recognizable linter output into a compact markdown table instead of applying text compression.

```yaml
structured:
  - tool: Bash
    detect: "golangci-lint"
    columns: [file, line, linter, message]
    max_rows: 25
  - tool: Bash
    detect: "go vet"
    columns: [file, line, message]
    max_rows: 25
  - tool: Bash
    detect: "staticcheck"
    columns: [file, line, message]
    max_rows: 25
```

| Field | Type | Description |
|-------|------|-------------|
| `tool` | string | Tool name to match against |
| `detect` | string | Substring that must appear in the output to activate this rule |
| `columns` | []string | Columns to include in the table |
| `max_rows` | int | Maximum table rows; additional findings are dropped |
| `sort_by` | string | Column to sort by (optional; errors before warnings) |

### Prompt File

**File**: `~/.config/piglet/extensions/sift/prompt.md`

The default content:

```
Large tool results are automatically compressed by Sift. Content below 4KB passes through
unchanged. Repeated patterns and excessive blank lines are collapsed. If a result shows a
[SIFT:] header, the original was larger — request the raw content if you need it.
```

Edit this file to change the system prompt note.

## How It Works (Developer Notes)

### Init Sequence

```
Register(e)
  └─ e.OnInit(...)             // runs after host sends CWD
      └─ cfg = LoadConfig()    // reads sift.yaml or writes defaults
      └─ projectCWD = x.CWD() // used for deprefixing paths in structured output
      └─ x.RegisterPromptSection() // Order 91
  └─ e.RegisterInterceptor()   // priority 50 After hook
```

`cfg` and `projectCWD` are captured by the `OnInit` closure so the interceptor closure sees the fully-loaded config before it runs any calls.

### Compression Pipeline

```
After(toolName, details)
  └─ check cfg.Enabled, cfg.Tools filter
  └─ toolresult.ExtractText(details) — bail if no text or below threshold
  └─ CompressWithTool(toolName, text, cfg, cwd)
      └─ CompressStructured() — try structured table first
          └─ matchRule() — find matching structured rule
          └─ parseLinterOutput() — extract file:line:message rows
          └─ buildTable() — markdown table
      └─ Compress() — fallback text compression
          └─ stripTrailingWhitespace()
          └─ collapseBlankLines()
          └─ collapseRepeatedLines()
          └─ truncate()
  └─ toolresult.ReplaceText(details, compressed)
```

If compression does not reduce size, the original text is returned unchanged (no header is added). The header is only added when compression actually saves bytes.

### Structured Parser

`parseLinterOutput` recognises three line formats (tried in order):

1. `file:line:col: message` — matched by `reFileLineCol`
2. `file:line: message` — matched by `reFileLine`
3. `file(line): message` — matched by `reFileParen`

For golangci-lint output, the linter name is extracted from the trailing `(linter-name)` suffix and placed in its own column.

Paths are deprefixed against `projectCWD` so table rows show `pkg/foo.go` instead of `/home/user/repo/pkg/foo.go`.

### Key Patterns

- Priority 50 — runs after safeguard (2000 before-hook) and RTK (100 before-hook); this is an after-hook so execution order is reversed: sift sees results before lower-priority after-hooks.
- `toolresult.ExtractText` / `toolresult.ReplaceText` are internal helpers that navigate the SDK's result envelope without caring about its exact shape.
- Setting `tools: []` in config disables the filter and applies sift to all tool calls.

## Related Extensions

- [safeguard](safeguard.md) — before-interceptor at priority 2000; runs before execution
- [rtk](rtk.md) — before-interceptor at priority 100; rewrites commands before execution
- [memory](memory.md) — memory-overflow interceptor at priority 30 also handles oversized results; it runs before sift
