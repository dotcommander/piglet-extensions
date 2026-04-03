# Loop

Run a prompt or command on a recurring interval within the active session.

## Quick Start

```
/loop 5m check whether the CI build has finished and report the status
```

The loop fires immediately, then repeats every 5 minutes until you stop it with `/loop-stop`.

## What It Does

Loop provides a session-scoped scheduler that sends a prompt (or slash command) to the piglet agent on a fixed interval. It is useful for polling long-running processes, periodic status checks, and timed reminders during a work session. Only one loop can be active at a time. The loop calls `e.SendMessage` on each tick, which injects the prompt as a user message into the conversation.

## Capabilities

| Capability | Registered name | Notes |
|---|---|---|
| Command | `/loop` | Start a recurring loop |
| Command | `/loop-stop` | Stop the active loop |
| Command | `/loop-status` | Show current loop state |
| Prompt section | Loop Scheduling | Injected at order 86 |

## Configuration

The system prompt hint text is user-editable.

| File | Location | Description |
|---|---|---|
| `prompt.md` | `~/.config/piglet/extensions/loop/prompt.md` | Text injected into system prompt |

Default content: `Use /loop <interval> <prompt> to run a prompt on a recurring interval (e.g. /loop 5m check build status). Use /loop-stop to cancel.`

If the file does not exist on first load, it is created from the embedded default.

## Commands Reference

### `/loop`

```
/loop <interval> <prompt or /command>
```

Starts a recurring loop. The prompt fires immediately on the first tick, then repeats every `<interval>`.

| Component | Format | Examples |
|---|---|---|
| `interval` | Go duration string | `30s`, `5m`, `1h`, `2h30m` |
| prompt | Any text or slash command | `check build status`, `/plan resume` |

**Minimum interval**: 30 seconds.

```
/loop 1m report current memory usage of the app server
/loop 10m summarize any new errors in the log since last check
/loop 30s check if the database migration has completed
```

Only one loop can run at a time. Starting a second loop while one is active returns an error. Stop the active loop first with `/loop-stop`.

### `/loop-stop`

```
/loop-stop
```

Cancels the active loop and waits for the in-flight goroutine to exit cleanly. Returns `"No active loop."` if nothing is running.

### `/loop-status`

```
/loop-status
```

Reports whether a loop is active, its interval, how many iterations have run, and the prompt text (truncated to 80 characters).

```
Loop active: every 5m0s, iteration #3, prompt: check whether the CI build has finished...
```

## How It Works (Developer Notes)

**Scheduler**: The `Scheduler` type in `loop.go` is SDK-free and fully testable in isolation. It holds a `context.CancelFunc` and a `done` channel so `Stop` can wait for the goroutine to exit before returning.

**Tick behavior**: The goroutine calls `onTick` immediately (iteration 1), then waits `interval` before the next tick. This means the prompt fires right away — there is no initial delay.

**Concurrency**: All `Scheduler` fields are protected by a `sync.Mutex`. `Stop` locks only to capture `cancel` and `done`, then releases before blocking on `<-done`. This avoids a deadlock if the goroutine tries to acquire the mutex during shutdown.

**Minimum interval enforcement**: `MinInterval` is a package-level variable (default `30s`) so tests can lower it without patching production code.

**`SendMessage` vs notification**: On each tick, the handler calls both `e.Notify` (surfaces a toast/status message) and `e.SendMessage` (injects the prompt into the conversation as a user turn).

**Prompt section**: `Register` calls `e.RegisterPromptSection` directly (not inside `OnInit`) because the prompt content is static and does not depend on CWD.

## Related Extensions

- [background](background.md) — One-shot read-only background task; use when you need a single deferred result
- [cron](cron.md) — Session-independent scheduled tasks that persist across restarts
