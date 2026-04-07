package tasklist

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func tempStore(t *testing.T) *Store {
	t.Helper()

	dir := t.TempDir()
	cwd := filepath.Join(dir, "project")
	require.NoError(t, os.MkdirAll(cwd, 0700))

	s, err := NewStore(cwd)
	require.NoError(t, err)
	return s
}

func TestAdd(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	task, err := s.Add("Fix the login bug", GroupTodo, "")
	require.NoError(t, err)
	assert.Equal(t, "fix-the-login-bug", task.ID)
	assert.Equal(t, StatusActive, task.Status)
	assert.Equal(t, GroupTodo, task.Group)
	assert.Equal(t, "", task.ParentID)
}

func TestAddCollision(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	_, err := s.Add("Fix the login bug", GroupTodo, "")
	require.NoError(t, err)

	task2, err := s.Add("Fix the login bug", GroupTodo, "")
	require.NoError(t, err)
	assert.Equal(t, "fix-the-login-bug-2", task2.ID)
}

func TestAddSubtask(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	parent, err := s.Add("Parent task", GroupTodo, "")
	require.NoError(t, err)

	child, err := s.Add("Child task", GroupTodo, parent.ID)
	require.NoError(t, err)
	assert.Equal(t, parent.ID, child.ParentID)
}

func TestAddInvalidParent(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	_, err := s.Add("Orphan child", GroupTodo, "nonexistent")
	assert.Error(t, err)
}

func TestGet(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	created, err := s.Add("Test task", GroupTodo, "")
	require.NoError(t, err)

	got, ok := s.Get(created.ID)
	assert.True(t, ok)
	assert.Equal(t, created.Title, got.Title)
}

func TestResolve(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	_, err := s.Add("Fix authentication flow", GroupTodo, "")
	require.NoError(t, err)

	// Exact.
	_, err = s.Resolve("fix-authentication-flow")
	assert.NoError(t, err)

	// Prefix.
	_, err = s.Resolve("fix-auth")
	assert.NoError(t, err)

	// Suffix.
	_, err = s.Resolve("flow")
	assert.NoError(t, err)

	// Not found.
	_, err = s.Resolve("nonexistent")
	assert.Error(t, err)
}

func TestResolveAmbiguous(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	_, err := s.Add("Fix auth login", GroupTodo, "")
	require.NoError(t, err)
	_, err = s.Add("Fix auth logout", GroupTodo, "")
	require.NoError(t, err)

	_, err = s.Resolve("fix-auth")
	assert.Error(t, err) // ambiguous prefix
}

func TestList(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	s.Add("Active todo", GroupTodo, "")
	s.Add("Backlog item", GroupBacklog, "")
	s.Add("Done item", GroupTodo, "")
	s.Done("done-item")

	// All active.
	active := s.List(StatusActive, "", "")
	assert.Len(t, active, 2)

	// Todo only.
	todo := s.List(StatusActive, GroupTodo, "")
	assert.Len(t, todo, 1)
	assert.Equal(t, "Active todo", todo[0].Title)

	// Backlog only.
	bl := s.List(StatusActive, GroupBacklog, "")
	assert.Len(t, bl, 1)

	// Done only.
	done := s.List(StatusDone, "", "")
	assert.Len(t, done, 1)

	// Root only.
	roots := s.List("", "", "!")
	assert.Len(t, roots, 3)
}

func TestUpdate(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	created, err := s.Add("Old title", GroupTodo, "")
	require.NoError(t, err)

	updated, err := s.Update(created.ID, "New title", "Some notes")
	require.NoError(t, err)
	assert.Equal(t, "New title", updated.Title)
	assert.Equal(t, "Some notes", updated.Notes)

	// Verify persisted.
	got, ok := s.Get(created.ID)
	assert.True(t, ok)
	assert.Equal(t, "New title", got.Title)
}

func TestAppendNotes(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	created, err := s.Add("Test", GroupTodo, "")
	require.NoError(t, err)

	_, err = s.AppendNotes(created.ID, "First note")
	require.NoError(t, err)

	_, err = s.AppendNotes(created.ID, "Second note")
	require.NoError(t, err)

	got, _ := s.Get(created.ID)
	assert.Contains(t, got.Notes, "First note")
	assert.Contains(t, got.Notes, "Second note")
}

func TestDone(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	parent, _ := s.Add("Parent", GroupTodo, "")
	s.Add("Child", GroupTodo, parent.ID)

	changed, err := s.Done(parent.ID)
	require.NoError(t, err)
	assert.Len(t, changed, 2)

	parent, _ = s.Get(parent.ID)
	assert.Equal(t, StatusDone, parent.Status)
}

func TestUndone(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	task, _ := s.Add("Test", GroupTodo, "")
	s.Done(task.ID)

	changed, err := s.Undone(task.ID)
	require.NoError(t, err)
	assert.Len(t, changed, 1)

	got, _ := s.Get(task.ID)
	assert.Equal(t, StatusActive, got.Status)
}

func TestDelete(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	parent, _ := s.Add("Parent", GroupTodo, "")
	s.Add("Child", GroupTodo, parent.ID)

	deleted, err := s.Delete(parent.ID)
	require.NoError(t, err)
	assert.Len(t, deleted, 2)

	_, ok := s.Get(parent.ID)
	assert.False(t, ok)
}

func TestMoveGroup(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	task, _ := s.Add("Test", GroupTodo, "")

	moved, err := s.Move(task.ID, GroupBacklog, "")
	require.NoError(t, err)
	assert.Equal(t, GroupBacklog, moved.Group)
}

func TestMoveReparent(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	parent, _ := s.Add("Parent", GroupTodo, "")
	child, _ := s.Add("Orphan", GroupTodo, "")

	moved, err := s.Move(child.ID, "", parent.ID)
	require.NoError(t, err)
	assert.Equal(t, parent.ID, moved.ParentID)
}

func TestMoveCycle(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	parent, _ := s.Add("Parent", GroupTodo, "")
	child, _ := s.Add("Child", GroupTodo, parent.ID)

	_, err := s.Move(parent.ID, "", child.ID)
	assert.Error(t, err) // would create cycle
}

func TestMoveSelfParent(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	task, _ := s.Add("Self", GroupTodo, "")

	_, err := s.Move(task.ID, "", task.ID)
	assert.Error(t, err)
}

func TestLink(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	task, _ := s.Add("Test", GroupTodo, "")

	linked, err := s.Link(task.ID, "linear_ticket", "CORE-456")
	require.NoError(t, err)
	assert.Equal(t, "CORE-456", linked.LinearTicket)

	linked, err = s.Link(task.ID, "github_pr", "https://github.com/org/repo/pull/1")
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/org/repo/pull/1", linked.GitHubPR)

	linked, err = s.Link(task.ID, "branch", "feat/auth")
	require.NoError(t, err)
	assert.Equal(t, "feat/auth", linked.Branch)

	linked, err = s.Link(task.ID, "url", "https://example.com")
	require.NoError(t, err)
	assert.Contains(t, linked.Links, "https://example.com")
}

func TestLinkInvalidField(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	task, _ := s.Add("Test", GroupTodo, "")

	_, err := s.Link(task.ID, "invalid", "value")
	assert.Error(t, err)
}

func TestSearch(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	s.Add("Fix authentication bug", GroupTodo, "")
	s.Add("Update README", GroupTodo, "")
	s.Add("Auth token refresh", GroupTodo, "")

	results := s.Search("auth")
	assert.Len(t, results, 2)
}

func TestSearchNotes(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	task, _ := s.Add("Generic title", GroupTodo, "")
	s.AppendNotes(task.ID, "This is about database migration")

	results := s.Search("database")
	assert.Len(t, results, 1)
}

func TestStats(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	s.Add("Todo 1", GroupTodo, "")
	s.Add("Todo 2", GroupTodo, "")
	s.Add("Backlog 1", GroupBacklog, "")
	task, _ := s.Add("Done 1", GroupTodo, "")
	s.Done(task.ID)

	active, done, backlog := s.Stats()
	assert.Equal(t, 2, active)
	assert.Equal(t, 1, done)
	assert.Equal(t, 1, backlog)
}

func TestActiveTasks(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	s.Add("Root 1", GroupTodo, "")
	parent, _ := s.Add("Root 2", GroupTodo, "")
	s.Add("Child", GroupTodo, parent.ID)
	s.Add("Backlog", GroupBacklog, "")

	tasks := s.ActiveTasks()
	assert.Len(t, tasks, 2) // only root-level active todo tasks
}

func TestSlugify(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"Fix the login bug", "fix-the-login-bug"},
		{"  spaces  ", "spaces"},
		{"UPPERCASE", "uppercase"},
		{"Multi---dash", "multi-dash"},
		{"Special!@#Characters", "special-characters"},
		{"a", "a"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, slugify(tt.input))
	}
}

func TestPersistence(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	cwd := filepath.Join(dir, "project")
	require.NoError(t, os.MkdirAll(cwd, 0700))

	// Create store and add task.
	s1, err := NewStore(cwd)
	require.NoError(t, err)

	task, err := s1.Add("Persist me", GroupTodo, "")
	require.NoError(t, err)
	assert.Equal(t, "persist-me", task.ID)

	// New store instance reads from same file.
	s2, err := NewStore(cwd)
	require.NoError(t, err)

	got, ok := s2.Get("persist-me")
	assert.True(t, ok)
	assert.Equal(t, "Persist me", got.Title)
}

func TestIsDescendant(t *testing.T) {
	t.Parallel()

	data := map[string]*Task{
		"a": {ID: "a", ParentID: ""},
		"b": {ID: "b", ParentID: "a"},
		"c": {ID: "c", ParentID: "b"},
	}

	assert.True(t, isDescendant(data, "c", "a"))
	assert.True(t, isDescendant(data, "b", "a"))
	assert.False(t, isDescendant(data, "a", "c"))
	assert.False(t, isDescendant(data, "a", "a"))
}

func TestEmptyTitle(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	_, err := s.Add("", GroupTodo, "")
	assert.Error(t, err)
}

func TestBuildPrompt(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	// Empty store.
	assert.Equal(t, "", buildPrompt(s))

	s.Add("Test task", GroupTodo, "")
	prompt := buildPrompt(s)
	assert.Contains(t, prompt, "Test task")
	assert.Contains(t, prompt, "1 active")
}

func TestCreatedAtPreserved(t *testing.T) {
	t.Parallel()

	s := tempStore(t)

	task, _ := s.Add("Test", GroupTodo, "")
	originalCreatedAt := task.CreatedAt

	// Wait a tiny bit and update.
	time.Sleep(time.Millisecond)
	s.Update(task.ID, "Updated", "")

	got, _ := s.Get(task.ID)
	assert.Equal(t, originalCreatedAt, got.CreatedAt)
	assert.True(t, got.UpdatedAt.After(originalCreatedAt))
}
