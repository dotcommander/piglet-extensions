package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dotcommander/piglet/sdk"
)

// resolvePipelinePath finds a pipeline file by name, trying .yaml then .yml.
func resolvePipelinePath(name string) (string, error) {
	dir := filepath.Join(configDir, pipelinesDir)
	for _, ext := range []string{".yaml", ".yml"} {
		path := filepath.Join(dir, name+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("pipeline %q not found in %s", name, dir)
}

func executePipeline(ctx context.Context, args map[string]any) (*sdk.ToolResult, error) {
	params := make(map[string]string)
	if raw, ok := args["params"].(map[string]any); ok {
		for k, v := range raw {
			params[k] = fmt.Sprint(v)
		}
	}
	dryRun, _ := args["dry_run"].(bool)

	var p *Pipeline

	if name, ok := args["name"].(string); ok && name != "" {
		path, err := resolvePipelinePath(name)
		if err != nil {
			return sdk.ErrorResult(err.Error()), nil
		}
		p, err = LoadFile(path)
		if err != nil {
			return sdk.ErrorResult(fmt.Sprintf("load pipeline %q: %s", name, err)), nil
		}
	} else if inline, ok := args["inline"].(map[string]any); ok {
		data, err := json.Marshal(inline)
		if err != nil {
			return sdk.ErrorResult(fmt.Sprintf("marshal inline pipeline: %s", err)), nil
		}
		p = new(Pipeline)
		if err := json.Unmarshal(data, p); err != nil {
			return sdk.ErrorResult(fmt.Sprintf("parse inline pipeline: %s", err)), nil
		}
	} else {
		return sdk.ErrorResult("provide either 'name' (saved pipeline) or 'inline' (ad-hoc definition)"), nil
	}

	var result *PipelineResult
	var err error

	if dryRun {
		result, err = DryRun(p, params)
	} else {
		result, err = Run(ctx, p, params)
	}
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("pipeline error: %s", err)), nil
	}

	data, err := json.Marshal(result)
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("marshal result: %s", err)), nil
	}
	return sdk.TextResult(string(data)), nil
}

func listPipelines() (*sdk.ToolResult, error) {
	dir := filepath.Join(configDir, pipelinesDir)
	pipes, err := LoadDir(dir)
	if err != nil {
		return sdk.ErrorResult(fmt.Sprintf("list pipelines: %s", err)), nil
	}
	if len(pipes) == 0 {
		return sdk.TextResult("No pipelines found in " + dir), nil
	}

	type entry struct {
		Name         string   `json:"name"`
		Description  string   `json:"description"`
		StepCount    int      `json:"step_count"`
		FinallyCount int      `json:"finally_count,omitempty"`
		Params       []string `json:"params,omitempty"`
	}

	entries := make([]entry, len(pipes))
	for i, p := range pipes {
		var paramNames []string
		for name := range p.Params {
			paramNames = append(paramNames, name)
		}
		entries[i] = entry{
			Name:         p.Name,
			Description:  p.Description,
			StepCount:    len(p.Steps),
			FinallyCount: len(p.Finally),
			Params:       paramNames,
		}
	}

	data, _ := json.Marshal(entries)
	return sdk.TextResult(string(data)), nil
}

func handlePipeCommand(ctx context.Context, e *sdk.Extension, args string) error {
	parts := strings.Fields(args)
	if len(parts) == 0 {
		e.ShowMessage("Usage: /pipe <name> [--param key=value ...] [--dry-run]")
		return nil
	}

	name := parts[0]
	params := make(map[string]string)
	dryRun := false

	for i := 1; i < len(parts); i++ {
		switch {
		case parts[i] == "--dry-run":
			dryRun = true
		case parts[i] == "--param" && i+1 < len(parts):
			i++
			k, v, ok := strings.Cut(parts[i], "=")
			if ok {
				params[k] = v
			}
		}
	}

	path, err := resolvePipelinePath(name)
	if err != nil {
		e.ShowMessage(err.Error())
		return nil
	}

	p, err := LoadFile(path)
	if err != nil {
		e.ShowMessage(fmt.Sprintf("Error loading pipeline %q: %s", name, err))
		return nil
	}

	e.ShowMessage(fmt.Sprintf("Running pipeline %q (%d steps, %d finally)...", p.Name, len(p.Steps), len(p.Finally)))

	var result *PipelineResult
	if dryRun {
		result, err = DryRun(p, params)
	} else {
		result, err = Run(ctx, p, params)
	}
	if err != nil {
		e.ShowMessage(fmt.Sprintf("Pipeline error: %s", err))
		return nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Pipeline: %s\n\n", result.Name))
	inFinally := false
	for _, sr := range result.Steps {
		if strings.HasPrefix(sr.Name, "finally:") && !inFinally {
			sb.WriteString("--- cleanup ---\n\n")
			inFinally = true
		}
		stepName := sr.Name
		if strings.HasPrefix(sr.Name, "finally:") {
			stepName = strings.TrimPrefix(sr.Name, "finally:")
		}
		icon := "+"
		if sr.Status == "error" {
			icon = "x"
		} else if sr.Status == "skipped" {
			icon = "-"
		}
		sb.WriteString(fmt.Sprintf("[%s] %s (%dms)\n", icon, stepName, sr.DurationMS))
		if sr.Output != "" {
			sb.WriteString(TruncateUTF8(sr.Output, 500) + "\n")
		}
		if sr.Error != "" {
			sb.WriteString(fmt.Sprintf("  error: %s\n", sr.Error))
		}
		sb.WriteString("\n")
	}
	sb.WriteString(fmt.Sprintf("**Result**: %s — %s", result.Status, result.Message))
	e.ShowMessage(sb.String())
	return nil
}

func handlePipeNewCommand(e *sdk.Extension, args string) error {
	name := strings.TrimSpace(args)
	if name == "" {
		e.ShowMessage("Usage: /pipe-new <name>")
		return nil
	}

	dir := filepath.Join(configDir, pipelinesDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		e.ShowMessage(fmt.Sprintf("Error creating pipelines dir: %s", err))
		return nil
	}

	path := filepath.Join(dir, name+".yaml")
	if _, err := os.Stat(path); err == nil {
		e.ShowMessage(fmt.Sprintf("Pipeline %q already exists at %s", name, path))
		return nil
	}

	template := fmt.Sprintf(`name: %s
description: TODO — describe what this pipeline does
params:
  root:
    default: "."
    description: Root directory

steps:
  - name: hello
    run: echo "Hello from pipeline %s"
    description: A starter step

  - name: list-files
    run: ls -la {param.root} | head -10
    description: List files in root directory
`, name, name)

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(template), 0o644); err != nil {
		e.ShowMessage(fmt.Sprintf("Error writing template: %s", err))
		return nil
	}
	if err := os.Rename(tmp, path); err != nil {
		e.ShowMessage(fmt.Sprintf("Error saving template: %s", err))
		os.Remove(tmp)
		return nil
	}

	e.ShowMessage(fmt.Sprintf("Created pipeline template at:\n%s\n\nEdit the file, then run: /pipe %s", path, name))
	return nil
}
