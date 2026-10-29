# --- Configuration & Paths ---
COMPOSE_FILE ?= infra/compose/docker-compose.yml
SERVICES_DIR ?= services
SERVICES      = identity housing transport

# Services that have a real contracts/openapi.yaml.
# Add housing / transport here once their spec is filled in.
GEN_SERVICES  = identity
GEN_FILES     = $(foreach s,$(GEN_SERVICES),$(SERVICES_DIR)/$(s)/api/internal/handler/generated.go)

# --- Tools & Binaries ---
DOCKER_COMPOSE ?= docker compose
PNPM           ?= pnpm
GO             ?= go
GO_MIGRATE     ?= $(GO) run ./cmd/migrate
LINTER         ?= golangci-lint run
CODEGEN        ?= $(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1
#NOTE: stable instead of latest

# A bare `make` prints the target list instead of running `up`
.DEFAULT_GOAL := help

# Remove a half-written output file if a recipe fails
.DELETE_ON_ERROR:

.PHONY: help up down test gen gen-go gen-ts check-gen lint migrate $(addprefix migrate-up-,$(SERVICES)) $(addprefix migrate-down-,$(SERVICES)) $(addprefix migrate-version-,$(SERVICES)) test-gateway test-platform $(addprefix test-service-,$(SERVICES))

help:
	@echo "targets: up down test gen gen-go gen-ts check-gen lint migrate"

up:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) --profile $(svc) up -d
down:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) --profile $(svc) down

# Global test runs everything in sequence
test: test-gateway test-platform $(addprefix test-service-,$(SERVICES))
	@echo "Running TypeScript tests..."
	$(PNPM) --recursive test

# Individual test targets
test-gateway:
	cd api-gateway && $(GO) test ./...

test-platform:
	cd platform/go && $(GO) test ./...

$(addprefix test-service-,$(SERVICES)): test-service-%:
	cd $(SERVICES_DIR)/$*/api && $(GO) test ./...

# --- Code Generation ---
gen: gen-go

gen-go: $(GEN_FILES)

# One rule per generated.go, rebuilt only when its openapi.yaml changes
$(SERVICES_DIR)/%/api/internal/handler/generated.go: $(SERVICES_DIR)/%/contracts/openapi.yaml
	@echo "  -> $*"
	@mkdir -p $(@D)
	@$(CODEGEN) -generate types,gin-server -package handler -o $@ $<

# CI: fail if generated code is out of date
check-gen: gen
	@git diff --exit-code -- $(GEN_FILES) || (echo "generated code is stale, run make gen"; exit 1)

lint:
	$(LINTER)
	$(PNPM) --recursive run lint

# --- Database Migrations ---
migrate: $(addprefix migrate-up-,$(SERVICES))

$(addprefix migrate-up-,$(SERVICES)): migrate-up-%:
	cd $(SERVICES_DIR)/$*/api && set -a && . ./.env && set +a && $(GO_MIGRATE) up

$(addprefix migrate-down-,$(SERVICES)): migrate-down-%:
	cd $(SERVICES_DIR)/$*/api && set -a && . ./.env && set +a && $(GO_MIGRATE) down 1

$(addprefix migrate-version-,$(SERVICES)): migrate-version-%:
	cd $(SERVICES_DIR)/$*/api && set -a && . ./.env && set +a && $(GO_MIGRATE) version

# gen-ts:
	# @echo "Generating TypeScript frontend clients..."
	# $(PNPM) gen

