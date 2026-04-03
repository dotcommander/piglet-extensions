# Provider

Streaming LLM provider implementations for Anthropic, Google, and OpenAI-compatible endpoints.

## Quick Start

The provider extension activates automatically — no commands to run. Once installed, piglet routes LLM requests through this extension for all configured providers:

```yaml
# ~/.config/piglet/extensions/provider/provider.yaml

# Route Anthropic through a corporate proxy:
overrides:
  anthropic:
    base_url: https://gateway.corp.com/anthropic
    key_command: "!security find-generic-password -ws anthropic"

# Add OpenRouter as a custom provider:
custom:
  openrouter:
    api: openai
    base_url: https://openrouter.ai/api/v1
    key_command: "!op read 'op://Dev/OpenRouter/api-key'"
```

## What It Does

Provider intercepts LLM stream requests from the host and delegates them to native provider implementations. It supports three wire formats — Anthropic Messages API, Google Generative AI (Gemini), and OpenAI Chat Completions — plus any number of custom OpenAI-compatible endpoints. API keys are resolved from the host's auth store, then from a configurable `key_command` (shell command, environment variable, or literal value).

## Capabilities

| Capability | Name | Description |
|-----------|------|-------------|
| Provider | `openai` | OpenAI and compatible endpoints |
| Provider | `anthropic` | Anthropic Messages API with prompt caching |
| Provider | `google` | Google Generative AI (Gemini) |
| Provider | *(custom)* | Any additional providers defined in config |

## Configuration

Config file: `~/.config/piglet/extensions/provider/provider.yaml`

The file ships with all options commented out and is created on first use.

### Provider overrides

Redirect an existing provider through a different base URL or add custom headers:

```yaml
overrides:
  anthropic:
    base_url: https://gateway.corp.com/anthropic
    headers:
      X-Corp-Auth: "token123"
    key_command: "!security find-generic-password -ws anthropic-api"

  openai:
    base_url: https://oai-proxy.internal/v1
```

| Field | Description |
|-------|-------------|
| `base_url` | Override the default API base URL |
| `headers` | Extra HTTP headers merged into every request |
| `key_command` | How to resolve the API key (see below) |

### Custom providers

Define new provider names that map to an existing wire format:

```yaml
custom:
  openrouter:
    api: openai                          # wire format: "openai", "anthropic", "google"
    base_url: https://openrouter.ai/api/v1
    headers:
      HTTP-Referer: "https://piglet.dev"
    key_command: "!op read 'op://Dev/OpenRouter/api-key'"

  azure:
    api: openai
    base_url: https://myinstance.openai.azure.com
    key_command: "AZURE_OPENAI_KEY"
```

| Field | Required | Description |
|-------|----------|-------------|
| `api` | Yes | Wire format: `openai`, `anthropic`, or `google` |
| `base_url` | Yes | Full base URL for the provider API |
| `headers` | No | Extra HTTP headers |
| `key_command` | No | Key resolution (see below) |

### Compat overrides

Handle quirks in OpenAI-compatible endpoints that deviate from the standard:

```yaml
compat:
  deepseek-:                    # prefix match (trailing dash convention)
    strip_tool_choice: true
    headers:
      X-Custom: "value"
  deepseek-coder-v2:            # exact match takes priority over prefix
    name: "DeepSeek Coder v2 (Custom)"
  mistral-:
    strip_stream_options: true
```

Exact model ID match takes priority over longest-prefix match.

### Key resolution

The `key_command` field supports three formats:

| Format | Example | Behavior |
|--------|---------|----------|
| Shell command | `!security find-generic-password -ws my-api` | Executes via `sh -c`; stdout is the key |
| Environment variable | `ANTHROPIC_API_KEY` | Reads `os.Getenv("ANTHROPIC_API_KEY")` |
| Literal value | `sk-abc123...` | Used as-is |

Shell command results are cached for the process lifetime — expensive lookups like 1Password (`op read`) or macOS Keychain (`security`) execute only once per session.

**Key resolution order:**

1. Host auth store (`e.AuthGetKey`)
2. `key_command` from `overrides` or `custom`

## How It Works (Developer Notes)

**SDK hooks used:** `e.OnInitAppend`, `e.RegisterProvider`, `e.OnProviderStream`, `e.AuthGetKey`.

**Registration inside OnInitAppend:** `e.RegisterProvider` sends a notification immediately, so calls must be deferred until the RPC pipe (FD 4) is open. `OnInitAppend` runs after the host sends the init message.

**Stream handler:** `e.OnProviderStream` receives a `ProviderStreamRequest` containing the model (as JSON), messages, tools, and options. The handler:
1. Unmarshals the model and applies config overrides
2. Resolves the API key
3. Selects the provider implementation by `model.API` field
4. Streams events back via `x.SendProviderDelta`

**Anthropic-specific:** Implements prompt caching via `cache_control: {type: "ephemeral"}` on:
- The system prompt block
- The last tool definition
- The second-to-last user message (conversation cache breakpoint)

**Google-specific:** Schema sanitization strips fields not accepted by the Gemini API (`sanitizeSchemaForGemini`). Uses a large SSE buffer (256 KB initial, 10 MB max) because Gemini can return large JSON blobs in a single SSE event.

**Thinking support:** `StreamThinkingDelta` events are forwarded for models that support extended thinking (Anthropic) or thought signatures (Google Gemini).

**Provider implementations:**
- `Anthropic` → `https://api.anthropic.com/v1/messages` (SSE)
- `Google` → `https://generativelanguage.googleapis.com/v1beta/models/<id>:streamGenerateContent?alt=sse`
- `OpenAI` → delegated to `pigletprovider.NewOpenAI` from the SDK

## Related Extensions

- [modelsdev](modelsdev.md) — syncs context window / token limit metadata into models.yaml
- [admin](admin.md) — inspect config paths including the provider config file
