package main

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/dotcommander/piglet-extensions/suggest"
	sdk "github.com/dotcommander/piglet/sdk"
)

var (
	suggester *suggest.Suggester
	cwd       atomic.Value
)

func main() {
	e := sdk.New("suggest", "0.1.0")

	e.OnInit(func(ext *sdk.Extension) {
		// Store CWD for git operations
		cwd.Store(ext.CWD())

		// Load config and prompt
		cfg := suggest.LoadConfig()
		prompt := suggest.LoadPrompt(ext)

		// Create suggester
		suggester = suggest.NewSuggester(cfg, prompt, ext)
	})

	// EventTurnEnd handler - generate suggestions after each turn
	e.RegisterEventHandler(sdk.EventHandlerDef{
		Name:     "suggest-turn-end",
		Priority: 200,
		Events:   []string{"EventTurnEnd"},
		Handle: func(ctx context.Context, _ string, data json.RawMessage) *sdk.Action {
			if suggester == nil {
				return nil
			}

			// Check cooldown
			if !suggester.ShouldSuggest() {
				return nil
			}

			// Parse turn data
			var turn suggest.TurnData
			if err := json.Unmarshal(data, &turn); err != nil {
				return nil
			}

			// Skip if no tool results (nothing actionable happened)
			if len(turn.ToolResults) == 0 {
				return nil
			}

			// Gather project context
			workingDir, _ := cwd.Load().(string)
			projCtx := suggest.GatherContext(workingDir, turn)

			// Create context with timeout
			suggestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			// Generate suggestion
			suggestion, err := suggester.Generate(suggestCtx, turn, projCtx)
			if err != nil || suggestion == "" {
				return nil
			}

			// Reset cooldown after successful suggestion
			suggester.ResetCooldown()

			// Show suggestion to user
			return sdk.ActionShowMessage(suggestion)
		},
	})

	e.Run()
}
