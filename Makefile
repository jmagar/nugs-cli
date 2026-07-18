VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT ?= $(shell git rev-parse --short=12 HEAD)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
INSTALL_DIR ?= $(HOME)/.local/lib/nugs
BINARY := $(HOME)/.local/bin/nugs
BINARY_DIR := $(dir $(BINARY))
VERSIONED_BINARY := $(INSTALL_DIR)/nugs-$(VERSION)
LDFLAGS := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildDate=$(BUILD_DATE)
SOURCES := $(shell find cmd internal -name '*.go') go.mod go.sum

.PHONY: build clean install test docs-check fmt-check module-check vet staticcheck vulncheck cross-build verify

build: $(VERSIONED_BINARY)
	@mkdir -p $(BINARY_DIR)
	@ln -sfn $(VERSIONED_BINARY) $(BINARY)
	@echo "installed: $(BINARY) -> $(VERSIONED_BINARY)"

$(VERSIONED_BINARY): $(SOURCES)
	@mkdir -p $(INSTALL_DIR)
	@echo "Building nugs $(VERSION)..."
	@go build -trimpath -ldflags '$(LDFLAGS)' -o $(VERSIONED_BINARY) ./cmd/nugs

clean:
	@rm -f $(BINARY) $(VERSIONED_BINARY) ./nugs ./nugs-cli

test:
	@go test -race -count=1 ./...

docs-check:
	@python3 scripts/check-docs.py

fmt-check:
	@test -z "$$(gofmt -l cmd internal)"

module-check:
	@go mod tidy -diff
	@go mod verify

vet:
	@go vet ./...

staticcheck:
	@go run honnef.co/go/tools/cmd/staticcheck@2025.1.1 ./...

vulncheck:
	@go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...

cross-build:
	@tmp="$$(mktemp -d)"; trap 'rm -rf "$$tmp"' EXIT; \
	  GOOS=linux GOARCH=amd64 go build -trimpath -o "$$tmp/nugs-linux-amd64" ./cmd/nugs; \
	  GOOS=linux GOARCH=arm64 go build -trimpath -o "$$tmp/nugs-linux-arm64" ./cmd/nugs; \
	  GOOS=darwin GOARCH=amd64 go build -trimpath -o "$$tmp/nugs-darwin-amd64" ./cmd/nugs; \
	  GOOS=darwin GOARCH=arm64 go build -trimpath -o "$$tmp/nugs-darwin-arm64" ./cmd/nugs; \
	  GOOS=windows GOARCH=amd64 go build -trimpath -o "$$tmp/nugs-windows-amd64.exe" ./cmd/nugs

verify: fmt-check module-check vet test docs-check staticcheck vulncheck cross-build

install: build
