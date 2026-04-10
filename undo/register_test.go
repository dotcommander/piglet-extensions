package undo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0B"},
		{1, "1B"},
		{100, "100B"},
		{1023, "1023B"},
		{1024, "1.0K"},
		{1536, "1.5K"},
		{1048576, "1.0M"},
		{1572864, "1.5M"},
		{1073741824, "1.0G"},
		{1610612736, "1.5G"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.want, formatSize(tt.bytes), "formatSize(%d)", tt.bytes)
	}
}
