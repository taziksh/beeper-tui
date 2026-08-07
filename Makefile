.PHONY: run install test vet lint check

run:
	go run ./cmd/beeper-tui

install:
	go install ./cmd/beeper-tui

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

check: test vet lint
