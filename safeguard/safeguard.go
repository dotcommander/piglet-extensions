// Package safeguard blocks dangerous commands before execution.
// Configuration loaded from ~/.config/piglet/safeguard.yaml.
// Supports three profiles: strict (block + workspace scoping), balanced (block only), off (log only).
package safeguard

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	"gopkg.in/yaml.v3"
)

// resolveConfigPath returns the path to safeguard.yaml in the namespaced
// extension directory, or "safeguard.yaml" as fallback.
func resolveConfigPath() string {
	dir, err := xdg.ExtensionDir("safeguard")
	if err != nil {
		return "safeguard.yaml"
	}
	return filepath.Join(dir, "safeguard.yaml")
}

// CompilePatterns compiles string patterns into case-insensitive regexps.
// Returns an error describing the first invalid pattern.
func CompilePatterns(patterns []string) ([]*regexp.Regexp, error) {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile("(?i)" + p)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", p, err)
		}
		compiled = append(compiled, re)
	}
	return compiled, nil
}

// BlockerWithConfig returns a Before interceptor that enforces the given profile.
// In strict mode, write/edit/bash calls targeting paths outside cwd are blocked.
// Blocked decisions are logged to the audit logger when provided.
func BlockerWithConfig(cfg Config, compiled []*regexp.Regexp, cwd string, audit *AuditLogger) func(ctx context.Context, toolName string, args map[string]any) (bool, map[string]any, error) {
	return func(ctx context.Context, toolName string, args map[string]any) (bool, map[string]any, error) {
		// Strict mode: workspace scoping for file-mutating tools
		if cfg.Profile == ProfileStrict && cwd != "" {
			switch toolName {
			case "write", "edit", "multi_edit":
				path, _ := args["file_path"].(string)
				if path != "" && !isInsideWorkspace(path, cwd) {
					audit.Log(toolName, "blocked", "outside workspace", path)
					return false, nil, fmt.Errorf("safeguard [strict]: blocked %s outside workspace %s", toolName, cwd)
				}
			}
		}

		// Pattern matching applies to bash in both strict and balanced
		if toolName == "bash" {
			command, _ := args["command"].(string)
			if command != "" {
				// Fast path: skip security checks for known read-only commands.
				if ClassifyCommand(command) == CommandReadOnly {
					audit.Log(toolName, "allowed", "read_only", truncate(command, 200))
					return true, args, nil
				}

				// Metacharacter injection checks (parser-level attacks).
				if err := ValidateInjection(command); err != nil {
					audit.Log(toolName, "blocked", err.Error(), truncate(command, 200))
					return false, nil, fmt.Errorf("safeguard: %v", err)
				}

				for _, re := range compiled {
					if re.MatchString(command) {
						audit.Log(toolName, "blocked", re.String(), truncate(command, 200))
						return false, nil, fmt.Errorf("safeguard: blocked dangerous command matching %q — edit %s to adjust", re.String(), resolveConfigPath())
					}
				}
			}
		}

		return true, args, nil
	}
}

// isInsideWorkspace checks if path is under the workspace directory.
func isInsideWorkspace(path, cwd string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absCwd, err := filepath.Abs(cwd)
	if err != nil {
		return false
	}
	// Ensure trailing separator for prefix check
	if !strings.HasSuffix(absCwd, string(filepath.Separator)) {
		absCwd += string(filepath.Separator)
	}
	return absPath == absCwd[:len(absCwd)-1] || strings.HasPrefix(absPath, absCwd)
}

// truncate returns the first n runes of s.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) > n {
		return string(runes[:n]) + "..."
	}
	return s
}

const (
	ProfileStrict   = "strict"
	ProfileBalanced = "balanced"
	ProfileOff      = "off"
)

// Config holds safeguard configuration.
type Config struct {
	Profile  string   `yaml:"profile"`
	Patterns []string `yaml:"patterns"`
}

// AuditLogger writes tool decisions to a JSONL file.
type AuditLogger struct {
	mu   sync.Mutex
	file *os.File
}

type auditEntry struct {
	Timestamp string `json:"ts"`
	Tool      string `json:"tool"`
	Decision  string `json:"decision"`
	Reason    string `json:"reason,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// NewAuditLogger opens or creates the audit log file in the extension directory.
// Returns nil with a warning to stderr if the audit log cannot be created.
func NewAuditLogger() *AuditLogger {
	dir, err := xdg.ExtensionDir("safeguard")
	if err != nil {
		fmt.Fprintf(os.Stderr, "[safeguard] audit: cannot resolve config dir: %v\n", err)
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "[safeguard] audit: cannot create dir %s: %v\n", dir, err)
		return nil
	}
	path := filepath.Join(dir, "safeguard-audit.jsonl")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[safeguard] audit: cannot open %s: %v\n", path, err)
		return nil
	}
	return &AuditLogger{file: f}
}

// Log writes an audit entry. Safe for concurrent use.
func (a *AuditLogger) Log(tool, decision, reason, detail string) {
	if a == nil || a.file == nil {
		return
	}
	entry := auditEntry{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Tool:      tool,
		Decision:  decision,
		Reason:    reason,
		Detail:    detail,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return
	}
	data = append(data, '\n')
	a.mu.Lock()
	_, _ = a.file.Write(data)
	a.mu.Unlock()
}

// Close closes the audit log file.
func (a *AuditLogger) Close() error {
	if a == nil || a.file == nil {
		return nil
	}
	return a.file.Close()
}

// LoadConfig reads the full safeguard configuration.
// Tries the namespaced extension directory first, falls back to flat config dir.
func LoadConfig() Config {
	dir, err := xdg.ExtensionDir("safeguard")
	if err != nil {
		return defaultConfig()
	}
	data, err := readConfigFile(dir)
	if err != nil {
		return defaultConfig()
	}
	return parseConfig(data)
}

func defaultConfig() Config {
	return Config{Profile: ProfileBalanced, Patterns: defaultPatterns()}
}

func parseConfig(data []byte) Config {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return defaultConfig()
	}
	if cfg.Profile == "" {
		cfg.Profile = ProfileBalanced
	}
	return cfg
}

// readConfigFile reads safeguard.yaml from the namespaced directory,
// falling back to the flat config dir (with migration) or creating defaults.
func readConfigFile(dir string) ([]byte, error) {
	path := filepath.Join(dir, "safeguard.yaml")
	data, err := os.ReadFile(path)
	if err == nil {
		return data, nil
	}
	if !os.IsNotExist(err) {
		return nil, err
	}

	// Fallback: try flat location
	flatDir, flatErr := xdg.ConfigDir()
	if flatErr != nil {
		return createDefaultConfig(path, dir), nil
	}
	flatPath := filepath.Join(flatDir, "safeguard.yaml")
	data, err = os.ReadFile(flatPath)
	if err != nil {
		return createDefaultConfig(path, dir), nil
	}

	// Migrate from flat to namespaced
	_ = xdg.WriteFileAtomic(path, data)
	return data, nil
}

// createDefaultConfig generates a default config file and returns its content.
func createDefaultConfig(path, dir string) []byte {
	// Try seed files first
	seedPaths := []string{
		filepath.Join(dir, "safeguard-default.yaml"),
	}
	if flatDir, err := xdg.ConfigDir(); err == nil {
		seedPaths = append(seedPaths, filepath.Join(flatDir, "safeguard-default.yaml"))
	}
	for _, seedPath := range seedPaths {
		data, err := os.ReadFile(seedPath)
		if err == nil {
			_ = xdg.WriteFileAtomic(path, data)
			return data
		}
	}

	// No seed file — write minimal default
	cfg := defaultConfig()
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return data
	}
	_ = xdg.WriteFileAtomic(path, data)
	return data
}
