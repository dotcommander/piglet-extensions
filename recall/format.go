package recall

import (
	"fmt"
	"strings"
)

// formatLabel returns a display label for a search result, truncating the
// session ID to 8 characters when no title is available.
func formatLabel(r SearchResult) string {
	if r.Title != "" {
		return r.Title
	}
	if len(r.SessionID) > 8 {
		return r.SessionID[:8]
	}
	return r.SessionID
}

// formatExcerpt collapses an excerpt to a single line with normalized whitespace.
func formatExcerpt(excerpt string) string {
	return strings.ReplaceAll(strings.TrimSpace(excerpt), "\n", " ")
}

// buildResultsText formats search results as a readable string.
func buildResultsText(results []SearchResult) string {
	var b strings.Builder
	for i, r := range results {
		fmt.Fprintf(&b, "%d. %s (score: %.4f)\n", i+1, formatLabel(r), r.Score)
		excerpt := readExcerpt(r.Path, searchExcerptLen)
		if excerpt != "" {
			fmt.Fprintf(&b, "   %s\n", formatExcerpt(excerpt))
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// readExcerpt reads the first maxChars characters of text from the session file.
func readExcerpt(path string, maxChars int) string {
	if path == "" {
		return ""
	}
	text, err := ExtractSessionText(path, maxChars*4) // bytes approx
	if err != nil {
		return ""
	}
	runes := []rune(text)
	if len(runes) > maxChars {
		return string(runes[:maxChars])
	}
	return text
}

// wordCount returns the approximate number of words in s.
func wordCount(s string) int {
	return len(strings.Fields(s))
}
