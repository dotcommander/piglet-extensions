package tasklist

import (
	"fmt"
	"time"
)

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
	if _, ok := s.data[id]; !ok {
		return nil, fmt.Errorf("tasklist: task %q not found", id)
	}

	now := time.Now().UTC()
	var changed []string

	// Build children index for O(N) traversal instead of scanning full map per node.
	children := make(map[string][]string)
	for tid, t := range s.data {
		if t.ParentID != "" {
			children[t.ParentID] = append(children[t.ParentID], tid)
		}
	}

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
		for _, childID := range children[taskID] {
			walk(childID)
		}
	}

	walk(id)

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

	// Build children index for O(N) traversal.
	children := make(map[string][]string)
	for tid, t := range s.data {
		if t.ParentID != "" {
			children[t.ParentID] = append(children[t.ParentID], tid)
		}
	}

	var walk func(taskID string)
	walk = func(taskID string) {
		for _, childID := range children[taskID] {
			walk(childID)
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
