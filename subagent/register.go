// Package subagent provides the tmux-based agent dispatch tool for piglet.
// Agents are spawned as full piglet instances in tmux panes, giving the user
// full visibility and intervention capability.
package subagent

import (
	sdk "github.com/dotcommander/piglet/sdk"
)

const Version = "0.1.0"

// Register adds the dispatch tool to the extension.
func Register(e *sdk.Extension) {
	e.RegisterTool(sdk.ToolDef{
		Name:        "dispatch",
		Description: "Spawn a piglet agent in a tmux pane to handle a task independently. The agent runs as a full piglet instance with complete tool access and streaming visibility. The user can observe and intervene via the tmux pane. Results are returned when the agent completes.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task":                  map[string]any{"type": "string", "description": "Task instructions for the agent"},
				"model":                 map[string]any{"type": "string", "description": "Model override (e.g. --model anthropic/claude-haiku-4-5)"},
				"split":                 map[string]any{"type": "string", "enum": []any{"horizontal", "vertical", "window"}, "description": "Tmux layout: horizontal split (default), vertical split, or new window"},
				"absolute_timeout_ms":   map[string]any{"type": "integer", "description": "Hard wall-clock timeout in milliseconds (default 1800000 = 30m). Set <= 0 to disable."},
				"inactivity_timeout_ms": map[string]any{"type": "integer", "description": "Kill the agent if its tmux pane shows no output for this many milliseconds (default 300000 = 5m). Catches stalled agents that connect but freeze mid-task. Set <= 0 to disable."},
			},
			"required": []any{"task"},
		},
		PromptHint: "Spawn an independent agent in a tmux pane for focused research, analysis, or parallel work",
		Execute:    dispatch,
	})
}
