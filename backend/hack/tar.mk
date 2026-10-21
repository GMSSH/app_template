# Compress the integration package into a tarball.

.PHONY: tar

TAR_OUT ?= $(DIST_DIR)/plugin.tar.gz

tar: integration
	@test -d "$(INTEGRATION)" || (echo "❌ integration dir missing: $(INTEGRATION)" >&2; exit 1)
	@test -f "$(INTEGRATION)/app/bin/main" || (echo "❌ missing binary: $(INTEGRATION)/app/bin/main" >&2; exit 1)
	@mkdir -p $$(dirname "$(TAR_OUT)")
	@find "$(INTEGRATION)" -name "._*" -delete 2>/dev/null || true
	@find "$(INTEGRATION)" -name ".DS_Store" -delete 2>/dev/null || true
	@echo "📦 creating $(TAR_OUT) from $(INTEGRATION) ..."
	@COPYFILE_DISABLE=1 tar --exclude='._*' --exclude='.DS_Store' -zcf "$(TAR_OUT)" -C "$(INTEGRATION)" .
	@echo "✅ tarball: $(TAR_OUT)"
