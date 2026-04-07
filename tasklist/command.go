package tasklist

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/dotcommander/piglet/sdk"
)

func todoCommand(sp **Store, e *sdk.Extension) func(context.Context, string) error {
	return func(_ context.Context, args string) error {
		s := *sp
		if s == nil {
			e.ShowMessage("tasklist: not initialized")
			return nil
		}

		parts := strings.Fields(args)
		if len(parts) == 0 {
			// Default: list active tasks.
			tasks := s.List(StatusActive, "", "")
			if len(tasks) == 0 {
				e.ShowMessage("No active tasks. Use /todo add <title> to create one.")
				return nil
			}

			var b strings.Builder
			active, done, backlog := s.Stats()
			fmt.Fprintf(&b, "Tasks: %d active · %d backlog · %d done\n\n", active, backlog, done)

			todoTasks := s.List(StatusActive, GroupTodo, "!")
			if len(todoTasks) > 0 {
				b.WriteString("TODO:\n")
				for _, t := range todoTasks {
					fmt.Fprintf(&b, "  ◉ %s [%s]\n", t.Title, t.ID)
					children := s.Children(t.ID)
					for _, c := range children {
						icon := "◉"
						if c.Status == StatusDone {
							icon = "✓"
						}
						fmt.Fprintf(&b, "    %s %s [%s]\n", icon, c.Title, c.ID)
					}
				}
			}

			blTasks := s.List(StatusActive, GroupBacklog, "!")
			if len(blTasks) > 0 {
				b.WriteString("\nBACKLOG:\n")
				for _, t := range blTasks {
					fmt.Fprintf(&b, "  ○ %s [%s]\n", t.Title, t.ID)
				}
			}

			e.ShowMessage(b.String())
			return nil
		}

		sub := parts[0]
		rest := strings.Join(parts[1:], " ")

		switch sub {
		case "add":
			handleCmdAdd(s, e, rest)
		case "done":
			handleCmdDone(s, e, rest)
		case "delete":
			handleCmdDelete(s, e, rest)
		case "backlog":
			handleCmdBacklog(s, e, rest)
		case "plan":
			handleCmdPlan(s, e, rest)
		case "list":
			handleCmdList(s, e, rest)
		default:
			e.ShowMessage("Usage: /todo [add <title>|done <id>|delete <id>|backlog <id>|plan <id> [text]|list]")
		}

		return nil
	}
}

func handleCmdAdd(s *Store, e *sdk.Extension, args string) {
	title := strings.TrimSpace(args)
	if title == "" {
		e.ShowMessage("Usage: /todo add <title>")
		return
	}

	t, err := s.Add(title, GroupTodo, "")
	if err != nil {
		e.ShowMessage("Error: " + err.Error())
		return
	}

	e.ShowMessage(fmt.Sprintf("Created: %s [%s]", t.Title, t.ID))
}

func handleCmdDone(s *Store, e *sdk.Extension, args string) {
	id := strings.TrimSpace(args)
	if id == "" {
		e.ShowMessage("Usage: /todo done <id>")
		return
	}

	t, err := s.Resolve(id)
	if err != nil {
		e.ShowMessage("Error: " + err.Error())
		return
	}

	changed, err := s.Done(t.ID)
	if err != nil {
		e.ShowMessage("Error: " + err.Error())
		return
	}

	e.ShowMessage(fmt.Sprintf("Done: %d task(s) marked complete", len(changed)))
}

func handleCmdDelete(s *Store, e *sdk.Extension, args string) {
	id := strings.TrimSpace(args)
	if id == "" {
		e.ShowMessage("Usage: /todo delete <id>")
		return
	}

	t, err := s.Resolve(id)
	if err != nil {
		e.ShowMessage("Error: " + err.Error())
		return
	}

	deleted, err := s.Delete(t.ID)
	if err != nil {
		e.ShowMessage("Error: " + err.Error())
		return
	}

	e.ShowMessage(fmt.Sprintf("Deleted %d task(s)", len(deleted)))
}

func handleCmdBacklog(s *Store, e *sdk.Extension, args string) {
	id := strings.TrimSpace(args)
	if id == "" {
		e.ShowMessage("Usage: /todo backlog <id>")
		return
	}

	t, err := s.Resolve(id)
	if err != nil {
		e.ShowMessage("Error: " + err.Error())
		return
	}

	_, err = s.Move(t.ID, GroupBacklog, "")
	if err != nil {
		e.ShowMessage("Error: " + err.Error())
		return
	}

	e.ShowMessage(fmt.Sprintf("Moved %s to backlog", t.ID))
}

func handleCmdPlan(s *Store, e *sdk.Extension, args string) {
	parts := strings.Fields(args)
	if len(parts) == 0 {
		e.ShowMessage("Usage: /todo plan <id> [text to append]")
		return
	}

	id := parts[0]
	t, err := s.Resolve(id)
	if err != nil {
		e.ShowMessage("Error: " + err.Error())
		return
	}

	text := strings.Join(parts[1:], " ")
	if text == "" {
		// Read notes.
		if t.Notes == "" {
			e.ShowMessage(fmt.Sprintf("No notes for %s", t.ID))
		} else {
			e.ShowMessage(fmt.Sprintf("# %s\n\n%s", t.Title, t.Notes))
		}
		return
	}

	// Append notes.
	_, err = s.AppendNotes(t.ID, text)
	if err != nil {
		e.ShowMessage("Error: " + err.Error())
		return
	}

	e.ShowMessage(fmt.Sprintf("Appended notes to %s", t.ID))
}

func handleCmdList(s *Store, e *sdk.Extension, args string) {
	var tasks []*Task
	switch strings.TrimSpace(args) {
	case "done":
		tasks = s.List(StatusDone, "", "")
	case "backlog":
		tasks = s.List(StatusActive, GroupBacklog, "")
	case "all":
		tasks = s.List("", "", "")
	default:
		tasks = s.List(StatusActive, "", "")
	}

	if len(tasks) == 0 {
		e.ShowMessage("No tasks found.")
		return
	}

	var b strings.Builder
	for _, t := range tasks {
		icon := "◉"
		switch {
		case t.Status == StatusDone:
			icon = "✓"
		case t.Group == GroupBacklog:
			icon = "○"
		}

		fmt.Fprintf(&b, "%s %s [%s]\n", icon, t.Title, t.ID)
	}

	e.ShowMessage(b.String())
}
