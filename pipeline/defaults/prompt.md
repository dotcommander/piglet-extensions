Use the pipeline tool to run multi-step workflows. Pipelines are YAML files in ~/.config/piglet/pipelines/.

## Step Fields

| Field | Description |
|-------|-------------|
| `name` | Step identifier (required, must be unique across steps/parallel/finally) |
| `run` | Shell command to execute (required) |
| `timeout` | Seconds before kill (default: 30) |
| `retries` | Max retry attempts on failure |
| `retry_delay` | Seconds between retries (default: 5) |
| `allow_failure` | Continue pipeline on error (default: false) |
| `when` | Shell predicate — skip if non-zero exit |
| `workdir` | Working directory for the command |
| `env` | Environment variables (map) |
| `shell` | Shell binary (default: sh) |
| `each` | List of values for iteration (`{item}`) |
| `loop` | Ranges for iteration (`{loop.<key>}`): numeric `1..5`, day offsets `-7d..-1d`, explicit lists |
| `max_output` | Truncate output to N bytes (default: 0 = unlimited) |
| `output_format` | Validate output: `text` (default) or `json` |

## Pipeline Fields

| Field | Description |
|-------|-------------|
| `name` | Pipeline identifier (required) |
| `description` | What this pipeline does |
| `params` | Parameters with `default`, `description`, `required` |
| `steps` | Sequential step list |
| `parallel` | Groups of concurrent steps: `[[step, step], [step, step]]` |
| `finally` | Cleanup steps that always run after steps and parallel, regardless of success or failure |
| `on_error` | `halt` (default) or `continue` — pipeline-level error policy |
| `concurrency` | Max parallel iterations (default: 4) |

## Template Variables

| Variable | Resolves to |
|----------|-------------|
| `{param.<name>}` | Pipeline parameter value |
| `{prev.stdout}` | Previous step's output |
| `{prev.json.<key>}` | Top-level JSON field from previous step |
| `{prev.status}` | Previous step's status (ok/error) |
| `{step.<name>.stdout}` | Named step's output |
| `{step.<name>.status}` | Named step's status |
| `{item}` | Current each-loop value |
| `{loop.<key>}` | Current loop variable |
| `{cwd}` | Current working directory |
| `{date}` | Date as YYYY-MM-DD |
| `{timestamp}` | Unix timestamp |

## Examples

### Basic with output chaining

```yaml
name: build-check
steps:
  - name: build
    run: go build ./...
  - name: test
    run: go test ./...
  - name: report
    run: echo "build={step.build.status} test={prev.status}"
```

### JSON output with format validation

```yaml
name: api-health
steps:
  - name: check
    run: curl -sf http://api.example.com/health
    output_format: json
    max_output: 4096
  - name: report
    run: echo "API status is {prev.json.status}"
```

### Error handling: on_error + finally

```yaml
name: integration-test
on_error: continue
steps:
  - name: start-db
    run: docker run -d --name test-db -p 5432:5432 postgres:15
  - name: migrate
    run: go run ./cmd/migrate
  - name: test
    run: go test -tags=integration ./...
    timeout: 120
finally:
  - name: stop-db
    run: docker stop test-db && docker rm test-db
```

### Parallel step groups

```yaml
name: full-ci
steps:
  - name: build
    run: go build ./...
parallel:
  - - name: lint
      run: golangci-lint run ./...
    - name: vet
      run: go vet ./...
    - name: staticcheck
      run: staticcheck ./...
  - - name: test-unit
      run: go test -short ./...
    - name: test-integration
      run: go test -tags=integration ./...
```

### Loops with day ranges

```yaml
name: daily-report
steps:
  - name: fetch
    loop:
      day: -7d..-1d
    run: curl -sf "https://api.example.com/events?date={loop.day}" >> results.json
    max_output: 65536
```

### Token budget control

```yaml
name: verbose-test
steps:
  - name: test
    run: go test -v -json ./...
    max_output: 16384
    timeout: 120
    output_format: json
```

Use pipeline_list to discover available pipelines. Use /pipe-new to create templates.
