# Behavior

Injects custom behavioral guidelines as the earliest system prompt section so they take effect before any other context.

## Quick Start

```bash
# Create your guidelines file
mkdir -p ~/.config/piglet/extensions/behavior
cat > ~/.config/piglet/extensions/behavior/behavior.md << 'EOF'
- Always use second person and active voice.
- Prefer explicit error handling over silent failures.
- Never add emoji to code or documentation unless asked.
EOF
```

The contents of `behavior.md` appear in the model's system prompt under the heading **Guidelines** before any other section. If the file is empty or missing, the extension registers nothing and exits cleanly.

## What It Does

Behavior reads `~/.config/piglet/extensions/behavior/behavior.md` during `OnInit` and registers it as a prompt section at order 10 — the lowest order value in the default extension set. This means your guidelines prepend every other injected context (gitcontext at 40, memory at 50, rtk at 90, sift at 91). The file is plain markdown; any formatting you write there is passed to the model as-is.

## Capabilities

| Capability | Name | Order | Description |
|-----------|------|-------|-------------|
| prompt section | `Guidelines` | 10 | Contents of `behavior.md` |

## Configuration

**File**: `~/.config/piglet/extensions/behavior/behavior.md`

This file is not created automatically. Create it and write any guidelines you want the model to follow in every session. The file is read once per session during `OnInit`.

There are no other configuration fields. The extension has no YAML config file.

### Example `behavior.md`

```markdown
## Code Style
- Use table-driven tests in Go.
- Always run `go mod tidy` before committing.
- Prefer `errors.Is` / `errors.As` over string matching on error messages.

## Communication
- Summarize what you did at the end of each response.
- If a task is ambiguous, ask one clarifying question before starting.

## Safety
- Never force-push to main.
- Confirm before deleting files.
```

## How It Works (Developer Notes)

### Init Sequence

```
Register(e)
  └─ e.OnInitAppend(...)
      └─ content = xdg.LoadOrCreateExt("behavior", "behavior.md", "")
      │   // reads ~/.config/piglet/extensions/behavior/behavior.md
      │   // if missing, creates empty file — does NOT write defaults
      └─ if content == "" { return }   // no prompt section registered
      └─ ext.RegisterPromptSection(
             Title:   "Guidelines",
             Content: content,
             Order:   10,
         )
```

`xdg.LoadOrCreateExt` is called with an empty string as the default content, so a missing file is created as empty rather than populated — the model sees no guidelines section until you put something in the file.

### Prompt Section Ordering

Order 10 is the lowest value assigned to any extension prompt section:

| Order | Section | Extension |
|-------|---------|-----------|
| 10 | Guidelines | behavior |
| 40 | Recent Changes | gitcontext |
| 50 | Project Memory | memory |
| 90 | RTK Token Optimization | rtk |
| 91 | Sift Result Compression | sift |

Lower order = earlier in the system prompt.

### Key Patterns

- Uses `OnInitAppend` (not `OnInit`) so it plays well with other extensions using `OnInit` — all append callbacks run after `OnInit` callbacks complete.
- No tool, command, or interceptor is registered — this extension is prompt injection only.
- Content changes take effect immediately on the next session start; no restart of the piglet host is needed.

## Related Extensions

- [gitcontext](gitcontext.md) — adds repo state at order 40
- [memory](memory.md) — adds project facts at order 50
