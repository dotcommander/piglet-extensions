# Modelsdev

Sync model metadata from models.dev and regenerate `models.yaml`.

## Quick Start

```
/models-sync
```

Fetches the latest model list from `https://models.dev/api.json` and regenerates `models.yaml` in the piglet config directory. Output reports the number of models loaded.

## What It Does

Modelsdev keeps piglet's model catalog up to date by fetching context window sizes and output token limits from the models.dev API. It uses a stale-while-revalidate strategy: on startup it checks whether the local cache is older than 24 hours and, if so, refreshes in the background without blocking session initialization. You can also trigger a synchronous refresh manually with `/models-sync`. The host RPC method `WriteModels` merges the fetched data with the host's curated model list, preserving cost data and other metadata.

## Capabilities

| Capability | Name | Description |
|-----------|------|-------------|
| Event Handler | `modelsdev` (OnInit) | Background refresh on startup if cache is stale |
| Command | `models-sync` | Synchronous refresh of models.yaml |

## Configuration

Config file: `~/.config/piglet/extensions/modelsdev/modelsdev.yaml`

Created automatically on first use.

```yaml
api_url: https://models.dev/api.json
```

| Option | Default | Description |
|--------|---------|-------------|
| `api_url` | `https://models.dev/api.json` | API endpoint to fetch model data from |

Override `api_url` to point at a local mirror or a different model registry that returns compatible JSON.

**Cache file:** `~/.config/piglet/.models-cache.json` — stores the raw API response with a `fetched_at` timestamp. Max age is 24 hours.

## Commands Reference

### `/models-sync`

```
/models-sync
```

No arguments. Fetches the models.dev API synchronously and calls the host `WriteModels` RPC to regenerate `models.yaml`. Reports the number of models written.

**Success:**

```
/models-sync
Fetching models from models.dev...
→ models.yaml regenerated — 142 model(s) loaded
```

**Failure:**

```
/models-sync
Fetching models from models.dev...
→ Sync failed: http get: context deadline exceeded
```

## How It Works (Developer Notes)

**SDK hooks used:** `e.OnInit`, `e.RegisterCommand`, `e.WriteModels`.

**Stale-while-revalidate pattern:**

```
OnInit:
  if CacheStale() → go Refresh() in background
  else → return immediately (no blocking)
```

The background goroutine has a 10-second context deadline (`refreshTimeout`). Errors are logged as warnings and do not surface to the user.

**`WriteModels` host RPC:** Sends a `map[string]sdk.ModelOverride` keyed by `"provider/model-id"` (lowercased). The host merges these overrides into its embedded curated model list, updating `ContextWindow` and `MaxTokens` while preserving pricing and other metadata. Returns the total number of models written.

**Provider mapping:** The `canonicalProvider` map translates models.dev provider keys to piglet's canonical provider names. Providers not in this map are ignored:

```go
var canonicalProvider = map[string]string{
    "anthropic":           "anthropic",
    "openai":              "openai",
    "google":              "google",
    "xai":                 "xai",
    "groq":                "groq",
    "zai":                 "zai",
    "zai-coding-plan":     "zai",
    "zhipuai-coding-plan": "zai",
}
```

**Index deduplication:** When the same model ID appears under multiple provider keys (e.g. `zai` and `zai-coding-plan`), the first-seen entry wins.

**Cache format:**

```json
{
  "fetched_at": "2024-03-15T14:30:00Z",
  "data": { "<provider>": { "models": { ... } } }
}
```

Written atomically via `xdg.WriteFileAtomic`.

## Related Extensions

- [admin](admin.md) — view config file paths including `models.yaml`
- [provider](provider.md) — streaming LLM providers that use the model catalog
