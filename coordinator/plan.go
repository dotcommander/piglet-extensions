package coordinator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/dotcommander/piglet/sdk"
)

// SubTask represents a decomposed sub-task.
type SubTask struct {
	Task     string `json:"task"`
	Tools    string `json:"tools"`
	Model    string `json:"model"`
	MaxTurns int    `json:"max_turns"`
}

const (
	planModel       = "small"
	planMaxTokens   = 1024
	maxSubTasks     = 5
	defaultMaxTurns = 10
	maxTurnsCap     = 20
	fallbackModel   = "default"
)

// fallbackTask returns a single-task fallback that delegates the entire request to one agent.
func fallbackTask(request string) []SubTask {
	return []SubTask{{
		Task:     request,
		Tools:    "all",
		Model:    fallbackModel,
		MaxTurns: defaultMaxTurns,
	}}
}

// PlanTasks decomposes a user request into sub-tasks using LLM classification.
func PlanTasks(ctx context.Context, ext *sdk.Extension, request string, caps []Capability) ([]SubTask, error) {
	capSummary := FormatCapabilities(caps)

	var b strings.Builder
	fmt.Fprintf(&b, "Available capabilities:\n%s\n", capSummary)

	// Ask route extension for intent/domain classification if available
	if hint := routeHint(ctx, ext, request); hint != "" {
		fmt.Fprintf(&b, "Routing analysis: %s\n\n", hint)
	}

	fmt.Fprintf(&b, "User request: %s", request)
	prompt := b.String()

	resp, err := ext.Chat(ctx, sdk.ChatRequest{
		System:    LoadPlanPrompt(),
		Messages:  []sdk.ChatMessage{{Role: "user", Content: prompt}},
		Model:     planModel,
		MaxTokens: planMaxTokens,
	})
	if err != nil {
		return fallbackTask(request), nil
	}

	var tasks []SubTask
	if err := json.Unmarshal([]byte(resp.Text), &tasks); err != nil {
		return fallbackTask(request), nil
	}

	if len(tasks) == 0 {
		return fallbackTask(request), nil
	}
	if len(tasks) > maxSubTasks {
		tasks = tasks[:maxSubTasks]
	}

	for i := range tasks {
		if tasks[i].Tools == "" {
			tasks[i].Tools = "all"
		}
		if tasks[i].Model == "" {
			tasks[i].Model = fallbackModel
		}
		if tasks[i].MaxTurns <= 0 || tasks[i].MaxTurns > maxTurnsCap {
			tasks[i].MaxTurns = defaultMaxTurns
		}
	}

	return tasks, nil
}

// routeHint calls the route tool via the host to get intent/domain classification.
// Returns empty string if route is unavailable — coordinator works without it.
func routeHint(ctx context.Context, ext *sdk.Extension, request string) string {
	result, err := ext.CallHostTool(ctx, "route", map[string]any{
		"prompt": request,
	})
	if err != nil || result.IsError {
		return ""
	}

	for _, block := range result.Content {
		if block.Text != "" {
			return block.Text
		}
	}
	return ""
}
