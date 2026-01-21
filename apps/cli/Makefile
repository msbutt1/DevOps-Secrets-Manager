.PHONY: build release install clean test help

# Default target
help:
	@echo "DevOps Secrets Manager CLI - Makefile"
	@echo "======================================"
	@echo ""
	@echo "Available targets:"
	@echo "  build     - Build debug binary"
	@echo "  release   - Build optimized release binary"
	@echo "  install   - Install binary to /usr/local/bin"
	@echo "  clean     - Remove build artifacts"
	@echo "  test      - Run tests"
	@echo "  help      - Show this help message"

# Build debug version
build:
	cargo build

# Build release version
release:
	cargo build --release
	@echo ""
	@echo "Binary built at: target/release/secrets"

# Install to system
install: release
	@echo "Installing secrets to /usr/local/bin..."
	sudo cp target/release/secrets /usr/local/bin/
	@echo "Installation complete! Run 'secrets --help' to get started."

# Clean build artifacts
clean:
	cargo clean

# Run tests
test:
	cargo test
