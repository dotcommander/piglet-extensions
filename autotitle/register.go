// Package autotitle provides automatic session title generation for piglet.
package autotitle

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	sdk "github.com/dotcommander/piglet/sdk"
)

//go:embed defaults/prompt.md
var defaultPrompt string

const (
	// Version is the autotitle extension version.
	Version       = "0.2.0"
	maxTitleRunes = 50
	chatTimeout   = 10 * time.Second
	chatModel     = "small"
	chatMaxTokens = 30
)

// handlerFired tracks whether the title generation has fired this session.
var handlerFired atomic.Bool

// Register adds autotitle's event handler and status tool to the extension.
func Register(e *sdk.Extension) {
	e.RegisterTool(toolStatus())

	e.RegisterEventHandler(sdk.EventHandlerDef{
		Name:     "autotitle",
		Priority: 100,
		Events:   []string{"EventAgentEnd"},
		Handle: func(_ context.Context, _ string, data json.RawMessage) (result *sdk.Action) {
			if !handlerFired.CompareAndSwap(false, true) {
				return nil
			}
			defer func() {
				if result == nil {
					handlerFired.Store(false)
				}
			}()

			p := xdg.LoadOrCreateExt("autotitle", "prompt.md", strings.TrimSpace(defaultPrompt))
			if p == "" {
				return nil
			}

			var evt struct {
				Messages []json.RawMessage `json:"Messages"`
			}
			if err := json.Unmarshal(data, &evt); err != nil || len(evt.Messages) < 2 {
				return nil
			}

			userText, assistantText := extractFirstExchange(evt.Messages)
			if userText == "" {
				return nil
			}

			content := "User: " + truncateTitle(userText, 200) + "\n\nAssistant: " + truncateTitle(assistantText, 200)

			ctx, cancel := context.WithTimeout(context.Background(), chatTimeout)
			defer cancel()
			resp, err := e.Chat(ctx, sdk.ChatRequest{
				System:    p,
				Messages:  []sdk.ChatMessage{{Role: "user", Content: content}},
				Model:     chatModel,
				MaxTokens: chatMaxTokens,
			})
			if err != nil || resp.Text == "" {
				return nil
			}

			title := strings.TrimSpace(resp.Text)
			title = truncateTitle(title, maxTitleRunes)
			if title != "" {
				return sdk.ActionSetSessionTitle(title)
			}
			return nil
		},
	})
}

func toolStatus() sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "autotitle_status",
		Description: "Show autotitle extension status, config, and prompt",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		PromptHint: "Check autotitle extension status",
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			prompt := xdg.ReadExt("autotitle", "prompt.md")
			fired := handlerFired.Load()

			state := "waiting"
			if fired {
				state = "fired (title generated)"
			}

			var b strings.Builder
			fmt.Fprintf(&b, "autotitle v%s\n", Version)
			fmt.Fprintf(&b, "  Handler: %s\n", state)
			fmt.Fprintf(&b, "  Event:   EventAgentEnd\n")
			fmt.Fprintf(&b, "  Model:   %s\n", chatModel)
			fmt.Fprintf(&b, "  Timeout: %s\n", chatTimeout)
			fmt.Fprintf(&b, "  Max tokens: %d\n", chatMaxTokens)
			fmt.Fprintf(&b, "  Max title:  %d runes\n", maxTitleRunes)
			if prompt != "" {
				fmt.Fprintf(&b, "  Prompt:\n    %s\n", strings.ReplaceAll(prompt, "\n", "\n    "))
			} else {
				fmt.Fprintf(&b, "  Prompt: (not installed)\n")
			}
			return sdk.TextResult(b.String()), nil
		},
	}
}

func extractFirstExchange(messages []json.RawMessage) (userText, assistantText string) {
	for _, raw := range messages {
		var msg struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		if json.Unmarshal(raw, &msg) != nil {
			continue
		}
		if msg.Role == "user" && userText == "" {
			userText = msg.Content
		}
		if msg.Role == "assistant" && assistantText == "" {
			assistantText = msg.Content
		}
		if userText != "" && assistantText != "" {
			break
		}
	}
	return
}

// truncateTitle truncates s to limit runes.
func truncateTitle(s string, limit int) string {
	runes := []rune(s)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return s
}
