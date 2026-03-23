EXTENSIONS_DIR := $(HOME)/.config/piglet/extensions

EXTENSION_NAMES := safeguard rtk autotitle clipboard skill memory subagent lsp repomap plan bulk mcp

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

clean:
	@for ext in $(EXTENSION_NAMES); do \
		rm -rf $(EXTENSIONS_DIR)/$$ext; \
	done
	@echo "Extensions removed from $(EXTENSIONS_DIR)"
