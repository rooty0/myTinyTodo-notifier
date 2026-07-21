# Portable across GNU make (Linux, macOS) and BSD make (FreeBSD):
# no $(shell ...), no != assignments, version computed in the recipe.
#
# Override the Go binary when it is installed under a versioned name,
# e.g. on FreeBSD: make build GO=go125
GO     ?= go
BINARY  = mtt_notify
PREFIX ?= /usr/local

# FreeBSD install layout
BINDIR    = $(PREFIX)/bin
RCDIR     = $(PREFIX)/etc/rc.d
CONFPATH  = $(PREFIX)/etc/$(BINARY).yaml

.PHONY: help
help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}' Makefile

.PHONY: build
build: ## Build the binary with the version from git
	$(GO) build -trimpath -ldflags "-X main.version=$$(git describe --tags --always --dirty 2>/dev/null || echo dev)" -o $(BINARY) .

.PHONY: install
install: build ## Install the binary to $(PREFIX)/bin
	install $(BINARY) $(PREFIX)/bin/

.PHONY: install-freebsd
install-freebsd: build ## FreeBSD: install the binary and rc.d script
	@[ "$$(id -u)" = "0" ] || { echo "install-freebsd must be run as root (use sudo)"; exit 1; }
	install -m 755 $(BINARY) $(BINDIR)/$(BINARY)
	@# rewrite procname for a non-default PREFIX; the repo script assumes /usr/local
	sed 's|/usr/local/bin/$(BINARY)|$(BINDIR)/$(BINARY)|' init/freebsd/$(BINARY) > /tmp/$(BINARY).rc
	install -m 755 /tmp/$(BINARY).rc $(RCDIR)/$(BINARY)
	rm -f /tmp/$(BINARY).rc
	@echo "installed:"
	@echo "  $(BINDIR)/$(BINARY)"
	@echo "  $(RCDIR)/$(BINARY)"
	@echo "if this is a new install, copy the config into place manually:"
	@echo "  cp $(BINARY).example.yaml $(CONFPATH)"
	@echo "  chown \$$USER $(CONFPATH) && chmod 600 $(CONFPATH)   # must not be group/world readable"
	@echo "  sysrc $(BINARY)_enable=YES $(BINARY)_user=\$$USER"
	@echo "start or restart the service manually:"
	@echo "  service $(BINARY) restart"

.PHONY: deinstall-freebsd
deinstall-freebsd: ## FreeBSD: remove the binary and rc.d script (keeps your config)
	@[ "$$(id -u)" = "0" ] || { echo "deinstall-freebsd must be run as root (use sudo)"; exit 1; }
	rm -f $(BINDIR)/$(BINARY)
	rm -f $(RCDIR)/$(BINARY)
	@echo "removed binary and rc.d script"
	@echo "stop the service manually if it is still running:"
	@echo "  service $(BINARY) stop"
	@echo "kept $(CONFPATH) (contains credentials); remove it manually if no longer needed"
	@echo "to drop the rc.conf entry: sysrc -x $(BINARY)_enable $(BINARY)_user"

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
