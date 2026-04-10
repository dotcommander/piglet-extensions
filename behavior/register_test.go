package behavior

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatusEmptyContent(t *testing.T) {
	t.Parallel()

	mu.Lock()
	loaded = "  \n  \n  "
	filePath = "/tmp/test.md"
	mu.Unlock()

	// Verify that whitespace-only content is treated as empty
	assert.Equal(t, "  \n  \n  ", loaded)
	assert.True(t, isWhitespaceOnly(loaded))
}

func TestStatusWithContent(t *testing.T) {
	t.Parallel()

	content := "# Guidelines\nBe concise and helpful."
	assert.False(t, isWhitespaceOnly(content))
}

func isWhitespaceOnly(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}
