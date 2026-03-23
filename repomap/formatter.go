package repomap

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

const groupPreviewLimit = 4

// FormatMap formats ranked files into a token-budgeted text representation.
// maxTokens controls the output size (estimated as len(text)/4).
// Returns empty string if no files have symbols.
// When verbose is true, shows all symbols without summarization.
func FormatMap(files []RankedFile, maxTokens int, verbose bool) string {
	totalFiles, totalSymbols := countTotals(files)
	if totalFiles == 0 {
		return ""
	}

	var b strings.Builder

	header := fmt.Sprintf("## Repository Map (%d files, %d symbols)\n\n", totalFiles, totalSymbols)
	fmt.Fprint(&b, header)

	if verbose {
		// Verbose mode: show everything, no budget truncation
		for _, f := range files {
			if len(f.Symbols) == 0 {
				continue
			}
			fmt.Fprint(&b, formatFileBlockVerbose(f))
		}
		return b.String()
	}

	shownFiles := 0
	shownSymbols := 0
	budgetBytes := maxTokens * 4

	for _, f := range files {
		if len(f.Symbols) == 0 {
			continue
		}

		block := formatFileBlock(f)

		// Always include at least the first file; truncate its groups if needed.
		if shownFiles == 0 {
			remaining := budgetBytes - b.Len()
			if len(block) > remaining {
				block = truncateFileBlock(f, remaining)
			}
		} else if b.Len()+len(block) > budgetBytes {
			break
		}

		fmt.Fprint(&b, block)
		shownFiles++
		shownSymbols += len(f.Symbols)
	}

	if shownFiles < totalFiles || shownSymbols < totalSymbols {
		fmt.Fprintf(&b, "(%d symbols across %d files, showing top %d)\n", totalSymbols, totalFiles, shownFiles)
	}

	return b.String()
}

// countTotals returns the count of files with symbols and the total symbol count.
func countTotals(files []RankedFile) (int, int) {
	nFiles := 0
	nSymbols := 0
	for _, f := range files {
		if len(f.Symbols) > 0 {
			nFiles++
			nSymbols += len(f.Symbols)
		}
	}
	return nFiles, nSymbols
}

// formatFileBlock returns the full formatted block for a single file.
func formatFileBlock(f RankedFile) string {
	var b strings.Builder
	fmt.Fprint(&b, formatFileLine(f))
	for _, line := range formatGroupLines(f) {
		fmt.Fprintf(&b, "  %s\n", line)
	}
	fmt.Fprint(&b, "\n")
	return b.String()
}

// formatFileBlockVerbose returns a verbose block showing all symbols without summarization.
func formatFileBlockVerbose(f RankedFile) string {
	var b strings.Builder
	fmt.Fprint(&b, formatFileLine(f))

	// Group symbols by category but show all names
	categorized := make(map[string][]Symbol)
	for _, s := range f.Symbols {
		categorized[symbolCategory(f.Path, s)] = append(categorized[symbolCategory(f.Path, s)], s)
	}

	order := []struct {
		key   string
		label string
	}{
		{"tests", "tests"},
		{"types", "types"},
		{"interfaces", "interfaces"},
		{"classes", "classes"},
		{"enums", "enums"},
		{"funcs", "funcs"},
		{"methods", "methods"},
		{"consts", "consts"},
		{"vars", "vars"},
		{"other", "other"},
	}

	for _, item := range order {
		syms := categorized[item.key]
		if len(syms) == 0 {
			continue
		}
		names := make([]string, 0, len(syms))
		for _, s := range syms {
			if item.key == "methods" && s.Receiver != "" {
				names = append(names, s.Receiver+"."+s.Name)
			} else {
				names = append(names, s.Name)
			}
		}
		sort.Strings(names)
		fmt.Fprintf(&b, "  %s: %s\n", item.label, strings.Join(names, ", "))
	}
	fmt.Fprint(&b, "\n")
	return b.String()
}

// formatFileLine returns the header line for a file block (path + tag annotation).
func formatFileLine(f RankedFile) string {
	switch {
	case f.Tag == "entry":
		return fmt.Sprintf("%s [entry]\n", f.Path)
	case f.Score > 0:
		return fmt.Sprintf("%s [%d refs]\n", f.Path, f.Score)
	default:
		return fmt.Sprintf("%s\n", f.Path)
	}
}

func formatGroupLines(f RankedFile) []string {
	groups := summarizeSymbols(f)
	lines := make([]string, 0, len(groups))
	for _, g := range groups {
		lines = append(lines, fmt.Sprintf("%s: %s", g.label, g.summary))
	}
	return lines
}

type symbolGroup struct {
	label   string
	summary string
	count   int
}

func summarizeSymbols(f RankedFile) []symbolGroup {
	categorized := make(map[string][]Symbol)
	for _, s := range f.Symbols {
		categorized[symbolCategory(f.Path, s)] = append(categorized[symbolCategory(f.Path, s)], s)
	}

	order := []struct {
		key   string
		label string
	}{
		{"tests", "tests"},
		{"types", "types"},
		{"interfaces", "interfaces"},
		{"classes", "classes"},
		{"enums", "enums"},
		{"funcs", "funcs"},
		{"methods", "methods"},
		{"consts", "consts"},
		{"vars", "vars"},
		{"other", "other"},
	}

	var groups []symbolGroup
	for _, item := range order {
		syms := categorized[item.key]
		if len(syms) == 0 {
			continue
		}
		groups = append(groups, symbolGroup{
			label:   item.label,
			summary: summarizeGroup(item.key, syms),
			count:   len(syms),
		})
	}
	return groups
}

func symbolCategory(path string, s Symbol) string {
	if isTestSymbol(path, s) {
		return "tests"
	}

	switch s.Kind {
	case "struct", "type":
		return "types"
	case "interface":
		return "interfaces"
	case "class":
		return "classes"
	case "enum":
		return "enums"
	case "function", "fn":
		return "funcs"
	case "method":
		return "methods"
	case "constant", "const":
		return "consts"
	case "variable", "static":
		return "vars"
	default:
		return "other"
	}
}

func isTestSymbol(path string, s Symbol) bool {
	if !strings.HasSuffix(path, "_test.go") {
		return false
	}
	return strings.HasPrefix(s.Name, "Test") || strings.HasPrefix(s.Name, "Benchmark") || strings.HasPrefix(s.Name, "Fuzz")
}

func summarizeGroup(category string, syms []Symbol) string {
	names := make([]string, 0, len(syms))
	for _, s := range syms {
		if category == "methods" && s.Receiver != "" {
			names = append(names, s.Name)
			continue
		}
		names = append(names, s.Name)
	}

	sort.Strings(names)
	if collapsed, ok := collapseCommonPrefix(names); ok {
		return withTotal(collapsed, len(names), true)
	}
	return withTotal(previewNames(names), len(names), len(names) > groupPreviewLimit)
}

func previewNames(names []string) string {
	if len(names) <= groupPreviewLimit {
		return strings.Join(names, ", ")
	}
	preview := append([]string{}, names[:groupPreviewLimit]...)
	preview = append(preview, "...")
	return strings.Join(preview, ", ")
}

func withTotal(summary string, total int, forced bool) string {
	if total == 0 {
		return ""
	}
	if forced || total > 1 {
		return fmt.Sprintf("%s (%d total)", summary, total)
	}
	return summary
}

func collapseCommonPrefix(names []string) (string, bool) {
	if len(names) < 3 {
		return "", false
	}

	prefix := longestCommonPrefix(names)
	if len(prefix) < 3 {
		return "", false
	}
	if strings.HasSuffix(prefix, "_") {
		prefix = strings.TrimSuffix(prefix, "_")
	}
	if len(prefix) < 3 {
		return "", false
	}

	suffixes := make([]string, 0, len(names))
	for _, name := range names {
		suffix := strings.TrimPrefix(name, prefix)
		suffix = strings.TrimPrefix(suffix, "_")
		if suffix == "" {
			return "", false
		}
		suffixes = append(suffixes, suffix)
	}

	preview := suffixes
	truncated := false
	if len(preview) > groupPreviewLimit {
		preview = append([]string{}, preview[:groupPreviewLimit]...)
		truncated = true
	}

	body := strings.Join(preview, ", ")
	if truncated {
		body += ", ..."
	}
	return fmt.Sprintf("%s{%s}", prefix, body), true
}

func longestCommonPrefix(names []string) string {
	if len(names) == 0 {
		return ""
	}
	prefix := names[0]
	for _, name := range names[1:] {
		for !strings.HasPrefix(name, prefix) {
			if prefix == "" {
				return ""
			}
			prefix = prefix[:len(prefix)-1]
		}
	}
	return trimIdentifierPrefix(prefix)
}

func trimIdentifierPrefix(prefix string) string {
	if prefix == "" {
		return ""
	}
	lastBoundary := -1
	for i := 1; i < len(prefix); i++ {
		if prefix[i] == '_' || isCamelBoundary(prefix[i-1], prefix[i]) {
			lastBoundary = i
		}
	}
	if lastBoundary > 0 {
		return prefix[:lastBoundary]
	}
	return prefix
}

func isCamelBoundary(prev, curr byte) bool {
	return prev >= 'a' && prev <= 'z' && curr >= 'A' && curr <= 'Z'
}

// truncateFileBlock formats the file header and as many group lines as fit within byteLimit.
func truncateFileBlock(f RankedFile, byteLimit int) string {
	if byteLimit <= 0 {
		return ""
	}

	var b strings.Builder
	header := formatFileLine(f)
	if len(header) > byteLimit {
		return header[:byteLimit]
	}
	fmt.Fprint(&b, header)

	for _, line := range formatGroupLines(f) {
		formatted := fmt.Sprintf("  %s\n", line)
		if b.Len()+len(formatted) > byteLimit {
			break
		}
		fmt.Fprint(&b, formatted)
	}
	if b.Len()+1 <= byteLimit {
		fmt.Fprint(&b, "\n")
	}
	return b.String()
}

func isTestFile(path string) bool {
	base := filepath.Base(path)
	return strings.HasSuffix(base, "_test.go")
}
