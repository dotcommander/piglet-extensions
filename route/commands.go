package route

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/dotcommander/piglet/sdk"
)

func registerCommand(e *sdk.Extension, s *state) {
	e.RegisterCommand(sdk.CommandDef{
		Name:        "route",
		Description: "Route a prompt, learn from feedback, or show stats",
		Handler: func(_ context.Context, args string) error {
			args = strings.TrimSpace(args)

			switch {
			case args == "":
				e.ShowMessage("Usage: /route <prompt> | /route learn | /route stats")
			case args == "learn":
				handleLearn(e, s)
			case args == "stats":
				handleStats(e, s)
			default:
				handleRoutePrompt(e, s, args)
			}
			return nil
		},
	})
}

func handleLearn(e *sdk.Extension, s *state) {
	s.mu.RLock()
	fb := s.feedback
	reg := s.reg
	s.mu.RUnlock()

	if fb == nil {
		e.ShowMessage("Feedback store not ready.")
		return
	}

	learned, err := fb.Learn()
	if err != nil {
		e.ShowMessage(fmt.Sprintf("Learn error: %v", err))
		return
	}

	if reg != nil {
		s.mu.Lock()
		mergeLearnedIntoRegistry(reg, learned)
		s.learned = learned
		s.mu.Unlock()
	}

	tc, atc := countLearned(learned)
	e.ShowMessage(fmt.Sprintf("Learned %d triggers, %d anti-triggers across %d components.",
		tc, atc, len(learned.Triggers)+len(learned.AntiTriggers)))
}

func handleStats(e *sdk.Extension, s *state) {
	s.mu.RLock()
	reg := s.reg
	learned := s.learned
	s.mu.RUnlock()

	var b strings.Builder
	if reg != nil {
		extCount, toolCount, cmdCount := countComponents(reg)
		fmt.Fprintf(&b, "Registry: %d extensions, %d tools, %d commands\n", extCount, toolCount, cmdCount)
	}
	if learned != nil {
		fmt.Fprintf(&b, "Learned triggers: %d components\n", len(learned.Triggers))
		fmt.Fprintf(&b, "Learned anti-triggers: %d components\n", len(learned.AntiTriggers))
	}
	e.ShowMessage(b.String())
}

func handleRoutePrompt(e *sdk.Extension, s *state, prompt string) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.ready || s.reg == nil {
		e.ShowMessage("Route registry not ready.")
		return
	}

	result := s.scorer.Score(prompt, s.cwd, s.reg)
	logRoute(s.fbDir, result, hashPrompt(prompt), "command")
	e.ShowMessage(FormatRouteResult(result))
}

// countComponents returns extension, tool, and command counts from the registry.
func countComponents(reg *Registry) (ext, tool, cmd int) {
	for _, c := range reg.Components {
		switch c.Type {
		case TypeExtension:
			ext++
		case TypeTool:
			tool++
		case TypeCommand:
			cmd++
		}
	}
	return ext, tool, cmd
}
