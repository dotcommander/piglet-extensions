# prompts

Slash-command prompt templates with positional argument expansion.

## Quick Start

```bash
# Install the extension
make extensions-prompts

# Create a global prompt template
mkdir -p ~/.config/piglet/prompts
cat > ~/.config/piglet/prompts/review.md << 'EOF'
---
description: Code review checklist for a file or function
---
Review $1 for: correctness, error handling, edge cases, and test coverage.
Flag any security concerns. Be specific about line numbers.
EOF

# Use it in the assistant
/review internal/api/users.go
```

## What It Does

The prompts extension scans two directories for `.md` files and registers each one as a slash command. When you invoke the command with arguments, placeholders like `$1`, `$2`, and `$@` expand to those arguments before the text is sent as a message. Project-local prompts in `.piglet/prompts/` override global ones with the same name.

## Capabilities

| Capability | Name | Description |
|------------|------|-------------|
| commands | one per `.md` file | Each file becomes a `/filename` slash command |

## Configuration

The prompts extension has no config file. Its behavior is entirely defined by the prompt files it discovers.

### Directory resolution

| Directory | Priority | Description |
|-----------|----------|-------------|
| `~/.config/piglet/prompts/` | Lower | Global prompts available in all projects |
| `<cwd>/.piglet/prompts/` | Higher | Project-local prompts; override globals on name collision |

Both directories are scanned on init. If a file named `review.md` exists in both locations, the project-local version wins.

### Prompt file format

```markdown
---
description: One-line description shown in command listings
---

Prompt body with optional $1, $2, $@ placeholders.
```

The YAML frontmatter is optional. If omitted, the command description defaults to `Prompt template: <filename>`.

**Frontmatter fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `description` | string | No | Shown in `/help` and command listings |

## Commands Reference

Each `.md` file in the prompt directories becomes a slash command named after the file (without `.md`). Arguments are passed as a space-separated string and expanded into the template body.

### Argument placeholders

| Placeholder | Expands to |
|-------------|-----------|
| `$1`, `$2` ... `$9` | The Nth argument (1-based) |
| `$@` | All arguments joined with spaces |
| `${@:N}` | Arguments from position N to end |
| `${@:N:L}` | L arguments starting at position N |

### Examples

Given this template at `~/.config/piglet/prompts/explain.md`:

```markdown
---
description: Explain a function or concept
---
Explain $@ in plain English. Include an example.
```

```
/explain how goroutine scheduling works
# Sends: "Explain how goroutine scheduling works in plain English. Include an example."
```

Given a template using `${@:2}`:

```markdown
---
description: Refactor a function with a goal
---
Refactor $1. Goal: ${@:2}
```

```
/refactor parseConfig reduce allocations and improve readability
# Sends: "Refactor parseConfig. Goal: reduce allocations and improve readability"
```

An empty argument expands to an empty string. Out-of-range positional references (`$5` when only 2 args are given) expand to an empty string.

## How It Works (Developer Notes)

### Init sequence

`Register` schedules all work in `e.OnInitAppend`, which runs after the host sends the CWD. On init:

1. Resolves `~/.config/piglet/prompts/` via `xdg.ConfigDir()`.
2. Calls `loadPromptDir` for the global directory into a shared `map[string]promptEntry`.
3. Calls `loadPromptDir` for `<cwd>/.piglet/prompts/` into the same map — project-local entries overwrite global ones.
4. Iterates the map and calls `e.RegisterCommand` for each entry.

Because all commands are registered inside `OnInitAppend`, they are available immediately after init completes.

### Template expansion

`expandTemplate` in `register.go` processes substitution in three passes:

1. **Slice patterns** (`${@:N}` and `${@:N:L}`) — replaced by a single `ReplaceAllStringFunc` call using the regex `` `\$\{@:(\d+)(?::(\d+))?\}` ``.
2. **`$@`** — replaced with all args joined by a space.
3. **`$1`–`$9`** — replaced in descending order (9 first) to avoid `$1` matching `$10` if double-digit args were ever added.

### SDK hooks used

| SDK call | Purpose |
|----------|---------|
| `e.OnInitAppend` | Defer directory scan until CWD is known |
| `e.RegisterCommand` | Register one command per prompt file |
| `e.SendMessage` | Send the expanded prompt as a user message |

### Collision semantics

When both directories contain a file with the same base name, `loadPromptDir` is called for globals first, then for project-local. Because both writes go into the same `map[string]promptEntry`, the project-local entry always replaces the global one — last write wins.

## Related Extensions

- [skill](skill.md) — loads domain knowledge on demand; complementary to prompts (knowledge vs. templates)
- [route](route.md) — classifies incoming messages; prompts feed into route's command registry
