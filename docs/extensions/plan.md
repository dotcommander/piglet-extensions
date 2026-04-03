# Plan

Persistent structured task tracking with steps, checkpoints, and propose/execute modes.

## Quick Start

```
# Ask the LLM to create a plan for a multi-step task
plan_create({
  "title": "Refactor HTTP client",
  "steps": [
    "Extract retry logic into RetryTransport",
    "Add context cancellation support",
    "Update tests",
    "Update documentation"
  ]
})
```

A `plan.md` file appears in your project directory. The active plan injects into the system prompt at every turn so the LLM always knows where it stands.

## What It Does

Plan creates and maintains a `plan.md` file in the project root that tracks multi-step work as a structured checklist. The file survives session restarts — it is plain Markdown, human-editable, and visible in git. When a plan is active, the extension injects a status summary into the system prompt and optionally creates checkpoint commits as steps complete.

## Capabilities

| Capability | Registered name | Notes |
|---|---|---|
| Tool | `plan_create` | Create a new plan |
| Tool | `plan_update` | Update step status, notes, or structure |
| Tool | `plan_mode` | Switch between propose and execute modes |
| Interceptor | `plan-mode` | Blocks write/edit/bash in propose mode (priority 1500) |
| Command | `/plan` | View, manage, or delete the plan |
| Prompt section | Active Plan | Injected at order 55 |

## Configuration

Plan has no external config file. Behavior is controlled per-plan via tool parameters and the `/plan` command.

| Setting | Default | Description |
|---|---|---|
| `checkpoints` | `true` in git repos | Auto-commit when a step reaches a terminal status |
| `mode` | `execute` | Whether mutating tools are allowed |

The mode and checkpoints flag are stored as an HTML comment at the bottom of `plan.md`:

```markdown
<!-- piglet:mode=execute checkpoints=true -->
```

## Tools Reference

### `plan_create`

Creates `plan.md` in the project directory with structured steps.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `title` | string | yes | Plan title (becomes the H1 heading and slug) |
| `steps` | array of strings | yes | Ordered step descriptions |
| `checkpoints` | boolean | no | Enable checkpoint commits. Defaults to `true` in git repos |

```
plan_create({
  "title": "Add OAuth2 support",
  "steps": ["Add provider config", "Implement token exchange", "Wire middleware"],
  "checkpoints": true
})
```

### `plan_update`

Updates an existing step. Use this to advance status, add notes, insert steps, or remove steps.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `step` | integer | yes | Step ID to operate on |
| `status` | string | no | New status: `pending`, `in_progress`, `done`, `skipped`, `failed` |
| `notes` | string | no | Freeform notes on this step |
| `add_after` | string | no | Insert a new pending step after this step ID with this text |
| `remove` | boolean | no | Remove this step entirely |
| `checkpoint` | boolean | no | Force a checkpoint commit regardless of status |

```
# Mark step 1 done (auto-creates checkpoint commit if git enabled)
plan_update({"step": 1, "status": "done"})

# Add context to step 2
plan_update({"step": 2, "notes": "Use oauth2.Config from golang.org/x/oauth2"})

# Insert a step after step 3
plan_update({"step": 3, "add_after": "Verify token refresh logic"})
```

When a step moves to a terminal status (`done`, `skipped`, `failed`) and git is enabled, the extension stages all changes and creates a commit with the message `[plan:<slug>] step <N>: <text>`.

### `plan_mode`

Switches the plan between propose and execute modes.

| Parameter | Type | Required | Description |
|---|---|---|---|
| `mode` | string | yes | `propose` or `execute` |

```
# Block mutating tools and capture intended changes as plan steps
plan_mode({"mode": "propose"})

# Allow changes to proceed
plan_mode({"mode": "execute"})
```

In **propose mode**, any call to `write`, `edit`, `bash`, or `multi_edit` is blocked and automatically recorded as a new plan step. This lets the LLM map out what it intends to do before committing to changes.

## Commands Reference

### `/plan`

```
/plan                   # Show the active plan
/plan delete            # Delete plan.md (or: /plan clear)
/plan approve           # Switch mode to execute
/plan mode              # Show current mode
/plan checkpoints       # Toggle checkpoint commits on/off
/plan resume            # Show the next incomplete step and last checkpoint
```

## How It Works (Developer Notes)

**Storage**: `plan.md` in the project's current working directory is the single source of truth. There is no database. The `Store` type reads and writes `plan.md` atomically (write to `.tmp`, then `os.Rename`).

**Markdown format**: Steps use GFM task-list syntax with custom markers:

| Marker | Status |
|---|---|
| `[ ]` | pending |
| `[>]` | in_progress |
| `[x]` | done |
| `[-]` | skipped |
| `[!]` | failed |

Metadata is embedded as an invisible HTML comment: `<!-- piglet:mode=execute checkpoints=true -->`. This makes the file fully human-editable without breaking the parser.

**Init sequence**: `Register` sets up an `OnInit` callback that fires after the host sends the current working directory. Inside the callback, `NewStore` is called with the resolved CWD. Because `OnInit` is the only place CWD is available, any state that depends on the project path must be initialized there.

**Interceptor**: The `plan-mode` interceptor runs at priority 1500 (higher than RTK at 100, lower than safeguard at 2000). It short-circuits before returning to allow (`true`) or block (`false`). When blocking, it appends a step description to the plan, saves, and returns a formatted error explaining what happened.

**Prompt injection**: `FormatPrompt` renders the plan as a `## Active Plan` section injected at order 55. It includes the resume point (first non-terminal step), the step list with markers, and a progress count. The propose-mode warning text is loaded from the embedded `defaults/mode-propose.md` file.

**Checkpoint commits**: `GitClient.Checkpoint` runs `git add -A` then `git commit -m "[plan:<slug>] step <N>: <text>"`. If nothing is staged, it returns the current HEAD SHA instead of creating an empty commit. The SHA is stored on the step and shown in the prompt.

## Related Extensions

- [coordinator](coordinator.md) — Decomposes tasks and runs them as parallel agents; can be combined with plan to track coordinated work
- [loop](loop.md) — Recurring prompts; useful for polling plan progress
