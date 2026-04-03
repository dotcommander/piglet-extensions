# Background

Run a read-only background task without blocking the active conversation.

## Quick Start

```
/bg summarize the last 50 git commits and identify any recurring problem areas
```

The task starts immediately in the background. You continue working while it runs. Use `/bg-cancel` to abort if needed.

## What It Does

Background provides two commands that wrap the SDK's `RunBackground` and `CancelBackground` host methods. A background task is a constrained agent run — read-only tools only, maximum 5 turns — that executes without holding up the main conversation thread. When the task completes its result surfaces in the UI.

## Capabilities

| Capability | Registered name | Notes |
|---|---|---|
| Command | `/bg` | Start a read-only background task |
| Command | `/bg-cancel` | Cancel the running background task |

## Configuration

Background has no config file. The tool access restrictions (read-only, max 5 turns) are enforced by the host, not this extension.

## Commands Reference

### `/bg`

```
/bg <prompt>
```

Starts a read-only background agent with the given prompt.

```
/bg check all Go files for unused imports and list them
/bg count the number of TODO comments in the codebase by package
/bg look up the current latest version of golang.org/x/crypto
```

If a task is already running, starting another may result in an error from the host (behavior depends on the host implementation). Cancel the running task first with `/bg-cancel`.

On success, shows: `Background task started: <prompt>`

On failure, shows: `Background task failed: <error>`

### `/bg-cancel`

```
/bg-cancel
```

Checks whether a background task is running and cancels it if so.

```
No background task running       # when idle
Background task cancelled        # on successful cancel
Failed to cancel background task: <error>   # on host error
```

## How It Works (Developer Notes)

**SDK methods**: Background is a thin wrapper around three SDK extension methods:

| SDK call | Purpose |
|---|---|
| `e.RunBackground(ctx, prompt)` | Dispatch a read-only agent run |
| `e.IsBackgroundRunning(ctx)` | Poll for an active background task |
| `e.CancelBackground(ctx)` | Cancel the active task |

All three calls go to the host via JSON-RPC. The extension itself holds no state.

**No `OnInit`**: There is no CWD-dependent state and no config to load. `Register` wires both commands directly.

**Error handling**: Both handlers use `e.ShowMessage` for all outcomes (success and failure). Neither returns a non-nil error from the handler — this prevents the host from treating a task failure as an extension crash.

**Read-only constraint**: The 5-turn cap and tool restrictions are enforced by the host `RunBackground` implementation. The extension does not set these limits itself.

## Related Extensions

- [loop](loop.md) — Recurring prompts on a fixed interval; use when you need repeated checks
- [coordinator](coordinator.md) — Parallel multi-agent dispatch with merged results; use for complex decomposable tasks
- [subagent](subagent.md) — Full piglet instance in a tmux pane with user visibility; use when you need to observe execution
