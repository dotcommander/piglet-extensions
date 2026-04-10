# Tasklist

Project task management with subtasks, groups, plan notes, and linking.

## Quick Start

```
# Add a task to the current project
tasklist_add({"title": "Refactor auth middleware"})

# Add a backlog item
tasklist_add({"title": "Investigate rate limiting", "group": "backlog"})

# Mark it done
tasklist_done({"id": "refactor-auth-middleware"})
```

A `Task List` section appears in the system prompt at every turn, showing active TODO items and counts. The task store is scoped to the current working directory — each project has its own list.

## What It Does

Tasklist maintains a per-project task store backed by a JSON file in your config directory. Tasks belong to one of two groups (`todo` or `backlog`) and can be nested into a parent–child hierarchy. Each task can carry freeform notes, links to Linear tickets, GitHub PRs, branches, and arbitrary URLs. Active todo tasks inject into the system prompt so the LLM stays aware of open work without being asked.

## Capabilities

| Capability | Registered name | Notes |
|---|---|---|
| Tool | `tasklist_add` | Create a task or subtask |
| Tool | `tasklist_list` | List tasks with filters |
| Tool | `tasklist_get` | Get full task details by ID |
| Tool | `tasklist_update` | Change title or notes |
| Tool | `tasklist_done` | Mark a task (and subtasks) complete |
| Tool | `tasklist_undone` | Reactivate a done task |
| Tool | `tasklist_delete` | Permanently remove a task |
| Tool | `tasklist_move` | Change group or reparent |
| Tool | `tasklist_plan` | Read, replace, or append task notes |
| Tool | `tasklist_link` | Attach a ticket, PR, branch, or URL |
| Tool | `tasklist_search` | Full-text search across title and notes |
| Tool | `tasklist_status` | Show version, store path, and counts |
| Command | `/todo` | Human-facing task management |
| Prompt section | Task List | Active tasks injected at order 55 |

## Tools Reference

### `tasklist_add`

Creates a new task. Defaults to the `todo` group.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `title` | string | yes | Task title (becomes the ID slug) |
| `group` | string | no | `todo` (default) or `backlog` |
| `parent_id` | string | no | Parent task ID to create a subtask |

```
tasklist_add({"title": "Write migration script"})
tasklist_add({"title": "Spike: Redis vs Valkey", "group": "backlog"})
tasklist_add({"title": "Add index", "parent_id": "write-migration-script"})
```

Task IDs are derived from the title by slugifying: lowercased, non-alphanumeric characters replaced with `-`, trimmed, max 64 characters. Collisions get a numeric suffix (`fix-bug-2`, `fix-bug-3`).

### `tasklist_list`

Returns tasks matching optional filters. All parameters are optional.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `status` | string | no | `active` or `done` |
| `group` | string | no | `todo` or `backlog` |
| `parent_id` | string | no | Parent ID to filter subtasks. Pass `!` to return root tasks only |

```
# All active tasks
tasklist_list({})

# Backlog only
tasklist_list({"group": "backlog"})

# Root-level active todo tasks
tasklist_list({"status": "active", "group": "todo", "parent_id": "!"})

# Subtasks of a specific parent
tasklist_list({"parent_id": "write-migration-script"})
```

Results sort active-first, then by `updated_at` descending. Done tasks always appear after active ones.

### `tasklist_get`

Returns full task details as JSON, including notes and all linked fields.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Task ID — exact, prefix, or unique suffix |

```
tasklist_get({"id": "write-migration"})
```

ID resolution tries exact match first, then unique prefix match, then unique suffix match. If multiple tasks match, the call returns an error listing the ambiguous IDs.

### `tasklist_update`

Updates a task's title or replaces its notes content entirely.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Task ID (exact or prefix) |
| `title` | string | no | New title |
| `notes` | string | no | Replacement notes content |

```
tasklist_update({"id": "write-migration", "title": "Write migration script v2"})
tasklist_update({"id": "spike-redis", "notes": "Redis 7.2 has TTL-per-key. Valkey is MIT. Prefer Valkey."})
```

To append to notes without replacing them, use `tasklist_plan` with `action: append`.

### `tasklist_done`

Marks a task and all its subtasks as done.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Task ID (exact or prefix) |

```
tasklist_done({"id": "write-migration-script"})
```

Returns the count of tasks marked complete (parent + any descendants).

### `tasklist_undone`

Reactivates a done task and all its subtasks.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Task ID (exact or prefix) |

```
tasklist_undone({"id": "write-migration-script"})
```

### `tasklist_delete`

Permanently removes a task and all its subtasks.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Task ID (exact or prefix) |

```
tasklist_delete({"id": "spike-redis"})
```

### `tasklist_move`

Changes a task's group or reparents it under a different task. Either parameter is optional, but at least one should be specified.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Task ID (exact or prefix) |
| `group` | string | no | `todo` or `backlog` |
| `new_parent_id` | string | no | New parent task ID. Pass empty string to unparent a subtask |

```
# Send to backlog
tasklist_move({"id": "spike-redis", "group": "backlog"})

# Pull back into active work
tasklist_move({"id": "spike-redis", "group": "todo"})

# Reparent a subtask
tasklist_move({"id": "add-index", "new_parent_id": "write-migration-script-v2"})
```

Moving a task to `todo` or `backlog` reactivates it if it was done. The store rejects cycles (reparenting a task under one of its own descendants).

### `tasklist_plan`

Reads, replaces, or appends to a task's freeform notes field.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Task ID (exact or prefix) |
| `action` | string | no | `read` (default), `replace`, or `append` |
| `content` | string | no | Required for `replace` and `append` |

```
# Read current notes
tasklist_plan({"id": "spike-redis"})

# Replace notes
tasklist_plan({"id": "spike-redis", "action": "replace", "content": "Decision: use Valkey."})

# Append a finding
tasklist_plan({"id": "spike-redis", "action": "append", "content": "Benchmark: Valkey 15% faster at 10k RPS."})
```

Append inserts a blank line between existing content and new content.

### `tasklist_link`

Attaches an external reference to a task. Link fields appear in `tasklist_list` output and full task JSON.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `id` | string | yes | Task ID (exact or prefix) |
| `field` | string | yes | `linear_ticket`, `github_pr`, `branch`, or `url` |
| `value` | string | yes | Link value |

```
tasklist_link({"id": "write-migration", "field": "linear_ticket", "value": "ENG-1234"})
tasklist_link({"id": "write-migration", "field": "github_pr", "value": "https://github.com/org/repo/pull/42"})
tasklist_link({"id": "write-migration", "field": "branch", "value": "feat/migration-v2"})
tasklist_link({"id": "write-migration", "field": "url", "value": "https://wiki.example.com/db-migrations"})
```

`url` is additive — each call appends a new entry to the task's `links` array. The other three fields are singular and overwrite on each call.

### `tasklist_search`

Case-insensitive full-text search across task titles and notes.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `query` | string | yes | Search text |

```
tasklist_search({"query": "migration"})
tasklist_search({"query": "valkey"})
```

Results sort by `updated_at` descending.

### `tasklist_status`

Returns version, store file path, and task counts. Takes no parameters.

```
tasklist_status({})
```

Output example:
```
tasklist v0.2.0
Store: ~/.config/piglet/extensions/tasklist/a3f9b2c1d4e5.json
Tasks: 8 total · 4 active · 2 backlog · 2 done
```

## Commands Reference

### `/todo`

```
/todo                        # Show active tasks (todo + backlog, with subtasks)
/todo add <title>            # Add a task to todo
/todo done <id>              # Mark a task done (supports prefix matching)
/todo delete <id>            # Delete a task permanently
/todo backlog <id>           # Move a task to backlog
/todo plan <id>              # Read a task's notes
/todo plan <id> <text>       # Append text to a task's notes
/todo list                   # List active tasks
/todo list done              # List completed tasks
/todo list backlog           # List backlog tasks
/todo list all               # List every task regardless of status
```

`/todo` with no arguments shows a formatted summary with task counts, active todo items with subtasks nested, and the backlog group. It is the fastest way to get a human-readable view without triggering an LLM tool call.

## Storage

Each project's tasks live in a single JSON file keyed by a 12-character hex prefix of `sha256(cwd)`:

```
~/.config/piglet/extensions/tasklist/<12hex>.json
```

The file is written atomically: a temp file is created alongside it and renamed into place on every save, preventing partial writes on crash or interrupt.

The JSON structure is a flat array of task objects sorted by ID. All timestamps are UTC. Example entry:

```json
{
  "id": "write-migration-script",
  "title": "Write migration script",
  "created_at": "2026-04-10T09:00:00Z",
  "updated_at": "2026-04-10T09:30:00Z",
  "status": "active",
  "group": "todo",
  "notes": "Use goose for migrations.",
  "linear_ticket": "ENG-1234",
  "branch": "feat/migration-v2"
}
```

## How It Works

**Init sequence**: `Register` installs an `OnInit` callback that fires after the host sends the current working directory. `NewStore` is called with the resolved CWD to derive the per-project file path. State that depends on CWD (store pointer, prompt section) is set up entirely inside `OnInit`.

**Prompt injection**: `buildPrompt` renders active root-level todo tasks with their active subtasks indented one level. The section title is `Task List` at prompt order 55. The prompt is capped at 4000 characters to avoid bloating context on large task lists. If there are no tasks at all (no active, no backlog, no done), the section is omitted entirely.

**ID resolution**: Tools accept a partial ID and resolve it by trying exact match → unique prefix match → unique suffix match. If a prefix or suffix matches more than one task, the call returns an error.

**Cascading operations**: `Done`, `Undone`, and `Delete` all walk the subtask tree recursively. Deleting a parent removes all descendants. Marking a parent done marks all descendants done.

**Cycle prevention**: `Move` with `new_parent_id` checks that the proposed parent is not a descendant of the task being moved before allowing the reparent.

## Related Extensions

- [plan](plan.md) — Step-based execution plans with checkpoint commits; useful when a task needs a structured multi-step breakdown
- [memory](memory.md) — Per-project key-value facts; complements tasklist for capturing decisions and context alongside tasks
