# skill

Domain knowledge and methodology loading from markdown files.

## Quick Start

```bash
# Install the extension
make extensions-skill

# Drop a skill file into the skills directory
cat > ~/.config/piglet/skills/go-testing.md << 'EOF'
---
name: go-testing
description: Go testing patterns and best practices
triggers:
  - write tests
  - table-driven
  - go test
---

Use table-driven tests with t.Run subtests. Call t.Parallel() at the top of
each test and subtest. Use testify/assert for assertions. Clean up with
t.Cleanup, not defer.
EOF

# The LLM can now call skill_load or you can type /skill go-testing
```

## What It Does

The skill extension scans `~/.config/piglet/skills/` for markdown files, parses their YAML frontmatter, and surfaces them to the LLM via tools, a slash command, a system prompt listing, and a message hook that auto-loads matching skills when trigger keywords appear in a user message. It registers nothing if the skills directory is missing or empty.

## Capabilities

| Capability | Name | Description |
|------------|------|-------------|
| tool | `skill_list` | List available skills with descriptions and trigger keywords |
| tool | `skill_load` | Load full skill content by name |
| command | `/skill` | `/skill list` or `/skill <name>` to view skills interactively |
| prompt | Skills | Lists all available skills at system prompt order 25 |
| messageHook | `skill-trigger` | Auto-loads the best-matching skill when trigger keywords appear in a message |

## Configuration

The skill extension has no config file. All configuration is implicit in the skill files themselves.

**Skills directory:** `~/.config/piglet/skills/`

If the directory does not exist or contains no `.md` files, the extension registers no capabilities and exits silently.

## Skill File Format

Skills are markdown files with optional YAML frontmatter between `---` delimiters.

```markdown
---
name: my-skill
description: Short one-line description shown in listings
triggers:
  - trigger phrase one
  - keyword two
---

Your full methodology content here. This becomes the skill body
that skill_load returns and the message hook injects.
```

**Frontmatter fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | No | Skill identifier. Defaults to the filename without `.md` |
| `description` | string | No | Shown in `skill_list` output and system prompt listing |
| `triggers` | []string | No | Keywords that cause auto-loading via the message hook |

Files without frontmatter are loaded with the filename as the name and no triggers.

## Tools Reference

### `skill_list`

Lists all loaded skills with their descriptions and trigger keywords.

**Parameters:** none

**Example response:**
```
- go-testing: Go testing patterns and best practices (triggers: write tests, table-driven, go test)
- security-review: Security audit checklist (triggers: security, audit, vuln)
```

---

### `skill_load`

Loads the full body of a single skill by name.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | yes | Skill name (case-insensitive match) |

**Example:**
```json
{ "name": "go-testing" }
```

Returns the skill body text (everything after the frontmatter). Returns an error string if the skill is not found.

## Commands Reference

### `/skill`

```
/skill             — show usage hint
/skill list        — list all available skills
/skill <name>      — display a skill's full content
```

**Examples:**
```
/skill list
/skill go-testing
/skill security-review
```

Output is shown as a message in the conversation. If the named skill does not exist, a "not found" message is shown with a reminder to run `/skill list`.

## How It Works (Developer Notes)

### Init sequence

`Register` schedules its work via `e.OnInitAppend`, which runs after the host sends the working directory. On init:

1. Resolves `~/.config/piglet/skills/` via `xdg.ConfigDir()`.
2. Calls `NewStore(dir)` which reads all `.md` files, parses YAML frontmatter, and caches each skill's body in memory.
3. If the store is empty, returns early — no tools, commands, or prompt section are registered.
4. Registers a prompt section (order 25) listing skill names and descriptions.

Tools and commands are registered unconditionally in `Register` (before `OnInitAppend` runs), but the tool handlers check `if store == nil` and return gracefully.

### Store

`Store` in `store.go` holds all parsed skills in a `[]Skill` slice. `Load(name)` does a case-insensitive linear scan. `Match(text)` checks each skill's triggers against the lowercased input and returns hits sorted by trigger length descending so the most specific match wins.

### Message hook

Priority 500. On each incoming user message, `Match` is called against the full message text. If at least one skill matches, the body of the highest-ranked skill (longest trigger) is prepended to the message as `# Skill: <name>\n\n<body>`. Only the top match is injected; the injection appears as an augmented message, not a separate turn.

### SDK hooks used

| SDK call | Purpose |
|----------|---------|
| `e.OnInitAppend` | Defer scan until CWD is available |
| `e.RegisterPromptSection` | Inject skills list into system prompt |
| `e.RegisterTool` | Expose `skill_list` and `skill_load` |
| `e.RegisterCommand` | Expose `/skill` |
| `e.RegisterMessageHook` | Auto-trigger on keyword match |
| `e.ShowMessage` | Display command output to user |

## Related Extensions

- [prompts](prompts.md) — slash-command-driven prompt templates (complementary: prompts for structured templates, skill for reference knowledge)
- [route](route.md) — uses manifest `intents` and `triggers` to route tasks; skill manifest declares `intents: [explain, write]`
- [behavior](behavior.md) — loads behavioral guidelines into the system prompt (similar pattern, different directory)
