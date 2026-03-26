package suggest

import (
	"context"
	"time"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	sdk "github.com/dotcommander/piglet/sdk"
)

// Config holds suggestion settings.
type Config struct {
	Model       string        `yaml:"model"`
	MaxTokens   int           `yaml:"max_tokens"`
	Timeout     time.Duration `yaml:"timeout"`
	Cooldown    int           `yaml:"cooldown"`
	Enabled     bool          `yaml:"enabled"`
	TriggerMode string        `yaml:"trigger_mode"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Model:       "small",
		MaxTokens:   50,
		Timeout:     5 * time.Second,
		Cooldown:    3,
		Enabled:     true,
		TriggerMode: "auto",
	}
}

// LoadConfig loads config from ~/.config/piglet/suggest.yaml, creating defaults if missing.
func LoadConfig() Config {
	return xdg.LoadYAML("suggest.yaml", DefaultConfig())
}

// LoadPrompt loads the prompt template from ~/.config/piglet/suggest.md, creating default if missing.
func LoadPrompt(ext *sdk.Extension) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	prompt, err := ext.ConfigReadExtension(ctx, "suggest")
	if err == nil && prompt != "" {
		return prompt
	}

	return xdg.LoadOrCreateFile("suggest.md", DefaultPrompt())
}

// DefaultPrompt returns the default suggestion prompt template.
func DefaultPrompt() string {
	return `You suggest the user's next prompt based on conversation context.

Rules:
- Output ONE short prompt (max 80 chars)
- Make it actionable and specific
- Reference files, functions, or tasks mentioned
- Skip obvious suggestions ("continue", "done?")
- If the task appears complete, suggest verification or next logical step
- If there was an error, suggest a fix or investigation

Output format: Just the prompt text, no quotes, no explanation.`
}
