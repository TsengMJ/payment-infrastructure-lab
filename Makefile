.PHONY: build test lint up down

COMPOSE_ENV := deploy/compose/.env
COMPOSE := docker compose -f deploy/compose/docker-compose.yml --env-file $(COMPOSE_ENV)

build:
	go build ./...

test:
	go test ./...

lint:
	go tool go-arch-lint check

up: $(COMPOSE_ENV)
	$(COMPOSE) up -d --wait

down: $(COMPOSE_ENV)
	$(COMPOSE) down

$(COMPOSE_ENV):
	@echo "missing $(COMPOSE_ENV); run: cp deploy/compose/env.example $(COMPOSE_ENV)" >&2; exit 1
