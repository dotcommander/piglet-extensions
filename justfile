# Extensions & CLI build targets for piglet-extensions
# Usage: just <target>

ext_dir  := env_var_or_default("PIGLET_EXTENSIONS_DIR", env_var("HOME") / ".config/piglet/extensions")
cfg_dir  := env_var_or_default("PIGLET_CONFIG_DIR", env_var("HOME") / ".config/piglet")
cli_dir  := env_var_or_default("GOPATH", env_var("HOME") / "go") / "bin"

# ── Default ────────────────────────────────────────────────────────────

default: build

# Build everything: extensions + cli + packs
build: extensions cli packs

# ── Extensions ─────────────────────────────────────────────────────────

# Build and install all extensions
extensions: \
    extensions-safeguard \
    extensions-rtk \
    extensions-autotitle \
    extensions-clipboard \
    extensions-skill \
    extensions-memory \
    extensions-subagent \
    extensions-lsp \
    extensions-repomap \
    extensions-plan \
    extensions-bulk \
    extensions-modelsdev \
    extensions-mcp \
    extensions-usage \
    extensions-gitcontext \
    extensions-prompts \
    extensions-behavior \
    extensions-export \
    extensions-admin \
    extensions-scaffold \
    extensions-undo \
    extensions-session-tools \
    extensions-background \
    extensions-extensions-list \
    extensions-pipeline \
    extensions-webfetch \
    extensions-loop \
    extensions-inbox \
    extensions-sift \
    extensions-provider \
    extensions-suggest \
    extensions-cron \
    extensions-tokengate \
    extensions-coordinator \
    extensions-route \
    extensions-changelog \
    extensions-tasklist
    @echo "Extensions installed to {{ext_dir}}"

# Build and install one extension (internal recipe)
[private]
build-ext ext:
    #!/usr/bin/env bash
    set -euo pipefail
    mkdir -p "{{ext_dir}}/{{ext}}"
    GOWORK=off go build -o "{{ext_dir}}/{{ext}}/{{ext}}" "./{{ext}}/cmd/"
    cp "{{ext}}/cmd/manifest.yaml" "{{ext_dir}}/{{ext}}/"
    awk '/^defaults:/{d=1;next} d&&/^  - src:/{src=$3} d&&/^    dest:/{print src,$2} d&&/^[^ ]/{d=0}' "{{ext}}/cmd/manifest.yaml" 2>/dev/null | while read src dest; do \
        [ -z "$src" ] && continue; \
        if [ ! -f "{{cfg_dir}}/$dest" ]; then \
            mkdir -p "$(dirname "{{cfg_dir}}/$dest")"; \
            cp "{{ext}}/$src" "{{cfg_dir}}/$dest"; \
        fi; \
    done

# ── Extension targets ──────────────────────────────────────────────────

extensions-safeguard:      (build-ext "safeguard")
extensions-rtk:            (build-ext "rtk")
extensions-autotitle:      (build-ext "autotitle")
extensions-clipboard:      (build-ext "clipboard")
extensions-skill:          (build-ext "skill")
extensions-memory:         (build-ext "memory")
extensions-subagent:       (build-ext "subagent")
extensions-lsp:            (build-ext "lsp")
extensions-repomap:        (build-ext "repomap")
extensions-plan:           (build-ext "plan")
extensions-bulk:           (build-ext "bulk")
extensions-modelsdev:      (build-ext "modelsdev")
extensions-mcp:            (build-ext "mcp")
extensions-usage:          (build-ext "usage")
extensions-gitcontext:     (build-ext "gitcontext")
extensions-prompts:        (build-ext "prompts")
extensions-behavior:       (build-ext "behavior")
extensions-export:         (build-ext "export")
extensions-admin:          (build-ext "admin")
extensions-scaffold:       (build-ext "scaffold")
extensions-undo:           (build-ext "undo")
extensions-session-tools:  (build-ext "session-tools")
extensions-background:     (build-ext "background")
extensions-extensions-list: (build-ext "extensions-list")
extensions-pipeline:       (build-ext "pipeline")
extensions-webfetch:       (build-ext "webfetch")
extensions-loop:           (build-ext "loop")
extensions-inbox:          (build-ext "inbox")
extensions-sift:           (build-ext "sift")
extensions-provider:       (build-ext "provider")
extensions-suggest:        (build-ext "suggest")
extensions-cron:           (build-ext "cron")
extensions-tokengate:      (build-ext "tokengate")
extensions-coordinator:    (build-ext "coordinator")
extensions-route:          (build-ext "route")
extensions-changelog:      (build-ext "changelog")
extensions-tasklist:       (build-ext "tasklist")

# Remove all installed extensions
clean:
    #!/usr/bin/env bash
    set -euo pipefail
    for ext in safeguard rtk autotitle clipboard skill memory subagent lsp repomap plan bulk modelsdev mcp usage gitcontext prompts behavior export admin scaffold undo session-tools background extensions-list pipeline webfetch loop inbox sift provider suggest cron tokengate coordinator route changelog tasklist; do
        rm -rf "{{ext_dir}}/$ext"
    done
    rm -f cmd
    @echo "Extensions removed from {{ext_dir}}"

# ── CLI tools ──────────────────────────────────────────────────────────

# Build and install all CLI tools
cli: \
    cli-repomap \
    cli-pipeline \
    cli-bulk \
    cli-confirm \
    cli-depgraph \
    cli-lspq \
    cli-webfetch \
    cli-memory \
    cli-sift \
    cli-fossil \
    cli-piglet-cron \
    cli-extest
    @echo "CLIs installed to {{cli_dir}}"

# Build and install one CLI tool (internal recipe)
[private]
build-cli name:
    GOWORK=off go build -o "{{cli_dir}}/{{name}}" "./cmd/{{name}}/"

cli-repomap:     (build-cli "repomap")
cli-pipeline:    (build-cli "pipeline")
cli-bulk:        (build-cli "bulk")
cli-confirm:     (build-cli "confirm")
cli-depgraph:    (build-cli "depgraph")
cli-lspq:        (build-cli "lspq")
cli-webfetch:    (build-cli "webfetch")
cli-memory:      (build-cli "memory")
cli-sift:        (build-cli "sift")
cli-fossil:      (build-cli "fossil")
cli-piglet-cron: (build-cli "piglet-cron")
cli-extest:      (build-cli "extest")

# ── Packs ──────────────────────────────────────────────────────────────

# Build all pack binaries
packs: pack-core pack-agent pack-context pack-code pack-workflow pack-cron pack-eval

# Build one pack binary (internal recipe)
[private]
build-pack name:
    GOWORK=off go build -o "pack-{{name}}" "./packs/{{name}}/"

pack-core:     (build-pack "core")
pack-agent:    (build-pack "agent")
pack-context:  (build-pack "context")
pack-code:     (build-pack "code")
pack-workflow: (build-pack "workflow")
pack-cron:     (build-pack "cron")
pack-eval:     (build-pack "eval")

# ── Verification ───────────────────────────────────────────────────────

# Run all tests
test:
    GOWORK=off go test ./...

# Build all packages (compile check)
check:
    GOWORK=off go build ./...

# Lint
lint:
    golangci-lint run ./...

# Full verification: build + test + lint
verify: check test
