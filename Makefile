.PHONY: run stub install test vet lint check

run:
	go run ./cmd/beeper-tui

# Fake API server with synthetic data. Point the TUI at it with
# BEEPER_API_BASE_URL=http://127.0.0.1:23374 BEEPER_ACCESS_TOKEN=stub
stub:
	go run ./cmd/beeper-stub

install:
	go install ./cmd/beeper-tui

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

check: test vet lint
