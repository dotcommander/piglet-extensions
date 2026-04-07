package tokengate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	sdk "github.com/dotcommander/piglet/sdk"
)

var budget *BudgetState

// Register registers the tokengate extension: scope limiter + budget tracker.
func Register(e *sdk.Extension, version string) {
	cfg := LoadConfig()
	budget = NewBudgetState(cfg)

	// OnInit: register prompt section (needs config to be loaded)
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

	// Scope interceptor: rewrites tool calls for token efficiency
	e.RegisterInterceptor(sdk.InterceptorDef{
		Name:     "tokengate-scope",
		Priority: 80,
		Before:   scopeBeforeInterceptor(cfg),
	})

	// Budget event handler: track token usage per turn
	e.RegisterEventHandler(sdk.EventHandlerDef{
		Name:     "tokengate-tracker",
		Priority: 110, // after usage-tracker (100)
		Events:   []string{"EventTurnUsage"},
		Handle: func(_ context.Context, _ string, data json.RawMessage) *sdk.Action {
			event, err := ParseTurnUsage(data)
			if err != nil {
				return nil
			}
			crossed := budget.Record(event)
			if crossed {
				return sdk.ActionNotify(fmt.Sprintf(
					"Context budget warning: %d%% of %s token window used",
					cfg.WarnPercent, fmtNum(cfg.ContextWindow),
				))
			}
			return nil
		},
	})

	// Summarize interceptor: auto-summarize large tool results via LLM
	registerSummarizer(e, cfg)

	// Budget tool
	e.RegisterTool(sdk.ToolDef{
		Name:        "context_budget",
		Description: "Show current context window token usage breakdown and budget status.",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
			"required":   []string{},
		},
		PromptHint: "Check context window token budget",
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			return sdk.TextResult(budget.Summary()), nil
		},
	})

	// Budget command
	e.RegisterCommand(sdk.CommandDef{
		Name:        "budget",
		Description: "Show context window token budget and usage breakdown",
		Handler: func(_ context.Context, _ string) error {
			e.ShowMessage(budget.Summary())
			return nil
		},
	})
	// Status tool
	e.RegisterTool(sdk.ToolDef{
		Name:        "tokengate_status",
		Description: "Show tokengate extension status: version, context window, and interceptor state.",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			var b strings.Builder
			fmt.Fprintf(&b, "tokengate %s\n", version)
			fmt.Fprintf(&b, "  Context window: %s tokens\n", fmtNum(cfg.ContextWindow))
			fmt.Fprintf(&b, "  Warn at: %d%%\n", cfg.WarnPercent)
			fmt.Fprintf(&b, "  Scope limiter: %s\n", boolStr(cfg.Enabled))
			fmt.Fprintf(&b, "  Summarizer: %s (threshold: %d chars)\n", boolStr(cfg.SummarizeEnabled), cfg.SummarizeThreshold)
			return sdk.TextResult(b.String()), nil
		},
	})
}

func boolStr(b bool) string {
	if b {
		return "enabled"
	}
	return "disabled"
}
