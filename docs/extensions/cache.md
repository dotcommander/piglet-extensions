# Cache

File-backed TTL cache library for use by other piglet extensions.

## Quick Start

```go
import "github.com/dotcommander/piglet-extensions/cache"

// Store a value for 24 hours
cache.Set("webfetch", "https://example.com", body, 24*time.Hour)

// Retrieve it (returns "" and false on miss or expiry)
if val, ok := cache.Get("webfetch", "https://example.com"); ok {
    return val
}
```

## What It Does

Cache provides a simple key-value store backed by JSON files under `~/.config/piglet/cache/<namespace>/`. Keys are SHA-256 hashed to produce safe filenames. Entries expire by TTL and are evicted by a GC that caps each namespace at 500 entries. Cache is a library-only extension — it registers no tools, commands, or prompt sections.

## Capabilities

Cache registers no capabilities with the extension host. It is a pure Go library imported by other extensions.

## Configuration

Cache has no configuration file. Storage location is fixed: `~/.config/piglet/cache/<namespace>/`.

| Setting | Value |
|---------|-------|
| Max entries per namespace | 500 |
| GC strategy | Evict oldest by `mtime` when count exceeds max |
| Entry format | JSON (`{key, value, expires_at}`) |
| Key hashing | SHA-256 hex, truncated to 64 chars + `.json` |

## API Reference

### `Get(namespace, key string) (string, bool)`

Retrieve a cached value. Returns `("", false)` on miss or if the entry has expired (also deletes the expired file).

```go
val, ok := cache.Get("modelsdev", "api-response")
```

### `Set(namespace, key, value string, ttl time.Duration) error`

Store a value. `ttl <= 0` means no expiry. Calls `GC` after each write to enforce the 500-entry limit.

```go
err := cache.Set("modelsdev", "api-response", jsonBody, 24*time.Hour)
```

### `Delete(namespace, key string) error`

Remove a single entry. Returns nil if the entry doesn't exist.

```go
err := cache.Delete("webfetch", url)
```

### `Purge(namespace string) error`

Remove all entries in a namespace via `os.RemoveAll`.

```go
err := cache.Purge("webfetch")
```

### `GC(namespace string, maxEntries int) error`

Evict the oldest entries when `count > maxEntries`. Called automatically by `Set`. You can call it manually with a custom limit.

```go
err := cache.GC("modelsdev", 100)
```

## How It Works (Developer Notes)

**Storage layout:**

```
~/.config/piglet/cache/
  webfetch/
    a3f2c1...json   # SHA-256 of key
    b8e9d4...json
  modelsdev/
    c7f1a2...json
```

**Atomic writes:** `Set` uses `xdg.WriteFileAtomic` (write to temp file, then rename) to prevent partial writes from leaving corrupt entries.

**Expiry check:** `Get` checks `entry.ExpiresAt.IsZero()` before comparing with `time.Now()`. Zero value means no expiry (TTL was ≤ 0 at write time). Expired entries are removed with `os.Remove` before returning the miss.

**GC algorithm:** Reads all `.json` files in the namespace directory, sorts by `mtime` ascending, then removes the oldest `len(files) - maxEntries` entries. Called after every `Set`.

**Extension binary:** The `cache` extension binary exists only to satisfy the piglet extension manifest convention. `Register` is a no-op — the library is imported directly by other extensions, not loaded as a separate process.

## Related Extensions

- [modelsdev](modelsdev.md) — uses the cache library to store API responses
- [webfetch](../cli-tools/webfetch.md) — uses the cache library for URL fetch results
