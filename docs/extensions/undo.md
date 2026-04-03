# Undo

Restore files to their pre-edit state from snapshots taken before model edits.

## Quick Start

```
/undo
```

Lists all files that have snapshots. To restore a specific file:

```
/undo main.go
```

## What It Does

Undo queries the host for undo snapshots — copies of files saved before the model made edits in the current session. You can list available snapshots or restore a specific file by name. Snapshots are managed by the piglet host, not by this extension; the extension is the user-facing interface to the host's snapshot store.

## Capabilities

| Capability | Name | Description |
|-----------|------|-------------|
| Command | `undo` | List available snapshots or restore a file |

## Configuration

Undo has no configuration file. Snapshot storage is managed by the host.

## Commands Reference

### `/undo`

```
/undo [filename]
```

| Argument | Description |
|---------|-------------|
| *(none)* | List all available snapshots with file sizes. |
| `filename` | Restore the named file to its snapshot. Matches by exact path or path suffix. |

**List all snapshots:**

```
/undo

Undo snapshots available:

  main.go (4.2K)
  internal/parser.go (12.1K)
  go.mod (1.1K)

Usage: /undo <filename>
```

**Restore a file:**

```
/undo main.go
→ Restored: /Users/gary/project/main.go

/undo parser.go
→ Restored: /Users/gary/project/internal/parser.go
```

> **Suffix matching.** The argument is matched against both the exact path and any path ending with `/<filename>`. This lets you type just `parser.go` instead of the full path.

**No snapshot found:**

```
/undo config.yaml
→ No snapshot for: config.yaml
```

**No snapshots in session:**

```
/undo
→ No undo snapshots available
```

## How It Works (Developer Notes)

**SDK hooks used:** `e.RegisterCommand`, `e.UndoSnapshots`, `e.UndoRestore`.

**Host RPC calls:**
- `e.UndoSnapshots(ctx)` returns `map[string]int` — full absolute path → byte size.
- `e.UndoRestore(ctx, path)` instructs the host to restore the file at the given path from its snapshot.

**Path matching:** The handler iterates `snapshots` and checks `path == target || strings.HasSuffix(path, "/"+target)`. The first match wins.

**Size formatting:** `formatSize` formats byte counts as B, K, M, or G with one decimal place for values ≥ 1024.

**No init required:** Undo has no `OnInit` hook — it makes live RPC calls at command time rather than caching state at startup.

## Related Extensions

- [session-tools](session-tools.md) — fork and hand off sessions
- [admin](admin.md) — inspect config paths and bootstrap the config directory
