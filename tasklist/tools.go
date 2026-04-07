package tasklist

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/dotcommander/piglet/sdk"
)

func toolAdd(sp **Store) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "tasklist_add",
		Description: "Create a new task with a title. Optionally set group (todo/backlog) and parent for subtasks.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title":     map[string]any{"type": "string", "description": "Task title"},
				"group":     map[string]any{"type": "string", "description": "Group: 'todo' (default) or 'backlog'"},
				"parent_id": map[string]any{"type": "string", "description": "Parent task ID to create a subtask"},
			},
			"required": []string{"title"},
		},
		PromptHint: "Add a task to the project task list",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			s := *sp
			if s == nil {
				return sdk.ErrorResult("tasklist: not initialized"), nil
			}
			title, _ := args["title"].(string)
			group, _ := args["group"].(string)
			parentID, _ := args["parent_id"].(string)

			if title == "" {
				return sdk.ErrorResult("title is required"), nil
			}

			t, err := s.Add(title, group, parentID)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			prefix := ""
			if parentID != "" {
				prefix = fmt.Sprintf(" (subtask of %s)", parentID)
			}
			return sdk.TextResult(fmt.Sprintf("Created task %s: %s%s", t.ID, t.Title, prefix)), nil
		},
	}
}

func toolList(sp **Store) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "tasklist_list",
		Description: "List tasks with optional filters for status, group, and parent.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status":    map[string]any{"type": "string", "description": "Filter by status: 'active' or 'done'"},
				"group":     map[string]any{"type": "string", "description": "Filter by group: 'todo' or 'backlog'"},
				"parent_id": map[string]any{"type": "string", "description": "Filter by parent. Use '!' for root tasks only."},
			},
		},
		PromptHint: "List tasks in the project",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			s := *sp
			if s == nil {
				return sdk.ErrorResult("tasklist: not initialized"), nil
			}
			status, _ := args["status"].(string)
			group, _ := args["group"].(string)
			parentID, _ := args["parent_id"].(string)

			tasks := s.List(status, group, parentID)
			if len(tasks) == 0 {
				return sdk.TextResult("No tasks found."), nil
			}

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

			return sdk.TextResult(b.String()), nil
		},
	}
}

func toolGet(sp **Store) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "tasklist_get",
		Description: "Get full details of a single task by ID (supports prefix matching).",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "Task ID (exact or prefix)"},
			},
			"required": []string{"id"},
		},
		PromptHint: "Get details of a specific task",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			s := *sp
			if s == nil {
				return sdk.ErrorResult("tasklist: not initialized"), nil
			}
			id, _ := args["id"].(string)
			if id == "" {
				return sdk.ErrorResult("id is required"), nil
			}

			t, err := s.Resolve(id)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			raw, _ := json.MarshalIndent(t, "", "  ")
			return sdk.TextResult(string(raw)), nil
		},
	}
}

func toolUpdate(sp **Store) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "tasklist_update",
		Description: "Update a task's title or replace its notes.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":    map[string]any{"type": "string", "description": "Task ID (exact or prefix)"},
				"title": map[string]any{"type": "string", "description": "New title (optional)"},
				"notes": map[string]any{"type": "string", "description": "Replace notes content (optional)"},
			},
			"required": []string{"id"},
		},
		PromptHint: "Update a task's title or notes",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			s := *sp
			if s == nil {
				return sdk.ErrorResult("tasklist: not initialized"), nil
			}
			id, _ := args["id"].(string)
			title, _ := args["title"].(string)
			notes, _ := args["notes"].(string)

			t, err := s.Resolve(id)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			updated, err := s.Update(t.ID, title, notes)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			return sdk.TextResult(fmt.Sprintf("Updated %s: %s", updated.ID, updated.Title)), nil
		},
	}
}

func toolDone(sp **Store) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "tasklist_done",
		Description: "Mark a task and all its subtasks as done.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "Task ID (exact or prefix)"},
			},
			"required": []string{"id"},
		},
		PromptHint: "Mark a task as done",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			s := *sp
			if s == nil {
				return sdk.ErrorResult("tasklist: not initialized"), nil
			}
			id, _ := args["id"].(string)
			if id == "" {
				return sdk.ErrorResult("id is required"), nil
			}

			t, err := s.Resolve(id)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			changed, err := s.Done(t.ID)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			msg := fmt.Sprintf("Done: %d task(s) marked complete", len(changed))
			return sdk.TextResult(msg), nil
		},
	}
}

func toolUndone(sp **Store) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "tasklist_undone",
		Description: "Reactivate a done task and all its subtasks.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "Task ID (exact or prefix)"},
			},
			"required": []string{"id"},
		},
		PromptHint: "Reactivate a done task",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			s := *sp
			if s == nil {
				return sdk.ErrorResult("tasklist: not initialized"), nil
			}
			id, _ := args["id"].(string)
			if id == "" {
				return sdk.ErrorResult("id is required"), nil
			}

			t, err := s.Resolve(id)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			changed, err := s.Undone(t.ID)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			msg := fmt.Sprintf("Reactivated: %d task(s)", len(changed))
			return sdk.TextResult(msg), nil
		},
	}
}

func toolDelete(sp **Store) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "tasklist_delete",
		Description: "Delete a task and all its subtasks permanently.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id": map[string]any{"type": "string", "description": "Task ID (exact or prefix)"},
			},
			"required": []string{"id"},
		},
		PromptHint: "Delete a task",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			s := *sp
			if s == nil {
				return sdk.ErrorResult("tasklist: not initialized"), nil
			}
			id, _ := args["id"].(string)
			if id == "" {
				return sdk.ErrorResult("id is required"), nil
			}

			t, err := s.Resolve(id)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			deleted, err := s.Delete(t.ID)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			return sdk.TextResult(fmt.Sprintf("Deleted %d task(s)", len(deleted))), nil
		},
	}
}

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
			s := *sp
			if s == nil {
				return sdk.ErrorResult("tasklist: not initialized"), nil
			}
			id, _ := args["id"].(string)
			group, _ := args["group"].(string)
			newParentID, _ := args["new_parent_id"].(string)

			if id == "" {
				return sdk.ErrorResult("id is required"), nil
			}

			t, err := s.Resolve(id)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

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
			s := *sp
			if s == nil {
				return sdk.ErrorResult("tasklist: not initialized"), nil
			}
			id, _ := args["id"].(string)
			action, _ := args["action"].(string)
			content, _ := args["content"].(string)

			if id == "" {
				return sdk.ErrorResult("id is required"), nil
			}

			t, err := s.Resolve(id)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

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
			s := *sp
			if s == nil {
				return sdk.ErrorResult("tasklist: not initialized"), nil
			}
			id, _ := args["id"].(string)
			field, _ := args["field"].(string)
			value, _ := args["value"].(string)

			if id == "" || field == "" || value == "" {
				return sdk.ErrorResult("id, field, and value are required"), nil
			}

			t, err := s.Resolve(id)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			updated, err := s.Link(t.ID, field, value)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			return sdk.TextResult(fmt.Sprintf("Linked %s %s → %s", field, value, updated.ID)), nil
		},
	}
}

func toolStatus(sp **Store, version string) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "tasklist_status",
		Description: "Show tasklist status: version, store path, and task counts by status and group.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		PromptHint: "Check tasklist status and counts",
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			s := *sp
			if s == nil {
				return sdk.ErrorResult("tasklist: not initialized"), nil
			}

			active, done, backlog := s.Stats()
			total := active + done + backlog
			return sdk.TextResult(fmt.Sprintf(
				"tasklist v%s\nStore: %s\nTasks: %d total · %d active · %d backlog · %d done",
				version, s.Path(), total, active, backlog, done,
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
			s := *sp
			if s == nil {
				return sdk.ErrorResult("tasklist: not initialized"), nil
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
