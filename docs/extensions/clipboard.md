# Clipboard

Copy images from the macOS system clipboard into conversations.

## Quick Start

Press `Ctrl+V` in the piglet input to attach a screenshot or image from your clipboard, or ask the model to read it:

```
Read the image in my clipboard
```

The model invokes `clipboard_read` automatically and receives the image as a base64-encoded content block.

## What It Does

Clipboard reads PNG or JPEG images from the macOS system clipboard using AppleScript and returns them as base64-encoded image data. It registers both a tool (for model-initiated reads) and a keyboard shortcut (for user-initiated attachment). Only macOS is supported — the extension uses `osascript` to inspect and retrieve clipboard content.

## Capabilities

| Capability | Name | Description |
|-----------|------|-------------|
| Tool | `clipboard_read` | Read a PNG or JPEG image from the system clipboard |
| Shortcut | `Ctrl+V` | Attach clipboard image directly to the input |

## Configuration

Clipboard has no configuration file. Behavior is fixed.

> **macOS only.** The extension depends on `osascript` and AppleScript clipboard access. It will not work on Linux or Windows.

## Tools Reference

### `clipboard_read`

Read an image from the macOS system clipboard.

**Parameters:** None.

**Returns:** An image content block with the base64-encoded image data and MIME type (`image/png` or `image/jpeg`).

**Returns an error string if:**
- No image is present in the clipboard
- The clipboard contains content that is not PNG or JPEG
- `osascript` is unavailable

**Example usage in a message:**

```
Take a screenshot of the error dialog and press Ctrl+V to attach it,
then ask: What is this error telling me?
```

## Commands Reference

No commands registered. Use the `Ctrl+V` shortcut or the `clipboard_read` tool.

## How It Works (Developer Notes)

**SDK hooks used:** `e.RegisterTool`, `e.RegisterShortcut`.

**Clipboard detection:** The extension first runs `osascript -e 'the clipboard info'` to inspect clipboard content. If the output contains `PNGf` the MIME type is `image/png`; if it contains `JPEG` the MIME type is `image/jpeg`. Any other content returns an error.

**Data retrieval:** A second AppleScript call reads the raw clipboard bytes as the detected class (`«class PNGf»` or `«class JPEG»`) by writing to `/dev/stdout`. The raw bytes are then base64-encoded with `encoding/base64`.

**Shortcut vs. tool:** The shortcut (`Ctrl+V`) returns `sdk.ActionAttachImage` which attaches the image to the user's pending message. The tool returns a `sdk.ToolResult` with an image content block for the model to process. Both call the same `ReadImage()` function.

**No init required:** Clipboard has no `OnInit` hook — it doesn't depend on CWD or config, so all registration happens at startup.

## Related Extensions

- [session-tools](session-tools.md) — session management with context handoff
