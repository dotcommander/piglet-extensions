// Package mcp provides MCP (Model Context Protocol) server integration for piglet.
// Connects to configured MCP servers and exposes their tools as piglet tools.
package mcp

import (
	"os"
	"regexp"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
)

// ServerConfig describes a single MCP server connection.
type ServerConfig struct {
	Type    string            `yaml:"type,omitempty"`    // "stdio" (default) or "http"
	Command string            `yaml:"command,omitempty"` // stdio: executable name
	Args    []string          `yaml:"args,omitempty"`    // stdio: arguments
	Env     map[string]string `yaml:"env,omitempty"`     // stdio: environment variables
	URL     string            `yaml:"url,omitempty"`     // http: endpoint URL
	Headers map[string]string `yaml:"headers,omitempty"` // http: custom headers
}

// Config is the top-level MCP configuration.
type Config struct {
	Servers map[string]ServerConfig `yaml:"servers"`
}

// LoadConfig reads MCP server configuration from the namespaced extension
// directory (~/.config/piglet/extensions/mcp/mcp.yaml), falling back to the
// flat location (~/.config/piglet/mcp.yaml) for backward compatibility.
// Returns empty Config if the file doesn't exist or is unparseable.
func LoadConfig() *Config {
	cfg := xdg.LoadYAMLExt("mcp", "mcp.yaml", Config{})
	return &cfg
}

var envVarRe = regexp.MustCompile(`\$\{([^}]+)\}`)

// expandEnvStr expands ${VAR} references from the OS environment.
func expandEnvStr(s string) string {
	return envVarRe.ReplaceAllStringFunc(s, func(match string) string {
		name := envVarRe.FindStringSubmatch(match)[1]
		return os.Getenv(name)
	})
}

// ExpandEnv converts env vars to KEY=VALUE format, expanding ${VAR} from OS env.
// Appended to the current process environment.
func ExpandEnv(env map[string]string) []string {
	base := os.Environ()
	for k, v := range env {
		base = append(base, k+"="+expandEnvStr(v))
	}
	return base
}

// ExpandHeaders expands ${VAR} in header values.
func ExpandHeaders(headers map[string]string) map[string]string {
	out := make(map[string]string, len(headers))
	for k, v := range headers {
		out[k] = expandEnvStr(v)
	}
	return out
}
