.PHONY: build test lint up down migrate-up migrate-down migrate-diff atlas-image

COMPOSE_ENV := deploy/compose/.env
COMPOSE := docker compose -f deploy/compose/docker-compose.yml --env-file $(COMPOSE_ENV)

ATLAS_IMAGE := payment-lab-atlas:1.3.0

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

atlas-image: bin/.atlas-image

bin/.atlas-image: deploy/atlas/Dockerfile
	mkdir -p bin
	docker build -t $(ATLAS_IMAGE) -f deploy/atlas/Dockerfile deploy/atlas
	touch $@

# --network host lets docker:// publish the dev database on 127.0.0.1 and lets
# apply reach the compose Postgres, which is bound to 127.0.0.1.
define atlas-run
	set -eu; \
	set -a; . ./$(COMPOSE_ENV); set +a; \
	logical_host=$${POSTGRES_HOST:-127.0.0.1}; \
	port=$${POSTGRES_PORT:-5432}; \
	user=$${POSTGRES_USER:-payment}; \
	db=$${POSTGRES_DB:-payment_dev}; \
	export DATABASE_URL="postgres://$$user:$$POSTGRES_PASSWORD@$$logical_host:$$port/$$db?sslmode=disable"; \
	docker run --rm --network host \
		-e DATABASE_URL \
		-v "$(CURDIR)":/work \
		-w /work \
		-v /var/run/docker.sock:/var/run/docker.sock \
		$(ATLAS_IMAGE) $(1)
endef

# $(1) is the atlas env name. Reverts every applied file in that directory.
# Refuses any host other than 127.0.0.1. --to-version 0 is not a real revision.
define atlas-down
	set -eu; \
	set -o pipefail; \
	set -a; . ./$(COMPOSE_ENV); set +a; \
	logical_host=$${POSTGRES_HOST:-127.0.0.1}; \
	if [ "$$logical_host" != "127.0.0.1" ]; then \
		echo "migrate-down refuses host $$logical_host; only 127.0.0.1 is allowed" >&2; \
		exit 1; \
	fi; \
	port=$${POSTGRES_PORT:-5432}; \
	user=$${POSTGRES_USER:-payment}; \
	db=$${POSTGRES_DB:-payment_dev}; \
	export DATABASE_URL="postgres://$$user:$$POSTGRES_PASSWORD@$$logical_host:$$port/$$db?sslmode=disable"; \
	count=$$(docker run --rm --network host \
		-e DATABASE_URL \
		-v "$(CURDIR)":/work \
		-w /work \
		$(ATLAS_IMAGE) migrate status --env $(1) | awk '/Executed Files:/ {print $$NF}'); \
	if [ "$$count" = "0" ]; then \
		echo "no $(1) migrations to revert"; \
		exit 0; \
	fi; \
	docker run --rm --network host \
		-e DATABASE_URL \
		-v "$(CURDIR)":/work \
		-w /work \
		-v /var/run/docker.sock:/var/run/docker.sock \
		$(ATLAS_IMAGE) migrate down --env $(1) $$count
endef

# allow-dirty: the other module's schemas already exist before this directory has a revisions table.
migrate-up: $(COMPOSE_ENV) bin/.atlas-image
	@$(call atlas-run,migrate apply --allow-dirty --env merchant)
	@$(call atlas-run,migrate apply --allow-dirty --env payment)

migrate-down: $(COMPOSE_ENV) bin/.atlas-image
	@$(call atlas-down,payment)
	@$(call atlas-down,merchant)

migrate-diff: $(COMPOSE_ENV) bin/.atlas-image
	@test -n "$(MODULE)" || { echo "usage: make migrate-diff MODULE=merchant NAME=add_merchants" >&2; exit 1; }
	@test -n "$(NAME)" || { echo "usage: make migrate-diff MODULE=merchant NAME=add_merchants" >&2; exit 1; }
	@test "$(MODULE)" = "merchant" -o "$(MODULE)" = "payment" || { echo "MODULE must be merchant or payment" >&2; exit 1; }
	@$(call atlas-run,migrate diff $(NAME) --env $(MODULE))

$(COMPOSE_ENV):
	@echo "missing $(COMPOSE_ENV); run: cp deploy/compose/env.example $(COMPOSE_ENV)" >&2; exit 1
