.PHONY: run stub install test vet lint check eval

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

# End-to-end query eval: real local model against the stub. Start the stub
# (make stub) and LM Studio first, then read the printed answers.
eval:
	BEEPER_API_BASE_URL=http://127.0.0.1:23374 BEEPER_ACCESS_TOKEN=stub go run ./cmd/chat-eval
