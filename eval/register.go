package eval

import (
	"context"
	_ "embed"
	"fmt"
	"strings"

	"github.com/dotcommander/piglet/sdk"
)

//go:embed defaults/judge-prompt.md
var defaultJudgePrompt string

//go:embed defaults/example-suite.yaml
var defaultExampleSuite string

// Register wires all eval capabilities into the extension.
func Register(e *sdk.Extension) {
	e.OnInitAppend(func(x *sdk.Extension) {
		_ = SeedDefaults()
	})

	registerCommands(e)
	registerTools(e)
}

func registerCommands(e *sdk.Extension) {
	e.RegisterCommand(sdk.CommandDef{
		Name:        "eval",
		Description: "Run evaluation suites: /eval run <suite>, /eval list, /eval results [suite], /eval compare <path1> <path2>",
		Handler: func(ctx context.Context, args string) error {
			sub := strings.TrimSpace(args)
			switch {
			case sub == "" || sub == "list":
				text, err := listSuitesText(e)
				showOrError(e, err, text)
			case strings.HasPrefix(sub, "run "):
				text, err := runSuite(ctx, e, strings.TrimSpace(strings.TrimPrefix(sub, "run ")), nil)
				showOrError(e, err, text)
			case strings.HasPrefix(sub, "results"):
				filter := strings.TrimSpace(strings.TrimPrefix(sub, "results"))
				showEvalResults(e, filter)
			case strings.HasPrefix(sub, "compare "):
				parts := strings.Fields(strings.TrimPrefix(sub, "compare "))
				if len(parts) < 2 {
					e.ShowMessage("Usage: /eval compare <path1> <path2>")
					return nil
				}
				text, err := compareResults(parts[0], parts[1])
				if err != nil {
					e.ShowMessage("Error: " + err.Error())
					return nil
				}
				e.ShowMessage("```\n" + text + "```")
			default:
				e.ShowMessage("Usage: /eval [run <suite>|list|results [suite]|compare <path1> <path2>]")
			}
			return nil
		},
	})
}

func showEvalResults(e *sdk.Extension, suiteFilter string) {
	summaries, err := ListResults(suiteFilter)
	if err != nil {
		e.ShowMessage("Error: " + err.Error())
		return
	}
	if len(summaries) == 0 {
		msg := "No results found."
		if suiteFilter != "" {
			msg = fmt.Sprintf("No results found for suite %q.", suiteFilter)
		}
		e.ShowMessage(msg)
		return
	}
	var b strings.Builder
	b.WriteString("**Evaluation Results**\n\n")
	for _, s := range summaries {
		fmt.Fprintf(&b, "- **%s** — %s — %d/%d passed\n  `%s`\n",
			s.Suite, s.RanAt.Format("2006-01-02 15:04"), s.Passed, s.Total, s.Path)
	}
	e.ShowMessage(b.String())
}

func registerTools(e *sdk.Extension) {
	e.RegisterTool(sdk.ToolDef{
		Name:        "eval_run",
		Description: "Run an evaluation suite by name and return the results summary",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"suite": map[string]any{
					"type":        "string",
					"description": "Name of the suite to run (filename without .yaml)",
				},
				"cases": map[string]any{
					"type":        "array",
					"description": "Optional list of case names to run (empty = run all)",
					"items":       map[string]any{"type": "string"},
				},
			},
			"required": []string{"suite"},
		},
		Execute: func(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
			return toolEvalRun(ctx, e, args)
		},
	})

	e.RegisterTool(sdk.ToolDef{
		Name:        "eval_list",
		Description: "List all available evaluation suites",
		Parameters: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			return toolEvalList(e)
		},
	})

	e.RegisterTool(sdk.ToolDef{
		Name:        "eval_compare",
		Description: "Compare two evaluation run results and show score deltas",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"run_a": map[string]any{
					"type":        "string",
					"description": "Path to first result JSON file",
				},
				"run_b": map[string]any{
					"type":        "string",
					"description": "Path to second result JSON file",
				},
			},
			"required": []string{"run_a", "run_b"},
		},
		Execute: func(_ context.Context, args map[string]any) (*sdk.ToolResult, error) {
			return toolEvalCompare(args)
		},
	})
}

func toolEvalRun(ctx context.Context, e *sdk.Extension, args map[string]any) (*sdk.ToolResult, error) {
	suiteName, _ := args["suite"].(string)
	if suiteName == "" {
		return sdk.ErrorResult("suite name required"), nil
	}

	var caseFilter []string
	if raw, ok := args["cases"].([]any); ok {
		for _, v := range raw {
			if s, ok := v.(string); ok {
				caseFilter = append(caseFilter, s)
			}
		}
	}

	text, err := runSuite(ctx, e, suiteName, caseFilter)
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}
	return sdk.TextResult(text), nil
}

func toolEvalList(e *sdk.Extension) (*sdk.ToolResult, error) {
	text, err := listSuitesText(e)
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}
	return sdk.TextResult(text), nil
}

func toolEvalCompare(args map[string]any) (*sdk.ToolResult, error) {
	pathA, _ := args["run_a"].(string)
	pathB, _ := args["run_b"].(string)
	if pathA == "" || pathB == "" {
		return sdk.ErrorResult("run_a and run_b are required"), nil
	}
	text, err := compareResults(pathA, pathB)
	if err != nil {
		return sdk.ErrorResult(err.Error()), nil
	}
	return sdk.TextResult(text), nil
}

// showOrError displays text or an error message via ShowMessage.
func showOrError(e *sdk.Extension, err error, text string) {
	if err != nil {
		e.ShowMessage("Error: " + err.Error())
		return
	}
	e.ShowMessage(text)
}

func formatRunSummary(result *RunResult, savedPath string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**%s** — %d/%d passed (avg score: %.2f)\n\n",
		result.Suite, result.Summary.Passed, result.Summary.Total, result.Summary.AvgScore)
	for _, c := range result.Cases {
		status := "PASS"
		if !c.Pass {
			status = "FAIL"
		}
		fmt.Fprintf(&b, "[%s] %s (%.2f) — %s\n", status, c.Name, c.Score, c.Reason)
	}
	if savedPath != "" {
		fmt.Fprintf(&b, "\nSaved: `%s`", savedPath)
	}
	return b.String()
}
