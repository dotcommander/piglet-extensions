package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/sync/errgroup"
)

// Run executes a pipeline sequentially, then parallel groups, then finally steps.
// Returns the aggregate result.
func Run(ctx context.Context, p *Pipeline, params map[string]string) (*PipelineResult, error) {
	p.defaults()

	merged := p.MergeParams(params)
	if err := p.Validate(merged); err != nil {
		return nil, err
	}

	cwd, _ := os.Getwd()
	now := time.Now()

	tc := &TemplateContext{
		Params:    merged,
		Steps:     make(map[string]*StepOutput),
		CWD:       cwd,
		StartTime: now,
	}

	start := now
	result := &PipelineResult{
		Name:  p.Name,
		Steps: make([]StepResult, 0, len(p.Steps)+len(p.Finally)),
	}

	okCount, errCount, skipCount := runMainSteps(ctx, p, tc, result)

	// Execute parallel groups
	for gi, group := range p.Parallel {
		groupResults, gOk, gErr := runParallelGroup(ctx, group, tc, p.Concurrency)
		result.Steps = append(result.Steps, groupResults...)
		okCount += gOk
		errCount += gErr

		for i, sr := range groupResults {
			tc.Steps[group[i].Name] = &StepOutput{
				Stdout: sr.Output,
				Status: sr.Status,
				Parsed: sr.Parsed,
			}
		}
		if len(groupResults) > 0 {
			lastSR := groupResults[len(groupResults)-1]
			tc.Prev = &StepOutput{
				Stdout: lastSR.Output,
				Status: lastSR.Status,
				Parsed: lastSR.Parsed,
			}
		}

		if gErr > 0 && p.OnError != "continue" {
			allAllowed := true
			for i, sr := range groupResults {
				if sr.Status == StatusError && !group[i].AllowFailure {
					allAllowed = false
					break
				}
			}
			if !allAllowed {
				result.Status = StatusError
				result.DurationMS = time.Since(start).Milliseconds()
				result.Message = fmt.Sprintf("%d ok, %d failed (halted at parallel group %d)", okCount, errCount, gi)
				break
			}
		}
	}

	// Always run finally steps
	finallyErrors := 0
	if len(p.Finally) > 0 {
		finallyResults := runFinally(ctx, p.Finally, tc, p.Concurrency)
		result.Steps = append(result.Steps, finallyResults...)
		for _, fr := range finallyResults {
			if fr.Status == StatusError {
				finallyErrors++
			}
		}
	}

	// Set final status if not already set (e.g., by parallel group halt)
	if result.Status == "" {
		if errCount > 0 {
			result.Status = StatusPartial
			result.Message = fmt.Sprintf("%d ok, %d failed (allowed), %d skipped", okCount, errCount, skipCount)
		} else {
			result.Status = StatusOK
			result.Message = fmt.Sprintf("%d steps completed in %dms", okCount+skipCount, time.Since(start).Milliseconds())
		}
	}

	// Append finally error info to message (does not change status)
	if finallyErrors > 0 {
		result.Message += fmt.Sprintf(" (finally: %d errors)", finallyErrors)
	}
	result.DurationMS = time.Since(start).Milliseconds()
	return result, nil
}

// runMainSteps executes the sequential Steps of a pipeline.
// Returns ok, error, and skip counts.
func runMainSteps(ctx context.Context, p *Pipeline, tc *TemplateContext, result *PipelineResult) (int, int, int) {
	var okCount, errCount, skipCount int

	for _, step := range p.Steps {
		if step.When != "" {
			when := tc.Expand(step.When)
			workDir := step.WorkDir
			if workDir != "" {
				workDir = tc.Expand(workDir)
			}
			if !shellPredicate(ctx, when, workDir) {
				sr := StepResult{
					Name:   step.Name,
					Status: StatusSkipped,
					Output: fmt.Sprintf("when predicate failed: %s", step.When),
				}
				result.Steps = append(result.Steps, sr)
				skipCount++
				continue
			}
		}

		sr := executeStep(ctx, &step, tc, p.Concurrency)
		result.Steps = append(result.Steps, sr)

		tc.Steps[step.Name] = &StepOutput{
			Stdout: sr.Output,
			Status: sr.Status,
			Parsed: sr.Parsed,
		}
		tc.Prev = &StepOutput{
			Stdout: sr.Output,
			Status: sr.Status,
			Parsed: sr.Parsed,
		}

		if sr.Status == StatusOK {
			okCount++
		} else {
			errCount++
			effectiveAllow := step.AllowFailure || p.OnError == "continue"
			if !effectiveAllow {
				result.Status = StatusError
				result.Message = fmt.Sprintf("%d ok, %d failed (halted at %q), %d skipped",
					okCount, errCount, step.Name, len(p.Steps)-len(result.Steps))
				return okCount, errCount, skipCount
			}
		}
	}

	return okCount, errCount, skipCount
}

// DryRun returns a preview of what would execute without running anything.
func DryRun(p *Pipeline, params map[string]string) (*PipelineResult, error) {
	p.defaults()

	merged := p.MergeParams(params)
	if err := p.Validate(merged); err != nil {
		return nil, err
	}

	cwd, _ := os.Getwd()
	tc := &TemplateContext{
		Params:    merged,
		Steps:     make(map[string]*StepOutput),
		CWD:       cwd,
		StartTime: time.Now(),
	}

	result := &PipelineResult{
		Name:   p.Name,
		Status: StatusDryRun,
		Steps:  make([]StepResult, 0, len(p.Steps)),
	}

	for _, step := range p.Steps {
		expanded := tc.Expand(step.Run)

		iters, _ := ExpandIterations(&step)
		iterCount := 1
		if len(iters) > 0 {
			iterCount = len(iters)
		}

		sr := StepResult{
			Name:       step.Name,
			Status:     StatusSkipped,
			Output:     fmt.Sprintf("dry run — would run: %s", strings.TrimSpace(expanded)),
			Iterations: iterCount,
		}
		result.Steps = append(result.Steps, sr)

		tc.Prev = &StepOutput{Stdout: "(dry run)", Status: StatusOK}
		tc.Steps[step.Name] = tc.Prev
	}

	// Preview parallel groups
	for gi, group := range p.Parallel {
		for _, step := range group {
			expanded := tc.Expand(step.Run)
			sr := StepResult{
				Name:   fmt.Sprintf("parallel:%d:%s", gi, step.Name),
				Status: StatusSkipped,
				Output: fmt.Sprintf("dry run — would run in parallel group %d: %s", gi, strings.TrimSpace(expanded)),
			}
			result.Steps = append(result.Steps, sr)
		}
		tc.Prev = &StepOutput{Stdout: "(dry run)", Status: StatusOK}
	}

	// Preview finally steps
	for _, step := range p.Finally {
		expanded := tc.Expand(step.Run)
		sr := StepResult{
			Name:   "finally:" + step.Name,
			Status: StatusSkipped,
			Output: fmt.Sprintf("dry run — would run finally: %s", strings.TrimSpace(expanded)),
		}
		result.Steps = append(result.Steps, sr)
		tc.Steps["finally:"+step.Name] = &StepOutput{Stdout: "(dry run)", Status: StatusOK}
	}

	result.Message = fmt.Sprintf("dry run: %d steps would execute", len(result.Steps))
	return result, nil
}

// runFinally executes cleanup steps that always run regardless of pipeline outcome.
// When predicates are NOT evaluated for finally steps.
func runFinally(ctx context.Context, steps []Step, tc *TemplateContext, concurrency int) []StepResult {
	results := make([]StepResult, 0, len(steps))
	for _, step := range steps {
		sr := executeStep(ctx, &step, tc, concurrency)
		sr.Name = "finally:" + sr.Name
		results = append(results, sr)
		tc.Steps["finally:"+step.Name] = &StepOutput{
			Stdout: sr.Output,
			Status: sr.Status,
			Parsed: sr.Parsed,
		}
	}
	return results
}

// runParallelGroup executes a slice of steps concurrently using errgroup.
func runParallelGroup(ctx context.Context, group []Step, tc *TemplateContext, concurrency int) ([]StepResult, int, int) {
	results := make([]StepResult, len(group))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	for i := range group {
		g.Go(func() error {
			results[i] = executeStep(gctx, &group[i], tc, concurrency)
			return nil
		})
	}
	_ = g.Wait()

	var okCount, errCount int
	for _, sr := range results {
		switch sr.Status {
		case StatusOK:
			okCount++
		case StatusError:
			errCount++
		}
	}
	return results, okCount, errCount
}

// executeStep runs a single step, handling iterations and retries.
func executeStep(ctx context.Context, step *Step, tc *TemplateContext, concurrency int) StepResult {
	start := time.Now()

	iters, err := ExpandIterations(step)
	if err != nil {
		return StepResult{
			Name:       step.Name,
			Status:     StatusError,
			Error:      err.Error(),
			DurationMS: time.Since(start).Milliseconds(),
		}
	}

	if iters == nil {
		return executeSingle(ctx, step, tc, start)
	}

	if concurrency <= 0 {
		concurrency = 4
	}

	type iterResult struct {
		idx    int
		output string
		err    error
	}

	results := make([]iterResult, len(iters))
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(concurrency)

	for i, iter := range iters {
		// Deep copy TemplateContext so iterations don't share mutable maps
		iterTC := tc.Clone()
		iterTC.Item = iter.Item
		iterTC.HasItem = true
		iterTC.LoopVars = iter.LoopVars

		g.Go(func() error {
			expanded := iterTC.Expand(step.Run)
			workDir := step.WorkDir
			if workDir != "" {
				workDir = iterTC.Expand(workDir)
			}

			out, execErr := shellRun(gctx, step.Shell, expanded, workDir, step.Env, step.StepTimeout())
			results[i] = iterResult{idx: i, output: out, err: execErr}
			return nil
		})
	}

	_ = g.Wait()

	var outputs []string
	var hasErr bool
	for _, r := range results {
		if r.err != nil {
			hasErr = true
			outputs = append(outputs, fmt.Sprintf("[%d] error: %s", r.idx, r.err))
		} else {
			out := r.output
			if step.MaxOutput > 0 {
				out = TruncateUTF8(out, step.MaxOutput)
			}
			outputs = append(outputs, out)
		}
	}

	status := StatusOK
	errMsg := ""
	if hasErr {
		status = StatusError
		errMsg = "one or more iterations failed"
	}

	joined := strings.Join(outputs, "\n")
	parsed, parseErr := validateAndParseOutput(joined, step.OutputFormat)
	if parseErr != nil {
		return StepResult{
			Name:       step.Name,
			Status:     StatusError,
			Output:     joined,
			Error:      parseErr.Error(),
			DurationMS: time.Since(start).Milliseconds(),
			Iterations: len(iters),
		}
	}

	return StepResult{
		Name:       step.Name,
		Status:     status,
		Output:     joined,
		Error:      errMsg,
		DurationMS: time.Since(start).Milliseconds(),
		Iterations: len(iters),
		Parsed:     parsed,
	}
}

// executeSingle runs one step command with retry support.
func executeSingle(ctx context.Context, step *Step, tc *TemplateContext, start time.Time) StepResult {
	expanded := tc.Expand(step.Run)
	workDir := step.WorkDir
	if workDir != "" {
		workDir = tc.Expand(workDir)
	}

	var lastErr error
	var output string
	retries := 0

	for attempt := range step.Retries + 1 {
		if attempt > 0 {
			time.Sleep(time.Duration(step.RetryDelay) * time.Second)
			retries++
		}

		out, err := shellRun(ctx, step.Shell, expanded, workDir, step.Env, step.StepTimeout())
		if err == nil {
			out = strings.TrimSpace(out)
			if step.MaxOutput > 0 {
				out = TruncateUTF8(out, step.MaxOutput)
			}

			parsed, parseErr := validateAndParseOutput(out, step.OutputFormat)
			if parseErr != nil {
				return StepResult{
					Name:       step.Name,
					Status:     StatusError,
					Output:     out,
					Error:      parseErr.Error(),
					DurationMS: time.Since(start).Milliseconds(),
					RetryCount: retries,
				}
			}
			return StepResult{
				Name:       step.Name,
				Status:     StatusOK,
				Output:     out,
				DurationMS: time.Since(start).Milliseconds(),
				RetryCount: retries,
				Parsed:     parsed,
			}
		}
		lastErr = err
		output = out
	}

	errOutput := strings.TrimSpace(output)
	if step.MaxOutput > 0 {
		errOutput = TruncateUTF8(errOutput, step.MaxOutput)
	}
	return StepResult{
		Name:       step.Name,
		Status:     StatusError,
		Output:     errOutput,
		Error:      lastErr.Error(),
		DurationMS: time.Since(start).Milliseconds(),
		RetryCount: retries,
	}
}

// validateAndParseOutput validates step output against its declared format.
func validateAndParseOutput(output string, format string) (any, error) {
	if format != "json" {
		return nil, nil
	}
	if output == "" {
		return nil, fmt.Errorf("output_format is json but output is empty")
	}
	var parsed any
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		return nil, fmt.Errorf("output_format is json but output is not valid JSON: %w", err)
	}
	return parsed, nil
}

// shellRun executes a shell command and returns stdout+stderr combined.
func shellRun(ctx context.Context, shell, command, workDir string, env map[string]string, timeout time.Duration) (string, error) {
	if shell == "" {
		shell = "sh"
	}

	cmdCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, shell, "-c", command)
	if workDir != "" {
		cmd.Dir = workDir
	}

	if len(env) > 0 {
		cmd.Env = cmd.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, k+"="+v)
		}
	}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Run(); err != nil {
		return buf.String(), fmt.Errorf("%w: %s", err, strings.TrimSpace(buf.String()))
	}
	return buf.String(), nil
}

// shellPredicate runs a shell command and returns true if exit code is 0.
func shellPredicate(ctx context.Context, command, workDir string) bool {
	cmdCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", command)
	if workDir != "" {
		cmd.Dir = workDir
	}
	return cmd.Run() == nil
}

// TruncateUTF8 safely truncates a string to at most maxBytes without splitting
// a multi-byte character. The suffix is included within the budget.
func TruncateUTF8(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	const suffix = "... (truncated)"
	cut := maxBytes - len(suffix)
	if cut <= 0 {
		return s[:maxBytes]
	}
	// Walk back from cut to find a valid UTF-8 boundary
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + suffix
}
