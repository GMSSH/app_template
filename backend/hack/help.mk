# Usage help.

.PHONY: help

help:
	@echo ""
	@echo "Available targets:"
	@echo "  make all             - check + build-backend + integration → dist/app"
	@echo "  make build           - linux-amd64 + linux-arm64 packages under dist/"
	@echo "  make build-backend   - compile binary to dist/backend/main.dist/main"
	@echo "  make integration     - assemble dist/app (platform layout + uninstall.sh)"
	@echo "  make tar             - integration then dist/plugin.tar.gz"
	@echo "  make linux-amd64     - package dist/linux-amd64"
	@echo "  make linux-arm64     - package dist/linux-arm64"
	@echo "  make check           - vet + test + gofmt gate"
	@echo "  make test / vet / fmt / lint"
	@echo "  make detect-os       - print OS / arch / Go env"
	@echo "  make clean           - remove build/ and dist/"
	@echo "  make help            - this help"
	@echo ""
	@echo "Variables: VERSION=$(VERSION) GO=$(GO) CGO_ENABLED=$(CGO_ENABLED)"
	@echo "Fragments: hack/*.mk (included by the root Makefile)"
	@echo ""
