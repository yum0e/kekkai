.PHONY: run test clean

# Run the CLI
run:
	uv run kekkai

# Run tests
test:
	uv run --with pytest pytest tests/ -v

# Clean build artifacts
clean:
	rm -rf .venv .pytest_cache __pycache__ src/kekkai/__pycache__ tests/__pycache__
