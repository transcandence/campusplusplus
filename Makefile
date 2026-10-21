.PHONY: up down test gen migrate

up:
	docker compose -f deploy/compose/docker-compose.yml up -d

down:
	docker compose -f deploy/compose/docker-compose.yml down

test:
	@echo "TODO"

gen:
	@echo "TODO"

migrate:
	@echo "TODO"
