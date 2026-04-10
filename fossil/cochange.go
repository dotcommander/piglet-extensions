package fossil

import (
	"fmt"
	"sort"
	"strings"
)

// CoChangeEntry holds a file and the number of commits in which it co-changed
// with the target file.
type CoChangeEntry struct {
	File  string `json:"file"`
	Count int    `json:"count"`
}

// CoChange returns files that frequently change together with the given file,
// sorted by co-occurrence count descending.
func CoChange(cwd, file string, limit int) ([]CoChangeEntry, error) {
	if file == "" {
		return nil, fmt.Errorf("file is required")
	}

	// Phase 1: get commit SHAs that touched the file.
	shaOut, err := gitRun(cwd, defaultTimeout,
		"log", "--format=%h", "-100", "--", file,
	)
	if err != nil {
		return nil, fmt.Errorf("git log: %w", err)
	}
	if shaOut == "" {
		return nil, nil
	}

	// Phase 2: single git log --no-walk to get all files for all commits.
	// --no-walk shows each specified commit independently, listing all changed files.
	// --name-only shows each file once per commit, so counting occurrences across
	// all commits directly gives us the co-change frequency.
	shas := strings.Fields(shaOut)
	args := []string{"log", "--name-only", "--no-walk", "--format="}
	args = append(args, shas...)

	filesOut, err := gitRun(cwd, defaultTimeout, args...)
	if err != nil {
		return nil, fmt.Errorf("git log --no-walk: %w", err)
	}

	counts := make(map[string]int)
	for _, f := range strings.Split(filesOut, "\n") {
		f = strings.TrimSpace(f)
		if f != "" && f != file {
			counts[f]++
		}
	}

	entries := make([]CoChangeEntry, 0, len(counts))
	for f, c := range counts {
		entries = append(entries, CoChangeEntry{File: f, Count: c})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Count != entries[j].Count {
			return entries[i].Count > entries[j].Count
		}
		return entries[i].File < entries[j].File
	})

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	return entries, nil
}
