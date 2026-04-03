# Memory

Per-project key-value fact store with graph relations, session-context tracking, automatic compaction, and overflow protection for persistent cross-session context.

## Quick Start

```
# In a piglet session — save a fact
memory_set("api_url", "https://api.example.com", "config")

# Retrieve it
memory_get("api_url")
# → https://api.example.com

# List everything
memory_list()

# Link two facts
memory_relate("api_url", "auth_token")

# Traverse the graph
memory_related("api_url", max_depth=2)

# Management slash command
/memory
/memory clear
/memory delete api_url
/memory related api_url
```

Facts persist across sessions. The store is keyed by a SHA-256 hash of the project CWD so each project has its own isolated fact set.

## What It Does

Memory provides a JSONL-backed key-value store scoped to the current project directory. During `OnInit` it loads the store, injects existing facts into the system prompt under **Project Memory** (order 50), and registers a compactor triggered at 50 000 context tokens. At every turn end, two event handlers run: one extracts structured context facts from tool results (files read, edits, commands, errors) and one micro-compacts old large tool results out of the conversation history to reclaim context space. An overflow interceptor handles tool results over 50 000 characters by persisting them to disk and replacing them with a truncated reference.

## Capabilities

| Capability | Name | Priority / Order | Description |
|-----------|------|-----------------|-------------|
| tool | `memory_set` | — | Save a key-value fact |
| tool | `memory_get` | — | Retrieve a fact by key |
| tool | `memory_list` | — | List all facts with optional category filter |
| tool | `memory_relate` | — | Create a bidirectional graph edge between two facts |
| tool | `memory_related` | — | Traverse the graph from a starting key |
| command | `/memory` | — | List, delete, clear, or traverse facts from the UI |
| prompt section | `Project Memory` | order 50 | Current facts injected into system prompt |
| compactor | `rolling-memory` | threshold 50 000 | Compact old messages, summarise context facts with LLM |
| event handler | `memory-context-reset` | priority 10 | Clear `_context` category facts on `EventAgentStart` |
| event handler | `memory-extractor` | priority 50 | Extract context facts from tool results on `EventTurnEnd` |
| event handler | `memory-clearer` | priority 60 | Micro-compact old large tool results on `EventTurnEnd` |
| interceptor | `memory-overflow` | priority 30 (After) | Persist tool results > 50 000 chars to disk |

## Tools Reference

### memory_set

```
memory_set(key, value, category?)
```

Saves or updates a fact. `CreatedAt` is preserved on update; `UpdatedAt` is always refreshed. Writes atomically to the JSONL backing file.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `key` | string | yes | Unique key for this fact |
| `value` | string | yes | Value to store |
| `category` | string | no | Grouping label (e.g. `"config"`, `"arch"`) |

```
memory_set("db_url", "postgres://localhost/myapp", "config")
# → Saved: db_url
```

### memory_get

```
memory_get(key)
```

Returns the value for `key`, or `"not found: <key>"` if absent.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `key` | string | yes | Fact key to retrieve |

```
memory_get("db_url")
# → postgres://localhost/myapp
```

### memory_list

```
memory_list(category?)
```

Returns all facts sorted by key, optionally filtered to a single category. Output is `key: value` lines.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `category` | string | no | Filter to this category; omit for all facts |

```
memory_list("config")
# → api_url: https://api.example.com
#    db_url: postgres://localhost/myapp
```

### memory_relate

```
memory_relate(key_a, key_b)
```

Creates a bidirectional link between two existing facts. Both keys must already exist. Calling it again with the same pair is a no-op (idempotent). Relations are sorted alphabetically in each fact's `Relations` list.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `key_a` | string | yes | First fact key |
| `key_b` | string | yes | Second fact key |

```
memory_relate("api_url", "auth_token")
# → Linked: api_url ↔ auth_token
```

### memory_related

```
memory_related(key, max_depth?)
```

Traverses the relation graph using BFS from `key`, returning all reachable facts up to `max_depth` hops. The starting key itself is excluded from results. Results are sorted by key. Depth is capped at 10 regardless of `max_depth`.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `key` | string | yes | Starting fact key |
| `max_depth` | integer | no | Max hops (default: 3, max: 10) |

```
memory_related("api_url", max_depth=2)
# → auth_token: Bearer sk-... [→ api_url]
#    rate_limit: 1000/hr [→ auth_token]
```

## Commands Reference

### /memory

```
/memory [clear | clear context | delete <key> | related <key>]
```

Management command for browsing and modifying the fact store from the UI.

| Subcommand | Description |
|-----------|-------------|
| (none) | List all facts with categories |
| `clear` | Delete all facts and remove the backing file |
| `clear context` | Delete only `_context`-category facts (session tracking) |
| `delete <key>` | Delete a single fact and clean up its relations |
| `related <key>` | Show facts related to `<key>` (depth 3) |

```
/memory
# → Project Memory:
#      api_url: https://api.example.com (config)
#      db_url: postgres://localhost/myapp (config)
#    2 fact(s) stored.

/memory delete api_url
# → Deleted: api_url

/memory clear context
# → Cleared 14 context fact(s).
```

## Storage

Facts are stored in `~/.config/piglet/memory/<12-char-sha256>.jsonl` where the SHA-256 is derived from the absolute CWD. Each line is a JSON-encoded `Fact`:

```json
{"key":"api_url","value":"https://api.example.com","category":"config","relations":["auth_token"],"created_at":"2026-04-01T10:00:00Z","updated_at":"2026-04-03T14:22:00Z"}
```

The file is rewritten atomically (temp file + rename) on every mutation. The in-memory map is the authoritative source; the file is a durable journal.

## Session Context Tracking

The `memory-extractor` event handler fires at `EventTurnEnd` and automatically writes `_context`-category facts from each tool result:

| Tool | Key pattern | Content |
|------|------------|---------|
| `Read` | `ctx:file:<path>` | First 300 chars of result |
| `Grep`, `Glob` | `ctx:search:<path>` | First 300 chars of result |
| `Edit`, `Write` | `ctx:edit:<path>` | First 200 chars of result |
| `Bash` (success) | `ctx:cmd:<turn>` | First 300 chars of result |
| `Bash` (error) | `ctx:error:<turn>` | First 300 chars of result |
| other | `ctx:tool:<name>:<turn>` | First 200 chars of result |

The store is capped at 50 `_context` facts. When the cap is exceeded, the 10 oldest facts (by `UpdatedAt`) are pruned.

These facts are cleared at `EventAgentStart` (new session) by the `memory-context-reset` handler.

## Compactor

The `rolling-memory` compactor triggers when the conversation exceeds 50 000 context tokens. It:

1. Truncates large tool results in messages being compacted (keeps 2 000 chars each)
2. Extracts prior compaction file lists from `<read-files>` / `<modified-files>` XML tags to maintain cumulative tracking
3. Builds a structured summary from `_context` facts (files, edits, errors, commands)
4. Refines the summary using a `small` LLM model call with a structured prompt
5. Writes the refined summary back to the store as `ctx:summary`
6. Gathers critical context for re-injection (recent edits, active plan, recent errors)
7. Keeps the last 6 messages, prepends a compaction summary message and a re-injection message

### Compactor Config

**File**: `~/.config/piglet/extensions/memory/compact.yaml`

| Field | Default | Description |
|-------|---------|-------------|
| `keep_recent` | `6` | Number of recent messages to keep after compaction |
| `truncate_tool_result` | `2000` | Max chars per tool result in messages being compacted |

### LLM Summarization System Prompt

The compaction LLM call uses the prompt in `~/.config/piglet/extensions/memory/compact-system.md` (embedded default shown below). Output format: Goal, Progress, Key Decisions, Next Steps, Critical Context sections, followed by machine-parseable `<read-files>` and `<modified-files>` XML tags.

## Overflow Protection

The `memory-overflow` interceptor (priority 30, After hook) catches tool results exceeding 50 000 characters. It persists the full content to `~/.config/piglet/sessions/<PIGLET_SESSION_ID>/tool-results/<tool>-<n>.json` and replaces the result with the first 2 048 characters plus a reference:

```
... (first 2048 chars) ...
[... full output (83421 chars) persisted to /path/to/tool-result-1.json]
```

This runs before sift (priority 50) so sift never sees the oversized content.

## Micro-Compactor (Clearer)

The `memory-clearer` handler fires at `EventTurnEnd` with priority 60 (after the extractor at 50). It reads the current conversation messages and replaces the text content of any tool result older than 3 turns that exceeds 4 096 bytes with a placeholder:

```
[Old tool result content cleared to save context — Bash]
```

### Clearer Config

**File**: `~/.config/piglet/extensions/memory/memory-clearer.yaml`

| Field | Default | Description |
|-------|---------|-------------|
| `clear_turns` | `3` | Tool results older than this many turns are eligible for clearing |

## System Prompt Injection

The `Project Memory` prompt section (order 50) shows:

1. A one-line tool preamble: `Tools: memory_set (save), memory_get (retrieve by key), memory_list (browse all)`
2. **User facts** — all non-`_context` facts as `- key: value (category)` lines, capped at ~7 500 chars (oldest trimmed if needed)
3. **Session context** — if a `ctx:summary` exists, the summary text; otherwise a count summary (`N file(s) read/searched`, `N file(s) edited`, etc.)

## How It Works (Developer Notes)

### Init Sequence

```
Register(e)
  └─ e.OnInitAppend(...)
      └─ NewStore(cwd)            // derive path from sha256(cwd), load JSONL
      └─ store = s
      └─ extractor = NewExtractor(s)
      └─ x.RegisterPromptSection(BuildMemoryPrompt(s), Order: 50)
      └─ x.RegisterCompactor("rolling-memory", Threshold: 50000)
  └─ e.RegisterEventHandler("memory-context-reset", EventAgentStart, priority 10)
  └─ e.RegisterEventHandler("memory-extractor", EventTurnEnd, priority 50)
  └─ registerClearer(e)           // EventTurnEnd priority 60
  └─ registerOverflow(e)          // After interceptor priority 30
  └─ e.RegisterTool(memory_set)
  └─ e.RegisterTool(memory_get)
  └─ e.RegisterTool(memory_list)
  └─ e.RegisterTool(memory_relate)
  └─ e.RegisterTool(memory_related)
  └─ e.RegisterCommand("memory")
```

### Concurrency

The `Store` uses `sync.RWMutex` — reads are concurrent, writes exclusive. `flush()` is called under the write lock and writes atomically via `xdg.WriteFileAtomic`. The extractor and clearer both run from event handlers (single-threaded per handler invocation) and share the store's own mutex.

### Graph Traversal

`Related` uses iterative BFS with a visited map to avoid cycles. The starting key is excluded from results. Depth is capped at `maxGraphDepth = 10` regardless of the `max_depth` argument. Results are sorted by key for deterministic output.

### Relation Cleanup

`Delete` removes the deleted key from the `Relations` slice of every other fact in the store, so dangling edges never accumulate.

### Key Patterns

- `OnInitAppend` — CWD is available by the time the callback fires.
- `ext.Chat(ctx, sdk.ChatRequest{Model: "small"})` — the host manages model selection; `"small"` routes to a low-cost model.
- `ext.ConversationMessages` / `ext.SetConversationMessages` — the clearer reads and rewrites the full message list via host RPC.
- All file writes use `xdg.WriteFileAtomic` (tmp + rename) — no partial writes.
- The `_context` category is the reserved internal namespace. User categories should not start with `_`.

## Related Extensions

- [behavior](behavior.md) — guidelines at order 10, appears before memory
- [gitcontext](gitcontext.md) — repo state at order 40, appears before memory
- [sift](sift.md) — output compressor runs after memory-overflow (priority 50 vs 30)
