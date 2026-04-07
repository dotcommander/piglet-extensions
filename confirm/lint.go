package confirm

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

func Lint(packages []string, root string) CheckResult {
	if len(packages) == 0 {
		return CheckResult{Name: "lint", Pass: true}
	}

	if _, err := exec.LookPath("golangci-lint"); err != nil {
		return CheckResult{Name: "lint", Pass: true, Output: "skipped: golangci-lint not found"}
	}

	// Convert import paths to relative paths using module prefix
	prefix := modulePrefix(root)
	relPaths := make([]string, len(packages))
	for i, pkg := range packages {
		if prefix != "" {
			pkg = strings.TrimPrefix(pkg, prefix+"/")
		}
		relPaths[i] = "./" + pkg
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	args := append([]string{"golangci-lint", "run", "--timeout=60s"}, relPaths...)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = root
	cmd.Env = cmdEnv()

	start := time.Now()
	out, err := cmd.CombinedOutput()
	elapsed := time.Since(start).Seconds()

	if err == nil {
		return CheckResult{Name: "lint", Pass: true, Elapsed: elapsed}
	}
	return CheckResult{Name: "lint", Pass: false, Output: capOutput(out, 4096), Elapsed: elapsed}
}
