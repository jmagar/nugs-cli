BINARY := $(HOME)/.local/bin/nugs
SOURCES := $(shell find cmd internal -name '*.go') go.mod go.sum

.PHONY: build clean install test

# Build only if source files are newer than the binary
build: $(BINARY)

$(BINARY): $(SOURCES)
	@mkdir -p $(HOME)/.local/bin
	@echo "Building nugs..."
	@go build -o $(BINARY) ./cmd/nugs
	@echo "done: $(BINARY)"

# Clean removes binary from ~/.local/bin
clean:
	@rm -f $(BINARY) ./nugs ./nugs-cli

test:
	@go test -race -count=1 ./...

# Install is the same as build (already in user's PATH)
install: build
