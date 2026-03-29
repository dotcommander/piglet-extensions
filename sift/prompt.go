package sift

import (
	"context"
	_ "embed"
	"strings"
	"time"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	sdk "github.com/dotcommander/piglet/sdk"
)

//go:embed defaults/prompt.md
var defaultPrompt string

func LoadPrompt(ext *sdk.Extension) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	prompt, err := ext.ConfigReadExtension(ctx, "sift")
	if err == nil && prompt != "" {
		return prompt
	}

	return xdg.LoadOrCreateFile("sift-prompt.md", strings.TrimSpace(defaultPrompt))
}
