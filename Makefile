.PHONY: build clean install

# Build binary to bin/nugs
build:
	@mkdir -p bin
	@echo "Building nugs..."
	@go build -o bin/nugs
	@echo "✓ Build complete: bin/nugs"

# Clean build artifacts
clean:
	@rm -rf bin/
	@echo "✓ Cleaned build artifacts"

# Install to system (optional - requires sudo)
install: build
	@echo "Installing to /usr/local/bin..."
	@sudo cp bin/nugs /usr/local/bin/
	@echo "✓ Installed to /usr/local/bin/nugs"
