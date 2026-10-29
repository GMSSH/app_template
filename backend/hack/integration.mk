# Integrate backend binary into the historical dist/app layout.

.PHONY: integration

integration:
	@echo "📦 integrating into $(INTEGRATION) ..."
	@BIN=""; \
	if [ -f "$(BACKEND_DIST)/main" ]; then \
		BIN="$(BACKEND_DIST)/main"; \
	elif [ -f "$(BACKEND_DIST)" ] && [ -x "$(BACKEND_DIST)" ]; then \
		BIN="$(BACKEND_DIST)"; \
	fi; \
	$(MAKE) bundle DEST="$(INTEGRATION)" BIN="$$BIN" VERSION="$(VERSION)"
	@echo "✅ integration complete: $(INTEGRATION)"
