package tasklist

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
)

// Status values.
const (
	StatusActive = "active"
	StatusDone   = "done"
)

// Group values.
const (
	GroupTodo    = "todo"
	GroupBacklog = "backlog"
)

// Task is a single tracked item with optional subtask hierarchy.
type Task struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Status       string    `json:"status"`
	Group        string    `json:"group"`
	ParentID     string    `json:"parent_id,omitempty"`
	Notes        string    `json:"notes,omitempty"`
	LinearTicket string    `json:"linear_ticket,omitempty"`
	GitHubPR     string    `json:"github_pr,omitempty"`
	Branch       string    `json:"branch,omitempty"`
	Links        []string  `json:"links,omitempty"`
}

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

// Add creates a new task. If parentID is non-empty, it is created as a subtask.
// group defaults to GroupTodo if empty.
func (s *Store) Add(title, group, parentID string) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if title == "" {
		return nil, fmt.Errorf("tasklist: title is required")
	}

	if group == "" {
		group = GroupTodo
	}

	now := time.Now().UTC()
	id := slugify(title)

	// Handle ID collisions.
	base := id
	for i := 2; s.data[id] != nil; i++ {
		id = fmt.Sprintf("%s-%d", base, i)
	}

	// Validate parent exists.
	if parentID != "" {
		if _, ok := s.data[parentID]; !ok {
			return nil, fmt.Errorf("tasklist: parent %q not found", parentID)
		}
	}

	t := &Task{
		ID:        id,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
		Status:    StatusActive,
		Group:     group,
		ParentID:  parentID,
	}

	s.data[id] = t
	if err := s.flush(); err != nil {
		return nil, err
	}

	return t, nil
}

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

	slices.SortFunc(out, func(a, b *Task) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})

	return out
}

// Update modifies a task's title and/or notes.
func (s *Store) Update(id, title, notes string) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.data[id]
	if !ok {
		return nil, fmt.Errorf("tasklist: task %q not found", id)
	}

	if title != "" {
		t.Title = title
	}
	if notes != "" {
		t.Notes = notes
	}
	t.UpdatedAt = time.Now().UTC()

	if err := s.flush(); err != nil {
		return nil, err
	}

	cp := *t
	return &cp, nil
}

// AppendNotes appends text to a task's notes.
func (s *Store) AppendNotes(id, text string) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.data[id]
	if !ok {
		return nil, fmt.Errorf("tasklist: task %q not found", id)
	}

	if t.Notes != "" {
		t.Notes += "\n\n" + text
	} else {
		t.Notes = text
	}
	t.UpdatedAt = time.Now().UTC()

	if err := s.flush(); err != nil {
		return nil, err
	}

	cp := *t
	return &cp, nil
}

// Done marks a task and all its descendants as done.
func (s *Store) Done(id string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.setStatusRecursive(id, StatusDone)
}

// Undone reactivates a task and all its descendants.
func (s *Store) Undone(id string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.setStatusRecursive(id, StatusActive)
}

func (s *Store) setStatusRecursive(id, status string) ([]string, error) {
	t, ok := s.data[id]
	if !ok {
		return nil, fmt.Errorf("tasklist: task %q not found", id)
	}

	now := time.Now().UTC()
	var changed []string

	var walk func(taskID string)
	walk = func(taskID string) {
		task := s.data[taskID]
		if task == nil {
			return
		}
		if task.Status != status {
			task.Status = status
			task.UpdatedAt = now
			changed = append(changed, taskID)
		}
		for _, t2 := range s.data {
			if t2.ParentID == taskID {
				walk(t2.ID)
			}
		}
	}

	walk(id)

	// If reactivating, move back to group todo unless it was backlog.
	if status == StatusActive && t.Group == GroupBacklog {
		t.Group = GroupBacklog // preserve backlog status
	}

	if err := s.flush(); err != nil {
		return nil, err
	}

	return changed, nil
}

// Delete removes a task and all its descendants.
func (s *Store) Delete(id string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data[id]; !ok {
		return nil, fmt.Errorf("tasklist: task %q not found", id)
	}

	var deleted []string

	var walk func(taskID string)
	walk = func(taskID string) {
		// Collect children first.
		for _, t := range s.data {
			if t.ParentID == taskID {
				walk(t.ID)
			}
		}
		delete(s.data, taskID)
		deleted = append(deleted, taskID)
	}

	walk(id)

	if err := s.flush(); err != nil {
		return nil, err
	}

	return deleted, nil
}

// Move changes a task's group or parent.
func (s *Store) Move(id, group, newParentID string) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.data[id]
	if !ok {
		return nil, fmt.Errorf("tasklist: task %q not found", id)
	}

	if group != "" {
		t.Group = group
		if group == GroupTodo || group == GroupBacklog {
			t.Status = StatusActive
		}
	}

	if newParentID != "" {
		if newParentID == id {
			return nil, fmt.Errorf("tasklist: cannot make task its own parent")
		}
		if _, ok := s.data[newParentID]; !ok {
			return nil, fmt.Errorf("tasklist: parent %q not found", newParentID)
		}
		// Check for cycles.
		if isDescendant(s.data, newParentID, id) {
			return nil, fmt.Errorf("tasklist: would create cycle")
		}
		t.ParentID = newParentID
	}

	t.UpdatedAt = time.Now().UTC()

	if err := s.flush(); err != nil {
		return nil, err
	}

	cp := *t
	return &cp, nil
}

// Link sets a link field on a task.
func (s *Store) Link(id, field, value string) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.data[id]
	if !ok {
		return nil, fmt.Errorf("tasklist: task %q not found", id)
	}

	switch field {
	case "linear_ticket":
		t.LinearTicket = value
	case "github_pr":
		t.GitHubPR = value
	case "branch":
		t.Branch = value
	case "url":
		if value != "" {
			t.Links = append(t.Links, value)
		}
	default:
		return nil, fmt.Errorf("tasklist: unknown link field %q (use: linear_ticket, github_pr, branch, url)", field)
	}

	t.UpdatedAt = time.Now().UTC()

	if err := s.flush(); err != nil {
		return nil, err
	}

	cp := *t
	return &cp, nil
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

	slices.SortFunc(out, func(a, b *Task) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})

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

	slices.SortFunc(out, func(a, b *Task) int {
		return b.UpdatedAt.Compare(a.UpdatedAt)
	})

	return out
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

// slugify converts a title to a URL-safe ID.
var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(title string) string {
	s := strings.ToLower(title)
	s = nonAlphaNum.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}

// isDescendant returns true if target is a descendant of ancestor.
func isDescendant(data map[string]*Task, target, ancestor string) bool {
	// Walk up from target's parent — target itself doesn't count.
	t, ok := data[target]
	if !ok {
		return false
	}
	cur := t.ParentID
	for cur != "" {
		if cur == ancestor {
			return true
		}
		t, ok = data[cur]
		if !ok {
			break
		}
		cur = t.ParentID
	}
	return false
}
