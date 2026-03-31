package tokengate

import (
	sdk "github.com/dotcommander/piglet/sdk"
)

func Register(e *sdk.Extension) {
	var cfg Config

	e.OnInit(func(x *sdk.Extension) {
		cfg = LoadConfig()
		if !cfg.Enabled {
			return
		}

		prompt := LoadPrompt()
		x.RegisterPromptSection(sdk.PromptSectionDef{
			Title:   "Token Gate",
			Content: prompt,
			Order:   15,
		})
	})

	e.RegisterInterceptor(sdk.InterceptorDef{
		Name:     "tokengate-scope",
		Priority: 80,
		Before:   scopeBeforeInterceptor(cfg),
	})
}
