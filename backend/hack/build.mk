# Compile the CLI binary.
#
# Targets:
#   build-backend  → $(BACKEND_DIST)/main  (host GOOS/GOARCH unless overridden)
#   build-one      → $(OUT)                (requires OUT=...)

.PHONY: build-backend build-one

build-backend:
	@echo "🚀 building backend binary → $(BACKEND_DIST)/main"
	@$(MAKE) build-one OUT=$(BACKEND_DIST)/main

# Generic compile: OUT is required. Honors GOOS / GOARCH / CGO_ENABLED / LDFLAGS.
build-one:
	@test -n "$(OUT)" || (echo "❌ OUT is required (e.g. OUT=dist/backend/main.dist/main)" >&2; exit 1)
	@mkdir -p $$(dirname "$(OUT)")
	@echo "🚀 building $(CMD_PKG) → $(OUT)"
	@echo "   GOOS=$${GOOS:-$$($(GO) env GOOS)} GOARCH=$${GOARCH:-$$($(GO) env GOARCH)} CGO_ENABLED=$(CGO_ENABLED) VERSION=$(VERSION)"
	@$(GO) build -buildvcs=false -trimpath -ldflags="$(LDFLAGS)" -o "$(OUT)" "$(CMD_PKG)"
	@chmod 755 "$(OUT)"
	@echo "✅ binary ready: $(OUT)"
