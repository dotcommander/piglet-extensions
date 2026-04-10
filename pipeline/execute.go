package pipeline

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

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
