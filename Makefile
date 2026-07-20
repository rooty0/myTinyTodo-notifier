# Portable across GNU make (Linux, macOS) and BSD make (FreeBSD):
# no $(shell ...), no != assignments, version computed in the recipe.
#
# Override the Go binary when it is installed under a versioned name,
# e.g. on FreeBSD: make build GO=go125
GO     ?= go
BINARY  = mtt_notify
PREFIX ?= /usr/local

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  \033[36m%-10s\033[0m %s\n", $$1, $$2}' Makefile

.PHONY: build
build: ## Build the binary with the version from git
	$(GO) build -trimpath -ldflags "-X main.version=$$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o $(BINARY) .

.PHONY: install
install: build ## Install the binary to $(PREFIX)/bin
	install $(BINARY) $(PREFIX)/bin/

.PHONY: test
test: ## Run tests
	$(GO) test ./...

.PHONY: lint
lint: ## Run golangci-lint and go vet
	$(GO) vet ./...
	golangci-lint run ./...

.PHONY: fmt
fmt: ## Format sources
	$(GO) fmt ./...

.PHONY: tidy
tidy: ## Tidy go.mod and go.sum
	$(GO) mod tidy

.PHONY: clean
clean: ## Remove build artifacts
	rm -f $(BINARY)
