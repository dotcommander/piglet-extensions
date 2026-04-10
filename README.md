# piglet-extensions

External extensions for [piglet](https://github.com/dotcommander/piglet), a minimalist TUI coding assistant.

Each extension runs as a standalone binary, communicating with piglet over JSON-RPC. Extensions register tools, commands, shortcuts, prompt sections, interceptors, and event handlers.

## Extensions

37 extensions across security, memory, orchestration, development, knowledge, content, and system administration. See the full [Extension Catalog](docs/extensions/index.md) for details on each.

| Category | Extensions |
|----------|-----------|
| Security & Filtering | safeguard, rtk, sift, tokengate |
| Context & Memory | memory, gitcontext, behavior, session-tools |
| Task Orchestration | plan, tasklist, coordinator, subagent, loop, background, cron, inbox |
| Development Tools | lsp, repomap, pipeline, bulk, webfetch, scaffold |
| Knowledge & Routing | skill, prompts, suggest, route |
| Content & Export | changelog, export, clipboard, autotitle, undo |
| System & Admin | admin, usage, cache, extensions-list, modelsdev, provider, mcp |

## CLI Tools

12 standalone command-line tools built from `cmd/`, independent of piglet. See the full [CLI Reference](docs/cli.md).

| Tool | Description |
|------|-------------|
| `repomap` | Token-budgeted repo structure map with symbols |
| `pipeline` | Multi-step YAML workflows with params, loops, retries |
| `bulk` | Parallel shell commands across repos, dirs, files |
| `confirm` | Scoped typecheck + test + lint for changed packages |
| `depgraph` | Dependency graph: deps, rdeps, impact, cycles, path |
| `lspq` | Language server queries: definition, refs, hover, rename |
| `webfetch` | Fetch URLs as markdown, web search |
| `memory` | Per-project key-value fact store |
| `sift` | Pipe filter: collapse blanks, repeats, truncate |
| `fossil` | Git history: blame, changes, ownership, co-change, log |
| `extest` | Exercise extensions via JSON-RPC for testing |
| `piglet-cron` | Run scheduled cron tasks (launchd daemon) |

## Install

Requires Go 1.26+.

```bash
git clone https://github.com/dotcommander/piglet-extensions
cd piglet-extensions
just extensions    # build all extensions -> ~/.config/piglet/extensions/
just cli           # build all CLI tools -> ~/go/bin/
```

Build a single target:

```bash
just extensions-safeguard   # one extension
just cli-repomap            # one CLI tool
```

## Development

Each extension follows a standard structure:

```
<name>/
  cmd/
    main.go          # entry point (SDK, JSON-RPC)
    manifest.yaml    # name, version, capabilities
  <name>.go          # core business logic
```

```bash
just build         # build everything
just test          # run all tests
just verify        # build + test
just clean         # remove installed extensions
```

## License

MIT
