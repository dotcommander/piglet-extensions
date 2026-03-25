package xdg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigDir_XDGOverride(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/config")

	dir, err := ConfigDir()
	require.NoError(t, err)
	assert.Equal(t, "/custom/config/piglet", dir)
}

func TestConfigDir_DefaultsToHomeConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	dir, err := ConfigDir()
	require.NoError(t, err)

	home, _ := os.UserHomeDir()
	assert.Equal(t, filepath.Join(home, ".config", "piglet"), dir)
}
