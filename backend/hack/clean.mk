# Remove build / dist artifacts.

.PHONY: clean

clean:
	@echo "🧹 cleaning build artifacts..."
	@rm -rf build dist
	@rm -f logs/*.log 2>/dev/null || true
	@echo "✅ clean complete"
