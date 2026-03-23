# piglet-extensions

External extensions for [piglet](https://github.com/dotcommander/piglet), a minimalist TUI coding assistant.

Each extension runs as a standalone binary, communicating with piglet over its extension API. Extensions can register tools, commands, shortcuts, prompt sections, interceptors, and event handlers.

## Extensions

| Extension | Capability | Description |
|-----------|-----------|-------------|
| **safeguard** | interceptor | Blocks dangerous commands (`rm -rf`, `DROP TABLE`, `git push --force`, etc.) with configurable profiles: strict, balanced, off. Audit logs decisions to JSONL. |
| **rtk** | interceptor, prompt | Integrates [RTK](https://github.com/reachingforthejack/rtk) to rewrite bash commands for 60-90% token savings. Auto-detects if `rtk` is in PATH. |
| **autotitle** | event handler | Generates short session titles from the first user/assistant exchange using a lightweight LLM call. |
| **clipboard** | tool, shortcut | Reads images from the macOS clipboard and injects them into the conversation as base64. |
| **skill** | tool, command, prompt, message hook | Manages markdown methodology files in `~/.config/piglet/skills/`. Load skills into conversations with `/skill <name>`. |
| **memory** | tool, command, prompt | Project-scoped fact storage with automatic extraction. Persists key-value facts and injects them into the system prompt. |
| **subagent** | tool | Dispatches independent sub-agents with configurable tool access, model override, and turn limits. |
| **lsp** | tool, prompt | LSP client for code intelligence -- go-to-definition, references, hover, rename, and document symbols via JSON-RPC 2.0. |
| **repomap** | tool, prompt, event | Builds a ranked repository map by scanning, parsing, and ranking symbols. Auto-rebuilds when files change. |
| **plan** | tool, command, prompt | Persistent structured task tracking with step statuses (pending, in_progress, done, skipped, failed) and propose/execute modes. |
| **bulk** | tool | Runs commands across multiple targets (git repos, directories, files, or lists) with parallel execution and filtering. |
| **mcp** | tool, command, prompt | Connects to configured MCP servers (stdio or HTTP/SSE) and exposes their tools as piglet tools. Config in `~/.config/piglet/mcp.yaml`. |

## Install

Requires Go 1.26+.

```bash
git clone https://github.com/dotcommander/piglet-extensions
cd piglet-extensions
make extensions
```

This builds each extension and installs it to `~/.config/piglet/extensions/`. Piglet discovers them automatically on next launch.

To remove all installed extensions:

```bash
make clean
```

## Configuration

Some extensions read config from `~/.config/piglet/`:

| File | Used by |
|------|---------|
| `safeguard.yaml` | safeguard -- patterns and profile |
| `autotitle.md` | autotitle -- title generation prompt |
| `subagent.md` | subagent -- sub-agent system prompt |
| `skills/` | skill -- methodology files |
| `mcp.yaml` | mcp -- server connections (stdio/HTTP) |

## Development

Each extension follows the same structure:

```
<name>/
  cmd/
    main.go          # entry point
    manifest.yaml    # name, version, runtime, capabilities
  <name>.go          # core logic
  register.go        # wires into piglet's ext.App
```

Build a single extension:

```bash
make extensions-<name>    # e.g. make extensions-safeguard
```

Run tests:

```bash
go test ./...
```

## License

MIT
