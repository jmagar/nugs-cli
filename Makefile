.PHONY: build clean install test

# Build binary directly to ~/.local/bin/nugs
build:
	@mkdir -p ~/.local/bin
	@echo "Building nugs..."
	@go build -o ~/.local/bin/nugs ./cmd/nugs
	@echo "done: ~/.local/bin/nugs"

# Clean removes binary from ~/.local/bin
clean:
	@rm -f ~/.local/bin/nugs ./nugs ./nugs-cli

test:
	@go test -race -count=1 ./...

# Install is the same as build (already in user's PATH)
install: build
