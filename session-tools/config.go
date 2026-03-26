package sessiontools

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"time"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
)

const (
	SummaryModeAuto     = "auto"
	SummaryModeTemplate = "template"
	SummaryModeLLM      = "llm"
)

type Config struct {
	Enabled      bool          `yaml:"enabled"`
	SummaryMode  string        `yaml:"summary_mode"`
	LLMTimeout   time.Duration `yaml:"llm_timeout"`
	LLMMaxTokens int           `yaml:"llm_max_tokens"`
	MaxQuerySize int64         `yaml:"max_query_size"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		Enabled:      true,
		SummaryMode:  SummaryModeAuto,
		LLMTimeout:   30 * time.Second,
		LLMMaxTokens: 1024,
		MaxQuerySize: 1 << 20, // 1MB
	}
}

// LoadConfig reads config from ~/.config/piglet/session-handoff.yaml, creating defaults if missing.
func LoadConfig() Config {
	return xdg.LoadYAML("session-handoff.yaml", DefaultConfig())
}

// LoadPromptContent reads the handoff prompt section from ~/.config/piglet/session-handoff.md,
// creating a default if missing.
func LoadPromptContent() string {
	return xdg.LoadOrCreateFile("session-handoff.md", defaultPromptContent())
}

func defaultPromptContent() string {
	return "Use /handoff or the handoff tool to transfer context to a new session with a structured summary of goal, progress, decisions, and next steps. " +
		"Use the session_query tool to search a parent session's content by keyword when you need to recover specific details after a handoff."
}

// MemoryStorePath returns the memory JSONL path for the given cwd (same logic as memory/store.go).
func MemoryStorePath(cwd string) (string, error) {
	base, err := xdg.ConfigDir()
	if err != nil {
		return "", fmt.Errorf("session-handoff: config dir: %w", err)
	}

	sum := sha256.Sum256([]byte(cwd))
	name := hex.EncodeToString(sum[:])[:12] + ".jsonl"

	return filepath.Join(base, "memory", name), nil
}
