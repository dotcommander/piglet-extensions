// Package inbox provides a file-based message inbox for piglet.
// External processes drop JSON envelopes into a directory; the scanner
// picks them up and injects them into the agent loop.
package inbox

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Deliverer injects messages into the agent loop.
// *sdk.Extension satisfies this interface.
type Deliverer interface {
	SendMessage(content string)
	Steer(content string)
	Notify(msg string)
}

// Envelope is an inbound message from an external process.
type Envelope struct {
	Version int    `json:"version"`
	ID      string `json:"id"`
	Text    string `json:"text"`
	Mode    string `json:"mode,omitzero"`
	Created string `json:"created,omitzero"`
	TTL     int    `json:"ttl,omitzero"`
	Source  string `json:"source,omitzero"`
}

// Ack is written after processing an envelope.
type Ack struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason,omitzero"`
	Ts     string `json:"ts"`
}

// Heartbeat is written periodically to the registry.
type Heartbeat struct {
	PID       int    `json:"pid"`
	CWD       string `json:"cwd"`
	Started   string `json:"started"`
	Heartbeat string `json:"heartbeat"`
}

// Stats tracks delivery counts for the current session.
type Stats struct {
	Delivered  int       `json:"delivered"`
	Failed     int       `json:"failed"`
	Duplicates int       `json:"duplicates"`
	Expired    int       `json:"expired"`
	StartedAt  time.Time `json:"startedAt"`
}

const (
	DefaultScanInterval = 750 * time.Millisecond
	HeartbeatInterval   = 2 * time.Second
	MaxFileBytes        = 32 * 1024
	MaxTextRunes        = 8000
	DedupCap            = 1000
	AckMaxAge           = time.Hour
	PruneInterval       = time.Minute

	ModeQueue     = "queue"
	ModeInterrupt = "interrupt"
)

// Scanner watches the inbox directory and delivers messages.
type Scanner struct {
	inboxDir  string
	pid       int
	cwd       string
	deliverer Deliverer
	started   string

	mu        sync.Mutex
	stats     Stats
	seen      map[string]struct{}
	lastPrune time.Time

	cancel func()
	wg     sync.WaitGroup
}

// New creates a scanner. Call Start to begin scanning.
func New(inboxDir, cwd string, pid int, d Deliverer) *Scanner {
	return &Scanner{
		inboxDir:  inboxDir,
		pid:       pid,
		cwd:       cwd,
		deliverer: d,
		started:   time.Now().UTC().Format(time.RFC3339),
		stats:     Stats{StartedAt: time.Now()},
		seen:      make(map[string]struct{}, DedupCap),
	}
}

func (s *Scanner) processDir() string {
	return filepath.Join(s.inboxDir, strconv.Itoa(s.pid))
}

func (s *Scanner) acksDir() string {
	return filepath.Join(s.processDir(), "acks")
}

func (s *Scanner) registryDir() string {
	return filepath.Join(s.inboxDir, "registry")
}

// Start launches the scan and heartbeat goroutines.
func (s *Scanner) Start(ctx context.Context) {
	ctx, s.cancel = context.WithCancel(ctx)
	_ = os.MkdirAll(s.processDir(), 0750)
	_ = os.MkdirAll(s.acksDir(), 0750)
	_ = os.MkdirAll(s.registryDir(), 0750)

	s.wg.Add(2)
	go s.scanLoop(ctx)
	go s.heartbeatLoop(ctx)
}

// Stop cancels the scanner and waits for goroutines to finish.
func (s *Scanner) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	_ = os.Remove(filepath.Join(s.registryDir(), fmt.Sprintf("%d.json", s.pid)))
}

// Stats returns a copy of the current delivery statistics.
func (s *Scanner) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stats
}

func (s *Scanner) scanLoop(ctx context.Context) {
	defer s.wg.Done()
	ticker := time.NewTicker(DefaultScanInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.scan()
		}
	}
}

func (s *Scanner) heartbeatLoop(ctx context.Context) {
	defer s.wg.Done()
	s.writeHeartbeat()
	ticker := time.NewTicker(HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.writeHeartbeat()
		}
	}
}

// Validate checks an envelope for structural validity.
func Validate(env *Envelope) (bool, string) {
	if env.Version != 1 {
		return false, "unsupported_version"
	}
	if env.ID == "" {
		return false, "missing_id"
	}
	if env.Text == "" {
		return false, "missing_text"
	}
	if len([]rune(env.Text)) > MaxTextRunes {
		return false, "text_too_long"
	}
	switch env.Mode {
	case "", ModeQueue, ModeInterrupt:
		// valid
	default:
		return false, "invalid_mode"
	}
	return true, ""
}
