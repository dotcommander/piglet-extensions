# Scaffold

Generate project scaffolds and boilerplate code.

## Quick Start

```
# Create a new extension skeleton
/ext-init my-tool
```

```
Created extension at ~/.config/piglet/extensions/my-tool/

Files:
  manifest.yaml — extension config
  index.ts      — your code

Install SDK: cd ~/.config/piglet/extensions/my-tool && bun add @piglet/sdk
Restart piglet to load.
```

## What It Does

Scaffold provides a single `/ext-init` command that writes a two-file TypeScript extension skeleton (manifest + entry point) into the piglet extensions directory. The generated files use the `@piglet/sdk` package and register a sample tool and slash command so you have a working starting point.

## Capabilities

| Capability | Detail |
|------------|--------|
| `commands` | `/ext-init` |

No tools, no prompt sections, no event handlers.

## Commands Reference

### `/ext-init`

Scaffold a new extension at `<extensions-dir>/<name>/`.

```
/ext-init <name>
```

```
/ext-init webhook-notifier
/ext-init my-tool
```

The command calls `e.ExtensionsDir(ctx)` to locate the host-configured extensions directory, then writes two files:

**`manifest.yaml`**

```yaml
name: <name>
version: 0.1.0
runtime: bun
entry: index.ts
capabilities:
  - tools
  - commands
```

**`index.ts`**

```typescript
import { piglet } from "@piglet/sdk";

piglet.setInfo("<name>", "0.1.0");

piglet.registerTool({
  name: "<name>_hello",
  description: "A greeting tool",
  parameters: {
    type: "object",
    properties: {
      name: { type: "string", description: "Name to greet" },
    },
    required: ["name"],
  },
  execute: async (args) => {
    return { text: "Hello, " + args.name + "!" };
  },
});

piglet.registerCommand({
  name: "<name>",
  description: "Run <name>",
  handler: async (args) => {
    piglet.notify("<name>: " + (args || "no args"));
  },
});
```

The command refuses if the directory already exists — it will not overwrite an existing extension.

## How It Works (Developer Notes)

**No OnInit**: Scaffold does not register an `OnInit` hook because it has no CWD-dependent state. `Register` directly calls `e.RegisterCommand`.

**Extensions directory**: The handler calls `e.ExtensionsDir(ctx)` — a host RPC method — to resolve the correct install path. This avoids hardcoding `~/.config/piglet/extensions/` and respects any host configuration overrides.

**File writes**: Both files are written atomically via `xdg.WriteFileAtomic` (temp file + rename). The command checks for an existing directory before writing and refuses to overwrite.

**Template substitution**: Templates are embedded from `scaffold/defaults/*.tmpl` using `//go:embed`. The `strings.NewReplacer` substitutes `{{NAME}}` placeholders at runtime.

**Input validation**: Extension names must match `[a-z][a-z0-9_-]+`. This rejects path traversal (`../`), spaces, special characters, and uppercase.

**TypeScript target**: The skeleton uses the `bun` runtime. To scaffold a Go extension instead, use the extension source in this repo (`lsp/`, `repomap/`, etc.) as a reference and follow the architecture described in `CLAUDE.md`.

## Related Extensions

- [lsp](lsp.md) — example of a Go extension with multiple tools and OnInit
- [pipeline](pipeline.md) — example of an extension with both tools and commands
