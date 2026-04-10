package cron

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	sdk "github.com/dotcommander/piglet/sdk"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
)

func registerTools(e *sdk.Extension) {
	e.RegisterTool(toolList())
	e.RegisterTool(toolHistory())
	e.RegisterTool(toolRemove())
	e.RegisterTool(toolAdd())
	e.RegisterTool(toolStatus())
}

func toolList() sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "cron_list",
		Description: "List all scheduled cron tasks with their schedules, last run, and next run times",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			summaries, err := ListTasks()
			if err != nil {
				return sdk.ErrorResult("Error: " + err.Error()), nil
			}
			if len(summaries) == 0 {
				return sdk.TextResult("No scheduled tasks configured."), nil
			}

			sort.Slice(summaries, func(i, j int) bool {
				return summaries[i].Name < summaries[j].Name
			})

			return sdk.TextResult(formatTaskList(summaries)), nil
		},
	}
}

func toolHistory() sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "cron_history",
		Description: "Show recent cron task execution history",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task": map[string]any{
					"type":        "string",
					"description": "Filter history by task name (optional)",
				},
				"limit": map[string]any{
					"type":        "number",
					"description": "Max entries to return (default 20)",
				},
			},
		},
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			entries, err := ReadHistory()
			if err != nil {
				return sdk.ErrorResult("Error: " + err.Error()), nil
			}

			taskFilter, _ := args["task"].(string)
			limit := 20
			if l, ok := args["limit"].(float64); ok && l > 0 {
				limit = int(l)
			}

			entries = filterHistory(entries, taskFilter)
			if len(entries) == 0 {
				return sdk.TextResult("No history found."), nil
			}

			return sdk.TextResult(formatHistory(entries, limit)), nil
		},
	}
}

func toolRemove() sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "cron_remove",
		Description: "Remove a scheduled cron task by name",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Task name to remove",
				},
			},
			"required": []string{"name"},
		},
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			name, _ := args["name"].(string)
			if name == "" {
				return sdk.ErrorResult("Task name required."), nil
			}

			cfg := LoadConfig()
			if _, ok := cfg.Tasks[name]; !ok {
				return sdk.ErrorResult(fmt.Sprintf("Task %q not found.", name)), nil
			}

			return removeTask(cfg, name)
		},
	}
}

func toolAdd() sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "cron_add",
		Description: "Add a new scheduled cron task",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Unique task name",
				},
				"action": map[string]any{
					"type":        "string",
					"description": "Action type: shell, prompt, or webhook",
					"enum":        []string{"shell", "prompt", "webhook"},
				},
				"command": map[string]any{
					"type":        "string",
					"description": "Shell command (for action=shell)",
				},
				"prompt": map[string]any{
					"type":        "string",
					"description": "Piglet prompt text (for action=prompt)",
				},
				"url": map[string]any{
					"type":        "string",
					"description": "Webhook URL (for action=webhook)",
				},
				"every": map[string]any{
					"type":        "string",
					"description": "Interval schedule, e.g. '10m', '1h'",
				},
				"daily_at": map[string]any{
					"type":        "string",
					"description": "Daily schedule, e.g. '18:00'",
				},
				"weekly": map[string]any{
					"type":        "string",
					"description": "Weekly schedule, e.g. 'monday 09:00'",
				},
			},
			"required": []string{"name", "action"},
		},
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			name, _ := args["name"].(string)
			action, _ := args["action"].(string)

			task := TaskConfig{
				Action:  action,
				Command: argStr(args, "command"),
				Prompt:  argStr(args, "prompt"),
				URL:     argStr(args, "url"),
			}

			spec := ScheduleSpec{
				Every:   argStr(args, "every"),
				DailyAt: argStr(args, "daily_at"),
				Weekly:  argStr(args, "weekly"),
			}
			sched, err := ParseSchedule(spec)
			if err != nil {
				return sdk.ErrorResult("Invalid schedule: " + err.Error()), nil
			}
			task.Schedule = spec

			cfg := LoadConfig()
			if cfg.Tasks == nil {
				cfg.Tasks = make(map[string]TaskConfig)
			}
			cfg.Tasks[name] = task

			if err := SaveConfig(cfg); err != nil {
				return sdk.ErrorResult("Error saving: " + err.Error()), nil
			}
			schedStr := "unknown"
			if sched != nil {
				schedStr = sched.String()
			}
			return sdk.TextResult(fmt.Sprintf("Task %q added (%s, %s).", name, action, schedStr)), nil
		},
	}
}

func toolStatus() sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "cron_status",
		Description: "Show cron extension status: version, config path, and task counts.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			cfg := LoadConfig()
			total := len(cfg.Tasks)
			enabled := 0
			for _, t := range cfg.Tasks {
				if t.IsEnabled() {
					enabled++
				}
			}
			dir, _ := xdg.ExtensionDir("cron")
			return sdk.TextResult(fmt.Sprintf("cron v%s\nConfig: %s\nTasks: %d total, %d enabled", Version, filepath.Join(dir, "schedules.yaml"), total, enabled)), nil
		},
	}
}

// formatTaskList produces a machine-readable listing of task summaries.
func formatTaskList(summaries []TaskSummary) string {
	var b strings.Builder
	for _, s := range summaries {
		lastRun := "never"
		if !s.LastRun.IsZero() {
			lastRun = s.LastRun.Format(time.RFC3339)
		}
		status := "enabled"
		if !s.Enabled {
			status = "disabled"
		}
		if s.Overdue {
			status = "OVERDUE"
		}
		fmt.Fprintf(&b, "%s: action=%s schedule=%q status=%s last_run=%s next_run=%s\n",
			s.Name, s.Action, s.Schedule, status, lastRun, s.NextRun.Format(time.RFC3339))
	}
	return b.String()
}

// formatHistory returns the last limit entries as formatted text.
func formatHistory(entries []RunEntry, limit int) string {
	start := 0
	if len(entries) > limit {
		start = len(entries) - limit
	}

	var b strings.Builder
	for _, entry := range entries[start:] {
		status := "ok"
		if !entry.Success {
			status = "FAIL"
		}
		fmt.Fprintf(&b, "%s [%s] %s (%dms)", entry.RanAt, status, entry.Task, entry.DurationMs)
		if entry.Error != "" {
			fmt.Fprintf(&b, " — %s", entry.Error)
		}
		b.WriteString("\n")
	}
	return b.String()
}
