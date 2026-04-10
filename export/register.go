package export

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	"github.com/dotcommander/piglet/sdk"
)

const Version = "0.1.0"

// Register registers the export extension's commands.
func Register(e *sdk.Extension) {
	e.RegisterCommand(sdk.CommandDef{
		Name:        "export",
		Description: "Export conversation",
		Handler: func(ctx context.Context, args string) error {
			raw, err := e.ConversationMessages(ctx)
			if err != nil {
				e.ShowMessage("Export failed: " + err.Error())
				return nil
			}

			var msgs []json.RawMessage
			if err := json.Unmarshal(raw, &msgs); err != nil || len(msgs) == 0 {
				e.ShowMessage("No messages to export")
				return nil
			}

			path := fmt.Sprintf("piglet-export-%s.md", time.Now().Format("20060102-150405"))
			if err := exportMarkdown(msgs, path); err != nil {
				e.ShowMessage("Export failed: " + err.Error())
				return nil
			}
			e.ShowMessage("Exported to " + path)
			return nil
		},
	})
}

type wireMessage struct {
	Role     string            `json:"role"`
	Content  json.RawMessage   `json:"content"`
	ToolName string            `json:"toolName,omitempty"`
	Blocks   []json.RawMessage `json:"blocks,omitempty"`
}

type contentBlock struct {
	Type     string `json:"type"`
	Text     string `json:"text"`
	Thinking string `json:"thinking"`
}

func exportMarkdown(msgs []json.RawMessage, path string) error {
	var b strings.Builder
	b.WriteString("# Piglet Conversation\n\n")

	for _, raw := range msgs {
		var msg wireMessage
		if json.Unmarshal(raw, &msg) != nil {
			continue
		}

		switch msg.Role {
		case "user":
			renderUser(&b, msg.Content)
		case "assistant":
			renderAssistant(&b, msg.Content)
		case "tool_result":
			renderToolResult(&b, msg.ToolName, msg.Content)
		}
	}

	return xdg.WriteFileAtomic(path, []byte(b.String()))
}

func renderUser(b *strings.Builder, content json.RawMessage) {
	b.WriteString("## User\n\n")
	var text string
	if json.Unmarshal(content, &text) == nil {
		b.WriteString(text)
	}
	b.WriteString("\n\n")
}

func renderAssistant(b *strings.Builder, content json.RawMessage) {
	b.WriteString("## Assistant\n\n")
	var blocks []json.RawMessage
	if json.Unmarshal(content, &blocks) != nil {
		return
	}
	for _, block := range blocks {
		var cb contentBlock
		if json.Unmarshal(block, &cb) != nil {
			continue
		}
		switch cb.Type {
		case "text":
			b.WriteString(cb.Text)
		case "thinking":
			b.WriteString("<details><summary>Thinking</summary>\n\n")
			b.WriteString(cb.Thinking)
			b.WriteString("\n</details>")
		}
	}
	b.WriteString("\n\n")
}

func renderToolResult(b *strings.Builder, toolName string, content json.RawMessage) {
	fmt.Fprintf(b, "### Tool: %s\n\n", toolName)
	var blocks []json.RawMessage
	if json.Unmarshal(content, &blocks) != nil {
		return
	}
	for _, block := range blocks {
		var cb contentBlock
		if json.Unmarshal(block, &cb) == nil && cb.Type == "text" {
			b.WriteString(cb.Text)
		}
	}
	b.WriteString("\n\n")
}
