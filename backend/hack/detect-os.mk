# Print host OS / arch (packaging diagnostics).

.PHONY: detect-os

detect-os:
	@if [ -f /etc/os-release ]; then \
		. /etc/os-release; \
		echo "OS=$${NAME:-unknown} $${VERSION_ID:-} ($${ID:-})"; \
	else \
		echo "OS=unknown"; \
	fi
	@uname -m
	@echo "GOOS=$$($(GO) env GOOS) GOARCH=$$($(GO) env GOARCH)"
