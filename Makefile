.PHONY: run install test vet check

run:
	go run ./cmd/beeper-tui

install:
	go install ./cmd/beeper-tui

test:
	go test ./...

vet:
	go vet ./...

check: test vet
