# Coordinator

Decompose complex tasks into parallel sub-tasks and dispatch them to independent agents.

## Quick Start

```
# Ask the LLM to coordinate a complex multi-part task
coordinate({
  "task": "Audit the authentication module: check for security issues, write missing tests, and update the API documentation",
  "max_tasks": 3
})
```

The coordinator classifies the request using a small LLM, produces 1–5 sub-tasks, dispatches them concurrently, and returns a merged result.

## What It Does

Coordinator provides a single `coordinate` tool that takes a high-level task description and breaks it into independent sub-tasks. Each sub-task runs in its own `RunAgent` call (via the host SDK). Tasks execute concurrently up to a limit of 3. Before decomposing, the coordinator discovers which tools and commands are available from all loaded extensions and passes that capability inventory to the planning LLM so sub-tasks can be scoped to the right capabilities.

## Capabilities

| Capability | Registered name | Notes |
|---|---|---|
| Tool | `coordinate` | Decompose and dispatch parallel sub-tasks |

## Configuration

The planning prompt that drives task decomposition is user-editable.

| File | Location | Description |
|---|---|---|
| `prompt.md` | `~/.config/piglet/extensions/coordinator/prompt.md` | System prompt for the task-planning LLM call |

If `prompt.md` does not exist on first load, it is created from the embedded default (`cmd/defaults/prompt.md`). Edit it to change how tasks are decomposed, what models are preferred, or what constraints apply.

**Default prompt behavior**: The planner is asked to return a JSON array of sub-tasks. Each entry has `task` (instruction string), `tools` (`"all"` or `"background_safe"`), `model` (`"default"` or `"small"`), and `max_turns` (integer 1–20). The planner uses `small` for simple lookups and `background_safe` for read-only research tasks.

## Tools Reference

### `coordinate`

Decomposes a task into parallel sub-tasks and returns a merged result.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `task` | string | yes | The task description to decompose and coordinate |
| `max_tasks` | integer | no | Maximum parallel sub-tasks (default: 3, hard max: 5) |

```
coordinate({
  "task": "Review the database layer: find N+1 queries, check index coverage, and summarize slow query patterns",
  "max_tasks": 3
})
```

**Return format**:

```
[coordinator: 3 task(s), 12 turns, 18k in / 4k out]

─── Task 1 ───
...result from sub-agent 1...

─── Task 2 ───
...result from sub-agent 2...
```

Each sub-task failure is reported independently without cancelling sibling tasks.

**Interrupt behavior**: `coordinate` sets `InterruptBehavior: "block"` — the tool runs to completion and is not interruptible mid-flight.

## How It Works (Developer Notes)

**Init**: `Register` calls `OnInitAppend` (not `OnInit`) so it appends to any existing init chain rather than replacing it. The init callback only logs timing; no CWD-dependent state is needed.

**Capability discovery**: Before planning, `DiscoverCapabilities` calls `ext.ExtInfos(ctx)` to enumerate all loaded extensions. Extensions with neither tools nor commands are filtered out. The coordinator skips itself. The result is formatted as a compact text block (`extension: tools=[...] commands=[...]`) and prepended to the planning prompt.

**Route hint**: If the host exposes a `route` tool, `PlanTasks` calls it via `ext.CallHostTool(ctx, "route", ...)` to get an intent/domain classification. This hint is appended to the planning prompt. If `route` is unavailable, planning continues without it.

**Planning LLM call**: `ext.Chat` is called with `Model: "small"` and a 1024-token limit. If the JSON response cannot be parsed, the coordinator falls back to a single task containing the full original request with `tools: "all"`, `model: "default"`, and `max_turns: 10`.

**Dispatch**: `Dispatch` uses `errgroup` with a concurrency limit of 3. Each goroutine calls `ext.RunAgent(ctx, sdk.AgentRequest{Task, Tools, Model, MaxTurns})`. Failures write to the result slot instead of returning an error — this ensures no sibling tasks are cancelled via `errgroup` error propagation.

**Concurrency cap**: The hard ceiling is `min(max_tasks, 5)`. The errgroup limit is always 3 regardless of `max_tasks`.

## Related Extensions

- [subagent](subagent.md) — Spawns agents in tmux panes with user-visible execution; use when you want interactive oversight
- [background](background.md) — Single read-only background task; lighter weight for simple research
- [plan](plan.md) — Track coordinated work as a persistent checklist
