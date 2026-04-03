# Inbox

Deliver messages from external processes into the active piglet agent session.

## Quick Start

```sh
# Drop a JSON envelope into the inbox directory for the running piglet process
PID=$(pgrep -f "piglet")
echo '{
  "version": 1,
  "id": "alert-001",
  "text": "Production error spike detected: 142 errors in the last minute",
  "mode": "interrupt",
  "source": "monitoring"
}' > ~/.config/piglet/inbox/$PID/alert-001.json
```

The running piglet agent receives the message and notifies you within 750ms.

## What It Does

Inbox watches a per-process directory under `~/.config/piglet/inbox/<pid>/` for incoming JSON envelopes. When an envelope arrives, the scanner validates it, checks for duplicates, and delivers it to the agent via `SendMessage` (queue mode) or `Steer` (interrupt mode). Each envelope is acknowledged and removed after delivery. A heartbeat file is written every 2 seconds so external tools can discover running sessions. The extension starts automatically on `EventAgentStart`.

## Capabilities

| Capability | Registered name | Notes |
|---|---|---|
| Tool | `inbox_status` | Check inbox health and delivery statistics |
| Event handler | `inbox.lifecycle` | Starts the scanner on `EventAgentStart` (priority 500) |

## Configuration

Inbox has no user-facing config file. The inbox directory is `~/.config/piglet/inbox/`.

| Directory / File | Purpose |
|---|---|
| `~/.config/piglet/inbox/<pid>/` | Incoming envelopes for the process with this PID |
| `~/.config/piglet/inbox/<pid>/acks/` | Acknowledgement files (pruned after 1 hour) |
| `~/.config/piglet/inbox/registry/<pid>.json` | Heartbeat file — updated every 2 seconds |

## Envelope Format

Drop a JSON file with a `.json` extension into `~/.config/piglet/inbox/<pid>/`.

```json
{
  "version": 1,
  "id": "unique-message-id",
  "text": "The message text to deliver to the agent",
  "mode": "queue",
  "source": "my-script",
  "created": "2026-04-03T12:00:00Z",
  "ttl": 300
}
```

| Field | Type | Required | Description |
|---|---|---|---|
| `version` | integer | yes | Must be `1` |
| `id` | string | yes | Unique message ID (used for dedup and ack) |
| `text` | string | yes | Message content (max 8000 runes) |
| `mode` | string | no | `queue` (default) or `interrupt` |
| `source` | string | no | Sender identifier shown in notification |
| `created` | string | no | RFC3339 timestamp for TTL calculation |
| `ttl` | integer | no | Seconds before the message expires |

**`queue` mode**: Delivers via `SendMessage` — injects as a user turn in the conversation.

**`interrupt` mode**: Delivers via `Steer` — interrupts the current agent turn if supported by the host. Falls back to `SendMessage` if `Steer` is not yet implemented.

### Validation rules

| Condition | Result |
|---|---|
| `version != 1` | Rejected: `unsupported_version` |
| `id` empty | Rejected: `missing_id` |
| `text` empty | Rejected: `missing_text` |
| `text` > 8000 runes | Rejected: `text_too_long` |
| `mode` not `queue` or `interrupt` | Rejected: `invalid_mode` |
| File size > 32KB | Rejected: `file_too_large` |
| Expired (TTL elapsed) | Rejected: `expired` |
| Duplicate `id` | Acknowledged as `duplicate`, removed |

## Acknowledgements

After processing each envelope, the scanner writes an ack file to `~/.config/piglet/inbox/<pid>/acks/<id>.json`:

```json
{
  "id": "unique-message-id",
  "status": "delivered",
  "ts": "2026-04-03T12:00:00Z"
}
```

Possible `status` values: `delivered`, `duplicate`, `failed`.

Ack files are pruned after 1 hour.

## Tools Reference

### `inbox_status`

Returns delivery statistics for the current session.

```
inbox_status({})
```

```
Inbox Status
  Uptime:     1h23m45s
  Delivered:  12
  Failed:     1
  Duplicates: 0
  Expired:    2
```

## Discovering Running Sessions

Read `~/.config/piglet/inbox/registry/<pid>.json` to find active sessions. Each file contains:

```json
{
  "pid": 12345,
  "cwd": "/Users/gary/my-project",
  "started": "2026-04-03T10:00:00Z",
  "heartbeat": "2026-04-03T12:34:56Z"
}
```

A session is active if its heartbeat was updated within the last few seconds. Sessions that exit cleanly remove their registry file. Stale registry files (from crashed processes) can be identified by an old heartbeat timestamp.

## How It Works (Developer Notes)

**No `OnInit`**: The extension starts on `EventAgentStart` (not `OnInit`) because it does not need CWD — the CWD is available from `e.CWD()` at event time. An `atomic.Bool` flag guards against re-entrancy so the scanner starts only once.

**Scanner goroutines**: `Scanner.Start` launches two goroutines: `scanLoop` (polls every 750ms) and `heartbeatLoop` (writes registry every 2s). Both respect the context passed to `Start`. `Stop` cancels the context and calls `s.wg.Wait()` before returning.

**File ordering**: Within each scan pass, files are sorted by modification time (oldest first) so messages are delivered in the order they arrived.

**Symlink guard**: `processFile` calls `os.Lstat` and rejects symlinks to prevent symlink attacks.

**Dedup strategy**: The scanner maintains an in-memory `seen` map of delivered IDs (capped at 1000 entries — cleared entirely when full). Before checking the map, it also checks for an ack file on disk. This means dedup survives a brief scanner restart as long as ack files exist.

**`delivererShim`**: The `Deliverer` interface (`SendMessage`, `Steer`, `Notify`) is satisfied by `delivererShim`, which wraps `*sdk.Extension`. Because `Steer` is not yet in the SDK, it falls back to `SendMessage`. This shim allows the `Scanner` type to be tested independently of the SDK.

## Related Extensions

- [background](background.md) — One-shot background task initiated from within the session
- [cron](cron.md) — Schedule recurring tasks; use cron's webhook action to trigger inbox delivery from external systems
