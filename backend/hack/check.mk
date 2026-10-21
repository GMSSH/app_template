# Quality gates (vet / test / fmt / check / lint).

.PHONY: test vet fmt check lint

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

# Local quality gate: vet + test + gofmt clean (no third-party tools).
check: vet test
	@files=$$(gofmt -l .); \
	if [ -n "$$files" ]; then \
		echo "⚠️  gofmt needed; run: make fmt"; \
		echo "$$files"; \
		exit 1; \
	fi
	@echo "✅ check passed"

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint not installed; skip. See https://golangci-lint.run/"; \
		exit 0; \
	}
	golangci-lint run ./...
