package tasklist

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
)

// Store holds tasks in memory backed by a per-project JSON file.
type Store struct {
	mu   sync.RWMutex
	path string
	data map[string]*Task
}

// NewStore creates a Store whose file path is derived from the sha256 of cwd.
func NewStore(cwd string) (*Store, error) {
	path, err := storePath(cwd)
	if err != nil {
		return nil, fmt.Errorf("tasklist: resolve path: %w", err)
	}

	s := &Store{
		path: path,
		data: make(map[string]*Task),
	}

	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

// Path returns the backing file path.
func (s *Store) Path() string { return s.path }

// Get retrieves a task by ID.
func (s *Store) Get(id string) (*Task, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.data[id]
	if !ok {
		return nil, false
	}
	cp := *t
	return &cp, true
}

// Resolve finds a task by exact ID, unique prefix, or unique suffix.
func (s *Store) Resolve(query string) (*Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.resolveLocked(query)
}

func (s *Store) resolveLocked(query string) (*Task, error) {
	// Exact match.
	if t, ok := s.data[query]; ok {
		cp := *t
		return &cp, nil
	}

	// Prefix match.
	var matches []*Task
	for _, t := range s.data {
		if strings.HasPrefix(t.ID, query) {
			matches = append(matches, t)
		}
	}

	if len(matches) == 1 {
		cp := *matches[0]
		return &cp, nil
	}

	// Suffix match.
	var suffixMatches []*Task
	for _, t := range s.data {
		if strings.HasSuffix(t.ID, query) {
			suffixMatches = append(suffixMatches, t)
		}
	}

	if len(suffixMatches) == 1 {
		cp := *suffixMatches[0]
		return &cp, nil
	}

	if len(matches) > 1 || len(suffixMatches) > 1 {
		return nil, fmt.Errorf("tasklist: %q matches multiple tasks", query)
	}

	return nil, fmt.Errorf("tasklist: task %q not found", query)
}

// List returns tasks matching the given filters. Empty string means "all".
func (s *Store) List(status, group, parentID string) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []*Task
	for _, t := range s.data {
		if status != "" && t.Status != status {
			continue
		}
		if group != "" && t.Group != group {
			continue
		}
		if parentID != "!" && t.ParentID != parentID {
			// "!" means root tasks only (parentID == "").
			continue
		}
		if parentID == "!" && t.ParentID != "" {
			continue
		}
		cp := *t
		out = append(out, &cp)
	}

	slices.SortFunc(out, func(a, b *Task) int {
		// Active first, then by updated_at descending.
		if a.Status != b.Status {
			if a.Status == StatusActive {
				return -1
			}
			return 1
		}
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})

	return out
}

// Children returns direct children of the given parent ID.
func (s *Store) Children(parentID string) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []*Task
	for _, t := range s.data {
		if t.ParentID == parentID {
			cp := *t
			out = append(out, &cp)
		}
	}

	slices.SortFunc(out, byUpdatedDesc)

	return out
}

// Search returns tasks whose title or notes contain the query (case-insensitive).
func (s *Store) Search(query string) []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lower := strings.ToLower(query)
	var out []*Task
	for _, t := range s.data {
		if strings.Contains(strings.ToLower(t.Title), lower) ||
			strings.Contains(strings.ToLower(t.Notes), lower) {
			cp := *t
			out = append(out, &cp)
		}
	}

	slices.SortFunc(out, byUpdatedDesc)

	return out
}

// Stats returns counts for the prompt section.
func (s *Store) Stats() (active, done, backlog int) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, t := range s.data {
		switch {
		case t.Status == StatusDone:
			done++
		case t.Group == GroupBacklog:
			backlog++
		default:
			active++
		}
	}
	return
}

// ActiveTasks returns root-level active todo tasks for the prompt section.
func (s *Store) ActiveTasks() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []*Task
	for _, t := range s.data {
		if t.Status == StatusActive && t.Group == GroupTodo && t.ParentID == "" {
			cp := *t
			out = append(out, &cp)
		}
	}

	slices.SortFunc(out, byUpdatedDesc)

	return out
}

// byUpdatedDesc sorts tasks by UpdatedAt descending.
func byUpdatedDesc(a, b *Task) int {
	return b.UpdatedAt.Compare(a.UpdatedAt)
}

// storePath returns the JSON file path for the given cwd.
func storePath(cwd string) (string, error) {
	base, err := xdg.ConfigDir()
	if err != nil {
		return "", fmt.Errorf("tasklist: user config dir: %w", err)
	}

	sum := sha256.Sum256([]byte(cwd))
	name := hex.EncodeToString(sum[:])[:12] + ".json"

	return filepath.Join(base, "extensions", "tasklist", name), nil
}

// load reads the JSON file into s.data. Missing file is not an error.
func (s *Store) load() error {
	raw, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("tasklist: read: %w", err)
	}

	if len(raw) == 0 {
		return nil
	}

	var tasks []*Task
	if err := json.Unmarshal(raw, &tasks); err != nil {
		return fmt.Errorf("tasklist: parse: %w", err)
	}

	for _, t := range tasks {
		s.data[t.ID] = t
	}

	return nil
}

// flush serialises s.data to JSON and writes atomically. Caller must hold s.mu.
func (s *Store) flush() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("tasklist: mkdir: %w", err)
	}

	tasks := make([]*Task, 0, len(s.data))
	for _, t := range s.data {
		tasks = append(tasks, t)
	}

	slices.SortFunc(tasks, func(a, b *Task) int {
		return strings.Compare(a.ID, b.ID)
	})

	raw, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return fmt.Errorf("tasklist: marshal: %w", err)
	}

	return xdg.WriteFileAtomic(s.path, append(raw, '\n'))
}
