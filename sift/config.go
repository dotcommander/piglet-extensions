package sift

import (
	"github.com/dotcommander/piglet-extensions/internal/xdg"
)

type CompressionConfig struct {
	CollapseBlankLines      int    `yaml:"collapse_blank_lines"`
	CollapseRepeatedLines   int    `yaml:"collapse_repeated_lines"`
	StripTrailingWhitespace bool   `yaml:"strip_trailing_whitespace"`
	TruncationMarker        string `yaml:"truncation_marker"`
}

type Config struct {
	Enabled       bool              `yaml:"enabled"`
	SizeThreshold int               `yaml:"size_threshold"`
	MaxSize       int               `yaml:"max_size"`
	Tools         []string          `yaml:"tools"`
	Compression   CompressionConfig `yaml:"compression"`
}

func DefaultConfig() Config {
	return Config{
		Enabled:       true,
		SizeThreshold: 4096,
		MaxSize:       32768,
		Tools:         []string{"Read", "Grep", "Bash"},
		Compression: CompressionConfig{
			CollapseBlankLines:      3,
			CollapseRepeatedLines:   5,
			StripTrailingWhitespace: true,
			TruncationMarker:        "\n[SIFT: truncated — {kept}/{total} bytes shown]",
		},
	}
}

func LoadConfig() Config {
	return xdg.LoadYAML("sift.yaml", DefaultConfig())
}
