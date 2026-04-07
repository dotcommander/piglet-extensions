package background

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/dotcommander/piglet/sdk"
)

var (
	mu      sync.Mutex
	version string
	lastRun string // last prompt sent to RunBackground
)

// Register adds the background extension's commands and status tool.
func Register(e *sdk.Extension, ver string) {
	mu.Lock()
	version = ver
	mu.Unlock()

	handler := cancelHandler(e)

	e.RegisterCommand(bgCommand(e))
	e.RegisterCommand(sdk.CommandDef{
		Name: "bg-cancel", Description: "Cancel running background task", Handler: handler,
	})
	e.RegisterCommand(sdk.CommandDef{
		Name: "bg-stop", Description: "Cancel running background task (alias for /bg-cancel)", Handler: handler,
	})
	e.RegisterCommand(sdk.CommandDef{
		Name: "cancel", Description: "Cancel running background task (alias for /bg-cancel)", Handler: handler,
	})
	e.RegisterTool(backgroundStatusTool(e))
}

func bgCommand(e *sdk.Extension) sdk.CommandDef {
	return sdk.CommandDef{
		Name:        "bg",
		Description: "Run a read-only background task",
		Handler: func(ctx context.Context, args string) error {
			prompt := strings.TrimSpace(args)
			if prompt == "" {
				e.ShowMessage("Usage: /bg <prompt>\nRuns a read-only background agent (max 5 turns).")
				return nil
			}
			if err := e.RunBackground(ctx, prompt); err != nil {
				e.ShowMessage(cleanError("start background task", err))
				return nil
			}
			mu.Lock()
			lastRun = prompt
			mu.Unlock()
			e.ShowMessage("Background task started: " + prompt)
			return nil
		},
	}
}

func cancelHandler(e *sdk.Extension) func(context.Context, string) error {
	return func(ctx context.Context, args string) error {
		running, err := e.IsBackgroundRunning(ctx)
		if err != nil {
			e.ShowMessage(cleanError("check background status", err))
			return nil
		}
		if !running {
			e.ShowMessage("No background task running")
			return nil
		}
		if err := e.CancelBackground(ctx); err != nil {
			e.ShowMessage(cleanError("cancel background task", err))
			return nil
		}
		e.ShowMessage("Background task cancelled")
		return nil
	}
}

func backgroundStatusTool(e *sdk.Extension) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "background_status",
		Description: "Show background extension status: running state, last task, version",
		Execute: func(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
			running := false
			if r, err := e.IsBackgroundRunning(ctx); err == nil {
				running = r
			}

			mu.Lock()
			ver := version
			last := lastRun
			mu.Unlock()

			state := "idle"
			if running {
				state = "running"
			}

			var b strings.Builder
			fmt.Fprintf(&b, "background %s\n", ver)
			fmt.Fprintf(&b, "  State:    %s\n", state)
			if last != "" {
				fmt.Fprintf(&b, "  Last run: %s\n", last)
			}
			fmt.Fprintf(&b, "  Commands: /bg, /bg-cancel (aliases: /bg-stop, /cancel)")

			return sdk.TextResult(b.String()), nil
		},
	}
}

// cleanError strips host/* RPC method names from error messages.
func cleanError(action string, err error) string {
	s := err.Error()
	for _, prefix := range []string{"host/runBackground: ", "host/isBackgroundRunning: ", "host/cancelBackground: "} {
		if after, ok := strings.CutPrefix(s, prefix); ok {
			s = after
			break
		}
	}
	return fmt.Sprintf("Failed to %s: %s", action, s)
}
