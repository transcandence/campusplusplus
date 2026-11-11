# --- Configuration & Paths ---
COMPOSE_FILE ?= infra/compose/docker-compose.yml
SERVICES_DIR ?= services
SERVICES      = identity housing transport
svc ?= all
GEN_SERVICES  = identity
GEN_FILES     = $(foreach s,$(GEN_SERVICES),$(SERVICES_DIR)/$(s)/api/internal/handler/generated.go)

##@ Secrets
SECRETS_DIR ?= infra/compose/secrets
JWT_PRIV    := $(SECRETS_DIR)/jwt_private_key.pem
JWT_PUB     := $(SECRETS_DIR)/jwt_public_key.pem
JWT_BITS    ?= 2048

# --- Tools & Binaries ---
DOCKER_COMPOSE ?= docker compose
PNPM           ?= pnpm
GO             ?= go
GO_MIGRATE     ?= $(GO) run ./cmd/migrate
LINTER         ?= golangci-lint run
CODEGEN        ?= $(GO) run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.4.1

# NOTE: stable instead of latest
.DEFAULT_GOAL := help
.DELETE_ON_ERROR:

.PHONY: help secrets secrets-clean build up down logs test gen gen-go check-gen lint migrate \
        $(addprefix migrate-up-,$(SERVICES)) \
        $(addprefix migrate-down-,$(SERVICES)) \
        $(addprefix migrate-version-,$(SERVICES)) \
        test-gateway test-platform \
        $(addprefix test-service-,$(SERVICES))

##@ Getting started
help:
	@printf '\nUsage: make <target> [VAR=value ...]\n'
	@printf 'Run a bare `make` to print this list.\n\n'
	@awk 'BEGIN {FS = ":.*##"} \
		/^[a-zA-Z_0-9-]+:.*?##/ { printf "  %-30s %s\n", $$1, $$2 } \
		/^##@/ { printf "\n%s\n", substr($$0, 5) }' $(MAKEFILE_LIST)
	@printf '\nCommon variables (override on the command line)\n'
	@printf '  %-30s %s\n' 'svc=<profile>'    'compose profile: all | identity | housing | transport (default: $(svc))'
	@printf '  %-30s %s\n' 'SERVICES=...'     'services for test/migrate (default: $(SERVICES))'
	@printf '  %-30s %s\n' 'GEN_SERVICES=...' 'services with OpenAPI codegen (default: $(GEN_SERVICES))'
	@printf '  %-30s %s\n' 'COMPOSE_FILE=...' 'compose file (default: $(COMPOSE_FILE))'
	@printf '  %-30s %s\n' 'JWT_BITS=...'     'RSA key size for dev keys (default: $(JWT_BITS))'
	@printf '\nExamples:\n'
	@printf '  make build svc=identity\n'
	@printf '  make up svc=identity\n'
	@printf '  make logs s=identity\n'
	@printf '  make test-service-identity\n'
	@printf '  make migrate-up-housing\n\n'

secrets: $(JWT_PRIV) $(JWT_PUB) ## Create dev JWT keys if missing (never overwrites)

$(JWT_PRIV):
	@mkdir -p $(SECRETS_DIR)
	@echo "Generating RSA-$(JWT_BITS) private key -> $@"
	@openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:$(JWT_BITS) -out $@
	@chmod 644 $@

# Public key is always derived from the private one, so the pair can never mismatch.
$(JWT_PUB): $(JWT_PRIV)
	@echo "Deriving public key -> $@"
	@openssl pkey -in $(JWT_PRIV) -pubout -out $@
	@chmod 644 $@

secrets-clean: ## Delete dev keys (invalidates every token issued with them)
	rm -f $(JWT_PRIV) $(JWT_PUB)

##@ Lifecycle
build: secrets ## Ensure secrets exist, then build images
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) --profile $(svc) build
up:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) --profile $(svc) up -d

down:
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) --profile $(svc) down

logs: ## Follow logs (svc=<profile>, optional s=<service>, e.g. make logs s=identity)
	$(DOCKER_COMPOSE) -f $(COMPOSE_FILE) --profile $(svc) logs -f --tail=100 $(s)
##@ Testing
test: test-gateway test-platform $(addprefix test-service-,$(SERVICES))
	@echo "Running TypeScript tests..."
	$(PNPM) --recursive test

test-gateway:
	cd api-gateway && $(GO) test ./...

test-platform:
	cd platform/go && $(GO) test ./...

$(addprefix test-service-,$(SERVICES)): test-service-%:
	cd $(SERVICES_DIR)/$*/api && $(GO) test ./...

##@ Code generation
gen: gen-go

gen-go: $(GEN_FILES)

$(SERVICES_DIR)/%/api/internal/handler/generated.go: $(SERVICES_DIR)/%/contracts/openapi.yaml
	@echo "  -> $*"
	@mkdir -p $(@D)
	@$(CODEGEN) -generate types,gin-server -package handler -o $@ $<

check-gen: gen
	@git diff --exit-code -- $(GEN_FILES) || (echo "generated code is stale, run make gen"; exit 1)

##@ Quality
lint:
	$(LINTER)
	$(PNPM) --recursive run lint

##@ Database migrations
$(addprefix migrate-up-,$(SERVICES)): migrate-up-%:
	cd $(SERVICES_DIR)/$*/api && set -a && . ./.env && set +a && $(GO_MIGRATE) up

$(addprefix migrate-down-,$(SERVICES)): migrate-down-%:
	cd $(SERVICES_DIR)/$*/api && set -a && . ./.env && set +a && $(GO_MIGRATE) down 1

$(addprefix migrate-version-,$(SERVICES)): migrate-version-%:
	cd $(SERVICES_DIR)/$*/api && set -a && . ./.env && set +a && $(GO_MIGRATE) version

##@ Seeding
seed-identity: ## Seed identity database with dev users (sara.benali@campus.ma, worker@campus.ma, admin@campus.ma)
	cd $(SERVICES_DIR)/identity/api && set -a && . ./.env && set +a && $(GO) run ./cmd/seed
