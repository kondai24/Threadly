# Makefile

CMD_DIR := ./cmd/api
SWAG_MAIN := $(CMD_DIR)/main.go
SWAG_OUT := ./docs
BIN_DIR := ./bin
SWAG := go tool swag
MOCKGEN := go tool mockgen
GO_TEST := go test
ATLAS ?= atlas
MIGRATION_DIR := file://migrations
ENV_FILE ?= .env
-include $(ENV_FILE)

DB_SCHEME ?= mysql
DATABASE_URL ?= $(if $(and $(DB_USER),$(DB_PASSWORD),$(DB_HOST),$(DB_PORT),$(DB)),$(DB_SCHEME)://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB),)
GOVULNCHECK := go tool govulncheck
AIR := go tool air

export DB_USER DB_PASSWORD DB_HOST DB_PORT DB PORT JWT_SECRET COOKIE_SECURE
export OTEL_SERVICE_NAME OTEL_EXPORTER_OTLP_ENDPOINT OTEL_EXPORTER_OTLP_PROTOCOL
export OTEL_TRACES_EXPORTER OTEL_METRICS_EXPORTER OTEL_LOGS_EXPORTER

.PHONY: tools dev dev-otelc build build-otelc swag test test-cover migrate-diff migrate-apply migrate-status integration-test vulncheck clean

tools:
	@if command -v "$(ATLAS)" >/dev/null 2>&1; then \
		echo "Atlas is already installed: $$(command -v "$(ATLAS)")"; \
	elif command -v brew >/dev/null 2>&1; then \
		brew install ariga/tap/atlas; \
	else \
		echo "Atlas is not installed. Install Homebrew or run 'brew install ariga/tap/atlas'." >&2; \
		exit 1; \
	fi

dev:
	$(AIR) -c .air.toml

dev-otelc:
	$(AIR) -c .air-otelc.toml

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/main $(SWAG_MAIN)

build-otelc:
	mkdir -p $(BIN_DIR)
	go tool otelc go build -o $(BIN_DIR)/main-otelc $(SWAG_MAIN)

swag:
	$(SWAG) init \
	  -g $(SWAG_MAIN) \
	  -o $(SWAG_OUT) \
	  --parseDependency \
	  --parseInternal

test:
	$(GO_TEST) ./...

test-cover:
	$(GO_TEST) ./... -cover

migrate-diff:
	$(ATLAS) migrate diff --env gorm

migrate-apply:
	@test -n "$(DATABASE_URL)" || (echo "Set DATABASE_URL or DB_USER, DB_PASSWORD, DB_HOST, DB_PORT, and DB in $(ENV_FILE)" >&2; exit 1)
	$(ATLAS) migrate apply --url "$(DATABASE_URL)" --dir "$(MIGRATION_DIR)"

migrate-status:
	@test -n "$(DATABASE_URL)" || (echo "Set DATABASE_URL or DB_USER, DB_PASSWORD, DB_HOST, DB_PORT, and DB in $(ENV_FILE)" >&2; exit 1)
	$(ATLAS) migrate status --url "$(DATABASE_URL)" --dir "$(MIGRATION_DIR)"

integration-test: migrate-apply
	$(GO_TEST) -tags=integration ./internal/infra/database ./internal/infra/database/repositories

vulncheck:
	$(GOVULNCHECK) ./...

clean:
	rm -rf $(BIN_DIR)
