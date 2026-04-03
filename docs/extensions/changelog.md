# Changelog

Generate changelogs from git history with conventional commit parsing.

## Quick Start

```
/changelog
```

Generates a changelog from the last tag to HEAD and displays it in the terminal with ANSI color. No configuration required — it auto-detects the revision range.

## What It Does

Changelog parses git history using the [Conventional Commits](https://www.conventionalcommits.org/) format (`type(scope)!: message`) and groups commits into labeled sections. When no tag exists, it falls back to the last 20 commits. You can preview output in the terminal or write it directly to `CHANGELOG.md`.

## Capabilities

| Capability | Name | Description |
|-----------|------|-------------|
| Command | `changelog` | Generate a changelog from git history |

## Configuration

Config file: `~/.config/piglet/extensions/changelog/changelog.yaml`

Created automatically on first use with these defaults:

```yaml
# Commit type mappings — label, emoji, and display order
types:
  feat:
    label: Features
    emoji: "✨"
    order: 1
  fix:
    label: Bug Fixes
    emoji: "🐛"
    order: 2
  perf:
    label: Performance
    emoji: "⚡"
    order: 3
  refactor:
    label: Refactoring
    emoji: "♻️"
    order: 4
  docs:
    label: Documentation
    emoji: "📚"
    order: 5
  test:
    label: Tests
    emoji: "✅"
    order: 6
  build:
    label: Build
    emoji: "📦"
    order: 7
  ci:
    label: CI
    emoji: "🔧"
    order: 8
  chore:
    label: Chores
    emoji: "🧹"
    order: 9
  style:
    label: Style
    emoji: "🎨"
    order: 10
  other:
    label: Other
    emoji: "📝"
    order: 99

# Fallback commit count when no tags exist
fallback_count: 20
```

Add new commit types by appending entries to `types`. The `order` field controls display order — lower numbers appear first.

## Commands Reference

### `/changelog`

```
/changelog [ref] [--write] [--dry-run] [--config]
```

| Argument | Description |
|---------|-------------|
| `ref` | Git revision range (e.g. `v1.0.0..HEAD`, `main..HEAD`). Auto-detected if omitted. |
| `--write` | Prepend the generated changelog to `CHANGELOG.md` in the current directory. |
| `--dry-run` | Preview the markdown output without writing to disk. |
| `--config` | Display the current type-to-label mappings. |

**Examples:**

```
/changelog                        # auto-detect range, show in terminal
/changelog v1.2.0..HEAD           # explicit range
/changelog --write                # write to CHANGELOG.md
/changelog v1.0.0..v1.1.0 --dry-run  # preview markdown for a range
/changelog --config               # show active type mappings
```

**Range detection priority:**

1. Explicit `ref` argument
2. `<last-tag>..HEAD` (uses `git describe --tags --abbrev=0`)
3. `HEAD~<fallback_count>..HEAD`

**Breaking changes** — commits with a `!` suffix (e.g. `feat!: drop Go 1.20`) appear in a dedicated `⚠️ BREAKING CHANGES` section at the top of the output.

**Remote links** — when writing to markdown, commit hashes become hyperlinks using `git remote get-url origin`. SSH remotes (`git@github.com:...`) are converted to HTTPS automatically.

## How It Works (Developer Notes)

**SDK hooks used:** `e.OnInitAppend` (loads CWD and config), `e.RegisterCommand`.

**Init sequence:** `OnInitAppend` runs after the host sends the working directory. CWD is captured to `cwd` and config is loaded via `xdg.LoadYAMLExt`. All git commands use the captured CWD as their working directory.

**Config loading pattern:** `xdg.LoadYAMLExt("changelog", "changelog.yaml", defaultConfig())` — if the file doesn't exist, it writes the default and returns it, so the first run is always valid.

**File write:** Uses `xdg.WriteFileAtomic` (temp file + rename) to prepend the new section to any existing `CHANGELOG.md`. This prevents partial writes from corrupting the file.

**Commit parsing:** `convCommitRegex = ^(\w+)(?:\(([^)]+)\))?(!)?:\s*(.+)` — non-matching commits fall through to `type = "other"`. The git log format is `%H|%ai|%an|%s` (hash, date, author, subject) with `--no-merges`.

**Output modes:** Terminal output uses ANSI color (`FormatANSI`). Markdown output uses `FormatMarkdown` with date-stamped headings and linked commit hashes.

## Related Extensions

- [admin](admin.md) — view config paths and bootstrap config directory
- [export](export.md) — export the full conversation to markdown
