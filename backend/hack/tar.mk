# Compress the integration package into a tarball.

.PHONY: tar

TAR_OUT ?= $(DIST_DIR)/plugin.tar.gz

tar: integration
	@test -d "$(INTEGRATION)" || (echo "❌ integration dir missing: $(INTEGRATION)" >&2; exit 1)
	@test -f "$(INTEGRATION)/app/bin/main" || (echo "❌ missing binary: $(INTEGRATION)/app/bin/main" >&2; exit 1)
	@mkdir -p $$(dirname "$(TAR_OUT)")
	@echo "📦 creating $(TAR_OUT) from $(INTEGRATION) ..."
	@tar -zcf "$(TAR_OUT)" -C "$(INTEGRATION)" .
	@echo "✅ tarball: $(TAR_OUT)"
