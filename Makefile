.PHONY: build test lint up down

build:
	go build ./...

test:
	go test ./...

lint:
	go tool go-arch-lint check

up:
	@echo "up: not configured yet (I-00.03)" >&2; exit 1

down:
	@echo "down: not configured yet (I-00.03)" >&2; exit 1
