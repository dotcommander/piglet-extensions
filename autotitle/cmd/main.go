package main

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	sdk "github.com/dotcommander/piglet/sdk"
)

const maxTitleRunes = 50
const chatTimeout = 10 * time.Second
const chatModel = "small"

func main() {
	e := sdk.New("autotitle", "0.1.0")

	var fired atomic.Bool
	var prompt atomic.Value // lazy-loaded from config on first event

	e.RegisterEventHandler(sdk.EventHandlerDef{
		Name:     "autotitle",
		Priority: 100,
		Events:   []string{"EventAgentEnd"},
		Handle: func(_ context.Context, _ string, data json.RawMessage) (result *sdk.Action) {
			if !fired.CompareAndSwap(false, true) {
				return nil
			}
			defer func() {
				if result == nil {
					fired.Store(false)
				}
			}()

			// Lazy-load prompt on first fire (cannot call host during OnInit — deadlock)
			p, _ := prompt.Load().(string)
			if p == "" {
				loaded, _ := e.ConfigReadExtension(context.Background(), "autotitle")
				if loaded == "" {
					loaded = ensureDefaultConfig()
				}
				if loaded == "" {
					return nil
				}
				prompt.Store(loaded)
				p = loaded
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

			content := "User: " + truncate(userText, 200) + "\n\nAssistant: " + truncate(assistantText, 200)

			ctx, cancel := context.WithTimeout(context.Background(), chatTimeout)
			defer cancel()
			resp, err := e.Chat(ctx, sdk.ChatRequest{
				System:    p,
				Messages:  []sdk.ChatMessage{{Role: "user", Content: content}},
				Model:     chatModel,
				MaxTokens: 30,
			})
			if err != nil || resp.Text == "" {
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

func ensureDefaultConfig() string {
	return strings.TrimSpace(xdg.LoadOrCreateFile("autotitle.md", "You generate concise session titles. Given a user-assistant exchange, output a 2-5 word title. No quotes, no punctuation, just the title."))
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
