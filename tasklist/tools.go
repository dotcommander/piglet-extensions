package tasklist

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/dotcommander/piglet/sdk"
)

func toolMove(sp **Store) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "tasklist_move",
		Description: "Move a task between groups (todo/backlog) or reparent it under another task.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":            map[string]any{"type": "string", "description": "Task ID (exact or prefix)"},
				"group":         map[string]any{"type": "string", "description": "New group: 'todo' or 'backlog'"},
				"new_parent_id": map[string]any{"type": "string", "description": "New parent task ID (empty string to unparent)"},
			},
			"required": []string{"id"},
		},
		PromptHint: "Move a task between groups or reparent",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			s, t, errRes := requireStoreAndResolve(sp, args)
			if errRes != nil {
				return errRes, nil
			}
			group, _ := args["group"].(string)
			newParentID, _ := args["new_parent_id"].(string)

			updated, err := s.Move(t.ID, group, newParentID)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			parts := []string{fmt.Sprintf("Moved %s", updated.ID)}
			if group != "" {
				parts = append(parts, fmt.Sprintf("to %s", group))
			}
			if newParentID != "" {
				parts = append(parts, fmt.Sprintf("under %s", newParentID))
			}
			return sdk.TextResult(strings.Join(parts, " ")), nil
		},
	}
}

func toolPlan(sp **Store) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "tasklist_plan",
		Description: "Read, replace, or append to a task's plan notes.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":      map[string]any{"type": "string", "description": "Task ID (exact or prefix)"},
				"action":  map[string]any{"type": "string", "description": "Action: 'read' (default), 'replace', or 'append'"},
				"content": map[string]any{"type": "string", "description": "Content for replace/append actions"},
			},
			"required": []string{"id"},
		},
		PromptHint: "Read or update a task's plan notes",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			s, t, errRes := requireStoreAndResolve(sp, args)
			if errRes != nil {
				return errRes, nil
			}
			action, _ := args["action"].(string)
			content, _ := args["content"].(string)

			switch action {
			case "", "read":
				if t.Notes == "" {
					return sdk.TextResult(fmt.Sprintf("No notes for %s", t.ID)), nil
				}
				return sdk.TextResult(fmt.Sprintf("# %s\n\n%s", t.Title, t.Notes)), nil

			case "replace":
				if content == "" {
					return sdk.ErrorResult("content is required for replace"), nil
				}
				updated, err := s.Update(t.ID, "", content)
				if err != nil {
					return sdk.ErrorResult(err.Error()), nil
				}
				return sdk.TextResult(fmt.Sprintf("Replaced notes for %s", updated.ID)), nil

			case "append":
				if content == "" {
					return sdk.ErrorResult("content is required for append"), nil
				}
				updated, err := s.AppendNotes(t.ID, content)
				if err != nil {
					return sdk.ErrorResult(err.Error()), nil
				}
				return sdk.TextResult(fmt.Sprintf("Appended notes for %s", updated.ID)), nil

			default:
				return sdk.ErrorResult(fmt.Sprintf("unknown action %q (use: read, replace, append)", action)), nil
			}
		},
	}
}

func toolLink(sp **Store) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "tasklist_link",
		Description: "Link a task to a Linear ticket, GitHub PR, branch name, or arbitrary URL.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":    map[string]any{"type": "string", "description": "Task ID (exact or prefix)"},
				"field": map[string]any{"type": "string", "description": "Link type: 'linear_ticket', 'github_pr', 'branch', or 'url'"},
				"value": map[string]any{"type": "string", "description": "Link value"},
			},
			"required": []string{"id", "field", "value"},
		},
		PromptHint: "Link a task to a ticket, PR, or branch",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			s, t, errRes := requireStoreAndResolve(sp, args)
			if errRes != nil {
				return errRes, nil
			}
			field, _ := args["field"].(string)
			value, _ := args["value"].(string)

			if field == "" || value == "" {
				return sdk.ErrorResult("field and value are required"), nil
			}

			updated, err := s.Link(t.ID, field, value)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			return sdk.TextResult(fmt.Sprintf("Linked %s %s → %s", field, value, updated.ID)), nil
		},
	}
}

func toolStatus(sp **Store) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "tasklist_status",
		Description: "Show tasklist status: version, store path, and task counts by status and group.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		PromptHint: "Check tasklist status and counts",
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			s, errRes := requireStore(sp)
			if errRes != nil {
				return errRes, nil
			}

			active, done, backlog := s.Stats()
			total := active + done + backlog
			return sdk.TextResult(fmt.Sprintf(
				"tasklist v%s\nStore: %s\nTasks: %d total · %d active · %d backlog · %d done",
				Version, s.Path(), total, active, backlog, done,
			)), nil
		},
	}
}

func toolSearch(sp **Store) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "tasklist_search",
		Description: "Search tasks by text in title or notes (case-insensitive).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query": map[string]any{"type": "string", "description": "Search query"},
			},
			"required": []string{"query"},
		},
		PromptHint: "Search tasks by text",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			s, errRes := requireStore(sp)
			if errRes != nil {
				return errRes, nil
			}
			query, _ := args["query"].(string)
			if query == "" {
				return sdk.ErrorResult("query is required"), nil
			}

			tasks := s.Search(query)
			if len(tasks) == 0 {
				return sdk.TextResult("No matching tasks."), nil
			}

			var b strings.Builder
			for _, t := range tasks {
				icon := "◉"
				if t.Status == StatusDone {
					icon = "✓"
				}
				fmt.Fprintf(&b, "%s %s [%s]\n", icon, t.Title, t.ID)
			}

			return sdk.TextResult(b.String()), nil
		},
	}
}

// formatTaskList produces a compact listing of tasks for tool output.
func formatTaskList(tasks []*Task) string {
	var b strings.Builder
	for _, t := range tasks {
		icon := "◉"
		switch {
		case t.Status == StatusDone:
			icon = "✓"
		case t.Group == GroupBacklog:
			icon = "○"
		}

		indent := ""
		if t.ParentID != "" {
			indent = "  "
		}

		fmt.Fprintf(&b, "%s%s %s [%s]\n", indent, icon, t.Title, t.ID)
		if t.LinearTicket != "" {
			fmt.Fprintf(&b, "%s  ticket: %s\n", indent, t.LinearTicket)
		}
		if t.GitHubPR != "" {
			fmt.Fprintf(&b, "%s  pr: %s\n", indent, t.GitHubPR)
		}
	}
	return b.String()
}

// formatTaskJSON returns pretty-printed JSON for a single task.
func formatTaskJSON(t *Task) string {
	raw, _ := json.MarshalIndent(t, "", "  ")
	return string(raw)
}
