# Usage

Track and display session token usage statistics.

## Quick Start

```
/usage
```

Prints cumulative token totals and a breakdown of the current prompt by component.

## What It Does

Usage listens for `EventTurnUsage` events emitted by the host at the end of each turn, accumulates input/output/cache token counts, and makes them available via a command and a tool. The breakdown shows how tokens are distributed across the system prompt, repo map, tool definitions, conversation history, and each extension's prompt section.

## Capabilities

| Capability | Name | Description |
|-----------|------|-------------|
| Command | `usage` | Show session token usage statistics |
| Tool | `session_stats` | Get current token statistics (model-callable) |
| Event Handler | `usage-tracker` | Record `EventTurnUsage` events |

## Configuration

Usage has no configuration file. All behavior is determined by events emitted by the host.

## Commands Reference

### `/usage`

```
/usage
```

No arguments. Displays cumulative totals and the latest prompt breakdown.

**Example output:**

```
Session Token Usage
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

CUMULATIVE TOTALS
Input                                  24,310
Output                                  8,142
Cache Read                             18,400
Cache Creation                          2,100

CURRENT PROMPT BREAKDOWN
System Prompt                           3,200
Repo Map                                  840
Tool Definitions                          612
Conversation History                   12,110

EXTENSION PROMPT SECTIONS
session-handoff                           280
memory                                    640

Total Turns                                 7
```

## Tools Reference

### `session_stats`

Get current session token usage statistics.

**Parameters:** None.

**Returns:** The same formatted summary as `/usage`, as a text tool result. Use this when you want the model to reason about token consumption.

**Example:**

```
How much context have we used so far?
```

The model calls `session_stats` and reports on the output.

## How It Works (Developer Notes)

**SDK hooks used:** `e.OnInitAppend`, `e.RegisterEventHandler`, `e.RegisterCommand`, `e.RegisterTool`.

**Event handler registration inside OnInitAppend:** The event handler is registered inside `OnInitAppend` rather than at the top level because `RegisterEventHandler` requires the RPC pipe to be open. `OnInitAppend` runs after the host sends the init message.

**Event payload:** `TurnUsageEvent` contains:
- `Turn int` — turn number
- `Usage TokenUsage` — `{Input, Output, CacheRead, CacheWrite, CacheCreation}`
- `Breakdown ComponentBreakdown` — `{SystemPrompt, Extensions []ExtensionTokens, RepoMap, Tools, History}`

**Accumulation:** `SessionStats.Record(event)` adds the turn's usage to running totals under a `sync.RWMutex`. The `current` field is replaced (not accumulated) since it reflects the latest prompt composition.

**Formatting:** `FormatSummary` right-aligns numbers in a 40-character field. Numbers ≥ 1000 are formatted with commas. The formatter uses no external dependencies — all formatting is done with hand-rolled string builders to avoid `fmt.Sprintf` overhead in the hot path.

**Handler priority:** 100 (mid-range default).

## Related Extensions

- [autotitle](autotitle.md) — auto-generate session titles
- [session-tools](session-tools.md) — session management and handoff
