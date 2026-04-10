package xdg

import "strings"

// SanitizeFilename replaces characters unsafe for filenames with hyphens.
// Preserves case, letters, digits, hyphens, underscores, dots, and parentheses.
func SanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' || r == ' ' {
			b.WriteRune('-')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}
