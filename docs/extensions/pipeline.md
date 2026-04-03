# Pipeline

Run multi-step YAML workflows with params, loops, retries, and conditionals.

## Quick Start

```yaml
# ~/.config/piglet/pipelines/deploy.yaml
name: deploy
description: Build, test, and deploy the app
params:
  env:
    default: staging
    description: Target environment

steps:
  - name: build
    run: go build ./...

  - name: test
    run: go test ./...
    retries: 2
    retry_delay: 5

  - name: deploy
    run: ./scripts/deploy.sh {param.env}
    timeout: 120
```

```
# Run it
/pipe deploy --param env=production

# Or from the LLM
"Run the deploy pipeline targeting production"
→ calls pipeline tool with name="deploy", params={"env": "production"}
```

## What It Does

Pipeline loads YAML workflow files from `~/.config/piglet/pipelines/`, resolves parameters, expands template variables, and executes steps sequentially. Each step can iterate over a list (`each`) or a range (`loop`), run iterations in parallel, retry on failure, be skipped based on a shell conditional (`when`), and pass its output to subsequent steps via template variables. The tool returns a structured JSON result with per-step status, output, and timing.

## Capabilities

| Capability | Detail |
|------------|--------|
| `tools` | `pipeline`, `pipeline_list` |
| `commands` | `/pipe`, `/pipe-new` |
| `prompt` | Injects a "Pipelines" section at order 75 |

## Configuration

Pipelines are stored as YAML files in `~/.config/piglet/pipelines/`. Both `.yaml` and `.yml` extensions are recognized.

The prompt section content lives at `~/.config/piglet/extensions/pipeline/prompt.md`.

### Pipeline YAML Schema

```yaml
name: string          # required, unique name
description: string   # optional, shown in /pipe list
concurrency: int      # parallelism for loop/each iterations (default: 4)

params:
  <name>:
    default: string   # default value
    description: string
    required: bool    # error if not provided and no default

steps:
  - name: string      # required
    run: string       # required, shell command (supports template vars)
    description: string
    shell: string     # default: sh
    timeout: int      # seconds, default: 30
    retries: int      # retry count (0 = no retry)
    retry_delay: int  # seconds between retries (default: 5)
    allow_failure: bool # continue pipeline even if step fails
    workdir: string   # working directory (supports template vars)
    env:              # extra environment variables
      KEY: value
    when: string      # shell predicate — step skipped if exit non-zero
    each:             # list of items; step runs once per item
      - item1
      - item2
    loop:             # range iterations; step runs for each combination
      day: "-7d..-1d" # time range
      n: "1..5"       # numeric range
      env: [staging, production]  # explicit list
```

### Template Variables

| Variable | Description |
|----------|-------------|
| `{param.<name>}` | Parameter value |
| `{prev.stdout}` | Previous step's stdout |
| `{prev.lines}` | Previous step's stdout, trimmed |
| `{prev.status}` | Previous step's status (`ok` or `error`) |
| `{prev.json.<key>}` | Extract a top-level key from previous step's JSON output |
| `{step.<name>.stdout}` | Named step's stdout |
| `{step.<name>.status}` | Named step's status |
| `{item}` | Current iteration value (in `each` loops) |
| `{loop.<key>}` | Current loop variable value |
| `{cwd}` | Working directory at pipeline start |
| `{date}` | Pipeline start date as `2006-01-02` |
| `{timestamp}` | Pipeline start time as Unix seconds |

### Loop Range Syntax

| Format | Example | Expands to |
|--------|---------|------------|
| Numeric range | `"1..5"` | `1, 2, 3, 4, 5` |
| Reverse range | `"5..1"` | `5, 4, 3, 2, 1` |
| Time range | `"-7d..-1d"` | seven dates from 7 days ago to yesterday |
| Explicit list | `[a, b, c]` | `a, b, c` |
| Single value | `"prod"` | `prod` |

Multiple loop keys produce a cartesian product. Combining `each` and `loop` also produces a cartesian product.

## Tools Reference

### `pipeline`

Run a saved pipeline by name or provide an inline definition.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | one of `name`/`inline` | Name of a saved pipeline (loads `~/.config/piglet/pipelines/<name>.yaml`) |
| `inline` | object | one of `name`/`inline` | Ad-hoc pipeline definition using the same schema as YAML |
| `params` | object | no | Parameter overrides as string key-value pairs |
| `dry_run` | boolean | no | Preview steps without executing (default: false) |

```json
{
  "name": "deploy",
  "params": { "env": "production" }
}
```

```json
{
  "inline": {
    "name": "quick-check",
    "steps": [
      { "name": "lint", "run": "golangci-lint run ./..." },
      { "name": "test", "run": "go test ./..." }
    ]
  }
}
```

Returns a `PipelineResult` JSON object:

```json
{
  "name": "deploy",
  "status": "ok",
  "steps": [
    { "name": "build", "status": "ok", "output": "...", "duration_ms": 1240 },
    { "name": "test",  "status": "ok", "output": "...", "duration_ms": 3820, "retry_count": 1 }
  ],
  "duration_ms": 5060,
  "message": "2 steps completed in 5060ms"
}
```

Status values: `ok`, `error`, `partial` (some steps failed but `allow_failure` was set), `dry_run`.

---

### `pipeline_list`

List all saved pipelines in `~/.config/piglet/pipelines/`.

```json
{}
```

Returns a JSON array:

```json
[
  { "name": "deploy", "description": "Build, test, and deploy", "step_count": 3, "params": ["env"] },
  { "name": "release", "description": "Cut a release", "step_count": 5, "params": ["version"] }
]
```

## Commands Reference

### `/pipe`

Run a saved pipeline from the slash command interface. Displays step-by-step output as a formatted message.

```
/pipe <name> [--param key=value ...] [--dry-run]
```

```
/pipe deploy --param env=staging
/pipe release --param version=1.2.0 --dry-run
```

---

### `/pipe-new`

Create a new pipeline from a template. Writes a starter YAML file to `~/.config/piglet/pipelines/<name>.yaml` and shows the path.

```
/pipe-new <name>
```

```
/pipe-new weekly-report
```

The generated template includes a `root` parameter, a `hello` step, and a `list-files` step as a starting point.

## How It Works (Developer Notes)

**Init**: `e.OnInit` resolves `~/.config/piglet/` via `xdg.ConfigDir()` and stores it in `configDir`. The prompt section is loaded (or created with defaults) from `~/.config/piglet/extensions/pipeline/prompt.md`.

**Execution flow**: `Run(ctx, p, params)` merges defaults with overrides via `MergeParams`, validates required params and non-empty step names, then iterates steps sequentially. For each step, `executeStep` calls `ExpandIterations` — if no `each` or `loop` is set, the step runs as a single `executeSingle`; otherwise iterations run in parallel via `errgroup` bounded by `p.Concurrency`.

**Retries**: `executeSingle` loops `retries+1` times. On each retry it sleeps `retry_delay` seconds then re-runs the shell command. The final attempt's error and output are returned in `StepResult`.

**Conditionals**: `when` is expanded through `TemplateContext.Expand`, then run as a shell predicate via `shellPredicate` (5-second timeout). Exit 0 = run the step; non-zero = skip it.

**Output chaining**: After each step, `tc.Prev` and `tc.Steps[name]` are updated with stdout and status so subsequent steps can reference them in template expansions.

**Dry run**: `DryRun` expands the `run` template but does not shell-execute anything. It computes iteration counts via `ExpandIterations` and reports them in each step's `Iterations` field.

**InterruptBehavior**: The `pipeline` tool is registered with `InterruptBehavior: "block"` — the host waits for pipeline completion before processing the next message.

**Prompt section order**: 75 — appears after skills (25) and before RTK (90), so pipeline guidance is visible in mid-priority context.

## Related Extensions

- [bulk](bulk.md) — parallel one-shot commands across many items; simpler than pipelines for fan-out
- [scaffold](scaffold.md) — scaffolds extension boilerplate; does not run pipelines
