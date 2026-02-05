.PHONY: build clean install

# Build binary directly to ~/.local/bin/nugs
build:
	@mkdir -p ~/.local/bin
	@echo "Building nugs..."
	@go build -o ~/.local/bin/nugs
	@echo "✓ Build complete: ~/.local/bin/nugs"
	@echo "✓ Binary is in your PATH at: ~/.local/bin/nugs"

# Clean removes binary from ~/.local/bin
clean:
	@rm -f ~/.local/bin/nugs
	@echo "✓ Removed ~/.local/bin/nugs"

# Install is the same as build (already in user's PATH)
install: build
