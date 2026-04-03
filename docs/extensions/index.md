# Extensions

Piglet extensions are standalone binaries that augment your LLM code assistant with tools, commands, interceptors, and system prompt sections. They communicate with the piglet host via JSON-RPC over file descriptors (FD 3/4, falling back to stdin/stdout).

## Quick Start

```bash
# Install all extensions
make extensions

# Install a single extension
make extensions-memory

# Extensions install to ~/.config/piglet/extensions/
```

## How Extensions Work

Each extension is a Go binary built with the [piglet SDK](https://github.com/dotcommander/piglet/sdk). At startup, the host launches each extension, sends an `init` message with the current working directory, and the extension registers its capabilities.

```go
func main() {
    e := sdk.New("my-extension", "0.1.0")
    myext.Register(e)
    e.Run()
}
```

Extensions register one or more capabilities:

| Capability | What it provides | Example |
|------------|-----------------|---------|
| `tools` | LLM-callable functions | `memory_set`, `memory_get` |
| `commands` | Slash commands (`/command`) | `/memory`, `/export` |
| `interceptors` | Before/after hooks on tool calls | safeguard blocks dangerous commands |
| `promptSections` | Sections injected into the system prompt | gitcontext adds branch info |
| `eventHandlers` | Lifecycle event observers | autotitle generates titles |
| `shortcuts` | Keyboard shortcuts | clipboard copies results |
| `messageHooks` | Pre/post message processing | skill loads domain knowledge |
| `provider` | LLM provider backends | provider adds streaming backends |
| `compactor` | Context window compaction | memory compacts with fact extraction |

## Extension Catalog

### Security & Filtering

| Extension | Description | Capabilities |
|-----------|-------------|--------------|
| [safeguard](safeguard.md) | Block dangerous commands and file operations | interceptors |
| [rtk](rtk.md) | Token-optimized CLI proxy (60-90% savings) | interceptors, prompt |
| [sift](sift.md) | Compress large tool output | interceptors, prompt |

### Context & Memory

| Extension | Description | Capabilities |
|-----------|-------------|--------------|
| [memory](memory.md) | Per-project key-value fact store | tools, commands, prompt, compactor |
| [gitcontext](gitcontext.md) | Git branch and status in system prompt | prompt |
| [behavior](behavior.md) | Load behavioral guidelines into prompt | prompt |
| [session-tools](session-tools.md) | Session management, handoff, and fork | commands, tools, prompt |

### Task Orchestration

| Extension | Description | Capabilities |
|-----------|-------------|--------------|
| [plan](plan.md) | Structured task tracking with checkpoints | tools, commands, prompt |
| [coordinator](coordinator.md) | Decompose tasks into parallel sub-tasks | tools |
| [subagent](subagent.md) | Delegate to independent sub-agents | tools |
| [loop](loop.md) | Recurring task execution | commands, prompt |
| [background](background.md) | Read-only background tasks | commands |
| [cron](cron.md) | Schedule recurring tasks | tools, commands, events |
| [inbox](inbox.md) | Pending notifications and task items | tools, events |

### Development Tools

| Extension | Description | Capabilities |
|-----------|-------------|--------------|
| [lsp](lsp.md) | Language server queries (definition, refs, hover) | tools, prompt |
| [repomap](repomap.md) | Token-budgeted repo structure maps | tools, prompt, events |
| [pipeline](pipeline.md) | Multi-step YAML workflows | tools, commands, prompt |
| [bulk](bulk.md) | Parallel shell commands across items | tools |
| [webfetch](webfetch.md) | Fetch URLs as markdown, web search | tools, prompt |
| [scaffold](scaffold.md) | Project scaffolds and boilerplate | commands |

### Knowledge & Routing

| Extension | Description | Capabilities |
|-----------|-------------|--------------|
| [skill](skill.md) | Domain knowledge and methodology files | tools, commands, prompt, messageHooks |
| [prompts](prompts.md) | Prompt template management | commands |
| [suggest](suggest.md) | Proactive context-based suggestions | events |
| [route](route.md) | Message routing and dispatch | tools, commands, messageHooks |

### Content & Export

| Extension | Description | Capabilities |
|-----------|-------------|--------------|
| [changelog](changelog.md) | Generate changelogs from git history | commands |
| [export](export.md) | Export conversations as markdown/JSON | commands |
| [clipboard](clipboard.md) | Copy/paste with system clipboard | tools, shortcuts |
| [autotitle](autotitle.md) | Auto-generate conversation titles | events |
| [undo](undo.md) | Restore files from edit snapshots | commands |

### System & Administration

| Extension | Description | Capabilities |
|-----------|-------------|--------------|
| [admin](admin.md) | Configuration viewer and model catalog | commands |
| [usage](usage.md) | Session token usage statistics | commands, tools, events |
| [cache](cache.md) | File-backed TTL cache library | (shared library) |
| [extensions-list](extensions-list.md) | List installed extensions | commands |
| [modelsdev](modelsdev.md) | Model catalog generation | events |
| [provider](provider.md) | Streaming LLM provider backends | provider |
| [mcp](mcp.md) | Model Context Protocol integration | tools, commands, prompt |

## Configuration

Extensions read configuration from `~/.config/piglet/extensions/<name>/`. Default config files are generated on first run if missing. Configuration is always in YAML.

## For Extension Developers

See the [piglet SDK documentation](https://github.com/dotcommander/piglet/tree/main/sdk) for the full API. Key patterns:

- **OnInit for CWD-dependent state**: Use `ext.OnInit(func(e *sdk.Extension) { ... })` to initialize after the host sends the working directory.
- **Host RPC methods**: Call `e.ConfigGet()`, `e.Chat()`, `e.RunAgent()` for host-managed operations.
- **Atomic file writes**: Use `xdg.WriteFileAtomic` for safe config/data persistence.
- **Prompt ordering**: Lower `Order` value = earlier in system prompt (skill=25, memory=50, rtk=90).
- **Interceptor priority**: Higher value = runs first (safeguard=2000, rtk=100).
