# Admin

View config file paths and bootstrap the piglet config directory.

## Quick Start

```
/config
```

Prints the config directory path and the status of each known config file.

## What It Does

Admin exposes two operations: listing the current state of all piglet config files (paths and whether they exist), and running a one-time setup that creates empty `config.yaml`, `behavior.md`, and a `sessions/` directory if they don't already exist.

## Capabilities

| Capability | Name | Description |
|-----------|------|-------------|
| Command | `config` | Show config paths or bootstrap the config directory |

## Configuration

Admin has no configuration file of its own — it manages other extensions' config files.

## Commands Reference

### `/config`

```
/config [--setup]
```

| Argument | Description |
|---------|-------------|
| *(none)* | Show the config directory path and file status. |
| `--setup` | Create missing config files and the `sessions/` directory. |

**Show config paths:**

```
/config

Config directory: /Users/gary/.config/piglet
  config.yaml:   /Users/gary/.config/piglet/config.yaml
  behavior.md:   /Users/gary/.config/piglet/behavior.md
  auth.json:     /Users/gary/.config/piglet/auth.json
  models.yaml:   /Users/gary/.config/piglet/models.yaml
  sessions/:     /Users/gary/.config/piglet/sessions
```

Files that do not exist show `(not created)` in place of the path. Files that exist show their absolute path.

**Bootstrap config:**

```
/config --setup
→ Created: config.yaml, behavior.md, sessions/ in /Users/gary/.config/piglet
```

If everything already exists:

```
/config --setup
→ Config already set up at /Users/gary/.config/piglet
```

> **`--setup` creates empty files.** It does not populate `config.yaml` or `behavior.md` with defaults — piglet's host binary handles that. Use `/config --setup` only when the config directory itself is missing.

## How It Works (Developer Notes)

**SDK hooks used:** `e.RegisterCommand`.

**Config dir resolution:** Uses `xdg.ConfigDir()` from the internal XDG helper, which returns `~/.config/piglet` on macOS/Linux following the XDG Base Directory spec.

**Known files:** The `configFiles` function returns a fixed list:
- `config.yaml`
- `behavior.md`
- `auth.json`
- `models.yaml`
- `sessions/`

**Setup:** `runSetup` calls `os.MkdirAll` on the config dir, then iterates `config.yaml` and `behavior.md` creating empty files with `os.Create` if they don't exist. It also creates `sessions/` with `os.MkdirAll`. Created file names are collected and reported.

**File status:** `formatFileStatus` calls `os.Stat` on each path. Existence → return full path. `IsNotExist` → return `"(not created)"`. Other error → return `"(error: ...)"`.

**No init required:** Admin has no `OnInit` hook — config directory doesn't depend on CWD.

## Related Extensions

- [modelsdev](modelsdev.md) — refresh `models.yaml` from models.dev
- [usage](usage.md) — track token consumption
- [extensions-list](extensions-list.md) — list all loaded extensions
