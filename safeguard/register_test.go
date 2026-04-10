package safeguard_test

import (
	"context"
	"testing"

	"github.com/dotcommander/piglet-extensions/safeguard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRegister_BlockerIntegration tests the full blocker pipeline that
// Register wires together: LoadConfig → CompilePatterns → BlockerWithConfig.
// The SDK doesn't expose internal interceptor fields, so we test the
// component integration directly.
func TestRegister_BlockerIntegration(t *testing.T) {
	t.Parallel()

	cfg := safeguard.LoadConfig()
	compiled, err := safeguard.CompilePatterns(cfg.Patterns)
	require.NoError(t, err)
	blocker := safeguard.BlockerWithConfig(cfg, compiled, "/tmp", nil)

	t.Run("blocks dangerous command", func(t *testing.T) {
		t.Parallel()
		allow, _, err := blocker(context.Background(), "bash", map[string]any{"command": "rm -rf /"})
		assert.False(t, allow)
		assert.Error(t, err)
	})

	t.Run("allows safe command", func(t *testing.T) {
		t.Parallel()
		allow, args, err := blocker(context.Background(), "bash", map[string]any{"command": "ls -la"})
		assert.True(t, allow)
		assert.NoError(t, err)
		assert.NotNil(t, args)
	})

	t.Run("allows read-only classified command", func(t *testing.T) {
		t.Parallel()
		allow, args, err := blocker(context.Background(), "bash", map[string]any{"command": "cat file.txt"})
		assert.True(t, allow)
		assert.NoError(t, err)
		assert.NotNil(t, args)
	})
}

// TestRegister_OffProfileNoBlocker verifies that when ProfileOff is set,
// BlockerWithConfig is not called and the blocker function is never created.
// The Register interceptor's Before hook falls through to allow-all when
// the atomic.Pointer is nil. We verify the integration by confirming that
// a blocker built with ProfileOff still allows all tool calls (since the
// pattern list is empty and workspace scoping is disabled).
func TestRegister_OffProfileNoBlocker(t *testing.T) {
	t.Parallel()

	// ProfileOff: no patterns, no cwd scoping.
	blocker := safeguard.BlockerWithConfig(safeguard.Config{Profile: safeguard.ProfileOff}, nil, "", nil)

	allow, _, err := blocker(context.Background(), "bash", map[string]any{"command": "rm -rf /"})
	assert.True(t, allow)
	assert.NoError(t, err)
}
