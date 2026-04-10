package safeguard

import "strings"

// stripSingleQuotes removes content inside single quotes where shell
// metacharacters are literal. Handles escaped single quotes (\').
func stripSingleQuotes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inSingle := false
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\'' && !inSingle {
			inSingle = true
			continue
		}
		if r == '\'' && inSingle {
			inSingle = false
			continue
		}
		if !inSingle {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// stripDoubleQuotes removes content inside double quotes.
// Handles backslash escaping within double quotes.
func stripDoubleQuotes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inDouble := false
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\\' && inDouble && i+1 < len(runes) {
			i++ // skip escaped char
			continue
		}
		if r == '"' {
			inDouble = !inDouble
			continue
		}
		if !inDouble {
			b.WriteRune(r)
		}
	}
	return b.String()
}
