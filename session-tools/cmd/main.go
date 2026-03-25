// Session-tools extension. Registers /search, /branch, and /title commands.
package main

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("session-tools", "0.1.0")

	e.RegisterCommand(sdk.CommandDef{
		Name:        "search",
		Description: "Search sessions by title or directory",
		Handler: func(ctx context.Context, args string) error {
			query := strings.TrimSpace(args)
			if query == "" {
				e.ShowMessage("Usage: /search <query>")
				return nil
			}
			sessions, err := e.Sessions(ctx)
			if err != nil {
				e.ShowMessage(err.Error())
				return nil
			}
			if len(sessions) == 0 {
				e.ShowMessage("No sessions found")
				return nil
			}

			q := strings.ToLower(query)
			var b strings.Builder
			count := 0
			for _, s := range sessions {
				if strings.Contains(strings.ToLower(s.Title), q) || strings.Contains(strings.ToLower(s.CWD), q) {
					label := s.Title
					if label == "" && len(s.ID) > 8 {
						label = s.ID[:8]
					}
					fmt.Fprintf(&b, "  %s — %s (%s)\n", label, s.CWD, s.CreatedAt)
					count++
				}
			}

			if count == 0 {
				e.ShowMessage("No sessions matching: " + query)
				return nil
			}
			e.ShowMessage(fmt.Sprintf("Search: %s (%d results)\n\n%s\nUse /session to open a specific session.", query, count, b.String()))
			return nil
		},
	})

	e.RegisterCommand(sdk.CommandDef{
		Name:        "branch",
		Description: "Fork conversation into a new session",
		Handler: func(ctx context.Context, args string) error {
			parentID, count, err := e.ForkSession(ctx)
			if err != nil {
				e.ShowMessage("Branch failed: " + err.Error())
				return nil
			}
			e.ShowMessage(fmt.Sprintf("Branched from %s (%d messages)", parentID, count))
			return nil
		},
	})

	e.RegisterCommand(sdk.CommandDef{
		Name:        "title",
		Description: "Set session title",
		Handler: func(ctx context.Context, args string) error {
			title := strings.TrimSpace(args)
			if title == "" {
				e.ShowMessage("Usage: /title <title>")
				return nil
			}
			if err := e.SetSessionTitle(ctx, title); err != nil {
				e.ShowMessage("Failed to set title: " + err.Error())
				return nil
			}
			e.ShowMessage("Session title: " + title)
			return nil
		},
	})

	e.Run()
}
