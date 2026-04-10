package tokengate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/dotcommander/piglet/sdk"
)

// Register wires all tokengate capabilities into the extension.
func Register(e *sdk.Extension, version string) {
	cfg := LoadConfig()
	budget := new(*BudgetState)
	*budget = NewBudgetState(cfg)

	e.OnInitAppend(func(x *sdk.Extension) {
		if cfg.Enabled {
			prompt := LoadPrompt()
			x.RegisterPromptSection(sdk.PromptSectionDef{
				Title:   "Token Gate",
				Content: prompt,
				Order:   15,
			})
		}
	})

	e.RegisterInterceptor(sdk.InterceptorDef{
		Name:     "tokengate-scope",
		Priority: 80,
		Before:   scopeBeforeInterceptor(cfg),
	})

	e.RegisterEventHandler(sdk.EventHandlerDef{
		Name:     "tokengate-tracker",
		Priority: 110,
		Events:   []string{"EventTurnUsage"},
		Handle:   eventTurnUsage(budget, &cfg),
	})

	registerSummarizer(e, cfg)

	e.RegisterTool(toolContextBudget(budget))
	e.RegisterCommand(budgetCommand(budget, e))
	e.RegisterTool(toolStatus(budget, version, cfg))
}

func eventTurnUsage(budget **BudgetState, cfg *Config) func(context.Context, string, json.RawMessage) *sdk.Action {
	return func(_ context.Context, _ string, data json.RawMessage) *sdk.Action {
		event, err := ParseTurnUsage(data)
		if err != nil {
			return nil
		}
		crossed := (*budget).Record(event)
		if crossed {
			return sdk.ActionNotify(fmt.Sprintf(
				"Context budget warning: %d%% of %s token window used",
				cfg.WarnPercent, fmtNum(cfg.ContextWindow),
			))
		}
		return nil
	}
}

func toolContextBudget(budget **BudgetState) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "context_budget",
		Description: "Show current context window token usage breakdown and budget status.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
			"required":   []string{},
		},
		PromptHint: "Check context window token budget",
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			return sdk.TextResult((*budget).Summary()), nil
		},
	}
}

func budgetCommand(budget **BudgetState, e *sdk.Extension) sdk.CommandDef {
	return sdk.CommandDef{
		Name:        "budget",
		Description: "Show context window token budget and usage breakdown",
		Handler: func(_ context.Context, _ string) error {
			e.ShowMessage((*budget).Summary())
			return nil
		},
	}
}

func toolStatus(budget **BudgetState, version string, cfg Config) sdk.ToolDef {
	return sdk.ToolDef{
		Name:        "tokengate_status",
		Description: "Show tokengate extension status: version, context window, and interceptor state.",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			var b strings.Builder
			fmt.Fprintf(&b, "tokengate %s\n", version)
			fmt.Fprintf(&b, "  Context window: %s tokens\n", fmtNum(cfg.ContextWindow))
			fmt.Fprintf(&b, "  Warn at: %d%%\n", cfg.WarnPercent)
			fmt.Fprintf(&b, "  Scope limiter: %s\n", enabledStr(cfg.Enabled))
			fmt.Fprintf(&b, "  Summarizer: %s (threshold: %d chars)\n", enabledStr(cfg.SummarizeEnabled), cfg.SummarizeThreshold)
			return sdk.TextResult(b.String()), nil
		},
	}
}

func enabledStr(b bool) string {
	if b {
		return "enabled"
	}
	return "disabled"
}
