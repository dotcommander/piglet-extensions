package tokengate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSkipSummarize_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		tool string
		want bool
	}{
		{"Read", true},
		{"read", true},
		{"Edit", true},
		{"Write", true},
		{"MultiEdit", true},
		{"dispatch", true},
		{"coordinate", true},
		{"Bash", false},
		{"Grep", false},
		{"Glob", false},
		{"Agent", false},
		{"", false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, skipSummarize(tt.tool), "skipSummarize(%q)", tt.tool)
	}
}
