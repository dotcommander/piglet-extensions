# Tokengate

Context window manager that limits token waste before it reaches the model, tracks cumulative usage per turn, and auto-summarizes large tool results.

## Quick Start

```bash
# Check current context window usage
/budget

# Or call the tool directly
context_budget
```

```
Context Budget
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Context window:    200,000 tokens
Current fill:      42,300 tokens (21%)
Remaining:         157,700 tokens

BREAKDOWN
  System prompt:     8,200
  Repo map:          12,400
  Tool definitions:  4,100
  Conversation:      17,600

  Extensions:
    memory             1,800
    rtk                  200

CUMULATIVE
  Turns:             4
  Total input:       68,400
  Total output:      9,100
```

## What It Does

Tokengate runs three independent mechanisms to keep the context window healthy:

**Scope limiter** — a `Before` interceptor (priority 80) that rewrites tool call arguments before execution. It pipes unbounded `grep -r` and `find /` commands through `| head -N`, and adds a `limit` parameter to unconstrained `Read` calls. Rewrites are skipped when the call already has a head/tail pipe or an explicit limit.

**Budget tracker** — an `EventTurnUsage` event handler (priority 110) that records the token breakdown emitted by the host after every turn. Once the prompt fill crosses `warn_percent` of `context_window`, it fires a one-time notification.

**Auto-summarizer** — an `After` interceptor (priority 20) that sends tool results larger than `summarize_threshold` to a small LLM for compression before the model sees them. File editing tools (`Read`, `Edit`, `Write`, `MultiEdit`) are never summarized — exact content matters there.

## Capabilities

| Capability | Name | Priority | Description |
|-----------|------|----------|-------------|
| interceptor (before) | `tokengate-scope` | 80 | Rewrites tool args to bound output size |
| interceptor (after) | `tokengate-summarize` | 20 | Auto-summarizes large tool results via LLM |
| event handler | `tokengate-tracker` | 110 | Tracks per-turn token usage; warns at threshold |
| tool | `context_budget` | — | Show current budget breakdown |
| tool | `tokengate_status` | — | Show extension version and active settings |
| slash command | `/budget` | — | Show current budget in the chat UI |
| prompt section | `Token Gate` | order 15 | System prompt guidance on scoping tool calls |

## Tools & Commands Reference

### `context_budget`

Returns the formatted budget table: context window size, current fill percentage, per-category breakdown, and cumulative turn totals. No parameters.

```
context_budget
```

### `/budget`

Same output as `context_budget`, displayed as a chat message. Use it when you want to see usage without it appearing as a tool call in the conversation.

```
/budget
```

### `tokengate_status`

Reports the running extension version and current config values.

```
tokengate_status
```

```
tokengate 0.2.0
  Context window: 200,000 tokens
  Warn at: 80%
  Scope limiter: enabled
  Summarizer: enabled (threshold: 8192 chars)
```

## Configuration

**File**: `~/.config/piglet/extensions/tokengate/config.yaml`

Created with defaults on first run if it does not exist.

```yaml
# Token budget configuration
context_window: 200000
warn_percent: 80
summarize_threshold: 8192
summarize_enabled: true
```

### Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `enabled` | bool | `true` | Enable or disable the scope limiter interceptor |
| `context_window` | int | `200000` | Total token capacity of the model's context window |
| `warn_percent` | int | `80` | Fire a warning notification when prompt fill reaches this percentage |
| `summarize_enabled` | bool | `true` | Enable LLM-based auto-summarization of large tool results |
| `summarize_threshold` | int | `8192` | Summarize results larger than this many characters |
| `rules` | []rule | see below | Scope-limiting rewrite rules (code only; not in YAML defaults) |

### Scope Rules

Rewrite rules are defined in Go defaults (see `DefaultConfig`) and applied by the scope interceptor. Each rule matches a tool name and optional command pattern, then applies an action.

| Field | Description |
|-------|-------------|
| `tool` | Tool name to match (`Bash`, `Read`, `Grep`) |
| `pattern` | Regex matched against the command string; empty matches all calls to that tool |
| `action` | `append_head` (pipes through `\| head -N`) or `limit_lines` (adds limit/head_limit param) |
| `value` | The N in `head -N` or the line count limit |

**Built-in rules:**

| Tool | Pattern | Action | Value |
|------|---------|--------|-------|
| `Bash` | `grep\s+-r\s+.*\.\*` | `append_head` | `100` |
| `Bash` | `find\s+/` | `append_head` | `50` |
| `Read` | _(any)_ | `limit_lines` | `200` |

Rules are skipped if the call already has a `| head` or `| tail` pipe, or if an explicit `limit`/`offset` is set on `Read`.

### Prompt File

**File**: `~/.config/piglet/extensions/tokengate/prompt.md`

Default content:

```
Before reading files to investigate issues, run deterministic tools first. Use repomap for
structure, depgraph for dependencies, fossil for history, confirm for change impact. Send
detector output for analysis, not raw files. Prefer targeted grep over file reads. Scope
queries before expanding them.
```

Edit this file to change the system prompt guidance injected at order 15.

### Summarize Prompt File

**File**: `~/.config/piglet/extensions/tokengate/summarize-prompt.md`

Controls what the LLM is asked to do when auto-summarizing a large tool result. Default:

```
Summarize this tool output concisely. Keep key facts, file paths, error messages, and
actionable information. Remove verbose formatting, redundant content, and boilerplate.
Output ONLY the summary, no preamble.
```

Edit this file to tune the summarization behavior (e.g., domain-specific preservation rules).

## How It Works

### Init Sequence

```
Register(e)
  └─ cfg = LoadConfig()               // reads config.yaml or writes defaults
  └─ budget = NewBudgetState(cfg)     // in-memory tracker, reset on restart
  └─ e.OnInitAppend(...)              // runs after host sends CWD
      └─ x.RegisterPromptSection()    // Order 15 (only if cfg.Enabled)
  └─ e.RegisterInterceptor(scope)     // Before, priority 80
  └─ e.RegisterEventHandler(tracker) // EventTurnUsage, priority 110
  └─ registerSummarizer(e, cfg)       // After, priority 20 (skipped if disabled)
  └─ e.RegisterTool(context_budget)
  └─ e.RegisterTool(tokengate_status)
  └─ e.RegisterCommand(/budget)
```

### Scope Limiter Pipeline

```
Before(toolName, args)
  └─ match rules by tool name
  └─ Bash: check pattern against command string
      └─ alreadyScoped()? → pass through unchanged
      └─ append_head → command + " | head -N"
  └─ Read: check for existing limit/offset
      └─ limit_lines → args["limit"] = N
  └─ Grep: check for existing head_limit
      └─ limit_lines → args["head_limit"] = N
  └─ return modified args
```

### Budget Tracker

```
EventTurnUsage → budget.Record(event)
  └─ accumulate totalInput, totalOutput, turns
  └─ update current Breakdown
  └─ compute promptTokens() = system + repomap + tools + history + extensions
  └─ if promptTokens >= (contextWindow * warnPercent / 100) and !warned
      └─ warned = true
      └─ return ActionNotify("Context budget warning: N% of M token window used")
```

The warning fires once per session (the `warned` flag is never reset). Restart the extension binary or restart piglet to reset it.

### Auto-Summarizer Pipeline

```
After(toolName, details)
  └─ toolresult.ExtractText(details) — bail if no text
  └─ len(text) <= summarizeThreshold? → pass through unchanged
  └─ skipSummarize(toolName)? → pass through unchanged (Read/Edit/Write/MultiEdit)
  └─ truncate input to 32,768 chars if needed
  └─ ext.Chat(ctx, {system: summarizePrompt, model: "small", maxTokens: 2048})
  └─ error or empty response? → return original unchanged
  └─ prepend "[Summarized from N chars]\n"
  └─ toolresult.ReplaceText(details, summary)
```

Summarization failures are silent — the original result passes through rather than breaking the tool call.

### Key Patterns

- Priority 20 for the summarizer — runs before memory-overflow (30) and sift (50), so sift compresses whatever the summarizer leaves behind.
- Priority 80 for the scope limiter — runs after safeguard (2000) and RTK (100); rewrites that RTK already handled won't be double-piped because `alreadyScoped()` detects existing head/tail.
- Priority 110 for the event handler — runs after the usage-tracker extension (100) so the breakdown data is fully populated before recording.
- The `BudgetState` is package-level and reset on process restart; it does not persist across piglet restarts.

## Related Extensions

- [sift](sift.md) — after-interceptor at priority 50; text compression for results tokengate's summarizer doesn't handle
- [rtk](rtk.md) — before-interceptor at priority 100; command rewriting; tokengate scope limiter runs after it
- [safeguard](safeguard.md) — before-interceptor at priority 2000; security checks run before tokengate scope rewrites
- [usage](usage.md) — emits `EventTurnUsage` that tokengate's budget tracker consumes
