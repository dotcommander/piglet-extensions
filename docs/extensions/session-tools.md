# Session Tools

Session management, handoff, fork, and context preservation across sessions.

## Quick Start

Hand off context to a fresh session when a conversation gets long:

```
/handoff
```

Reads memory facts for the current project, builds a structured markdown summary, forks the session, and injects the summary as the first message in the new session.

Search previous sessions:

```
/search authentication refactor
```

## What It Does

Session Tools provides four commands and two tools for managing the lifecycle of piglet sessions. The `/handoff` and `handoff` tool fork the current session and inject a structured summary — built from memory facts grouped by goal, progress, decisions, files, commands, and errors — into the new session. The `/search` command finds sessions by title or directory. `/branch` forks without a summary. `/title` sets the session title manually. The `session_query` tool searches a session JSONL file for messages matching a keyword.

## Capabilities

| Capability | Name | Description |
|-----------|------|-------------|
| Command | `/search` | Search sessions by title or directory |
| Command | `/branch` | Fork conversation into a new session |
| Command | `/title` | Set the session title |
| Command | `/handoff` | Transfer context to a new session with summary |
| Tool | `session_query` | Search a session JSONL file by keyword |
| Tool | `handoff` | Fork session with structured handoff summary (model-callable) |
| Prompt | `Session Handoff` | Instructions injected at order 95 |

## Configuration

Config file: `~/.config/piglet/extensions/session-handoff/session-handoff.yaml`

```yaml
enabled: true
summary_mode: auto       # "auto", "template", or "llm"
llm_timeout: 30s
llm_max_tokens: 1024
max_query_size: 1048576  # 1MB — max session file size for session_query
```

| Option | Default | Description |
|--------|---------|-------------|
| `enabled` | `true` | Enable or disable handoff. When false, `/handoff` and the `handoff` tool return an error. |
| `summary_mode` | `auto` | How to build the handoff summary (see below). |
| `llm_timeout` | `30s` | Timeout for LLM enhancement calls. |
| `llm_max_tokens` | `1024` | Max tokens for LLM-enhanced summaries. |
| `max_query_size` | `1048576` | Max session file size the `session_query` tool will read. |

### Summary modes

| Mode | Behavior |
|------|----------|
| `template` | Build summary from memory facts only, no LLM call. |
| `llm` | Always refine the template summary with a small LLM call. |
| `auto` | Use LLM if there are more than 20 facts, or if there are 3+ `ctx:error` keys. Otherwise use template. |

### Prompt file

`~/.config/piglet/extensions/session-handoff/prompt.md` — injected into every session's system prompt at order 95. Default content:

```
Use /handoff or the handoff tool to transfer context to a new session
with a structured summary of goal, progress, decisions, and next steps.
Use the session_query tool to search a parent session's content by
keyword when you need to recover specific details after a handoff.
```

Edit this file to change the instruction phrasing or disable the prompt entirely by clearing the file.

### Enhancement prompt

`~/.config/piglet/extensions/session-handoff/enhance-prompt.md` — the system prompt used when `summary_mode` is `llm` or `auto` triggers LLM enhancement. Default instructs the model to infer a goal, mark completions, synthesize error root causes, and add next steps.

## Commands Reference

### `/search`

```
/search <query>
```

Case-insensitive substring match against session title and working directory.

```
/search auth
→ Search: auth (2 results)

  Authentication Refactor — /Users/gary/myapp (2024-03-10T14:22:00Z)
  Fix OAuth Token Expiry — /Users/gary/myapp (2024-03-12T09:15:00Z)

Use /session to open a specific session.
```

### `/branch`

```
/branch
```

Forks the current session without injecting a summary. The new session starts empty; the parent session is preserved.

```
/branch
→ Branched from abc12345 (47 messages)
```

### `/title`

```
/title <title>
```

Sets the current session's display title.

```
/title Fix Worker Pool Race Condition
→ Session title: Fix Worker Pool Race Condition
```

### `/handoff`

```
/handoff [focus]
```

| Argument | Description |
|---------|-------------|
| *(none)* | Build summary from all memory facts for the current project. |
| `focus` | Optional focus area appended as a `## Requested Focus` section in the summary. |

```
/handoff
→ Handoff complete. Forked from abc12345 (47 messages).
  Summary injected into new session.

/handoff refactor the auth module
```

The injected summary has this structure:

```markdown
# Session Handoff Summary

## Goal
- <from ctx:goal memory keys>

## Progress
- <from ctx:edit memory keys>

## Key Decisions
- <from ctx:decision memory keys>

## Context

### Files
- <from ctx:file memory keys>

### Commands
- `<from ctx:cmd memory keys>`

## Errors Encountered
- <from ctx:error memory keys>

## Other Facts
- **key**: value

## Requested Focus
<focus argument>
```

After the summary the parent session's JSONL path is injected as `[Parent Session: /path/to/session.jsonl]` so `session_query` can reference it without guessing.

## Tools Reference

### `session_query`

Search a session JSONL file for messages matching a keyword query.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `session_path` | string | Yes | Path to the session JSONL file |
| `query` | string | Yes | Keyword or phrase to search for |

Returns up to 50 matching excerpts (500 characters each), prefixed with the message role. Returns an error if the file exceeds `max_query_size`.

```
Recover the database schema we discussed last session.
→ [uses session_query with the parent session path and query "database schema"]
```

### `handoff`

Transfer context to a new session. Model-callable equivalent of `/handoff`.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `focus` | string | No | Focus area for the new session |
| `reason` | string | No | Why the handoff is needed (appended to focus) |

## How It Works (Developer Notes)

**SDK hooks used:** `e.OnInitAppend`, `e.RegisterCommand`, `e.RegisterTool`, `e.RegisterPromptSection`, `e.Sessions`, `e.ForkSession`, `e.SetSessionTitle`, `e.Chat`, `e.SendMessage`.

**Init sequence:** `OnInitAppend` captures the CWD (needed for memory file path resolution) and loads config. The prompt section is registered inside `OnInitAppend` so it has access to the configured content.

**Memory integration:** `BuildSummary` computes a SHA-256 hash of the CWD, takes the first 12 hex characters, and appends `.jsonl` to produce the memory file path: `~/.config/piglet/memory/<hash>.jsonl`. This is the same path scheme used by the `memory` extension — session-tools reads it directly without going through the memory extension.

**Fact grouping:** Facts are bucketed by key prefix: `ctx:goal`, `ctx:edit`, `ctx:file`, `ctx:error`, `ctx:cmd`, `ctx:decision`. Everything else goes into `## Other Facts`.

**LLM enhancement:** `EnhanceSummary` calls `e.Chat` with `Model: "small"` and a 30-second deadline. On any error or empty response, the original template summary is returned unchanged.

**`ForkSession` behavior:** After forking, `ext.Sessions` is called to find the parent session's JSONL path. If found, a second `SendMessage` injects `[Parent Session: /path]` into the new session so `session_query` has a concrete reference.

**Config namespace:** Uses `"session-handoff"` as the XDG extension namespace (not `"session-tools"`) for backward compatibility with existing config files.

## Related Extensions

- [autotitle](autotitle.md) — automatically generate session titles
- [usage](usage.md) — monitor token consumption to decide when to hand off
- [export](export.md) — export full conversation history to markdown
