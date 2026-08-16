.PHONY: run stub build install install-bin uninstall test vet lint check eval tinfoil-timing verify-tinfoil

PREFIX  ?= $(HOME)/.local
BINDIR  ?= $(PREFIX)/bin
REPO    := $(CURDIR)
WRAPPER := scripts/beeper-tui-wrapper.sh

# Same artifact as the PATH launcher (`./beeper-tui`). `go run` would compile
# into the Go cache and leave PATH pointed at a different, older binary.
run: build
	@set -a; [ -f .env ] && . ./.env; set +a; ./beeper-tui

# Fake API server with synthetic data. Point the TUI at it with
# BEEPER_API_BASE_URL=http://127.0.0.1:23374 BEEPER_ACCESS_TOKEN=stub
stub:
	go run ./cmd/beeper-stub

build:
	go build -o beeper-tui ./cmd/beeper-tui

# Install a PATH launcher that rebuilds from this checkout when source is newer.
# Result: `beeper-tui` works from any directory and never goes stale.
install:
	@mkdir -p "$(BINDIR)"
	@sed "s|@REPO@|$(REPO)|g" "$(WRAPPER)" > "$(BINDIR)/beeper-tui"
	@chmod +x "$(BINDIR)/beeper-tui"
	@echo "Installed launcher → $(BINDIR)/beeper-tui"
	@echo "  rebuilds from $(REPO) when source is newer than the binary"
	@# Prime the binary so first launch is fast.
	@$(MAKE) build
	@command -v beeper-tui >/dev/null 2>&1 || \
		echo "note: $(BINDIR) is not on PATH — add it to your shell config"

# One-shot static binary install (no auto-rebuild). Prefer plain `install` for daily use.
install-bin: build
	@mkdir -p "$(BINDIR)"
	install -m 755 beeper-tui "$(BINDIR)/beeper-tui"
	@echo "Installed static binary → $(BINDIR)/beeper-tui"
	@echo "  re-run 'make install-bin' after pulling changes, or use 'make install' for auto-rebuild"

uninstall:
	rm -f "$(BINDIR)/beeper-tui"
	@echo "Removed $(BINDIR)/beeper-tui"

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

# Skipped when shellcheck is not installed.
shellcheck:
	@if command -v shellcheck >/dev/null; then shellcheck scripts/*.sh; else echo "shellcheck not installed, skipping"; fi

check: test vet lint shellcheck

# End-to-end query eval: real local model against the stub. Start the stub
# (make stub) and LM Studio first, then read the printed answers.
eval:
	@set -a; [ -f .env ] && . ./.env; set +a; BEEPER_API_BASE_URL=http://127.0.0.1:23374 BEEPER_ACCESS_TOKEN=stub go run ./cmd/chat-eval

tinfoil-timing:
	@scripts/tinfoil-timing.sh

# Attest inference.tinfoil.sh with this binary's SDK, print the document, exit.
verify-tinfoil:
	@set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/beeper-tui --verify-tinfoil
