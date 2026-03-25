// Usage extension binary. Tracks and displays token usage statistics.
package main

import (
	"context"
	"encoding/json"

	"github.com/dotcommander/piglet-extensions/usage"
	sdk "github.com/dotcommander/piglet/sdk"
)

var stats *usage.SessionStats

func main() {
	e := sdk.New("usage", "0.1.0")
	stats = usage.NewSessionStats()

	e.OnInit(func(x *sdk.Extension) {
		// Register for EventTurnUsage (when host emits it)
		x.RegisterEventHandler(sdk.EventHandlerDef{
			Name:     "usage-tracker",
			Priority: 100,
			Events:   []string{"EventTurnUsage"},
			Handle: func(_ context.Context, _ string, data json.RawMessage) *sdk.Action {
				event, err := usage.ParseEvent(data)
				if err != nil {
					return nil
				}
				stats.Record(event)
				return nil
			},
		})
	})

	// /usage command — show token usage
	e.RegisterCommand(sdk.CommandDef{
		Name:        "usage",
		Description: "Show session token usage statistics",
		Handler: func(_ context.Context, _ string) error {
			e.ShowMessage(stats.FormatSummary())
			return nil
		},
	})

	// memory_set tool for querying stats programmatically
	e.RegisterTool(sdk.ToolDef{
		Name:        "session_stats",
		Description: "Get current session token usage statistics. Returns cumulative totals and prompt breakdown.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
			"required":   []string{},
		},
		PromptHint: "Check token usage for the current session",
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			return sdk.TextResult(stats.FormatSummary()), nil
		},
	})

	e.Run()
}
