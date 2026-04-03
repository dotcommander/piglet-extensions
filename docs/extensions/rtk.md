# RTK

Token-optimized bash command rewriter that intercepts every `bash` tool call and rewrites it through the RTK CLI for reduced output.

## Quick Start

```bash
# Verify rtk is installed
which rtk
rtk --version

# The extension activates automatically when rtk is in PATH.
# Bash commands are rewritten transparently — no configuration required.
```

The extension also injects a one-line system prompt note so the model knows rewriting is active:

> Bash commands are automatically optimized by RTK for reduced token output. No action needed.

## What It Does

RTK registers a `Before` interceptor on the `bash` tool. When a command arrives, the interceptor shells out to `rtk rewrite <command>` and substitutes the rewritten command before execution. If `rtk` is not found in `PATH`, the entire extension skips registration — piglet starts cleanly without it.

The extension also registers a prompt section (`Order: 90`) so the model does not second-guess why command output may be shorter than expected.

## Capabilities

| Capability | Name | Priority | Description |
|-----------|------|----------|-------------|
| interceptor | `rtk` | 100 | Before-hook; rewrites `bash` commands via the RTK CLI |
| prompt section | `RTK Token Optimization` | order 90 | One-line system prompt note |

## Configuration

**Prompt file**: `~/.config/piglet/extensions/rtk/prompt.md`

The prompt content is loaded from this file using `xdg.LoadOrCreateExt`. If the file does not exist, the embedded default is written there on first run and then read back. Edit this file to change the prompt note without recompiling.

Default content:

```
Bash commands are automatically optimized by RTK for reduced token output. No action needed.
```

The RTK CLI itself has its own config and analytics (`rtk gain`, `rtk gain --history`). The extension does not manage RTK's configuration — it only calls `rtk rewrite`.

## How It Works (Developer Notes)

### Registration Flow

```
Register(e)
  └─ exec.LookPath("rtk")     // if not found, return immediately — no panic
  └─ e.RegisterInterceptor()  // priority 100 Before hook
  └─ e.RegisterPromptSection() // Order 90
```

No `OnInit` hook is needed because the extension does not depend on CWD. The `rtk` binary path is resolved once at registration time and captured in the closure.

### Interceptor Logic

```go
Before: func(ctx, toolName, args) (bool, map[string]any, error) {
    if toolName != "bash" { return true, args, nil }
    command := args["command"]
    rewritten, err := rewrite(ctx, rtkPath, command)
    if err != nil || rewritten == "" || rewritten == command {
        return true, args, nil   // pass through unchanged on any failure
    }
    modified := maps.Clone(args)
    modified["command"] = rewritten
    return true, modified, nil
}
```

The interceptor never blocks (always returns `true`). Rewrite failures are silent — the original command passes through unchanged. This keeps RTK as a best-effort optimization with no failure surface.

### Prompt Section Order

Prompt sections render from lowest to highest `Order` value:

| Order | Section |
|-------|---------|
| 10 | behavior guidelines |
| 40 | gitcontext recent changes |
| 50 | memory project facts |
| 90 | RTK token optimization note |
| 91 | sift compression note |

### Key Patterns

- `exec.LookPath` is called once at registration, not per-call.
- `maps.Clone(args)` prevents mutation of the original args map.
- `exec.CommandContext` propagates cancellation from the interceptor context to the RTK subprocess.
- The prompt file lives outside the binary (editable at runtime) via `xdg.LoadOrCreateExt`.

## Related Extensions

- [safeguard](safeguard.md) — runs before RTK at priority 2000; blocks dangerous commands before they reach RTK
- [sift](sift.md) — compresses tool output after execution at priority 50
