package changelog

import (
	"sort"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
)

type TypeConfig struct {
	Label string `yaml:"label"`
	Emoji string `yaml:"emoji"`
	Order int    `yaml:"order"`
}

type Config struct {
	Types         map[string]TypeConfig `yaml:"types"`
	FallbackCount int                   `yaml:"fallback_count"`
}

func defaultConfig() Config {
	return Config{
		Types: map[string]TypeConfig{
			"feat":     {Label: "Features", Emoji: "\u2728", Order: 1},
			"fix":      {Label: "Bug Fixes", Emoji: "\U0001F41B", Order: 2},
			"perf":     {Label: "Performance", Emoji: "\u26A1", Order: 3},
			"refactor": {Label: "Refactoring", Emoji: "\u267B\uFE0F", Order: 4},
			"docs":     {Label: "Documentation", Emoji: "\U0001F4DA", Order: 5},
			"test":     {Label: "Tests", Emoji: "\u2705", Order: 6},
			"build":    {Label: "Build", Emoji: "\U0001F4E6", Order: 7},
			"ci":       {Label: "CI", Emoji: "\U0001F527", Order: 8},
			"chore":    {Label: "Chores", Emoji: "\U0001F9F9", Order: 9},
			"style":    {Label: "Style", Emoji: "\U0001F3A8", Order: 10},
			"other":    {Label: "Other", Emoji: "\U0001F4DD", Order: 99},
		},
		FallbackCount: 20,
	}
}

func loadConfig() Config {
	return xdg.LoadYAMLExt("changelog", "changelog.yaml", defaultConfig())
}

func typeOrder(types map[string]TypeConfig) []string {
	keys := make([]string, 0, len(types))
	for k := range types {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return types[keys[i]].Order < types[keys[j]].Order
	})
	return keys
}
