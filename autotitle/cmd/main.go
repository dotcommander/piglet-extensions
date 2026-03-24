// Autotitle extension binary. Generates session titles after first exchange.
// Communicates with piglet host via JSON-RPC over stdin/stdout.
// Uses the SDK host Chat method to generate titles without a direct provider dependency.
package main

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"

	sdk "github.com/dotcommander/piglet/sdk"
)

const maxTitleRunes = 50

func main() {
	e := sdk.New("autotitle", "0.1.0")

	var fired atomic.Bool
	var prompt atomic.Value // lazy-loaded from config on first event

	e.RegisterEventHandler(sdk.EventHandlerDef{
		Name:     "autotitle",
		Priority: 100,
		Events:   []string{"EventAgentEnd"},
		Handle: func(_ context.Context, _ string, data json.RawMessage) *sdk.Action {
			if !fired.CompareAndSwap(false, true) {
				return nil
			}

			// Lazy-load prompt on first fire (cannot call host during OnInit — deadlock)
			p, _ := prompt.Load().(string)
			if p == "" {
				loaded, err := e.ConfigReadExtension(context.Background(), "autotitle")
				if err != nil || loaded == "" {
					return nil
				}
				prompt.Store(loaded)
				p = loaded
			}

			var evt struct {
				Messages []json.RawMessage `json:"Messages"`
			}
			if err := json.Unmarshal(data, &evt); err != nil || len(evt.Messages) < 2 {
				fired.Store(false)
				return nil
			}

			userText, assistantText := extractFirstExchange(evt.Messages)
			if userText == "" {
				fired.Store(false)
				return nil
			}

			content := "User: " + truncate(userText, 200) + "\n\nAssistant: " + truncate(assistantText, 200)

			resp, err := e.Chat(context.Background(), sdk.ChatRequest{
				System:    p,
				Messages:  []sdk.ChatMessage{{Role: "user", Content: content}},
				Model:     "small",
				MaxTokens: 30,
			})
			if err != nil || resp.Text == "" {
				fired.Store(false)
				return nil
			}

			title := strings.TrimSpace(resp.Text)
			title = truncate(title, maxTitleRunes)
			if title != "" {
				return sdk.ActionSetSessionTitle(title)
			}
			return nil
		},
	})

	e.Run()
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

func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return s
}
