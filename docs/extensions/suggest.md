# suggest

Proactive next-prompt suggestions based on conversation context and project state.

## Quick Start

```bash
# Install the extension
make extensions-suggest

# Suggestions appear automatically after turns that used tools.
# To tune behavior, edit the config:
cat ~/.config/piglet/extensions/suggest/suggest.yaml
```

After a turn where the LLM called tools (e.g. Read, Edit, Bash), the suggest extension fires an LLM call and shows a suggested next prompt in the conversation — something like:

```
run go test ./internal/api/... to verify the handler changes
```

## What It Does

The suggest extension listens for `EventTurnEnd` events. When the turn involved at least one tool call, it gathers project context (git status, modified files, last tool used), calls a small LLM with a focused system prompt, filters the result for quality, and surfaces the suggestion as a message. A cooldown counter prevents suggestions on every consecutive turn.

## Capabilities

| Capability | Name | Priority | Description |
|------------|------|----------|-------------|
| eventHandler | `suggest-turn-end` | 200 | Fires after each turn, generates and shows a suggestion |

## Configuration

**Config file:** `~/.config/piglet/extensions/suggest/suggest.yaml`  
**Prompt file:** `~/.config/piglet/extensions/suggest/prompt.md`

Both files are created with defaults on first run if missing.

### suggest.yaml

```yaml
model: small          # LLM model alias to use for generation
max_tokens: 50        # Maximum tokens in the suggestion response
timeout: 5s           # LLM call timeout
cooldown: 3           # Turns to skip after making a suggestion
enabled: true         # Set to false to disable entirely
trigger_mode: auto    # Reserved for future use
```

**Fields:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `model` | string | `small` | Model alias passed to `e.Chat()`. The host resolves `small` to a configured fast model |
| `max_tokens` | int | `50` | Hard cap on response length; keeps suggestions short |
| `timeout` | duration | `5s` | Context timeout for the LLM call |
| `cooldown` | int | `3` | Number of turns to skip after a suggestion fires. Prevents suggestion spam |
| `enabled` | bool | `true` | Master switch. `false` disables all suggestions |
| `trigger_mode` | string | `auto` | Currently unused; reserved for future per-intent triggering |

### prompt.md

`~/.config/piglet/extensions/suggest/prompt.md` is the system prompt sent to the suggestion LLM. Edit it to change the suggestion style or focus.

Default content:

```
You suggest the user's next prompt based on conversation context.

Rules:
- Output ONE short prompt (max 80 chars)
- Make it actionable and specific
- Reference files, functions, or tasks mentioned
- Skip obvious suggestions ("continue", "done?")
- If the task appears complete, suggest verification or next logical step
- If there was an error, suggest a fix or investigation

Output format: Just the prompt text, no quotes, no explanation.
```

## How It Works (Developer Notes)

### Init sequence

`Register` uses `e.OnInit` (not `OnInitAppend`) to capture the CWD and build the `Suggester` after the host sends initialization data.

```go
e.OnInit(func(x *sdk.Extension) {
    cwd.Store(x.CWD())
    cfg := LoadConfig()
    prompt := LoadPrompt()
    s = NewSuggester(cfg, prompt, x)
})
```

`LoadConfig` and `LoadPrompt` both call `xdg.LoadYAMLExt` / `xdg.LoadOrCreateExt`, which create the config files with defaults via atomic write if they don't exist, then read them back.

### Event flow

On `EventTurnEnd`:

1. `s.ShouldSuggest()` — checks `Enabled` and decrements the cooldown counter. Returns false if disabled or on cooldown.
2. Checks `len(turn.ToolResults) == 0` — skips turns with no tool use (pure text turns don't need suggestions).
3. `GatherContext(cwd, turn)` — runs `git status --porcelain` in the project directory to determine git state and collect up to 5 modified filenames.
4. `s.Generate(ctx, turn, projCtx)` — builds a context string from git state, last tool name, and a 500-character truncation of the assistant's last response, then calls `e.Chat()`.
5. `s.filter(suggestion)` — applies three filters:
   - Truncates to 80 characters at a word boundary.
   - Blocks generic patterns (`continue`, `proceed`, `done?`, etc.) when the suggestion is shorter than 20 characters.
   - Deduplicates against the last 100 seen suggestions (case-insensitive).
6. If a non-empty suggestion passes all filters, `s.ResetCooldown()` sets the cooldown counter and `sdk.ActionShowMessage(suggestion)` displays it.

### Cooldown mechanics

`ShouldSuggest` decrements the cooldown counter on each call. After a suggestion fires, `ResetCooldown` sets it to `config.Cooldown` (default 3). This means three consecutive turns are skipped before the next suggestion can appear.

### Filter: duplicate detection

The `seen` map tracks suggestions case-insensitively. When the map grows beyond 100 entries it is reset entirely to prevent unbounded memory growth. This means very long sessions may occasionally re-surface an old suggestion.

### SDK hooks used

| SDK call | Purpose |
|----------|---------|
| `e.OnInit` | Capture CWD and build Suggester after host init |
| `e.RegisterEventHandler` | Subscribe to `EventTurnEnd` |
| `e.Chat` | Call small LLM for suggestion generation |
| `sdk.ActionShowMessage` | Surface the suggestion as a visible message |

### Extending the prompt

The prompt file at `~/.config/piglet/extensions/suggest/prompt.md` is loaded at init time. Edit it and restart the session to change suggestion behavior. You can make suggestions domain-specific (e.g., always suggest running a specific test suite) or adjust tone without touching Go code.

## Related Extensions

- [route](route.md) — classifies intent; suggest could be combined with route to produce intent-aware suggestions
- [plan](plan.md) — task tracking; suggest naturally complements plan by recommending the next task step
- [skill](skill.md) — domain knowledge loading; a loaded skill gives the LLM better context for suggesting relevant next steps
