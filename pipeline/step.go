package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

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

	return executeIterated(ctx, step, tc, iters, concurrency, start)
}

// executeIterated runs multiple iterations of a step in parallel.
func executeIterated(ctx context.Context, step *Step, tc *TemplateContext, iters []Iteration, concurrency int, start time.Time) StepResult {
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
