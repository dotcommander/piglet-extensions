package tokengate

import (
	_ "embed"
	"strings"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
)

//go:embed defaults/prompt.md
var defaultPrompt string

func LoadPrompt() string {
	return xdg.LoadOrCreateExt("tokengate", "prompt.md", strings.TrimSpace(defaultPrompt))
}
