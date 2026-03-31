package tokengate

import (
	"github.com/dotcommander/piglet-extensions/internal/xdg"
)

type RewriteRule struct {
	Tool    string `yaml:"tool"`
	Pattern string `yaml:"pattern"`
	Action  string `yaml:"action"`
	Value   string `yaml:"value"`
}

type Config struct {
	Enabled bool          `yaml:"enabled"`
	Rules   []RewriteRule `yaml:"rules"`
}

func DefaultConfig() Config {
	return Config{
		Enabled: true,
		Rules: []RewriteRule{
			{Tool: "Bash", Pattern: `grep\s+-r\s+.*\.\*`, Action: "append_head", Value: "100"},
			{Tool: "Bash", Pattern: `find\s+/`, Action: "append_head", Value: "50"},
			{Tool: "Read", Pattern: "", Action: "limit_lines", Value: "200"},
		},
	}
}

func LoadConfig() Config {
	return xdg.LoadYAMLExt("tokengate", "tokengate.yaml", DefaultConfig())
}
