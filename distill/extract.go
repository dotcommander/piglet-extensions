// Package distill extracts reusable skills from completed sessions.
// Skills are written as markdown files to ~/.config/piglet/skills/.
package distill

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dotcommander/piglet-extensions/internal/xdg"
	sdk "github.com/dotcommander/piglet/sdk"
)

// writeSkill writes a skill file to ~/.config/piglet/skills/.
// Content should be a complete markdown file with YAML frontmatter.
// Returns the path written.
func writeSkill(content string) (string, error) {
	base, err := xdg.ConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return writeSkillTo(filepath.Join(base, "skills"), content)
}

// writeSkillTo writes a skill file to an explicit directory.
// Extracted for testability — tests pass t.TempDir() instead of the real skills dir.
func writeSkillTo(dir, content string) (string, error) {
	name := extractFrontmatterName(content)
	if name == "" {
		name = "distilled-skill"
		content = wrapWithFrontmatter(content, name)
	}

	name = sanitizeName(name)
	path := filepath.Join(dir, name+".md")
	if err := xdg.WriteFileAtomic(path, []byte(content)); err != nil {
		return "", fmt.Errorf("write skill %s: %w", path, err)
	}
	return path, nil
}

var frontmatterNameRe = regexp.MustCompile(`(?m)^name:\s*(.+)$`)

// extractFrontmatterName parses the name field from YAML frontmatter.
func extractFrontmatterName(content string) string {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "---") {
		return ""
	}
	// Find closing --- to limit search to frontmatter section only.
	rest := trimmed[3:]
	closeIdx := strings.Index(rest, "\n---")
	if closeIdx < 0 {
		closeIdx = len(rest)
	}
	frontmatter := rest[:closeIdx]
	m := frontmatterNameRe.FindStringSubmatch(frontmatter)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9-]`)
var multiHyphenRe = regexp.MustCompile(`-{2,}`)

// sanitizeName produces a safe filename: lowercase, hyphens, no special chars.
func sanitizeName(name string) string {
	s := strings.ToLower(xdg.SanitizeFilename(name))
	s = nonAlphanumRe.ReplaceAllString(s, "")
	s = multiHyphenRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if s == "" {
		return "distilled-skill"
	}
	return s
}

// wrapWithFrontmatter wraps bare content with minimal frontmatter.
func wrapWithFrontmatter(content, name string) string {
	fm := fmt.Sprintf("---\nname: %s\ndescription: Auto-extracted skill\ntriggers:\n  - skill\nsource: distill\n---\n\n", name)
	return fm + content
}

// distillSession runs the full extraction pipeline on messages.
func distillSession(ctx context.Context, e *sdk.Extension, messages []json.RawMessage) (string, error) {
	transcript := formatMessages(messages, maxFormatChars)
	if transcript == "" {
		return "", fmt.Errorf("no transcript content to distill")
	}

	prompt := xdg.LoadOrCreateExt("distill", "extract-prompt.md", strings.TrimSpace(defaultPrompt))

	resp, err := e.Chat(ctx, sdk.ChatRequest{
		System:    prompt,
		Messages:  []sdk.ChatMessage{{Role: "user", Content: transcript}},
		Model:     "small",
		MaxTokens: 2048,
	})
	if err != nil {
		return "", fmt.Errorf("chat: %w", err)
	}

	text := strings.TrimSpace(resp.Text)
	if text == "" || strings.EqualFold(text, "SKIP") {
		return "", fmt.Errorf("model skipped: no reusable skill found")
	}

	path, err := writeSkill(text)
	if err != nil {
		return "", fmt.Errorf("write skill: %w", err)
	}
	return path, nil
}

// readSessionMessages reads a session JSONL file and returns the message entries.
// Meta-type entries are skipped. Returns the data field of each non-meta entry.
func readSessionMessages(path string) ([]json.RawMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open session %s: %w", path, err)
	}
	defer f.Close()

	var messages []json.RawMessage
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var entry struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if json.Unmarshal(line, &entry) != nil {
			continue
		}
		if entry.Type == "meta" || len(entry.Data) == 0 {
			continue
		}
		messages = append(messages, entry.Data)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read session %s: %w", path, err)
	}
	return messages, nil
}
