package sift

import (
	"context"
	"os"
	"path/filepath"
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

	defaultText := "Large tool results are automatically compressed by Sift. Content below 4KB passes through unchanged. Repeated patterns and excessive blank lines are collapsed. If a result shows a [SIFT:] header, the original was larger — request the raw content if you need it."

	dir, err := xdg.ConfigDir()
	if err != nil {
		return defaultText
	}

	promptPath := filepath.Join(dir, "sift-prompt.md")
	data, err := os.ReadFile(promptPath)
	if err != nil {
		tmp := promptPath + ".tmp"
		if os.WriteFile(tmp, []byte(defaultText+"\n"), 0644) == nil {
			os.Rename(tmp, promptPath)
		}
		return defaultText
	}

	return string(data)
}
