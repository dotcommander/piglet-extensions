package route

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	sdk "github.com/dotcommander/piglet/sdk"
)

// state holds mutable state shared across handlers.
type state struct {
	mu       sync.RWMutex
	scorer   *Scorer
	reg      *Registry
	config   Config
	feedback *FeedbackStore
	learned  *LearnedTriggers
	cwd      string
	fbDir    string
	ready    bool
}

// Register wires the route extension into a shared SDK extension.
func Register(e *sdk.Extension, version string) {
	s := &state{}

	e.OnInitAppend(func(x *sdk.Extension) {

		s.cwd = x.CWD()
		s.config = LoadConfig()

		intents := LoadIntents()
		domains := LoadDomains()

		ic := NewIntentClassifier(intents)
		de := NewDomainExtractor(domains)
		s.scorer = NewScorer(s.config, ic, de)

		// Build registry from host data
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		reg, err := BuildRegistry(ctx, x)
		if err != nil {
			x.Log("warn", fmt.Sprintf("[route] registry build failed: %v", err))
			reg = nil
		}

		// Load feedback store and learned triggers
		fbDir, _ := feedbackDir()
		fb := NewFeedbackStore(fbDir)
		learned := fb.LoadLearned()

		// Merge learned triggers into registry
		if reg != nil {
			mergeLearnedIntoRegistry(reg, learned)
		}

		s.mu.Lock()
		s.reg = reg
		s.feedback = fb
		s.learned = learned
		s.fbDir = fbDir
		s.ready = true
		s.mu.Unlock()
	})

	// Tool: route — explicit routing query from LLM
	e.RegisterTool(sdk.ToolDef{
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
	})

	// Tool: route_feedback — record correct/wrong routing for learning
	e.RegisterTool(sdk.ToolDef{
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
	})

	// Tool: route_status — extension status for diagnostics
	e.RegisterTool(sdk.ToolDef{
		Name:        "route_status",
		Description: "Show route extension status: version, registry state, and learned trigger counts.",
		Parameters:  map[string]any{"type": "object", "properties": map[string]any{}},
		Execute: func(_ context.Context, _ map[string]any) (*sdk.ToolResult, error) {
			s.mu.RLock()
			defer s.mu.RUnlock()

			if !s.ready || s.reg == nil {
				return sdk.TextResult(fmt.Sprintf("route v%s\n  State: registry not loaded", version)), nil
			}
			tc, atc := 0, 0
			if s.learned != nil {
				for _, v := range s.learned.Triggers {
					tc += len(v)
				}
				for _, v := range s.learned.AntiTriggers {
					atc += len(v)
				}
			}
			return sdk.TextResult(fmt.Sprintf("route v%s\n  State: ready\n  Triggers: %d learned, %d anti-triggers\n  Components: %d", version, tc, atc, len(s.reg.Components))), nil
		},
	})

	// Command: /route — diagnostic routing, learn, stats
	e.RegisterCommand(sdk.CommandDef{
		Name:        "route",
		Description: "Route a prompt, learn from feedback, or show stats",
		Handler: func(_ context.Context, args string) error {
			args = strings.TrimSpace(args)

			switch {
			case args == "":
				e.ShowMessage("Usage: /route <prompt> | /route learn | /route stats")
				return nil

			case args == "learn":
				s.mu.RLock()
				fb := s.feedback
				reg := s.reg
				s.mu.RUnlock()

				if fb == nil {
					e.ShowMessage("Feedback store not ready.")
					return nil
				}

				learned, err := fb.Learn()
				if err != nil {
					e.ShowMessage(fmt.Sprintf("Learn error: %v", err))
					return nil
				}

				// Merge into live registry
				if reg != nil {
					s.mu.Lock()
					mergeLearnedIntoRegistry(reg, learned)
					s.learned = learned
					s.mu.Unlock()
				}

				trigCount := 0
				antiCount := 0
				for _, v := range learned.Triggers {
					trigCount += len(v)
				}
				for _, v := range learned.AntiTriggers {
					antiCount += len(v)
				}
				e.ShowMessage(fmt.Sprintf("Learned %d triggers, %d anti-triggers across %d components.",
					trigCount, antiCount, len(learned.Triggers)+len(learned.AntiTriggers)))
				return nil

			case args == "stats":
				s.mu.RLock()
				reg := s.reg
				learned := s.learned
				s.mu.RUnlock()

				var b strings.Builder
				if reg != nil {
					extCount := 0
					toolCount := 0
					cmdCount := 0
					for _, c := range reg.Components {
						switch c.Type {
						case TypeExtension:
							extCount++
						case TypeTool:
							toolCount++
						case TypeCommand:
							cmdCount++
						}
					}
					fmt.Fprintf(&b, "Registry: %d extensions, %d tools, %d commands\n", extCount, toolCount, cmdCount)
				}
				if learned != nil {
					fmt.Fprintf(&b, "Learned triggers: %d components\n", len(learned.Triggers))
					fmt.Fprintf(&b, "Learned anti-triggers: %d components\n", len(learned.AntiTriggers))
				}
				e.ShowMessage(b.String())
				return nil

			default:
				s.mu.RLock()
				defer s.mu.RUnlock()

				if !s.ready || s.reg == nil {
					e.ShowMessage("Route registry not ready.")
					return nil
				}

				result := s.scorer.Score(args, s.cwd, s.reg)
				logRoute(s.fbDir, result, hashPrompt(args), "command")
				e.ShowMessage(FormatRouteResult(result))
				return nil
			}
		},
	})

	// Message hook: auto-classify and inject routing context
	e.RegisterMessageHook(sdk.MessageHookDef{
		Name:     "route-classify",
		Priority: 900, // high priority, runs early
		OnMessage: func(_ context.Context, msg string) (string, error) {
			s.mu.RLock()
			defer s.mu.RUnlock()

			if !s.ready || s.reg == nil || !s.config.MessageHook.Enabled {
				return "", nil
			}

			result := s.scorer.Score(msg, s.cwd, s.reg)

			if result.Confidence < s.config.MessageHook.MinConfidence && len(result.Primary) == 0 {
				return "", nil
			}

			logRoute(s.fbDir, result, hashPrompt(msg), "hook")
			return FormatHookContext(result), nil
		},
	})
}

// mergeLearnedIntoRegistry adds learned triggers and anti-triggers to matching
// registry components. Learned triggers extend existing Keywords; anti-triggers
// extend AntiTriggers.
func mergeLearnedIntoRegistry(reg *Registry, lt *LearnedTriggers) {
	if lt == nil {
		return
	}
	for i := range reg.Components {
		comp := &reg.Components[i]
		keys := componentKeys(comp)
		comp.Keywords = mergeField(comp.Keywords, keys, lt.Triggers)
		comp.AntiTriggers = mergeField(comp.AntiTriggers, keys, lt.AntiTriggers)
	}
}

// componentKeys returns the lookup keys for a component: its Name and Extension (if different).
func componentKeys(comp *Component) []string {
	if comp.Extension != "" && comp.Extension != comp.Name {
		return []string{comp.Name, comp.Extension}
	}
	return []string{comp.Name}
}

// mergeField appends any learned entries matching keys, deduplicating the result.
func mergeField(existing []string, keys []string, learned map[string][]string) []string {
	var added []string
	for _, k := range keys {
		if vals, ok := learned[k]; ok {
			added = append(added, vals...)
		}
	}
	if len(added) == 0 {
		return existing
	}
	return dedupStrings(append(existing, added...))
}
