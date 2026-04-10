package admin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// configCommand returns a handler for the /config command with subcommand routing.
func configCommand(m messenger) func(context.Context, string) error {
	return func(_ context.Context, args string) error {
		parts := strings.Fields(args)
		if len(parts) == 0 {
			if dir, ok := configDir(m); ok {
				showConfigListing(m, dir)
			}
			return nil
		}

		sub := parts[0]
		switch sub {
		case "setup", "--setup":
			if dir, ok := configDir(m); ok {
				runSetup(m, dir)
			}
		case "list", "status", "ls":
			if dir, ok := configDir(m); ok {
				showConfigListing(m, dir)
			}
		case "read", "cat":
			filename := strings.Join(parts[1:], " ")
			handleConfigRead(m, filename)
		case "--version", "-v":
			m.ShowMessage(fmt.Sprintf("admin v%s", Version))
		default:
			m.ShowMessage("Usage: /config [setup|list|read <file>|--version]\nUnknown argument: " + sub)
		}
		return nil
	}
}

// handleConfigRead displays the contents of a config file.
func handleConfigRead(m messenger, filename string) {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		m.ShowMessage("Usage: /config read <filename>")
		return
	}
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		m.ShowMessage("Filename must be a simple name, not a path")
		return
	}
	dir, ok := configDir(m)
	if !ok {
		return
	}
	data, err := os.ReadFile(filepath.Join(dir, filename))
	if err != nil {
		if os.IsNotExist(err) {
			m.ShowMessage(fmt.Sprintf("File %s does not exist in %s", filename, dir))
			return
		}
		m.ShowMessage("Read error: " + err.Error())
		return
	}
	m.ShowMessage(string(data))
}
