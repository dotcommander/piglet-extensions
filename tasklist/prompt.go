package tasklist

import (
	"fmt"
	"strings"
)

const maxPromptChars = 4000

// buildPrompt generates the system prompt section showing active tasks.
func buildPrompt(s *Store) string {
	active, done, backlog := s.Stats()

	tasks := s.ActiveTasks()
	if len(tasks) == 0 && backlog == 0 && done == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Current tasks: %d active, %d backlog, %d done.", active, backlog, done)

	if len(tasks) > 0 {
		b.WriteString("\n\nActive TODO:\n")
		for _, t := range tasks {
			line := fmt.Sprintf("- %s [%s]", t.Title, t.ID)
			children := s.Children(t.ID)
			if len(children) > 0 {
				for _, c := range children {
					if c.Status == StatusActive {
						line += fmt.Sprintf("\n  - %s [%s]", c.Title, c.ID)
					}
				}
			}
			b.WriteString(line + "\n")
		}
	}

	result := b.String()
	if len(result) > maxPromptChars {
		result = result[:maxPromptChars] + "\n... (truncated)"
	}

	return result
}
