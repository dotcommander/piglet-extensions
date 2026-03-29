// Loop extension binary. Runs a prompt or slash command on a recurring interval.
// Communicates with the piglet host via JSON-RPC over stdin/stdout.
package main

import (
	_ "embed"

	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	"github.com/dotcommander/piglet-extensions/loop"
	sdk "github.com/dotcommander/piglet/sdk"
)

//go:embed defaults/prompt.md
var defaultPrompt string

var scheduler = &loop.Scheduler{}

func main() {
	e := sdk.New("loop", "0.1.0")

	e.RegisterPromptSection(sdk.PromptSectionDef{
		Title:   "Loop Scheduling",
		Content: xdg.LoadOrCreateFile("loop-prompt.md", strings.TrimSpace(defaultPrompt)),
		Order:   86,
	})

	e.RegisterCommand(sdk.CommandDef{
		Name:        "loop",
		Description: "Start a recurring loop: /loop <interval> <prompt or /command>",
		Handler:     makeLoopHandler(e),
	})

	e.RegisterCommand(sdk.CommandDef{
		Name:        "loop-stop",
		Description: "Stop the active loop",
		Handler:     makeLoopStopHandler(e),
	})

	e.RegisterCommand(sdk.CommandDef{
		Name:        "loop-status",
		Description: "Show current loop state",
		Handler:     makeLoopStatusHandler(e),
	})

	e.Run()
}

func makeLoopHandler(e *sdk.Extension) func(context.Context, string) error {
	return func(_ context.Context, args string) error {
		args = strings.TrimSpace(args)
		if args == "" {
			e.ShowMessage("Usage: /loop <interval> <prompt or /command>\nExample: /loop 5m check build status\nMinimum interval: 30s")
			return nil
		}

		parts := strings.SplitN(args, " ", 2)
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			e.ShowMessage("Usage: /loop <interval> <prompt or /command>\nExample: /loop 5m check build status")
			return nil
		}

		intervalStr := parts[0]
		prompt := strings.TrimSpace(parts[1])

		interval, err := time.ParseDuration(intervalStr)
		if err != nil {
			e.ShowMessage(fmt.Sprintf("Invalid interval %q: %v\nExamples: 30s, 5m, 1h", intervalStr, err))
			return nil
		}

		if err := scheduler.Start(interval, prompt, func(iteration int, p string) {
			display := truncate(p, 50)
			e.Notify(fmt.Sprintf("Loop iteration #%d: %s", iteration, display))
			e.SendMessage(p)
		}); err != nil {
			e.ShowMessage(fmt.Sprintf("Cannot start loop: %v", err))
			return nil
		}

		e.ShowMessage(fmt.Sprintf("Loop started: every %s — %s", interval, truncate(prompt, 60)))
		return nil
	}
}

func makeLoopStopHandler(e *sdk.Extension) func(context.Context, string) error {
	return func(_ context.Context, _ string) error {
		if scheduler.Stop() {
			e.ShowMessage("Loop stopped.")
		} else {
			e.ShowMessage("No active loop.")
		}
		return nil
	}
}

func makeLoopStatusHandler(e *sdk.Extension) func(context.Context, string) error {
	return func(_ context.Context, _ string) error {
		running, interval, prompt, iterations := scheduler.Status()
		if !running {
			e.ShowMessage("No active loop.")
			return nil
		}
		e.ShowMessage(fmt.Sprintf("Loop active: every %s, iteration #%d, prompt: %s",
			interval, iterations, truncate(prompt, 80)))
		return nil
	}
}

func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) > limit {
		return string(runes[:limit]) + "..."
	}
	return s
}
