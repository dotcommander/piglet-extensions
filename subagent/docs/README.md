# Subagent

Delegates tasks to independent sub-agents that run as full piglet instances in tmux panes. The user can observe and intervene in real time via the tmux pane. Results are returned when the agent completes.

## Capabilities

| Capability | Name | Description |
|------------|------|-------------|
| tool | `dispatch` | Spawn a piglet agent in a tmux pane |

## Tool Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `task` | string | yes | Task instructions for the agent |
| `model` | string | no | Model override (e.g., `anthropic/claude-haiku-4-5`) |
| `split` | enum | no | Tmux layout: `horizontal` (default), `vertical`, or `window` |

## How It Works

1. Validates that tmux is available and the session is running inside tmux
2. Creates a temporary directory for the agent result file
3. Spawns a full piglet instance in a new tmux pane with the task as prompt
4. Polls for the result file until the agent completes or a 5-minute timeout is reached
5. Returns the agent output, or a status message if cancelled/timed out

The spawned piglet instance runs with full tool access. The pane stays open after completion so the user can review output before closing it.

## Requirements

- tmux must be installed and available in `PATH`
- piglet must be running inside a tmux session (`TMUX` environment variable set)
