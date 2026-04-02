package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Pipeline is a multi-step workflow loaded from YAML.
type Pipeline struct {
	Name        string           `yaml:"name"        json:"name"`
	Description string           `yaml:"description" json:"description"`
	Params      map[string]Param `yaml:"params"      json:"params,omitzero"`
	Steps       []Step           `yaml:"steps"        json:"steps"`
	Concurrency int              `yaml:"concurrency"  json:"concurrency,omitzero"` // for loop/each parallelism
	OnError     string           `yaml:"on_error"     json:"on_error,omitempty"`   // "halt" (default) | "continue"
	Finally     []Step           `yaml:"finally"      json:"finally,omitzero"`
	Parallel    [][]Step         `yaml:"parallel"     json:"parallel,omitzero"`
}

// Param is a pipeline parameter with optional default.
type Param struct {
	Default     string `yaml:"default"     json:"default,omitempty"`
	Description string `yaml:"description" json:"description,omitempty"`
	Required    bool   `yaml:"required"    json:"required,omitempty"`
}

// Step is a single unit of work in a pipeline.
type Step struct {
	Name         string            `yaml:"name"          json:"name"`
	Run          string            `yaml:"run"           json:"run"`
	Description  string            `yaml:"description"   json:"description,omitempty"`
	Shell        string            `yaml:"shell"         json:"shell,omitempty"`
	Timeout      int               `yaml:"timeout"       json:"timeout,omitempty"` // seconds
	Retries      int               `yaml:"retries"       json:"retries,omitempty"`
	RetryDelay   int               `yaml:"retry_delay"   json:"retry_delay,omitempty"` // seconds
	AllowFailure bool              `yaml:"allow_failure" json:"allow_failure,omitempty"`
	Each         []string          `yaml:"each"          json:"each,omitempty"`
	Loop         map[string]any    `yaml:"loop"          json:"loop,omitempty"`
	WorkDir      string            `yaml:"workdir"       json:"workdir,omitempty"`
	Env          map[string]string `yaml:"env"           json:"env,omitempty"`
	When         string            `yaml:"when"          json:"when,omitempty"`
	MaxOutput    int               `yaml:"max_output"    json:"max_output,omitempty"`    // bytes, 0 = unlimited
	OutputFormat string            `yaml:"output_format" json:"output_format,omitempty"` // "text" | "json"
}

// Status constants for step and pipeline results.
const (
	StatusOK      = "ok"
	StatusError   = "error"
	StatusSkipped = "skipped"
	StatusPartial = "partial"
	StatusDryRun  = "dry_run"
)

// StepResult is the outcome of executing one step.
type StepResult struct {
	Name       string `json:"name"`
	Status     string `json:"status"` // StatusOK, StatusError, StatusSkipped
	Output     string `json:"output"`
	Error      string `json:"error,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	Iterations int    `json:"iterations,omitempty"`
	RetryCount int    `json:"retry_count,omitempty"`
	Parsed     any    `json:"parsed,omitempty"` // pre-parsed JSON when OutputFormat="json"
}

// PipelineResult is the aggregate outcome of a pipeline run.
type PipelineResult struct {
	Name       string       `json:"name"`
	Status     string       `json:"status"` // "ok", "error", "partial"
	Steps      []StepResult `json:"steps"`
	DurationMS int64        `json:"duration_ms"`
	Message    string       `json:"message"`
}

// defaults fills zero-value fields.
func (p *Pipeline) defaults() {
	if p.Concurrency <= 0 {
		p.Concurrency = 4
	}
	for i := range p.Steps {
		applyStepDefaults(&p.Steps[i])
	}
	for i := range p.Finally {
		applyStepDefaults(&p.Finally[i])
	}
	for _, group := range p.Parallel {
		for i := range group {
			applyStepDefaults(&group[i])
		}
	}
	if p.OnError == "" {
		p.OnError = "halt"
	}
}

// applyStepDefaults fills zero-value fields on a single step.
func applyStepDefaults(s *Step) {
	if s.Shell == "" {
		s.Shell = "sh"
	}
	if s.Timeout <= 0 {
		s.Timeout = 30
	}
	if s.RetryDelay <= 0 && s.Retries > 0 {
		s.RetryDelay = 5
	}
}

// StepTimeout returns the step's timeout as a Duration.
func (s *Step) StepTimeout() time.Duration {
	if s.Timeout <= 0 {
		return 30 * time.Second
	}
	return time.Duration(s.Timeout) * time.Second
}

// Validate checks that a pipeline is well-formed.
func (p *Pipeline) Validate(params map[string]string) error {
	if p.Name == "" {
		return fmt.Errorf("pipeline name is required")
	}
	if len(p.Steps) == 0 && len(p.Parallel) == 0 {
		return fmt.Errorf("pipeline %q has no steps or parallel groups", p.Name)
	}
	for name, param := range p.Params {
		if param.Required {
			if _, ok := params[name]; !ok {
				if param.Default == "" {
					return fmt.Errorf("required parameter %q not provided", name)
				}
			}
		}
	}
	switch p.OnError {
	case "", "halt", "continue":
		// valid
	default:
		return fmt.Errorf("invalid on_error %q (must be halt or continue)", p.OnError)
	}

	// Collect names for collision detection across Steps, Parallel, Finally.
	totalSteps := len(p.Steps) + len(p.Finally)
	for _, g := range p.Parallel {
		totalSteps += len(g)
	}
	names := make(map[string]bool, totalSteps)
	for i, s := range p.Steps {
		if s.Name == "" {
			return fmt.Errorf("step %d has no name", i)
		}
		if s.Run == "" {
			return fmt.Errorf("step %q has no run command", s.Name)
		}
		if names[s.Name] {
			return fmt.Errorf("duplicate step name %q", s.Name)
		}
		names[s.Name] = true
		if err := validateStepFormat(s.Name, s.OutputFormat); err != nil {
			return err
		}
	}
	for gi, group := range p.Parallel {
		for i, step := range group {
			if step.Name == "" {
				return fmt.Errorf("parallel group %d step %d has no name", gi, i)
			}
			if step.Run == "" {
				return fmt.Errorf("parallel step %q has no run command", step.Name)
			}
			if names[step.Name] {
				return fmt.Errorf("parallel group %d has duplicate step name %q", gi, step.Name)
			}
			names[step.Name] = true
			if err := validateStepFormat(step.Name, step.OutputFormat); err != nil {
				return err
			}
		}
	}
	for _, step := range p.Finally {
		if step.Name == "" {
			return fmt.Errorf("finally step has no name")
		}
		if step.Run == "" {
			return fmt.Errorf("finally step %q has no run command", step.Name)
		}
		if names[step.Name] {
			return fmt.Errorf("finally has duplicate step name %q", step.Name)
		}
		names[step.Name] = true
		if err := validateStepFormat(step.Name, step.OutputFormat); err != nil {
			return err
		}
	}
	return nil
}

// validateStepFormat checks that a step's output_format is valid.
func validateStepFormat(stepName, format string) error {
	switch format {
	case "", "text", "json":
		return nil
	default:
		return fmt.Errorf("step %q has invalid output_format %q (must be text or json)", stepName, format)
	}
}

// MergeParams merges pipeline defaults with user-supplied overrides.
// Returns the final parameter map.
func (p *Pipeline) MergeParams(overrides map[string]string) map[string]string {
	merged := make(map[string]string, len(p.Params))
	for name, param := range p.Params {
		if param.Default != "" {
			merged[name] = param.Default
		}
	}
	for k, v := range overrides {
		merged[k] = v
	}
	return merged
}

// LoadFile reads a pipeline from a YAML file.
func LoadFile(path string) (*Pipeline, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read pipeline %s: %w", path, err)
	}
	var p Pipeline
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parse pipeline %s: %w", path, err)
	}
	p.defaults()
	return &p, nil
}

// LoadDir reads all .yaml pipeline files from a directory.
// Returns pipelines indexed by name.
func LoadDir(dir string) ([]*Pipeline, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read pipeline dir %s: %w", dir, err)
	}
	var pipes []*Pipeline
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".yaml" && filepath.Ext(name) != ".yml" {
			continue
		}
		p, err := LoadFile(filepath.Join(dir, name))
		if err != nil {
			continue // skip malformed files
		}
		pipes = append(pipes, p)
	}
	return pipes, nil
}
