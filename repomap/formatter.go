package repomap

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const groupPreviewLimit = 4

// categoryOrder defines the display order for symbol categories.
var categoryOrder = []struct {
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

// typeKey identifies a type within a specific file.
type typeKey struct {
	path string
	name string
}

// FormatMap formats ranked files into a token-budgeted text representation.
// maxTokens controls the output size (estimated as len(text)/4).
// Returns empty string if no files have symbols.
// When verbose is true, shows all symbols without summarization.
// When detail is true, shows signatures for funcs/methods and fields for structs.
func FormatMap(files []RankedFile, maxTokens int, verbose, detail bool) string {
	totalFiles, totalSymbols := countTotals(files)
	if totalFiles == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprint(&b, buildHeader(files, totalFiles, totalSymbols))

	if verbose {
		for _, f := range files {
			if len(f.Symbols) == 0 {
				continue
			}
			if detail {
				fmt.Fprint(&b, formatFileBlockDetail(f))
			} else {
				fmt.Fprint(&b, formatFileBlockVerbose(f))
			}
		}
		return b.String()
	}

	// Collect top-ranked struct/interface names for field display in compact mode.
	topTypes := collectTopTypes(files, 10)

	shownFiles := 0
	shownSymbols := 0
	budgetBytes := maxTokens * 4

	for _, f := range files {
		if len(f.Symbols) == 0 {
			continue
		}

		block := formatFileBlockCompact(f, topTypes)

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

// buildHeader returns the shared header block (title + dependency graph) used
// by all format modes.
func buildHeader(files []RankedFile, totalFiles, totalSymbols int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## Repository Map (%d files, %d symbols)\n\n", totalFiles, totalSymbols)
	if graph := formatDependencyGraph(files); graph != "" {
		fmt.Fprint(&b, graph)
		fmt.Fprint(&b, "\n")
	}
	return b.String()
}

// formatFileBlockVerbose returns a verbose block showing all symbols without summarization.
func formatFileBlockVerbose(f RankedFile) string {
	var b strings.Builder
	fmt.Fprint(&b, formatFileLine(f))

	categorized := make(map[string][]Symbol)
	for _, s := range f.Symbols {
		cat := symbolCategory(f.Path, s)
		categorized[cat] = append(categorized[cat], s)
	}

	for _, item := range categoryOrder {
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

// formatFileBlockDetail returns a detailed block showing signatures and struct fields.
func formatFileBlockDetail(f RankedFile) string {
	var b strings.Builder
	fmt.Fprint(&b, formatFileLine(f))

	categorized := make(map[string][]Symbol)
	for _, s := range f.Symbols {
		cat := symbolCategory(f.Path, s)
		categorized[cat] = append(categorized[cat], s)
	}

	for _, item := range categoryOrder {
		syms := categorized[item.key]
		if len(syms) == 0 {
			continue
		}

		sort.Slice(syms, func(i, j int) bool {
			return syms[i].Name < syms[j].Name
		})

		var lines []string
		for _, s := range syms {
			var line string
			switch {
			case item.key == "methods" && s.Receiver != "":
				if s.Signature != "" {
					line = fmt.Sprintf("%s.%s%s", s.Receiver, s.Name, s.Signature)
				} else {
					line = fmt.Sprintf("%s.%s", s.Receiver, s.Name)
				}
			case (item.key == "types" || item.key == "interfaces") && s.Signature != "":
				line = fmt.Sprintf("%s %s", s.Name, s.Signature)
			case item.key == "funcs" && s.Signature != "":
				line = fmt.Sprintf("%s%s", s.Name, s.Signature)
			default:
				line = s.Name
			}
			lines = append(lines, line)
		}

		fmt.Fprintf(&b, "  %s:\n", item.label)
		for _, line := range lines {
			fmt.Fprintf(&b, "    %s\n", line)
		}
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
		cat := symbolCategory(f.Path, s)
		categorized[cat] = append(categorized[cat], s)
	}

	var groups []symbolGroup
	for _, item := range categoryOrder {
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

// formatFileBlockCompact returns the compact block with struct fields for top-ranked types.
func formatFileBlockCompact(f RankedFile, topTypes map[typeKey]bool) string {
	var b strings.Builder
	fmt.Fprint(&b, formatFileLine(f))

	groups := summarizeSymbols(f)
	for _, g := range groups {
		fmt.Fprintf(&b, "  %s: %s\n", g.label, g.summary)
	}

	for _, s := range f.Symbols {
		if s.Signature == "" || s.Signature == "{}" {
			continue
		}
		if (s.Kind == "struct" || s.Kind == "interface") && topTypes[typeKey{f.Path, s.Name}] {
			fmt.Fprintf(&b, "  %s %s\n", s.Name, s.Signature)
		}
	}

	fmt.Fprint(&b, "\n")
	return b.String()
}

// collectTopTypes returns a set of path+name keys for the top N ranked
// struct/interface types across all files. These get their fields shown
// in compact mode.
func collectTopTypes(files []RankedFile, limit int) map[typeKey]bool {
	type candidate struct {
		key   typeKey
		score int
	}
	var candidates []candidate

	for _, f := range files {
		for _, s := range f.Symbols {
			if (s.Kind == "struct" || s.Kind == "interface") && s.Signature != "" && s.Signature != "{}" {
				candidates = append(candidates, candidate{
					key:   typeKey{f.Path, s.Name},
					score: f.Score,
				})
			}
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	result := make(map[typeKey]bool, limit)
	for i, c := range candidates {
		if i >= limit {
			break
		}
		result[c.key] = true
	}
	return result
}

// FormatLines formats ranked files showing actual source code lines.
// root is the project root for resolving file paths.
func FormatLines(files []RankedFile, maxTokens int, root string) string {
	totalFiles, totalSymbols := countTotals(files)
	if totalFiles == 0 {
		return ""
	}

	var b strings.Builder
	fmt.Fprint(&b, buildHeader(files, totalFiles, totalSymbols))

	budgetBytes := maxTokens * 4
	shownFiles := 0

	for _, f := range files {
		if len(f.Symbols) == 0 {
			continue
		}

		if isTestFile(f.Path) {
			continue
		}

		block := formatFileBlockLines(f, root)

		if shownFiles > 0 && budgetBytes > 0 && b.Len()+len(block) > budgetBytes {
			break
		}

		fmt.Fprint(&b, block)
		shownFiles++
	}

	if shownFiles < totalFiles {
		fmt.Fprintf(&b, "(%d files shown of %d total)\n", shownFiles, totalFiles)
	}

	return b.String()
}

// formatFileBlockLines shows actual source lines for each symbol in a file.
// Reads the source file on demand to avoid loading files that get budget-cut.
func formatFileBlockLines(f RankedFile, root string) string {
	var b strings.Builder
	fmt.Fprint(&b, formatFileLine(f))

	var lines []string
	absPath := filepath.Join(root, f.Path)
	if data, err := os.ReadFile(absPath); err == nil {
		lines = strings.Split(string(data), "\n")
	}

	// Collect symbols with known line numbers, sorted by position.
	type symLine struct {
		sym  Symbol
		line int
	}
	var syms []symLine
	for _, s := range f.Symbols {
		if s.Line > 0 && s.Exported && isSignificantKind(f.Language, s.Kind) {
			syms = append(syms, symLine{sym: s, line: s.Line})
		}
	}

	sort.Slice(syms, func(i, j int) bool {
		return syms[i].line < syms[j].line
	})

	for _, sl := range syms {
		var text string

		// For structs/interfaces with field info, prefer synthesized line
		if (sl.sym.Kind == "struct" || sl.sym.Kind == "interface") &&
			sl.sym.Signature != "" && sl.sym.Signature != "{}" {
			text = synthesizeLine(sl.sym)
		} else if lines != nil && sl.line > 0 && sl.line <= len(lines) {
			text = strings.TrimRight(lines[sl.line-1], " \t\r")
			text = strings.TrimLeft(text, "\t ")
			// Trim trailing opening brace from Go definitions
			text = strings.TrimRight(text, " ")
			if strings.HasSuffix(text, " {") {
				text = strings.TrimSuffix(text, " {")
			}
		}

		if text == "" {
			text = synthesizeLine(sl.sym)
		}
		fmt.Fprintf(&b, "│ %s\n", text)
	}

	fmt.Fprint(&b, "\n")
	return b.String()
}

// synthesizeLine creates a readable definition line from symbol metadata
// when source line is unavailable.
func synthesizeLine(s Symbol) string {
	switch s.Kind {
	case "function", "fn":
		if s.Signature != "" {
			return "func " + s.Name + s.Signature
		}
		return "func " + s.Name + "()"
	case "method":
		prefix := "func "
		if s.Receiver != "" {
			prefix += "(" + s.Receiver + ") "
		}
		if s.Signature != "" {
			return prefix + s.Name + s.Signature
		}
		return prefix + s.Name + "()"
	case "struct":
		if s.Signature != "" && s.Signature != "{}" {
			return "type " + s.Name + " struct " + s.Signature
		}
		return "type " + s.Name + " struct{}"
	case "interface":
		if s.Signature != "" && s.Signature != "{}" {
			return "type " + s.Name + " interface " + s.Signature
		}
		return "type " + s.Name + " interface{}"
	case "type":
		return "type " + s.Name
	case "class":
		return "class " + s.Name
	case "constant", "const":
		return "const " + s.Name
	case "variable", "static":
		return "var " + s.Name
	case "enum":
		return "enum " + s.Name
	default:
		return s.Name
	}
}

// isSignificantKind returns true for symbol kinds worth showing in lines mode.
// Filters out local variables and constants for non-Go languages where ctags
// picks up function-scoped declarations.
func isSignificantKind(lang, kind string) bool {
	switch kind {
	case "function", "fn", "method", "class", "struct", "interface",
		"type", "enum", "trait", "impl":
		return true
	case "constant", "const":
		// Go package-level constants and PHP class constants are significant.
		return lang == "go" || lang == "php"
	case "variable", "static":
		// Only Go package-level vars are significant — ctags picks up
		// local variables in other languages.
		return lang == "go"
	default:
		return false
	}
}

// formatDependencyGraph builds a compact package dependency graph header.
// Only shows Go packages with import paths and at least one internal dependency.
func formatDependencyGraph(files []RankedFile) string {
	internalPkgs := make(map[string]bool)
	for _, f := range files {
		if f.ImportPath != "" {
			internalPkgs[f.ImportPath] = true
		}
	}
	if len(internalPkgs) < 2 {
		return ""
	}

	// Map package import path to its internal dependencies (deduped).
	pkgDeps := make(map[string]map[string]bool)
	for _, f := range files {
		if f.ImportPath == "" {
			continue
		}
		for _, imp := range f.Imports {
			if internalPkgs[imp] && imp != f.ImportPath {
				if pkgDeps[f.ImportPath] == nil {
					pkgDeps[f.ImportPath] = make(map[string]bool)
				}
				pkgDeps[f.ImportPath][imp] = true
			}
		}
	}

	if len(pkgDeps) == 0 {
		return ""
	}

	// Find shortest common prefix to trim for readability.
	allPaths := make([]string, 0, len(internalPkgs))
	for p := range internalPkgs {
		allPaths = append(allPaths, p)
	}
	sort.Strings(allPaths)
	prefix := longestCommonPrefix(allPaths)
	if idx := strings.LastIndex(prefix, "/"); idx >= 0 {
		prefix = prefix[:idx+1]
	}

	var b strings.Builder
	fmt.Fprint(&b, "### Dependencies\n")

	// Sort packages for deterministic output.
	sortedPkgs := make([]string, 0, len(pkgDeps))
	for p := range pkgDeps {
		sortedPkgs = append(sortedPkgs, p)
	}
	sort.Strings(sortedPkgs)

	for _, pkg := range sortedPkgs {
		deps := pkgDeps[pkg]
		depNames := make([]string, 0, len(deps))
		for d := range deps {
			depNames = append(depNames, strings.TrimPrefix(d, prefix))
		}
		sort.Strings(depNames)
		fmt.Fprintf(&b, "%s → %s\n", strings.TrimPrefix(pkg, prefix), strings.Join(depNames, ", "))
	}

	return b.String()
}
