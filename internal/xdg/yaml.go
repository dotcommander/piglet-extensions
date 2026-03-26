package xdg

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadYAML reads a YAML config file from the piglet config directory.
// If the file is missing, it writes the marshaled defaults atomically and returns them.
func LoadYAML[T any](filename string, defaults T) T {
	dir, err := ConfigDir()
	if err != nil {
		return defaults
	}

	cfgPath := filepath.Join(dir, filename)
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		defaultData, _ := yaml.Marshal(defaults)
		_ = WriteFileAtomic(cfgPath, defaultData)
		return defaults
	}

	_ = yaml.Unmarshal(data, &defaults)
	return defaults
}
