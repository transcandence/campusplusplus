.PHONY: up down test gen migrate

up:
	docker compose -f deploy/compose/docker-compose.yml up -d

down:
	docker compose -f deploy/compose/docker-compose.yml down

test:
	@echo "TODO"

gen:
	@echo "TODO"

migrate-up-%:
	cd services/$*/api && set -a && . ./.env && set +a && go run ./cmd/migrate up

migrate-down-%:
	cd services/$*/api && set -a && . ./.env && set +a && go run ./cmd/migrate down 1

migrate-version-%:
	cd services/$*/api && set -a && . ./.env && set +a && go run ./cmd/migrate version
