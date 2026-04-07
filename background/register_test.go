package background

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		action string
		err    error
		want   string
	}{
		{
			name:   "strips host/runBackground prefix",
			action: "start background task",
			err:    errors.New("host/runBackground: connection refused"),
			want:   "Failed to start background task: connection refused",
		},
		{
			name:   "strips host/isBackgroundRunning prefix",
			action: "check background status",
			err:    errors.New("host/isBackgroundRunning: timeout"),
			want:   "Failed to check background status: timeout",
		},
		{
			name:   "strips host/cancelBackground prefix",
			action: "cancel background task",
			err:    errors.New("host/cancelBackground: not running"),
			want:   "Failed to cancel background task: not running",
		},
		{
			name:   "passes through non-host errors unchanged",
			action: "start background task",
			err:    errors.New("something else went wrong"),
			want:   "Failed to start background task: something else went wrong",
		},
		{
			name:   "handles nested host prefix in message",
			action: "check background status",
			err:    errors.New("host/isBackgroundRunning: extest: host/isBackgroundRunning not available"),
			want:   "Failed to check background status: extest: host/isBackgroundRunning not available",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := cleanError(tt.action, tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}
