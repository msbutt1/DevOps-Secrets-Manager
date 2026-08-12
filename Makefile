# Local development without Docker.
#
#   make db && make migrate-up && make dev
#
# Every variable below can be overridden on the command line (make db PGPORT=5432)
# or in a gitignored local.mk, e.g. to put a specific Go or PostgreSQL on PATH.
-include local.mk

SHELL := /bin/bash
.DEFAULT_GOAL := help

DEV_DIR      ?= $(HOME)/.local/share/devops-secrets-manager
PGDATA       ?= $(DEV_DIR)/pgdata
PGHOST       ?= localhost
PGPORT       ?= 5433
PGSUPERUSER  ?= postgres
DB_USER      ?= secrets_user
DB_PASSWORD  ?= secrets_password
DB_NAME      ?= secrets_db
TEST_DB_NAME ?= secrets_test
TEST_DATABASE_URL ?= postgres://$(PGSUPERUSER)@$(PGHOST):$(PGPORT)/$(TEST_DB_NAME)?sslmode=disable

API_URL ?= http://localhost:8080
API_ENV := apps/api/.env
PSQL    := psql -h $(PGHOST) -p $(PGPORT) -U $(PGSUPERUSER) -d postgres -v ON_ERROR_STOP=1 -qtA

export PGHOST PGPORT DB_USER DB_PASSWORD DB_NAME

.PHONY: help
help: ## Show available targets
	@grep -hE '^[a-zA-Z_-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  \033[1m%-14s\033[0m %s\n", $$1, $$2}'

# ---------------------------------------------------------------------------
# Database
# ---------------------------------------------------------------------------

.PHONY: db
db: ## Start a local PostgreSQL (initialised on first run) and create the app and test databases
	@command -v pg_ctl >/dev/null || { echo "pg_ctl not found: install PostgreSQL 17, or use 'docker compose up -d db'"; exit 1; }
	@mkdir -p $(DEV_DIR)
	@if [ ! -f "$(PGDATA)/PG_VERSION" ]; then \
		echo "Initialising $(PGDATA)"; \
		initdb -D "$(PGDATA)" -U $(PGSUPERUSER) --auth=trust >/dev/null; \
	fi
	@if ! pg_ctl -D "$(PGDATA)" status >/dev/null 2>&1; then \
		pg_ctl -D "$(PGDATA)" -o "-p $(PGPORT) -k $(DEV_DIR)" -l "$(DEV_DIR)/postgres.log" -w start >/dev/null; \
	fi
	@$(PSQL) -c "SELECT 1 FROM pg_roles WHERE rolname = '$(DB_USER)'" | grep -q 1 || \
		$(PSQL) -c "CREATE ROLE $(DB_USER) LOGIN PASSWORD '$(DB_PASSWORD)'"
	@$(PSQL) -c "SELECT 1 FROM pg_database WHERE datname = '$(DB_NAME)'" | grep -q 1 || \
		$(PSQL) -c "CREATE DATABASE $(DB_NAME) OWNER $(DB_USER)"
	@$(PSQL) -c "SELECT 1 FROM pg_database WHERE datname = '$(TEST_DB_NAME)'" | grep -q 1 || \
		$(PSQL) -c "CREATE DATABASE $(TEST_DB_NAME)"
	@echo "PostgreSQL is running on $(PGHOST):$(PGPORT) ($(DB_NAME), $(TEST_DB_NAME))"

.PHONY: db-stop
db-stop: ## Stop the local PostgreSQL
	@pg_ctl -D "$(PGDATA)" status >/dev/null 2>&1 && pg_ctl -D "$(PGDATA)" -w stop || echo "PostgreSQL is not running"

.PHONY: env
env: ## Create apps/api/.env with generated keys (never overwrites)
	@./scripts/dev-env.sh $(API_ENV)

.PHONY: migrate-up
migrate-up: env ## Apply all database migrations
	cd apps/api && go run ./cmd/migrate up

.PHONY: migrate-down
migrate-down: env ## Roll back migrations: N=1 (default), N=3 or N=all
	cd apps/api && go run ./cmd/migrate down $(or $(N),1)

# ---------------------------------------------------------------------------
# Apps
# ---------------------------------------------------------------------------

apps/web/node_modules: apps/web/package-lock.json
	cd apps/web && npm ci
	@touch $@

.PHONY: web-deps
web-deps: apps/web/node_modules

.PHONY: api
api: env ## Run the API on :8080
	cd apps/api && go run ./cmd/server

.PHONY: web
web: web-deps ## Run the web app on :5173 (proxies /api to :8080)
	cd apps/web && npm run dev

.PHONY: cli
cli: ## Build the CLI release binary (apps/cli/target/release/secrets)
	cd apps/cli && cargo build --release

.PHONY: dev
dev: db env web-deps ## Run the API and web app together (Ctrl-C stops both)
	@trap 'trap - INT TERM EXIT; kill 0 2>/dev/null' INT TERM EXIT; \
		$(MAKE) --no-print-directory api & \
		$(MAKE) --no-print-directory web; \
		wait

# ---------------------------------------------------------------------------
# Checks
# ---------------------------------------------------------------------------

.PHONY: test test-api test-web test-cli
test: test-api test-web test-cli ## Run all tests

test-api: ## Go tests (integration tests use TEST_DATABASE_URL)
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test ./...

test-web: web-deps ## Web unit tests
	cd apps/web && npm test -- --run

test-cli: ## CLI tests
	cd apps/cli && cargo test

.PHONY: lint lint-api lint-web lint-cli
lint: lint-api lint-web lint-cli ## Run all linters, formatters (check mode) and type checks

lint-api:
	@unformatted="$$(gofmt -l apps/api)"; if [ -n "$$unformatted" ]; then echo "gofmt needed:"; echo "$$unformatted"; exit 1; fi
	go vet ./...

lint-web: web-deps
	cd apps/web && npm run lint && npm run format:check && npx tsc -p tsconfig.app.json --noEmit

lint-cli:
	cd apps/cli && cargo fmt --check && cargo clippy --all-targets -- -D warnings

.PHONY: build build-api build-web build-cli
build: build-api build-web build-cli ## Build everything

build-api:
	go build ./...

build-web: web-deps
	cd apps/web && npm run build

build-cli: cli
