// Behavior extension. Loads behavioral guidelines from ~/.config/piglet/behavior.md
// and injects them as the earliest system prompt section.
package main

import (
	"context"

	sdk "github.com/dotcommander/piglet/sdk"
)

const behaviorOrder = 10

func main() {
	e := sdk.New("behavior", "0.1.0")

	e.OnInit(func(ext *sdk.Extension) {
		content, _ := ext.ConfigReadExtension(context.Background(), "behavior")
		if content == "" {
			return
		}
		ext.RegisterPromptSection(sdk.PromptSectionDef{
			Title:   "Guidelines",
			Content: content,
			Order:   behaviorOrder,
		})
	})

	e.Run()
}
