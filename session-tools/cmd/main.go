// Session-tools extension. Registers /search, /branch, /title, /handoff commands,
// session_query tool, and handoff prompt section.
package main

import (
	"context"
	"fmt"
	"strings"

	sessiontools "github.com/dotcommander/piglet-extensions/session-tools"
	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	e := sdk.New("session-tools", "0.2.0")

	var (
		cwd string
		cfg sessiontools.Config
	)

	e.OnInit(func(x *sdk.Extension) {
		cwd = x.CWD()
		cfg = sessiontools.LoadConfig()

		content := sessiontools.LoadPromptContent()
		if content != "" {
			x.RegisterPromptSection(sdk.PromptSectionDef{
				Title:   "Session Handoff",
				Content: content,
				Order:   95,
			})
		}
	})

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

	e.RegisterCommand(sdk.CommandDef{
		Name:        "handoff",
		Description: "Transfer context to a new session with structured summary",
		Handler: func(ctx context.Context, args string) error {
			if !cfg.Enabled {
				e.ShowMessage("Session handoff is disabled in config.")
				return nil
			}
			focus := strings.TrimSpace(args)
			if err := sessiontools.Handoff(ctx, e, cwd, focus); err != nil {
				e.ShowMessage("Handoff failed: " + err.Error())
			}
			return nil
		},
	})

	e.RegisterTool(sdk.ToolDef{
		Name:        "session_query",
		Description: "Search a session's JSONL file for content matching a keyword query. Use to recover specific details from a parent session after a handoff.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"session_path": map[string]any{
					"type":        "string",
					"description": "Path to the session JSONL file (from SessionInfo.Path)",
				},
				"query": map[string]any{
					"type":        "string",
					"description": "Keyword or phrase to search for in session content",
				},
			},
			"required": []any{"session_path", "query"},
		},
		PromptHint: "Search a session file for specific content by keyword",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			path, _ := args["session_path"].(string)
			query, _ := args["query"].(string)

			if path == "" || query == "" {
				return sdk.ErrorResult("session_path and query are required"), nil
			}

			result, err := sessiontools.QuerySession(path, query, cfg.MaxQuerySize)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			return sdk.TextResult(result), nil
		},
	})

	e.Run()
}
