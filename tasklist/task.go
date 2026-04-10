package tasklist

import (
	"regexp"
	"strings"
	"time"
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
