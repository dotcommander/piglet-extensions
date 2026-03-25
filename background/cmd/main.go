// Background extension. Registers /bg and /bg-cancel commands.
package main

import (
	"context"
	"strings"

	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("background", "0.1.0")

	e.RegisterCommand(sdk.CommandDef{
		Name:        "bg",
		Description: "Run a read-only background task",
		Handler: func(ctx context.Context, args string) error {
			prompt := strings.TrimSpace(args)
			if prompt == "" {
				e.ShowMessage("Usage: /bg <prompt>\nRuns a read-only background agent (max 5 turns).")
				return nil
			}
			if err := e.RunBackground(ctx, prompt); err != nil {
				e.ShowMessage("Background task failed: " + err.Error())
				return nil
			}
			e.ShowMessage("Background task started: " + prompt)
			return nil
		},
	})

	e.RegisterCommand(sdk.CommandDef{
		Name:        "bg-cancel",
		Description: "Cancel running background task",
		Handler: func(ctx context.Context, args string) error {
			running, err := e.IsBackgroundRunning(ctx)
			if err != nil {
				e.ShowMessage("Failed to check background status: " + err.Error())
				return nil
			}
			if !running {
				e.ShowMessage("No background task running")
				return nil
			}
			if err := e.CancelBackground(ctx); err != nil {
				e.ShowMessage("Failed to cancel background task: " + err.Error())
				return nil
			}
			e.ShowMessage("Background task cancelled")
			return nil
		},
	})

	e.Run()
}
