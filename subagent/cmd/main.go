// Subagent extension binary. Delegates tasks to independent sub-agents.
// Communicates with piglet host via JSON-RPC over stdin/stdout.
// Uses SDK RunAgent host method instead of creating its own agent/provider.
package main

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/dotcommander/piglet/sdk"
)

func main() {
	ext := sdk.New("subagent", "0.1.0")

	var prompt string

	ext.OnInit(func(e *sdk.Extension) {
		p, _ := e.ConfigReadExtension(context.Background(), "subagent")
		prompt = p
	})

	ext.RegisterTool(sdk.ToolDef{
		Name:        "dispatch",
		Description: "Delegate a task to an independent sub-agent that runs to completion and returns results. Use for research, analysis, or any task that benefits from focused execution with its own context.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"task":      map[string]any{"type": "string", "description": "Task instructions for the sub-agent"},
				"context":   map[string]any{"type": "string", "description": "Additional context to include in the sub-agent's system prompt"},
				"tools":     map[string]any{"type": "string", "enum": []any{"read_only", "all"}, "description": "Tool access level (default: read_only)"},
				"max_turns": map[string]any{"type": "integer", "description": "Maximum turns for the sub-agent"},
				"model":     map[string]any{"type": "string", "description": "Model override (e.g. anthropic/claude-haiku-4-5)"},
				"prefer":    map[string]any{"type": "string", "enum": []any{"default", "small"}, "description": "Model preference: default (main model) or small (cheaper model for background tasks)"},
			},
			"required": []any{"task"},
		},
		PromptHint: "Delegate focused tasks to independent sub-agents for research, analysis, or exploration",
		Execute: func(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
			task, _ := args["task"].(string)
			if task == "" {
				return sdk.ErrorResult("task is required"), nil
			}

			system := prompt
			if extra, _ := args["context"].(string); extra != "" {
				system = system + "\n\n" + extra
			}

			// Resolve tools filter
			tools := "background_safe"
			if access, _ := args["tools"].(string); access == "all" {
				tools = "all"
			}

			// Resolve model
			model := "default"
			if m, _ := args["model"].(string); m != "" {
				model = m
			} else if prefer, _ := args["prefer"].(string); prefer == "small" {
				model = "small"
			}

			maxTurns := 10
			if mt, ok := args["max_turns"].(float64); ok && int(mt) > 0 {
				maxTurns = int(mt)
			}

			resp, err := ext.RunAgent(ctx, sdk.AgentRequest{
				System:   system,
				Task:     task,
				Tools:    tools,
				Model:    model,
				MaxTurns: maxTurns,
			})
			if err != nil {
				return sdk.ErrorResult("agent error: " + err.Error()), nil
			}

			if resp.Text == "" {
				return sdk.TextResult("[sub-agent completed with no text output]"), nil
			}

			var b strings.Builder
			fmt.Fprintf(&b, "[sub-agent: %d turns, %dk in / %dk out tokens]\n\n", resp.Turns, resp.Usage.Input/1000, resp.Usage.Output/1000)
			b.WriteString(resp.Text)
			return sdk.TextResult(b.String()), nil
		},
	})

	ext.Run()
}
