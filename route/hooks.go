package route

import (
	"context"

	sdk "github.com/dotcommander/piglet/sdk"
)

func registerHook(e *sdk.Extension, s *state) {
	e.RegisterMessageHook(sdk.MessageHookDef{
		Name:     "route-classify",
		Priority: 900,
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
