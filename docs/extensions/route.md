# route

Prompt classification and ranked extension/tool routing using weighted intent, domain, and trigger scoring.

## Quick Start

```bash
# Install the extension
make extensions-route

# Ask the LLM which tools to use for a task
# (The LLM calls route automatically via the message hook, or you can use it directly)
/route fix the memory leak in the websocket handler

# Output:
# Intent: debug (90%)
# Domains: go, concurrency
# Confidence: 0.38
#
# Primary:
#   lsp (extension) — 0.72 [leak, debug, fix]
#   memory (extension) — 0.61 [memory, leak]
#   subagent (extension) — 0.44
#
# Secondary:
#   webfetch (extension) — 0.18
```

## What It Does

The route extension builds a registry of all loaded piglet extensions, tools, and commands by querying the host at startup. It then classifies each incoming user message (or explicit query) using a three-signal weighted scoring pipeline — intent, domain, and trigger keywords — and returns a ranked list of relevant components. The message hook injects routing context as a `[routing: ...]` annotation prepended to each user message so the LLM knows which tools are most relevant before it responds.

## Capabilities

| Capability | Name | Priority | Description |
|------------|------|----------|-------------|
| tool | `route` | — | Classify a prompt and return ranked extensions/tools as JSON |
| tool | `route_feedback` | — | Record correct/wrong routing for feedback-driven learning |
| command | `/route` | — | Interactive routing, learning, and stats |
| messageHook | `route-classify` | 900 | Auto-classify messages and inject routing context |

## Configuration

**Config file:** `~/.config/piglet/extensions/route/config.yaml`  
**Intent taxonomy:** `~/.config/piglet/extensions/route/intents.yaml`  
**Domain taxonomy:** `~/.config/piglet/extensions/route/domains.yaml`

All three files are created with defaults on first run.

### config.yaml

```yaml
weights:
  intent: 0.40    # Weight for intent signal
  domain: 0.30    # Weight for domain signal
  trigger: 0.30   # Weight for trigger/keyword signal
  anti: 0.50      # Penalty multiplier for anti-triggers

primary_threshold: 0.25   # Minimum score to appear in primary tier
max_primary: 5            # Max components in primary results
max_secondary: 10         # Max components in secondary results

message_hook:
  enabled: true
  min_confidence: 0.10    # Skip injection below this confidence gap

trigger_keyword_ratio: 0.7  # Weight of multi-word triggers vs. single keywords
```

**Scoring formula per component:**

```
score = intent_score * 0.40
      + domain_score * 0.30
      + trigger_score * 0.30

# Anti-trigger penalty applied after:
score *= (1.0 - anti_score * 0.50)
```

### intents.yaml

Defines the intent taxonomy used to classify prompts. Each intent has `verbs` (matched as the first token or after question words) and `keywords` (substring matches anywhere in the prompt).

Built-in intents: `debug`, `build`, `refactor`, `test`, `explain`, `optimize`, `configure`, `deploy`, `design`, `write`, `search`, `review`.

Add custom intents or extend existing ones:

```yaml
intents:
  debug:
    verbs: [fix, debug, diagnose, troubleshoot, investigate, trace, bisect]
    keywords: [bug, error, crash, panic, fail, broken, wrong, issue, leak, race, deadlock]
  migrate:
    verbs: [migrate, upgrade, convert, port]
    keywords: [migration, upgrade, v2, breaking change]
```

### domains.yaml

Defines the domain taxonomy for technology detection. Each domain has `keywords` (prompt substring matches), `projects` (marker files/dirs whose presence implies the domain), and `extensions` (file extensions).

Built-in domains: `go`, `typescript`, `javascript`, `python`, `rust`, `php`, `frontend`, `database`, `infrastructure`, `concurrency`, `api`, `security`, `git`.

```yaml
domains:
  go:
    keywords: [golang, goroutine, gomod]
    projects: [go.mod, go.sum]
    extensions: [.go]
```

## Tools Reference

### `route`

Classifies a prompt and returns ranked piglet components as JSON.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `prompt` | string | yes | The task description to classify and route |

**Example:**
```json
{ "prompt": "write integration tests for the auth handler" }
```

**Response** (JSON):
```json
{
  "primary": [
    { "name": "lsp", "type": "extension", "score": 0.68, "matched": ["test", "handler"] },
    { "name": "skill_load", "type": "tool", "score": 0.55, "matched": ["write", "test"] }
  ],
  "secondary": [
    { "name": "memory", "type": "extension", "score": 0.19 }
  ],
  "intent": { "Primary": "test", "Confidence": 0.9, "Secondary": "write" },
  "domains": ["go", "api"],
  "confidence": 0.13
}
```

The `confidence` field is the score gap between the top two primary results. A larger gap means the top result is more clearly dominant.

---

### `route_feedback`

Records whether a routing recommendation was correct or wrong. Feedback is stored in `~/.config/piglet/extensions/route/feedback.jsonl` and used by `/route learn` to update trigger weights.

**Parameters:**

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `prompt` | string | yes | The original prompt that was routed |
| `component` | string | yes | Extension or tool name to give feedback on |
| `correct` | boolean | yes | `true` if this component was the right choice, `false` if wrong |

**Example:**
```json
{ "prompt": "fix the memory leak", "component": "lsp", "correct": true }
```

After recording feedback, run `/route learn` to apply the learned triggers to the live registry.

## Commands Reference

### `/route`

```
/route <prompt>    — score and display routing for a prompt
/route learn       — process feedback.jsonl and update learned triggers
/route stats       — show registry and feedback statistics
```

**Examples:**

```
/route refactor the database layer
/route learn
/route stats
```

**`/route stats` output:**
```
Registry: 14 extensions, 38 tools, 12 commands
Learned triggers: 3 components
Learned anti-triggers: 1 components
```

**`/route learn`** reads `feedback.jsonl`, tokenizes each correct-feedback prompt into learned triggers (up to 20 tokens per component) and each wrong-feedback prompt into anti-triggers (up to 10 tokens per component), saves the results to `learned-triggers.json`, and merges them into the live in-memory registry immediately.

## How It Works (Developer Notes)

### Init sequence

`Register` uses `e.OnInitAppend`. On init:

1. Loads config, intents, and domains from `~/.config/piglet/extensions/route/`.
2. Builds `IntentClassifier` and `DomainExtractor` from the taxonomies.
3. Builds `Scorer` from config + classifier + extractor.
4. Calls `BuildRegistry(ctx, x)` — queries the host for all loaded extensions (`ext.ExtInfos`), all tool descriptions (`ext.ListHostTools`), and the extensions directory path (`ext.ExtensionsDir`). Reads each extension's `manifest.yaml` from disk for `triggers`, `intents`, `domains`, and `anti_triggers` fields.
5. Loads `feedback.jsonl` feedback and `learned-triggers.json` via `FeedbackStore`, then merges learned triggers into the registry.

### Scoring pipeline

`Scorer.Score` runs four steps for each message:

1. **Tokenize** — splits on whitespace and punctuation, lowercases, strips stop words.
2. **Classify intent** — four-rule cascade: question detection → leading verb match → problem keyword → best keyword overlap. Returns primary intent with confidence 0.6–0.9 and optional secondary intent.
3. **Extract domains** — three sources: prompt keyword matches, file extension references (backtick-quoted or bare filenames in text), and project marker files in the CWD.
4. **Score each component** — computes `intent_score * w.Intent + domain_score * w.Domain + trigger_score * w.Trigger`, then applies anti-trigger penalty.

Components with `score >= primary_threshold` (default 0.25) go into the primary tier, up to `max_primary` (5). The remainder fill the secondary tier up to `max_secondary` (10).

### Registry enrichment

`BuildRegistry` reads each extension's `manifest.yaml` for routing-specific fields. Add these to your own extension manifests to improve routing accuracy:

```yaml
# manifest.yaml
triggers: [multi-word trigger phrase, another trigger]
intents: [debug, test]
domains: [go, api]
anti_triggers: [unrelated, keyword]
```

Extensions without these fields fall back to keyword extraction from their name and tool descriptions.

### Message hook

Priority 900 (runs early, before most other hooks). Formats the top routing result as a compact annotation:

```
[routing: intent=test | domains=go,api | relevant: lsp, skill_load, subagent]
```

This string is prepended to the user message text so the LLM sees it as part of the conversation context. The hook skips injection when `confidence < min_confidence` (default 0.10) and when the registry is not ready.

### Feedback learning

`FeedbackStore.Record` appends JSONL entries to `feedback.jsonl`. `FeedbackStore.Learn` reads all entries, tokenizes each prompt, and assigns tokens to `triggers` (correct=true) or `anti_triggers` (correct=false) for each component, capped at 20 and 10 tokens respectively. `mergeLearnedIntoRegistry` extends `Component.Keywords` and `Component.AntiTriggers` in-place. Learning is purely additive — the base taxonomy is never modified.

### Route log

Every scoring evaluation (from either the tool, hook, or command) appends a `RouteLogEntry` to `~/.config/piglet/extensions/route/route-log.jsonl`. Each entry records timestamp, prompt hash, intent, domains, primary results with scores, confidence, and source (`tool`/`hook`/`command`).

### SDK hooks used

| SDK call | Purpose |
|----------|---------|
| `e.OnInitAppend` | Defer registry build until CWD and host are ready |
| `e.ExtInfos` | Discover all loaded extensions and their tools/commands |
| `e.ListHostTools` | Fetch tool descriptions from the host |
| `e.ExtensionsDir` | Locate extension manifests on disk |
| `e.RegisterTool` | Expose `route` and `route_feedback` |
| `e.RegisterCommand` | Expose `/route` |
| `e.RegisterMessageHook` | Auto-classify messages at priority 900 |
| `e.ShowMessage` | Display command output |

## Related Extensions

- [skill](skill.md) — declares `intents: [explain, write]` and `triggers` in its manifest, which route uses for scoring
- [suggest](suggest.md) — fires after turns end; combining suggest's context with route's classification could enable intent-aware suggestions
- [safeguard](safeguard.md) — runs interceptors at priority 2000; route's message hook runs at 900, so routing context is available when safeguard evaluates tool calls
