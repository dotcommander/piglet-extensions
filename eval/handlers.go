package eval

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dotcommander/piglet/sdk"
)

// runSuite executes an evaluation suite and returns a formatted summary.
func runSuite(ctx context.Context, e *sdk.Extension, suiteName string, caseFilter []string) (string, error) {
	dir, err := suitesDir()
	if err != nil {
		return "", fmt.Errorf("resolve suites dir: %w", err)
	}

	suite, err := LoadSuite(filepath.Join(dir, suiteName+".yaml"))
	if err != nil {
		return "", fmt.Errorf("load suite: %w", err)
	}

	runner := NewRunner(e)
	result, err := runner.Run(ctx, suite, caseFilter)
	if err != nil {
		return "", fmt.Errorf("run suite: %w", err)
	}

	saved, _ := SaveResult(result)
	return formatRunSummary(result, saved), nil
}

// listSuitesText returns a formatted list of available evaluation suites.
func listSuitesText(e *sdk.Extension) (string, error) {
	dir, err := suitesDir()
	if err != nil {
		return "", fmt.Errorf("resolve suites dir: %w", err)
	}
	summaries, err := ListSuites(dir)
	if err != nil {
		return "", fmt.Errorf("list suites: %w", err)
	}
	if len(summaries) == 0 {
		return "No evaluation suites found.", nil
	}
	var b strings.Builder
	for _, s := range summaries {
		fmt.Fprintf(&b, "%s: %s (%d cases) — %s\n", s.Name, s.Description, s.CaseCount, s.Path)
	}
	return b.String(), nil
}

// compareResults loads two result files and returns a formatted comparison.
func compareResults(pathA, pathB string) (string, error) {
	a, err := LoadResult(pathA)
	if err != nil {
		return "", fmt.Errorf("load run_a: %w", err)
	}
	b, err := LoadResult(pathB)
	if err != nil {
		return "", fmt.Errorf("load run_b: %w", err)
	}
	return Compare(a, b).Format(), nil
}
