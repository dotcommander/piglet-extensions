package tasklist

import (
	"context"
	"fmt"

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
			s, errRes := requireStore(sp)
			if errRes != nil {
				return errRes, nil
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
			s, errRes := requireStore(sp)
			if errRes != nil {
				return errRes, nil
			}
			status, _ := args["status"].(string)
			group, _ := args["group"].(string)
			parentID, _ := args["parent_id"].(string)

			tasks := s.List(status, group, parentID)
			if len(tasks) == 0 {
				return sdk.TextResult("No tasks found."), nil
			}

			return sdk.TextResult(formatTaskList(tasks)), nil
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
			_, t, errRes := requireStoreAndResolve(sp, args)
			if errRes != nil {
				return errRes, nil
			}

			return sdk.TextResult(formatTaskJSON(t)), nil
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
			s, t, errRes := requireStoreAndResolve(sp, args)
			if errRes != nil {
				return errRes, nil
			}
			title, _ := args["title"].(string)
			notes, _ := args["notes"].(string)

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
			s, t, errRes := requireStoreAndResolve(sp, args)
			if errRes != nil {
				return errRes, nil
			}

			changed, err := s.Done(t.ID)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			return sdk.TextResult(fmt.Sprintf("Done: %d task(s) marked complete", len(changed))), nil
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
			s, t, errRes := requireStoreAndResolve(sp, args)
			if errRes != nil {
				return errRes, nil
			}

			changed, err := s.Undone(t.ID)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			return sdk.TextResult(fmt.Sprintf("Reactivated: %d task(s)", len(changed))), nil
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
			s, t, errRes := requireStoreAndResolve(sp, args)
			if errRes != nil {
				return errRes, nil
			}

			deleted, err := s.Delete(t.ID)
			if err != nil {
				return sdk.ErrorResult(err.Error()), nil
			}

			return sdk.TextResult(fmt.Sprintf("Deleted %d task(s)", len(deleted))), nil
		},
	}
}
