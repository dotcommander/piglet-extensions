package inbox

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
)

// scan performs one pass over the inbox directory.
func (s *Scanner) scan() {
	dir := s.processDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	type fileEntry struct {
		name    string
		modTime time.Time
	}
	var files []fileEntry
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		if strings.Contains(name, "..") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileEntry{name: name, modTime: info.ModTime()})
	}

	slices.SortFunc(files, func(a, b fileEntry) int {
		return a.modTime.Compare(b.modTime)
	})

	for _, f := range files {
		s.processFile(filepath.Join(dir, f.name))
	}

	s.pruneAcks()
}

func (s *Scanner) processFile(path string) {
	info, err := os.Lstat(path)
	if err != nil {
		return
	}
	if info.Mode()&os.ModeSymlink != 0 {
		_ = os.Remove(path)
		return
	}
	if info.Size() > MaxFileBytes {
		s.ackAndRemove(path, "", "failed", "file_too_large")
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		s.ackAndRemove(path, "", "failed", "invalid_json")
		return
	}

	ok, reason := Validate(&env)
	if !ok {
		s.ackAndRemove(path, env.ID, "failed", reason)
		return
	}

	if s.isExpired(&env, info.ModTime()) {
		s.mu.Lock()
		s.stats.Expired++
		s.mu.Unlock()
		s.ackAndRemove(path, env.ID, "failed", "expired")
		return
	}

	if s.isDuplicate(env.ID) {
		s.mu.Lock()
		s.stats.Duplicates++
		s.mu.Unlock()
		s.ackAndRemove(path, env.ID, "duplicate", "")
		return
	}

	s.deliver(path, &env)
}

// isExpired checks whether the envelope's TTL has elapsed.
func (s *Scanner) isExpired(env *Envelope, fallback time.Time) bool {
	if env.TTL <= 0 {
		return false
	}
	created := fallback
	if env.Created != "" {
		if t, err := time.Parse(time.RFC3339, env.Created); err == nil {
			created = t
		}
	}
	return time.Now().After(created.Add(time.Duration(env.TTL) * time.Second))
}

// deliver sends the envelope content and records the successful delivery.
func (s *Scanner) deliver(path string, env *Envelope) {
	mode := env.Mode
	if mode == "" {
		mode = ModeQueue
	}
	source := env.Source
	if source == "" {
		source = "unknown"
	}

	switch mode {
	case ModeQueue:
		s.deliverer.SendMessage(env.Text)
	case ModeInterrupt:
		s.deliverer.Steer(env.Text)
	}

	s.deliverer.Notify(fmt.Sprintf("Inbox: message from %s", source))

	s.mu.Lock()
	s.stats.Delivered++
	if len(s.seen) >= DedupCap {
		clear(s.seen)
	}
	s.seen[env.ID] = struct{}{}
	s.mu.Unlock()

	s.writeAck(env.ID, "delivered", "")
	_ = os.Remove(path)
}

func (s *Scanner) isDuplicate(id string) bool {
	s.mu.Lock()
	_, inMem := s.seen[id]
	s.mu.Unlock()
	if inMem {
		return true
	}
	ackPath := filepath.Join(s.acksDir(), id+".json")
	_, err := os.Stat(ackPath)
	return err == nil
}

func (s *Scanner) ackAndRemove(path, id, status, reason string) {
	if id != "" {
		s.writeAck(id, status, reason)
	}
	if status == "failed" {
		s.mu.Lock()
		s.stats.Failed++
		s.mu.Unlock()
	}
	_ = os.Remove(path)
}

func (s *Scanner) writeAck(id, status, reason string) {
	ack := Ack{
		ID:     id,
		Status: status,
		Reason: reason,
		Ts:     time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(ack)
	if err != nil {
		return
	}
	_ = xdg.WriteFileAtomic(filepath.Join(s.acksDir(), id+".json"), data)
}

func (s *Scanner) writeHeartbeat() {
	hb := Heartbeat{
		PID:       s.pid,
		CWD:       s.cwd,
		Started:   s.started,
		Heartbeat: time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.Marshal(hb)
	if err != nil {
		return
	}
	_ = xdg.WriteFileAtomic(filepath.Join(s.registryDir(), fmt.Sprintf("%d.json", s.pid)), data)
}

func (s *Scanner) pruneAcks() {
	if time.Since(s.lastPrune) < PruneInterval {
		return
	}
	s.lastPrune = time.Now()
	dir := s.acksDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	now := time.Now()
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if now.Sub(info.ModTime()) > AckMaxAge {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
