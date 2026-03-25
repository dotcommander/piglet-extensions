package sessiontools

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	"gopkg.in/yaml.v3"
)

// Config holds session-handoff settings.
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
		SummaryMode:  "template",
		LLMTimeout:   30 * time.Second,
		LLMMaxTokens: 1024,
		MaxQuerySize: 1 << 20, // 1MB
	}
}

// LoadConfig reads config from ~/.config/piglet/session-handoff.yaml, creating defaults if missing.
func LoadConfig() Config {
	cfg := DefaultConfig()

	dir, err := xdg.ConfigDir()
	if err != nil {
		return cfg
	}

	cfgPath := filepath.Join(dir, "session-handoff.yaml")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			saveConfigDefaults(cfgPath, cfg)
		}
		return cfg
	}

	_ = yaml.Unmarshal(data, &cfg)
	return cfg
}

func saveConfigDefaults(path string, cfg Config) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return
	}
	atomicWrite(path, data)
}

// LoadPromptContent reads the handoff prompt section from ~/.config/piglet/session-handoff.md,
// creating a default if missing.
func LoadPromptContent() string {
	dir, err := xdg.ConfigDir()
	if err != nil {
		return defaultPromptContent()
	}

	promptPath := filepath.Join(dir, "session-handoff.md")
	data, err := os.ReadFile(promptPath)
	if err != nil {
		if os.IsNotExist(err) {
			content := defaultPromptContent()
			writePromptDefault(promptPath, content)
			return content
		}
		return defaultPromptContent()
	}

	return string(data)
}

func defaultPromptContent() string {
	return "Use /handoff to transfer context to a new session with a structured summary of goal, progress, decisions, and next steps. " +
		"Use the session_query tool to search a parent session's content by keyword when you need to recover specific details after a handoff."
}

func writePromptDefault(path, content string) {
	atomicWrite(path, []byte(content+"\n"))
}

func atomicWrite(path string, data []byte) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
	}
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
