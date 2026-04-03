# Subagent

Delegate tasks to independent agents running in visible tmux panes.

## Quick Start

```
# Requires an active tmux session
dispatch({
  "task": "Research the top 5 Go HTTP client libraries and compare their retry and timeout APIs",
  "split": "horizontal"
})
```

A new tmux pane opens with a full `piglet` instance running your task. The parent agent waits for the result and continues when the sub-agent completes.

## What It Does

Subagent provides a `dispatch` tool that spawns a complete piglet instance in a tmux pane for a focused, isolated task. Unlike `coordinate` (which uses the SDK's internal `RunAgent`), `dispatch` creates a real shell process visible to the user. You can observe the sub-agent's tool calls in real time and intervene if needed. Results are written to a temporary file that the parent agent polls; when the file appears, the result is returned and the temp directory is cleaned up.

## Capabilities

| Capability | Registered name | Notes |
|---|---|---|
| Tool | `dispatch` | Spawn a piglet agent in a tmux pane |

## Configuration

Subagent has no config file. All options are passed as tool parameters.

**Requirements**:

- `piglet` must be in `PATH`
- `tmux` must be in `PATH`
- The current piglet session must be running inside a tmux session (`$TMUX` must be set)

## Tools Reference

### `dispatch`

Spawns a piglet agent in a tmux pane and returns its result.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `task` | string | yes | Instructions for the sub-agent |
| `model` | string | no | Model override, e.g. `anthropic/claude-haiku-4-5` |
| `split` | string | no | Tmux layout: `horizontal` (default), `vertical`, or `window` |

```
# Horizontal split (default) — pane appears to the right
dispatch({"task": "Summarize all TODO comments in the codebase"})

# Open in a new window
dispatch({
  "task": "Run the full test suite and collect all failure messages",
  "split": "window"
})

# Use a cheaper model for a simple lookup
dispatch({
  "task": "What does the OWASP Top 10 say about SQL injection mitigation?",
  "model": "anthropic/claude-haiku-4-5",
  "split": "vertical"
})
```

**Timeout**: The parent agent polls for the result file every 500ms for up to 5 minutes. If the sub-agent does not complete within that window, a timeout message is returned and the tmux pane remains open for inspection.

**Result format**:

```
[agent a1b2c3d4]

...sub-agent output text...
```

**Pane lifecycle**: After the piglet command finishes, the pane displays `[agent <id> complete — press enter to close]` and waits for a keypress before closing. This lets you review the output before it disappears.

## How It Works (Developer Notes)

**Agent ID**: Each dispatch generates an 8-character UUID prefix (`uuid.New().String()[:8]`) used to name the temp directory and identify the pane.

**Temp directory**: Created at `$TMPDIR/piglet-agent-<id>/`. Contains `result.md` (written by piglet via `--result`). The directory is removed after the result is read or if tmux spawn fails.

**Piglet command construction**: The shell command is:

```sh
piglet [--model <model>] --result /tmp/piglet-agent-<id>/result.md "<task>"; \
  echo ''; echo '[agent <id> complete — press enter to close]'; read
```

The task string is shell-quoted with `fmt.Sprintf("%q", task)`.

**Tmux commands**:

| `split` value | tmux command |
|---|---|
| `horizontal` (default) | `split-window -h <cmd>` |
| `vertical` | `split-window -v <cmd>` |
| `window` | `new-window -n agent-<id> <cmd>` |

**Polling loop**: Uses `time.NewTimer` for the initial poll (500ms) then `timer.Reset(500ms)` on each subsequent iteration. The context is checked on each tick so a parent cancellation stops the poll immediately.

**No `OnInit`**: There is no `OnInit` callback — the extension has no CWD-dependent state. The tool is registered directly in `Register`.

## Related Extensions

- [coordinator](coordinator.md) — Internal parallel dispatch without tmux; use when user visibility is not needed
- [background](background.md) — Single read-only background task; no tmux required
