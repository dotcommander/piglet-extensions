EXTENSIONS_DIR := $(HOME)/.config/piglet/extensions

EXTENSION_NAMES := safeguard rtk autotitle clipboard skill memory subagent lsp repomap plan bulk cache modelsdev mcp usage gitcontext prompts behavior export admin scaffold undo session-tools background extensions-list pipeline webfetch loop inbox sift provider

.PHONY: extensions clean $(addprefix extensions-,$(EXTENSION_NAMES))

extensions: $(addprefix extensions-,$(EXTENSION_NAMES))
	@echo "Extensions installed to $(EXTENSIONS_DIR)"

define EXT_RULE
extensions-$(1):
	@mkdir -p $(EXTENSIONS_DIR)/$(1)
	go build -o $(EXTENSIONS_DIR)/$(1)/$(1) ./$(1)/cmd/
	cp $(1)/cmd/manifest.yaml $(EXTENSIONS_DIR)/$(1)/
endef

$(foreach ext,$(EXTENSION_NAMES),$(eval $(call EXT_RULE,$(ext))))

CLI_NAMES := repomap pipeline bulk lspq
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

cli-lspq:
	go build -o $(CLI_DIR)/lspq ./cmd/lspq/

clean:
	@for ext in $(EXTENSION_NAMES); do \
		rm -rf $(EXTENSIONS_DIR)/$$ext; \
	done
	@rm -f cmd
	@echo "Extensions removed from $(EXTENSIONS_DIR)"
