package clipboard

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/dotcommander/piglet/sdk"
)

// Version is the clipboard extension version.
const Version = "0.2.0"

// Register adds clipboard's tools and shortcut to the extension.
func Register(e *sdk.Extension) {
	e.RegisterTool(sdk.ToolDef{
		Name:        "clipboard_read",
		Description: "Read text or an image from the system clipboard. Auto-detects content type. Only works on macOS.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"format": map[string]any{
					"type":        "string",
					"description": "Force read mode: 'text' or 'image'. Default: auto-detect.",
				},
			},
		},
		PromptHint: "Read text or images from system clipboard (macOS)",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			format, _ := args["format"].(string)

			if format == "text" {
				text, err := ReadText()
				if err != nil {
					return sdk.ErrorResult("error: " + err.Error()), nil
				}
				return sdk.TextResult(text), nil
			}

			if format == "image" {
				img, err := ReadImage()
				if err != nil {
					return sdk.ErrorResult("error: " + err.Error()), nil
				}
				return &sdk.ToolResult{
					Content: []sdk.ContentBlock{{Type: "image", Data: img.Data, Mime: img.MimeType}},
				}, nil
			}

			// Auto-detect: try image first, fall back to text
			img, imgErr := ReadImage()
			if imgErr == nil {
				return &sdk.ToolResult{
					Content: []sdk.ContentBlock{{Type: "image", Data: img.Data, Mime: img.MimeType}},
				}, nil
			}

			if !strings.Contains(imgErr.Error(), "no image") &&
				!strings.Contains(imgErr.Error(), "clipboard not available") {
				return sdk.ErrorResult("error: " + imgErr.Error()), nil
			}

			text, err := ReadText()
			if err != nil {
				return sdk.ErrorResult("error: " + err.Error()), nil
			}
			if text == "" {
				return sdk.ErrorResult("clipboard is empty"), nil
			}
			return sdk.TextResult(text), nil
		},
	})

	e.RegisterTool(sdk.ToolDef{
		Name:        "clipboard_write",
		Description: "Write text or a base64-encoded image to the system clipboard. Only works on macOS.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"text": map[string]any{
					"type":        "string",
					"description": "Text to copy to clipboard. Use this OR image_data, not both.",
				},
				"image_data": map[string]any{
					"type":        "string",
					"description": "Base64-encoded image data to copy. Use this OR text, not both.",
				},
				"image_mime": map[string]any{
					"type":        "string",
					"description": "MIME type for image data. Required with image_data. 'image/png' or 'image/jpeg'.",
				},
			},
		},
		PromptHint: "Write text or images to system clipboard (macOS)",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			text, hasText := args["text"].(string)
			imageData, hasImage := args["image_data"].(string)

			if hasText && hasImage {
				return sdk.ErrorResult("error: provide either text or image_data, not both"), nil
			}
			if !hasText && !hasImage {
				return sdk.ErrorResult("error: provide either text or image_data"), nil
			}

			if hasText {
				if err := WriteText(text); err != nil {
					return sdk.ErrorResult("error: " + err.Error()), nil
				}
				return sdk.TextResult(fmt.Sprintf("Copied %d characters to clipboard", len(text))), nil
			}

			mime, _ := args["image_mime"].(string)
			if mime == "" {
				mime = "image/png"
			}
			if err := WriteImage(imageData, mime); err != nil {
				return sdk.ErrorResult("error: " + err.Error()), nil
			}
			return sdk.TextResult("Copied image to clipboard"), nil
		},
	})

	e.RegisterShortcut(sdk.ShortcutDef{
		Key:         "ctrl+v",
		Description: "Attach clipboard image",
		Handler: func(_ context.Context) (*sdk.Action, error) {
			img, err := ReadImage()
			if err != nil {
				return sdk.ActionNotify("No image: " + err.Error()), nil
			}
			return sdk.ActionAttachImage(img.Data, img.MimeType), nil
		},
	})
}
