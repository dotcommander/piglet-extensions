# Plan

Persistent structured task tracking for multi-step work with a propose/execute mode that can block destructive operations during planning.

## Capabilities

| Capability | Name | Description |
|------------|------|-------------|
| tool | `plan_create` | Create a new plan with steps |
| tool | `plan_update` | Update step status, add/remove steps, attach notes |
| tool | `plan_mode` | Switch between propose and execute modes |
| command | `/plan` | Manage plans via slash commands |
| prompt | Active plan | Injects active plan into system prompt |
| interceptor | Plan mode gate | Blocks mutating tools in propose mode (priority: 1500) |

## Prompt Order

55

## Command Usage

```
/plan                # show active plan
/plan list           # list all plans with progress
/plan switch <slug>  # activate a plan
/plan archive        # deactivate active plan
/plan clear          # delete active plan
/plan delete <slug>  # delete specific plan
/plan approve        # switch to execute mode
/plan mode           # show current mode
```

## Modes

| Mode | Behavior |
|------|----------|
| **execute** | Normal operation — all tools allowed |
| **propose** | Blocks write, edit, multi_edit, bash — records proposed changes as plan steps |

## Step Statuses

`pending` → `in_progress` → `done` / `skipped` / `failed`

Setting a step to `in_progress` auto-reverts any previous in-progress step to pending. Plans auto-archive when all steps reach a terminal status.

## Storage

Plans stored as YAML in `~/.config/piglet/plans/<cwd-hash>/`. One file per plan, keyed by auto-generated slug. Thread-safe with RWMutex; atomic writes via temp file + rename.
