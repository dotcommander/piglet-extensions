package loop

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testMinInterval = 10 * time.Millisecond

func newTestScheduler() *Scheduler {
	return &Scheduler{MinInterval: testMinInterval}
}

func TestStartValid(t *testing.T) {
	t.Parallel()

	s := newTestScheduler()
	ticked := make(chan int, 1)

	err := s.Start(50*time.Millisecond, "hello", func(iter int, _ string) {
		ticked <- iter
	})
	require.NoError(t, err)
	defer s.Stop()

	select {
	case n := <-ticked:
		assert.Equal(t, 1, n)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("onTick not called within timeout")
	}
}

func TestStartTooShortInterval(t *testing.T) {
	t.Parallel()

	s := newTestScheduler()
	err := s.Start(1*time.Millisecond, "hello", func(_ int, _ string) {})
	assert.ErrorContains(t, err, "interval must be at least")
}

func TestStartWhileAlreadyRunning(t *testing.T) {
	t.Parallel()

	s := newTestScheduler()
	require.NoError(t, s.Start(50*time.Millisecond, "first", func(_ int, _ string) {}))
	defer s.Stop()

	err := s.Start(50*time.Millisecond, "second", func(_ int, _ string) {})
	assert.ErrorContains(t, err, "loop already running")
}

func TestStopWhenRunning(t *testing.T) {
	t.Parallel()

	s := newTestScheduler()
	started := make(chan struct{})

	require.NoError(t, s.Start(50*time.Millisecond, "test", func(_ int, _ string) {
		select {
		case started <- struct{}{}:
		default:
		}
	}))

	// Wait for at least one tick so the goroutine is definitely live.
	select {
	case <-started:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("loop never started")
	}

	stopped := s.Stop()
	assert.True(t, stopped)

	running, _, _, _ := s.Status()
	assert.False(t, running)
}

func TestStopWhenNotRunning(t *testing.T) {
	t.Parallel()

	s := newTestScheduler()
	stopped := s.Stop()
	assert.False(t, stopped)
}

func TestStatusReflectsState(t *testing.T) {
	t.Parallel()

	s := newTestScheduler()

	running, interval, prompt, iters := s.Status()
	assert.False(t, running)
	assert.Zero(t, interval)
	assert.Empty(t, prompt)
	assert.Zero(t, iters)

	require.NoError(t, s.Start(50*time.Millisecond, "my prompt", func(_ int, _ string) {}))
	defer s.Stop()

	running, interval, prompt, _ = s.Status()
	assert.True(t, running)
	assert.Equal(t, 50*time.Millisecond, interval)
	assert.Equal(t, "my prompt", prompt)
}

func TestOnTickCalledMultipleTimes(t *testing.T) {
	t.Parallel()

	s := newTestScheduler()
	var count atomic.Int32
	done := make(chan struct{})

	require.NoError(t, s.Start(20*time.Millisecond, "ping", func(iter int, _ string) {
		n := count.Add(1)
		if n >= 3 {
			select {
			case done <- struct{}{}:
			default:
			}
		}
	}))
	defer s.Stop()

	select {
	case <-done:
		assert.GreaterOrEqual(t, int(count.Load()), 3)
	case <-time.After(2 * time.Second):
		t.Fatal("did not tick 3 times within timeout")
	}
}

func TestStopPreventsAdditionalTicks(t *testing.T) {
	t.Parallel()

	s := newTestScheduler()
	var count atomic.Int32

	require.NoError(t, s.Start(20*time.Millisecond, "ping", func(_ int, _ string) {
		count.Add(1)
	}))

	// Let it run briefly, then stop.
	time.Sleep(15 * time.Millisecond)
	s.Stop()

	snapshot := count.Load()
	// Sleep past another interval — count must not grow.
	time.Sleep(60 * time.Millisecond)
	assert.Equal(t, snapshot, count.Load())
}

func TestRestartAfterStop(t *testing.T) {
	t.Parallel()

	s := newTestScheduler()
	ticked := make(chan struct{}, 1)

	require.NoError(t, s.Start(50*time.Millisecond, "first", func(_ int, _ string) {}))
	s.Stop()

	require.NoError(t, s.Start(50*time.Millisecond, "second", func(_ int, _ string) {
		select {
		case ticked <- struct{}{}:
		default:
		}
	}))
	defer s.Stop()

	select {
	case <-ticked:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("loop did not restart")
	}
}
