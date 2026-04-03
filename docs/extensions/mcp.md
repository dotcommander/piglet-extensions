# MCP

Connect to Model Context Protocol servers and expose their tools to piglet.

## Quick Start

Add a server to `~/.config/piglet/extensions/mcp/mcp.yaml`:

```yaml
servers:
  filesystem:
    command: npx
    args:
      - "-y"
      - "@modelcontextprotocol/server-filesystem"
      - "/Users/me/projects"
```

Restart piglet. The MCP server's tools appear automatically and the model can invoke them like any built-in tool.

## What It Does

MCP connects to one or more MCP servers (stdio subprocess or HTTP/SSE endpoint) at startup, discovers their tools, and registers each tool as a piglet tool named `mcp__<server>__<tool>`. If a server provides instructions in its initialize response, those instructions are injected as a prompt section. Run `/mcp` to check connection status.

## Capabilities

| Capability | Name | Description |
|-----------|------|-------------|
| Tools | `mcp__<server>__<tool>` | One tool per discovered MCP tool |
| Command | `mcp` | Show MCP server connection status |
| Prompt | `MCP Server Instructions` | Server instructions injected at order 80 |

The exact tool names and count depend on your configured servers.

## Configuration

Config file: `~/.config/piglet/extensions/mcp/mcp.yaml`

Created on first use with commented examples:

```yaml
servers:
  # Stdio server (subprocess):
  filesystem:
    type: stdio             # default; can be omitted
    command: npx
    args:
      - "-y"
      - "@modelcontextprotocol/server-filesystem"
      - "/path/to/allowed/dir"
    env:
      NODE_ENV: production

  # HTTP/SSE server:
  remote-api:
    type: http
    url: "https://mcp.example.com/sse"
    headers:
      Authorization: "Bearer ${MCP_API_KEY}"

servers: {}   # default — no servers configured
```

### Server config fields

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | `"stdio"` (default) or `"http"` |
| `command` | string | stdio: executable to launch |
| `args` | list | stdio: command arguments |
| `env` | map | stdio: environment variables appended to the current environment |
| `url` | string | http: SSE endpoint URL |
| `headers` | map | http: custom request headers |

### Environment variable expansion

Use `${VAR}` syntax in `env` values and `headers` values to reference OS environment variables:

```yaml
env:
  API_KEY: "${MY_SECRET_KEY}"
headers:
  Authorization: "Bearer ${MCP_TOKEN}"
```

## Commands Reference

### `/mcp`

```
/mcp
```

No arguments. Shows connection status for all configured servers.

**Connected:**

```
MCP Servers:

  filesystem: connected (12 tools)
  remote-api: connected (4 tools)
```

**With errors:**

```
MCP Servers:

  filesystem: connected (12 tools)
  remote-api: error — connect tcp: connection refused
```

**No servers configured:**

```
No MCP servers configured. Add servers to
/Users/gary/.config/piglet/extensions/mcp/mcp.yaml
```

## Tools Reference

Tools are discovered at startup and named `mcp__<server-name>__<tool-name>`. For example, if the `filesystem` server exposes a `read_file` tool, it appears as:

| Tool | Description |
|------|-------------|
| `mcp__filesystem__read_file` | [MCP:filesystem] Read a file from the filesystem |
| `mcp__filesystem__list_directory` | [MCP:filesystem] List directory contents |

The model sees the full description prefixed with `[MCP:<server>]` to make the source clear.

## How It Works (Developer Notes)

**SDK hooks used:** `e.OnInit`, `e.RegisterTool`, `e.RegisterCommand`, `e.RegisterPromptSection`.

**Init sequence:** All server connections and tool registration happen inside `e.OnInit`. The host calls OnInit after sending the CWD; this ensures the RPC pipe is ready before tool registrations are sent.

**Parallel startup:** `Manager.StartAll` connects to all servers concurrently using goroutines. Partial failures are collected but do not prevent successful servers from registering tools.

**MCP protocol version:** `2025-03-26` (sent in the `Initialize` request).

**Tool naming:** `mcp__<server>__<tool>` — double underscore separators allow the model to identify the source server from the tool name.

**Connection timeout:** 30 seconds per server (`connectTimeout`).

**Transport implementations:**
- `stdio` → `mcpclient.NewStdioMCPClient` from `github.com/mark3labs/mcp-go`
- `http` → `mcpclient.NewSSEMCPClient` from `github.com/mark3labs/mcp-go`

**Text extraction:** `CallAndExtract` calls the MCP tool and concatenates all `TextContent` blocks from the result. Non-text content (images, etc.) is currently ignored. Tool errors are returned as `sdk.ErrorResult`.

**Prompt section order:** 80 (appears after skills at 25, memory at 50, but before RTK at 90).

## Related Extensions

- [provider](provider.md) — streaming LLM providers
- [extensions-list](extensions-list.md) — verify MCP tools are registered
