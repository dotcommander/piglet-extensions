package sessiontools

import (
	"context"
	"strings"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	sdk "github.com/dotcommander/piglet/sdk"
)

// EnhanceSummary refines a template summary via LLM.
// Returns the original unchanged if the call fails.
func EnhanceSummary(ctx context.Context, ext *sdk.Extension, templateSummary string, cfg Config) string {
	system := xdg.LoadOrCreateFile("session-handoff-enhance.md", defaultEnhancePrompt())

	resp, err := ext.Chat(ctx, sdk.ChatRequest{
		System: system,
		Messages: []sdk.ChatMessage{
			{Role: "user", Content: templateSummary},
		},
		Model:     "small",
		MaxTokens: cfg.LLMMaxTokens,
	})
	if err != nil {
		return templateSummary
	}

	if resp.Text == "" {
		return templateSummary
	}

	return resp.Text
}

// shouldEnhance returns true if the facts warrant LLM refinement.
func shouldEnhance(facts []memoryFact, cfg Config) bool {
	switch cfg.SummaryMode {
	case SummaryModeLLM:
		return true
	case SummaryModeTemplate:
		return false
	default: // "auto"
		if len(facts) > 20 {
			return true
		}
		errorCount := 0
		for _, f := range facts {
			if strings.HasPrefix(f.Key, "ctx:error") {
				errorCount++
				if errorCount >= 3 {
					return true
				}
			}
		}
		return false
	}
}

func defaultEnhancePrompt() string {
	lines := []string{
		"You refine session handoff summaries. Given a structured template summary built from memory facts, produce a cleaner version that:",
		"1. Infers a clear one-sentence goal if the Goal section is missing or vague",
		"2. Groups related progress items and marks clear completions with [x]",
		"3. Synthesizes errors into root causes rather than listing each one",
		"4. Suggests concrete next steps based on what was done and what failed",
		"",
		"Keep the same markdown structure (## Goal, ## Progress, ## Key Decisions, ## Context, ## Errors, ## Next Steps).",
		"Be concise. Output only the refined summary, no commentary.",
	}
	return strings.Join(lines, "\n")
}
