package sift

import (
	"context"
	"time"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	sdk "github.com/dotcommander/piglet/sdk"
)

func LoadPrompt(ext *sdk.Extension) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	prompt, err := ext.ConfigReadExtension(ctx, "sift")
	if err == nil && prompt != "" {
		return prompt
	}

	return xdg.LoadOrCreateFile("sift-prompt.md", "Large tool results are automatically compressed by Sift. Content below 4KB passes through unchanged. Repeated patterns and excessive blank lines are collapsed. If a result shows a [SIFT:] header, the original was larger — request the raw content if you need it.")
}
