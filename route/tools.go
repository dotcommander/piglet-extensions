package route

import (
	"context"
	"encoding/json"
	"fmt"

	sdk "github.com/dotcommander/piglet/sdk"
)

func registerTools(e *sdk.Extension, s *state) {
	e.RegisterTool(toolRoute(s))
	e.RegisterTool(toolFeedback(s))
	e.RegisterTool(toolStatus(s))
}

func toolRoute(s *state) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "route",
		Description: "Classify a prompt and return ranked piglet extensions/tools most relevant to it. Use when you need to discover which tools or extensions are best suited for a task.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prompt": map[string]any{
					"type":        "string",
					"description": "The prompt or task description to route",
				},
			},
			"required": []string{"prompt"},
		},
		PromptHint: "Find the most relevant tools and extensions for a task",
		Execute: func(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
			prompt, _ := args["prompt"].(string)
			if prompt == "" {
				return sdk.ErrorResult("prompt is required"), nil
			}

			s.mu.RLock()
			defer s.mu.RUnlock()

			if !s.ready || s.reg == nil {
				return sdk.ErrorResult("route registry not ready"), nil
			}

			result := s.scorer.Score(prompt, s.cwd, s.reg)
			logRoute(s.fbDir, result, hashPrompt(prompt), "tool")

			data, err := json.MarshalIndent(result, "", "  ")
			if err != nil {
				return sdk.ErrorResult(fmt.Sprintf("marshal: %v", err)), nil
			}
			return sdk.TextResult(string(data)), nil
		},
	}
}

func toolFeedback(s *state) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "route_feedback",
		Description: "Record whether a routing recommendation was correct or wrong. Use after completing a task to improve future routing accuracy.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prompt": map[string]any{
					"type":        "string",
					"description": "The original prompt that was routed",
				},
				"component": map[string]any{
					"type":        "string",
					"description": "The component name (extension or tool) to give feedback on",
				},
				"correct": map[string]any{
					"type":        "boolean",
					"description": "True if this component was the right choice, false if wrong",
				},
			},
			"required": []string{"prompt", "component", "correct"},
		},
		PromptHint: "Record routing accuracy feedback to improve future recommendations",
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			prompt, _ := args["prompt"].(string)
			component, _ := args["component"].(string)
			correct, _ := args["correct"].(bool)

			if prompt == "" || component == "" {
				return sdk.ErrorResult("prompt and component are required"), nil
			}

			s.mu.RLock()
			fb := s.feedback
			s.mu.RUnlock()

			if fb == nil {
				return sdk.ErrorResult("feedback store not ready"), nil
			}

			if err := fb.Record(prompt, component, correct); err != nil {
				return sdk.ErrorResult(fmt.Sprintf("record feedback: %v", err)), nil
			}

			action := "correct"
			if !correct {
				action = "wrong"
			}
			return sdk.TextResult(fmt.Sprintf("Recorded %s feedback for %q on prompt %q. Run /route learn to apply.", action, component, truncatePrompt(prompt, 50))), nil
		},
	}
}

func toolStatus(s *state) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "route_status",
		Description: "Show route extension status: version, registry state, and learned trigger counts.",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			s.mu.RLock()
			defer s.mu.RUnlock()

			if !s.ready || s.reg == nil {
				return sdk.TextResult(fmt.Sprintf("route v%s\n  State: registry not loaded", Version)), nil
			}
			tc, atc := countLearned(s.learned)
			return sdk.TextResult(fmt.Sprintf("route v%s\n  State: ready\n  Triggers: %d learned, %d anti-triggers\n  Components: %d", Version, tc, atc, len(s.reg.Components))), nil
		},
	}
}

// countLearned returns total trigger and anti-trigger counts.
func countLearned(lt *LearnedTriggers) (triggers, antiTriggers int) {
	if lt == nil {
		return 0, 0
	}
	for _, v := range lt.Triggers {
		triggers += len(v)
	}
	for _, v := range lt.AntiTriggers {
		antiTriggers += len(v)
	}
	return triggers, antiTriggers
}
