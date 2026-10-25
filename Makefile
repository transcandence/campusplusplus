# --- Configuration & Paths ---
COMPOSE_FILE ?= infra/compose/docker-compose.yml
SERVICES_DIR ?= services
SERVICES      = identity housing transport

# --- Tools & Binaries ---
DOCKER_COMPOSE ?= docker compose
PNPM           ?= pnpm
GO             ?= go
GO_MIGRATE     ?= $(GO) run ./cmd/migrate
LINTER         ?= golangci-lint run
CODEGEN        ?= $(GO) run github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest

.PHONY: up down test gen lint migrate $(addprefix migrate-up-,$(SERVICES)) $(addprefix migrate-down-,$(SERVICES)) $(addprefix migrate-version-,$(SERVICES)) test-gateway test-platform $(addprefix test-service-,$(SERVICES))

up:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) up -d

down:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) down

# Global test runs everything in sequence
test: test-gateway test-platform $(addprefix test-service-,$(SERVICES))
	@echo "Running TypeScript tests..."
	$(PNPM) --recursive test

# Individual test targets
test-gateway:
	cd api-gateway && $(GO) test ./...

test-platform:
	cd platform/go && $(GO) test ./...

test-service-%:
	cd $(SERVICES_DIR)/$*/api && $(GO) test ./...

gen:
	@echo "Generating TypeScript frontend clients..."
	$(PNPM) gen
	@echo "Generating Go backend types and handlers..."
	$(foreach svc,$(SERVICES), \
		$(CODEGEN) -generate types,chi-server -package api \
		$(SERVICES_DIR)/$(svc)/contracts/openapi.yaml \
		> $(SERVICES_DIR)/$(svc)/api/internal/handler/generated.go;)

lint:
	$(LINTER)
	$(PNPM) --recursive run lint

# --- Database Migrations ---
migrate: $(addprefix migrate-up-,$(SERVICES))

migrate-up-%:
	cd $(SERVICES_DIR)/$*/api && set -a && . ./.env && set +a && $(GO_MIGRATE) up

migrate-down-%:
	cd $(SERVICES_DIR)/$*/api && set -a && . ./.env && set +a && $(GO_MIGRATE) down 1

migrate-version-%:
	cd $(SERVICES_DIR)/$*/api && set -a && . ./.env && set +a && $(GO_MIGRATE) version
