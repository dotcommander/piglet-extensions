# Autotitle

Automatically generate session titles from the first user-assistant exchange.

## Quick Start

Install and forget — autotitle fires once after your first message exchange and sets the session title without any interaction:

```
You: Fix the race condition in the worker pool
Assistant: Looking at the worker pool implementation...
# → Session title becomes: "Fix Worker Pool Race"
```

## What It Does

Autotitle listens for the `EventAgentEnd` event. After the first completed exchange in a session, it extracts the first user message and the first assistant response (up to 200 characters each), sends them to a small LLM with a terse system prompt, and sets the session title to the 2–5 word result. It fires exactly once per session — subsequent exchanges are ignored.

## Capabilities

| Capability | Name | Description |
|-----------|------|-------------|
| Event Handler | `autotitle` | Generate a title after the first agent turn |

## Configuration

Config file: `~/.config/piglet/extensions/autotitle/prompt.md`

Created automatically on first use with this content:

```
You generate concise session titles. Given a user-assistant exchange,
output a 2-5 word title. No quotes, no punctuation, just the title.
```

Edit this file to change the titling style. For example, to include emoji:

```
You generate concise session titles. Given a user-assistant exchange,
output a 2-5 word title with a leading emoji. No quotes, no other punctuation.
```

> **No YAML config.** The only configuration is the prompt file. Model, timeout, and token limits are fixed in code.

| Setting | Value |
|---------|-------|
| Model | `small` (host-resolved small model) |
| Timeout | 10 seconds |
| Max tokens | 30 |
| Max title length | 50 runes |
| Trigger event | `EventAgentEnd` (fires once, on first exchange) |

## How It Works (Developer Notes)

**SDK hooks used:** `e.RegisterEventHandler` (event: `EventAgentEnd`), `e.Chat`.

**Once-only guard:** An `atomic.Bool` (`fired`) uses `CompareAndSwap(false, true)` to ensure the handler runs at most once. If the LLM call fails or returns empty text, `fired` is reset to `false` to allow a retry on the next turn.

**Event payload:** The `EventAgentEnd` data is a JSON struct with a `Messages` array. The handler decodes it and calls `extractFirstExchange` to pull the first `"user"` and `"assistant"` messages from the list.

**LLM call:** Uses `e.Chat` (host RPC) with `Model: "small"` — the host resolves this alias to the configured small model. The call has a 10-second deadline to avoid blocking session startup.

**Prompt loading:** `xdg.LoadOrCreateExt("autotitle", "prompt.md", defaultPrompt)` — if the config file is missing, it writes the embedded default (from `//go:embed defaults/prompt.md`) before reading it back.

**Priority:** Event handler priority is 100 (mid-range; no ordering requirement relative to other handlers).

## Related Extensions

- [session-tools](session-tools.md) — manually set titles with `/title`, search sessions with `/search`
- [usage](usage.md) — track token usage per turn
