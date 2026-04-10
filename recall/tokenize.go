package recall

import (
	"strings"
	"unicode"
)

// stopwords is the set of terms excluded during tokenization.
var stopwords = map[string]struct{}{
	"the": {}, "a": {}, "an": {}, "is": {}, "are": {}, "was": {}, "were": {},
	"in": {}, "on": {}, "at": {}, "to": {}, "for": {}, "of": {}, "and": {},
	"or": {}, "but": {}, "not": {}, "with": {}, "this": {}, "that": {},
	"it": {}, "be": {}, "as": {}, "by": {}, "from": {},
}

// tokenize lowercases text, splits on non-alphanumeric characters, removes
// stopwords, deduplicates, and filters tokens shorter than 2 characters.
func tokenize(text string) []string {
	lower := strings.ToLower(text)
	parts := strings.FieldsFunc(lower, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	seen := make(map[string]struct{}, len(parts))
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) < 2 {
			continue
		}
		if _, stop := stopwords[p]; stop {
			continue
		}
		if _, dup := seen[p]; dup {
			continue
		}
		seen[p] = struct{}{}
		result = append(result, p)
	}
	return result
}
