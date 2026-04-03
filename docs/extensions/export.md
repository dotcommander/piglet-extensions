# Export

Export conversation history as a markdown file.

## Quick Start

```
/export
```

Saves the current conversation to a file named `piglet-export-YYYYMMDD-HHMMSS.md` in the current directory.

## What It Does

Export retrieves all messages in the current session via the host RPC and serializes them to a Markdown document. User turns become `## User` sections, assistant turns become `## Assistant` sections (with thinking blocks collapsed into `<details>` elements), and tool results become `### Tool: <name>` sections.

## Capabilities

| Capability | Name | Description |
|-----------|------|-------------|
| Command | `export` | Export conversation to a markdown file |

## Configuration

Export has no configuration file. Output path format is fixed: `piglet-export-<timestamp>.md` written to the working directory.

## Commands Reference

### `/export`

```
/export
```

No arguments. Exports the full conversation to a timestamped markdown file in the current directory.

**Example output filename:**

```
piglet-export-20240315-143022.md
```

**Document structure:**

```markdown
# Piglet Conversation

## User

<user message text>

## Assistant

<assistant text>

<details><summary>Thinking</summary>

<thinking content>

</details>

### Tool: bash

<tool result text>
```

> **Note:** The export file is written to the current working directory, not to the piglet config directory. Move it to a permanent location after exporting if you need to retain it.

## How It Works (Developer Notes)

**SDK hooks used:** `e.RegisterCommand`, `e.ConversationMessages`.

**Message deserialization:** Messages arrive as `json.RawMessage` from the host. The handler decodes each message by `role` field:
- `"user"` — content is a JSON string
- `"assistant"` — content is a JSON array of blocks; `type: "text"` blocks are rendered as text, `type: "thinking"` blocks are wrapped in `<details>`
- `"tool_result"` — content is a JSON array of blocks; the `toolName` field provides the heading

**File write:** Uses `os.WriteFile` with mode `0644`. The file is written atomically by Go's standard library buffering; no partial-write protection beyond that.

**No configuration or config path:** Export deliberately has no config file. All behavior is fixed — the timestamp format, output directory, and document structure are hardcoded in `register.go`.

## Related Extensions

- [changelog](changelog.md) — generate changelogs from git history
- [session-tools](session-tools.md) — fork sessions and transfer context with structured handoffs
