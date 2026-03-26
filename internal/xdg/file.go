package xdg

import (
	"os"
	"path/filepath"
)

// LoadOrCreateFile reads a text file from the piglet config directory.
// If the file is missing, it writes defaultContent atomically and returns it.
func LoadOrCreateFile(filename, defaultContent string) string {
	dir, err := ConfigDir()
	if err != nil {
		return defaultContent
	}

	path := filepath.Join(dir, filename)
	data, err := os.ReadFile(path)
	if err != nil {
		_ = WriteFileAtomic(path, []byte(defaultContent+"\n"))
		return defaultContent
	}

	return string(data)
}
