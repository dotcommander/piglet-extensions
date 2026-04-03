# Webfetch

Fetch URLs as clean markdown or search the web with automatic provider fallback.

## Quick Start

```
# Fetch a URL
"Summarize the content at https://go.dev/doc/effective_go"
→ web_fetch tool: url="https://go.dev/doc/effective_go"

# Search the web
"What are the latest changes in Go 1.23?"
→ web_search tool: query="Go 1.23 release notes"

# Fetch a GitHub repo
"Show me the structure of github.com/go-rod/rod"
→ web_fetch tool: url="https://github.com/go-rod/rod"
```

No configuration is required. Jina reader (free tier, no key) handles fetches. Brave, Exa, Gemini, and Perplexity activate when you add API keys.

## What It Does

Webfetch provides two tools — `web_fetch` and `web_search` — backed by a cascading provider chain. Fetch tries providers in order: Colly (local scraper) → Jina reader proxy → Rod (headless Chrome) → agent-browser (CDP automation) → Gemini → Perplexity. Search tries: Brave → Exa → Gemini → Perplexity → Jina → DuckDuckGo fallback. A provider is skipped if its API key is missing or if it returns a non-recoverable error. Results are cached (24 hours for fetches, 1 hour for searches). A third tool, `webfetch_get_stored`, retrieves full cached content when the original response was truncated.

GitHub URLs (`github.com/owner/repo`) receive special handling: shallow clone for small repos, GitHub API fallback for large ones (>350 MB).

## Capabilities

| Capability | Detail |
|------------|--------|
| `tools` | `web_fetch`, `web_search`, `webfetch_get_stored` |
| `prompt` | Injects a "Web Access" section at order 85 |

## Configuration

```
~/.config/piglet/extensions/webfetch/webfetch.yaml
```

The file is created with defaults on first use. Legacy path `~/.config/piglet/webfetch.yaml` is automatically migrated.

```yaml
# API keys — also read from environment variables (e.g. JINA_API_KEY)
jina_api_key: ""
brave_api_key: ""
exa_api_key: ""
gemini_api_key: ""
perplexity_api_key: ""

github:
  enabled: true
  skip_large_repos: true     # use API for repos >350 MB

# Provider endpoints — safe to omit; defaults are shown
gemini_config:
  api_base: "https://generativelanguage.googleapis.com/v1beta"
  model: "gemini-2.0-flash"

perplexity_config:
  api_url: "https://api.perplexity.ai/chat/completions"
  model: "llama-3.1-sonar-small-128k-online"

exa_config:
  search_url: "https://api.exa.ai/search"
  contents_url: "https://api.exa.ai/contents"

jina_config:
  reader_base: "https://r.jina.ai/"
  search_base: "https://s.jina.ai/"

brave_config:
  search_url: "https://api.search.brave.com/res/v1/web/search"

duckduckgo_config:
  search_url: "https://html.duckduckgo.com/html/"
```

API keys are read from the config file first, then from environment variables:

| Provider | Env variable |
|----------|--------------|
| Jina | `JINA_API_KEY` |
| Brave | `BRAVE_API_KEY` |
| Exa | `EXA_API_KEY` |
| Gemini | `GEMINI_API_KEY` |
| Perplexity | `PERPLEXITY_API_KEY` |

## Tools Reference

### `web_fetch`

Fetch a URL and return its content as clean markdown text. Handles GitHub repos, static pages, JS-rendered SPAs, and Cloudflare-protected pages depending on which providers are available.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | yes | URL to fetch |
| `raw` | boolean | no | Return raw HTML instead of extracted text (default: false) |

```json
{ "url": "https://pkg.go.dev/net/http" }
```

```json
{ "url": "https://api.example.com/data.json", "raw": true }
```

Output is truncated at 100 KB with a `[Content truncated at 100KB]` note appended. Use `webfetch_get_stored` to retrieve the full cached content.

---

### `web_search`

Search the web and return results as a numbered markdown list with title, URL, and snippet per result.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `query` | string | yes | Search query |
| `limit` | integer | no | Maximum number of results (default: 5) |

```json
{ "query": "golang context cancellation patterns", "limit": 10 }
```

Output format:
```
1. **Title of Result**
   https://example.com/page
   Brief description of the result...

2. ...
```

---

### `webfetch_get_stored`

Retrieve the full cached content from a previous `web_fetch` or `web_search` call. Use this when the original response was truncated and you need the complete content.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | one of `url`/`query` | URL from a previous `web_fetch` call |
| `query` | string | one of `url`/`query` | Query from a previous `web_search` call |

```json
{ "url": "https://pkg.go.dev/net/http" }
```

Returns an error if the URL or query is not in the session cache.

## Provider Chain

### Fetch Providers

Providers are tried in order. The first successful result is returned and cached.

| Order | Provider | Requires | Description |
|-------|----------|----------|-------------|
| 1 | **Colly** | nothing | Local HTML scraper. Fast and free. Falls through if page content is under 200 bytes (likely JS-rendered). |
| 2 | **Jina** | nothing (free tier) | Reader proxy at `r.jina.ai`. Converts any URL to clean markdown. API key unlocks higher rate limits. |
| 3 | **Rod** | Chrome/Chromium in PATH | Headless Chrome via CDP. Handles SPAs and dynamic content. Lazy-launches a shared browser instance. |
| 4 | **agent-browser** | `agent-browser` binary in PATH | CDP automation via the agent-browser CLI. Handles Cloudflare and complex interactive pages. |
| 5 | **Gemini** | `GEMINI_API_KEY` | Asks Gemini to fetch and summarize the URL. Detects LLM refusals and falls through. |
| 6 | **Perplexity** | `PERPLEXITY_API_KEY` | Asks Perplexity to fetch and summarize the URL. Rate-limited to 10 req/min (6-second minimum interval). |

### Search Providers

| Order | Provider | Requires | Description |
|-------|----------|----------|-------------|
| 1 | **Brave** | `BRAVE_API_KEY` | Structured search API. Returns titles, URLs, and descriptions. |
| 2 | **Exa** | `EXA_API_KEY` | Neural search API with `type: "auto"`. |
| 3 | **Gemini** | `GEMINI_API_KEY` | LLM-generated search results wrapped as a single `SearchResult`. |
| 4 | **Perplexity** | `PERPLEXITY_API_KEY` | LLM-generated search results. Rate-limited. |
| 5 | **Jina** | nothing | Search via `s.jina.ai`. Free tier. |
| 6 | **DuckDuckGo** | nothing | Fallback: fetches a DuckDuckGo HTML search page via the fetch provider chain. |

### Recoverability Logic

A provider error is non-recoverable (chain stops) when:
- HTTP 4xx, excluding 401 (unauthorized), 403 (forbidden), 429 (rate limit), 451 (legal)

All other errors are recoverable: network failures, 5xx, 429, 204 (empty content), LLM refusals.

### LLM Refusal Detection

Gemini and Perplexity sometimes respond to fetch requests with "I cannot access URLs" instead of content. The refusal detector checks the first 100 bytes against a list of known refusal prefixes. Detected refusals are treated as recoverable errors so the chain continues.

### GitHub Handling

For `github.com` URLs, the GitHub client runs before the fetch provider chain:

1. **Parse URL**: Supports `owner/repo`, `owner/repo/tree/branch`, `owner/repo/tree/branch/path`, and `owner/repo/commit/<sha>`.
2. **Size check**: If `skip_large_repos` is true, checks the GitHub API for repo size. Repos over 350 MB skip to API fallback.
3. **Clone**: `git clone --depth 1` to a temp directory (`/tmp/piglet-gh-<owner>-<repo>`). Reuses existing clone if present.
4. **API fallback**: For large repos or clone failures, fetches README and file tree via GitHub API.
5. **Cleanup**: Clones older than 1 hour are deleted on the next fetch.

## How It Works (Developer Notes)

**Init**: `Register` runs at extension startup (not in `OnInit`) because config loading does not depend on CWD. `LoadConfig` reads `~/.config/piglet/extensions/webfetch/webfetch.yaml`, migrating from the legacy flat path if needed. `NewWithConfig` constructs all providers, skipping those with empty API keys.

**Caching**: Two layers. The `cache` package (shared across extensions) stores results keyed by URL or `query:limit` with 24h TTL for fetches and 1h for searches. `Storage` holds the same results in-memory for the session to support `webfetch_get_stored` after context truncation.

**Provider construction**: `NewWithConfig` always creates Colly, Jina, and Rod (they need no key). `NewAgentBrowserProvider` returns nil if `agent-browser` is not in PATH. Gemini, Perplexity, Brave, and Exa return nil if their key is empty — nil providers are not appended to the chain.

**Prompt customization**: Perplexity and Gemini use prompt templates for both fetch and search. The active prompts live at `~/.config/piglet/extensions/webfetch/{fetch,search}-prompt.txt` and `{gemini-fetch,gemini-search}-prompt.txt`. Edit them to change what the LLM is asked to do with each URL or query. Defaults are embedded in the binary via `//go:embed` and written on first use.

**Rate limiting**: `PerplexityProvider` holds a `rateLimiter` with a 6-second interval (10 req/min). `Wait()` blocks until the interval has elapsed since the last call. The limiter is per-provider instance, not per-process.

**Prompt section order**: 85 — after repomap (95 is last; webfetch at 85 appears before repomap in system prompt ordering since lower order = earlier).

## Related Extensions

- [repomap](repomap.md) — for reading local code structure; prefer over `web_fetch` for local files
- [pipeline](pipeline.md) — for multi-step workflows that include web fetch steps
