# Cron

Schedule and manage recurring tasks that run outside of active sessions.

## Quick Start

```
# Add a daily summary task
cron_add({
  "name": "daily-summary",
  "action": "prompt",
  "prompt": "Summarize my coding activity from the last 24 hours",
  "daily_at": "18:00"
})

# Install the macOS launchd agent so tasks run automatically
/cron install

# Check status
/cron status
```

## What It Does

Cron lets you define tasks that run on a schedule — every N minutes, daily at a specific time, or weekly on a given day. Tasks are executed by `piglet-cron`, a standalone daemon binary designed for launchd. The extension provides an interactive interface inside piglet (tools and commands) for managing tasks and reviewing history. Tasks can run shell commands, send piglet prompts, or hit webhook URLs. All runs are recorded in a JSONL history file.

## Capabilities

| Capability | Registered name | Notes |
|---|---|---|
| Tool | `cron_list` | List all tasks with schedules and next-run times |
| Tool | `cron_history` | Show recent run history |
| Tool | `cron_add` | Add a new scheduled task |
| Tool | `cron_remove` | Remove a task by name |
| Command | `/cron` | Interactive task management |
| Event handler | `cron-status` | Shows overdue task count on `EventAgentStart` (priority 100) |

## Configuration

| File | Location | Description |
|---|---|---|
| `schedules.yaml` | `~/.config/piglet/extensions/cron/schedules.yaml` | Task definitions |
| `cron-history.jsonl` | `~/.config/piglet/extensions/cron/cron-history.jsonl` | Run history (auto-rotated at 1000 lines) |
| `com.piglet.cron.plist` | `~/Library/LaunchAgents/com.piglet.cron.plist` | macOS launchd agent (created by `/cron install`) |

### `schedules.yaml` format

```yaml
tasks:
  daily-summary:
    action: prompt
    prompt: "Summarize my coding activity from the last 24 hours"
    schedule:
      daily_at: "18:00"
    timeout: 5m

  session-backup:
    action: shell
    command: "tar czf ~/.config/piglet/backups/sessions-$(date +%Y%m%d).tar.gz ~/.config/piglet/sessions/"
    schedule:
      weekly: "sunday 02:00"
    timeout: 2m
    work_dir: /tmp

  health-ping:
    action: webhook
    url: "https://hc-ping.com/your-uuid"
    method: POST
    schedule:
      every: 5m
```

### Task fields

| Field | Type | Description |
|---|---|---|
| `action` | string | `shell`, `prompt`, or `webhook` |
| `command` | string | Shell command (`action: shell`) |
| `prompt` | string | Piglet prompt text (`action: prompt`) |
| `url` | string | Webhook URL (`action: webhook`) |
| `method` | string | HTTP method for webhook (default: `POST`) |
| `body` | string | HTTP body for webhook |
| `headers` | map | HTTP headers for webhook |
| `schedule` | object | See schedule formats below |
| `timeout` | string | Duration string (default: `5m`) |
| `enabled` | boolean | Whether the task is active (default: `true`) |
| `work_dir` | string | Working directory for shell actions |
| `env` | map | Extra environment variables for shell actions |

### Schedule formats

Exactly one of these must be set per task:

| Field | Format | Examples |
|---|---|---|
| `every` | Go duration string, minimum 1m | `10m`, `1h`, `30m` |
| `daily_at` | `HH:MM` in local time | `"09:00"`, `"18:30"` |
| `weekly` | `"weekday HH:MM"` | `"monday 09:00"`, `"friday 17:00"` |

## Tools Reference

### `cron_list`

Lists all configured tasks with their current status.

```
cron_list({})
```

Returns one line per task:
```
daily-summary: action=prompt schedule="daily at 18:00" status=enabled last_run=2026-04-02T18:00:01Z next_run=2026-04-03T18:00:00Z
```

### `cron_history`

Shows recent task execution history.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `task` | string | no | Filter by task name |
| `limit` | number | no | Maximum entries (default: 20) |

```
cron_history({})
cron_history({"task": "daily-summary", "limit": 5})
```

### `cron_add`

Adds a new scheduled task to `schedules.yaml`.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Unique task identifier |
| `action` | string | yes | `shell`, `prompt`, or `webhook` |
| `command` | string | no | Shell command (action=shell) |
| `prompt` | string | no | Piglet prompt (action=prompt) |
| `url` | string | no | Webhook URL (action=webhook) |
| `every` | string | no | Interval schedule |
| `daily_at` | string | no | Daily schedule `HH:MM` |
| `weekly` | string | no | Weekly schedule `weekday HH:MM` |

```
cron_add({
  "name": "morning-standup",
  "action": "prompt",
  "prompt": "List all open issues assigned to me and their priority",
  "daily_at": "09:00"
})
```

### `cron_remove`

Removes a task by name.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `name` | string | yes | Task name to remove |

```
cron_remove({"name": "health-ping"})
```

## Commands Reference

### `/cron`

```
/cron                        # List all tasks
/cron status                 # Show launchd agent status + task summary
/cron test <name>            # Force-run a task immediately
/cron history                # Show last 20 run entries
/cron history <name>         # Show history for one task
/cron add                    # Print schedules.yaml path for manual editing
/cron remove <name>          # Remove a task
/cron install                # Install and load the launchd agent
/cron uninstall              # Unload and remove the launchd agent
```

**`/cron install`** generates a plist at `~/Library/LaunchAgents/com.piglet.cron.plist` and loads it via `launchctl bootstrap`. The agent runs `piglet-cron run` every 60 seconds. Requires `piglet-cron` to be installed (`make cli-piglet-cron`).

**`/cron test <name>`** delegates to `piglet-cron run --verbose --task <name>`, which forces the named task regardless of schedule. Use this to verify a task works before relying on the schedule.

## How It Works (Developer Notes)

**Daemon binary**: `piglet-cron` (`cmd/piglet-cron/`) is a separate standalone binary. When the launchd agent fires, `piglet-cron run` loads `schedules.yaml`, checks which tasks are due, runs them in parallel (via `errgroup`), and appends results to `cron-history.jsonl`. The extension binary only provides the interactive interface.

**Process lock**: `piglet-cron` acquires a `flock`-based exclusive lock at `$TMPDIR/piglet-cron.lock` on startup. A second instance running concurrently exits cleanly with code 0 (not an error — this is the expected launchd behavior).

**Schedule semantics**: Each `Schedule` implementation's `ShouldRun(lastRun)` method is called with the most recent successful run time. On first run, the zero time is passed. Missed runs (daily gaps > 24h, weekly gaps > 7d) trigger an immediate run on the next check.

**History file**: `cron-history.jsonl` is append-only. Each line is a JSON `RunEntry`. After every run cycle, `RotateHistory` trims the file to the most recent 1000 entries. The file is written via temp-then-rename for atomicity.

**Event handler**: `cron-status` fires on `EventAgentStart` at priority 100. It reads the task list, counts enabled and overdue tasks, and returns an `sdk.ActionSetStatus` action so the host can display `"3 tasks, 1 overdue"` in the UI.

**Executor timeout**: Each task gets its own `context.WithTimeout` derived from the run context. Default timeout is 5 minutes. Shell output is capped at 4096 bytes before being stored in history.

**Prompt action**: `executePrompt` shells out to `piglet --prompt "<text>"`. This means prompt tasks run as a full piglet session with access to all loaded extensions.

## Related Extensions

- [loop](loop.md) — Session-scoped recurring prompts; use when you want a loop tied to an active conversation
- [background](background.md) — One-shot background agent; use for a single deferred task
