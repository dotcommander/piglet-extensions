# Extensions List

List all currently loaded extensions and their capabilities.

## Quick Start

```
/extensions
```

Displays every loaded extension with its version, kind, and registered capabilities.

## What It Does

Extensions List queries the host for metadata about all running extensions and formats it as a human-readable list. It shows each extension's name, version, kind (process or built-in), and which capabilities it has registered: tools, commands, interceptors, event handlers, shortcuts, message hooks, and compactors.

## Capabilities

| Capability | Name | Description |
|-----------|------|-------------|
| Command | `extensions` | List loaded extensions and their capabilities |

## Configuration

Extensions List has no configuration file.

## Commands Reference

### `/extensions`

```
/extensions [install | update]
```

| Argument | Description |
|---------|-------------|
| *(none)* | List all loaded extensions with capabilities. |
| `install` | Print instructions for installing extensions. |
| `update` | Print instructions for updating extensions. |

**List output:**

```
Loaded extensions (5)

  changelog v0.1.0  [process]
    commands: changelog
  mcp v0.1.0  [process]
    tools: mcp__filesystem__read_file, mcp__filesystem__list_directory
    commands: mcp
  session-tools v0.2.0  [process]
    commands: search, branch, title, handoff
    tools: session_query, handoff
  usage v0.1.0  [process]
    commands: usage
    tools: session_stats
    events: usage-tracker

Use /extensions install to install official extensions.
```

**Install/update:**

```
/extensions install
→ please install piglet extensions using: cd ~/go/src/piglet-extensions && make extensions
```

> **Kind field.** The `[process]` kind means the extension runs as a separate OS process communicating via JSON-RPC. Built-in extensions are compiled into the host binary and show `[builtin]`.

## How It Works (Developer Notes)

**SDK hooks used:** `e.RegisterCommand`, `e.ExtInfos`.

**`e.ExtInfos(ctx)`** returns `[]sdk.ExtInfo` from the host, where each entry contains:
- `Name`, `Version`, `Kind` (string)
- `Tools []string`, `Commands []string`, `Interceptors []string`
- `EventHandlers []string`, `Shortcuts []string`, `MessageHooks []string`
- `Compactor string`

**Install/update handling:** The subcommands `install` and `update` are intercepted before calling `ExtInfos` and redirect to the `make extensions` build instruction via `e.SendMessage`. This is intentional — piglet extensions are source-built, not downloaded.

**Capability display:** Each capability slice is joined with `", "` and rendered as indented lines below the extension header. Empty slices are omitted.

## Related Extensions

- [admin](admin.md) — view config file paths
- [modelsdev](modelsdev.md) — sync model catalog
