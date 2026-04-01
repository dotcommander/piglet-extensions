EXTENSIONS_DIR := $(HOME)/.config/piglet/extensions

EXTENSION_NAMES := safeguard rtk autotitle clipboard skill memory subagent lsp repomap plan bulk cache modelsdev mcp usage gitcontext prompts behavior export admin scaffold undo session-tools background extensions-list pipeline webfetch loop inbox sift provider suggest cron tokengate coordinator route

.PHONY: extensions clean $(addprefix extensions-,$(EXTENSION_NAMES))

extensions: $(addprefix extensions-,$(EXTENSION_NAMES))
	@echo "Extensions installed to $(EXTENSIONS_DIR)"

CONFIG_DIR := $(HOME)/.config/piglet

define EXT_RULE
extensions-$(1):
	@mkdir -p $(EXTENSIONS_DIR)/$(1)
	go build -o $(EXTENSIONS_DIR)/$(1)/$(1) ./$(1)/cmd/
	cp $(1)/cmd/manifest.yaml $(EXTENSIONS_DIR)/$(1)/
	@awk '/^defaults:/{d=1;next} d&&/^  - src:/{src=$$$$3} d&&/^    dest:/{print src,$$$$2} d&&/^[^ ]/{d=0}' $(1)/cmd/manifest.yaml 2>/dev/null | while read src dest; do \
		[ -z "$$$$src" ] && continue; \
		if [ ! -f "$(CONFIG_DIR)/$$$$dest" ]; then \
			mkdir -p "$$$$(dirname "$(CONFIG_DIR)/$$$$dest")"; \
			cp "$(1)/$$$$src" "$(CONFIG_DIR)/$$$$dest"; \
		fi; \
	done
endef

$(foreach ext,$(EXTENSION_NAMES),$(eval $(call EXT_RULE,$(ext))))

CLI_NAMES := repomap pipeline bulk confirm depgraph lspq webfetch memory sift fossil piglet-cron
CLI_DIR := $(HOME)/go/bin

.PHONY: cli $(addprefix cli-,$(CLI_NAMES))

cli: $(addprefix cli-,$(CLI_NAMES))
	@echo "CLIs installed to $(CLI_DIR)"

cli-repomap:
	go build -o $(CLI_DIR)/repomap ./cmd/repomap/

cli-pipeline:
	go build -o $(CLI_DIR)/pipeline ./cmd/pipeline/

cli-bulk:
	go build -o $(CLI_DIR)/bulk ./cmd/bulk/

cli-confirm:
	go build -o $(CLI_DIR)/confirm ./cmd/confirm/

cli-depgraph:
	go build -o $(CLI_DIR)/depgraph ./cmd/depgraph/

cli-lspq:
	go build -o $(CLI_DIR)/lspq ./cmd/lspq/

cli-webfetch:
	go build -o $(CLI_DIR)/webfetch ./cmd/webfetch/

cli-memory:
	go build -o $(CLI_DIR)/memory ./cmd/memory/

cli-sift:
	go build -o $(CLI_DIR)/sift ./cmd/sift/

cli-fossil:
	go build -o $(CLI_DIR)/fossil ./cmd/fossil/

cli-piglet-cron:
	go build -o $(CLI_DIR)/piglet-cron ./cmd/piglet-cron/

# Pack targets — install path (single binaries bundling multiple extensions)
.PHONY: packs
packs:
	go build -o pack-core ./packs/core/
	go build -o pack-agent ./packs/agent/
	go build -o pack-context ./packs/context/
	go build -o pack-code ./packs/code/
	go build -o pack-workflow ./packs/workflow/
	go build -o pack-cron ./packs/cron/
	go build -o pack-eval ./packs/eval/

clean:
	@for ext in $(EXTENSION_NAMES); do \
		rm -rf $(EXTENSIONS_DIR)/$$ext; \
	done
	@rm -f cmd
	@echo "Extensions removed from $(EXTENSIONS_DIR)"
