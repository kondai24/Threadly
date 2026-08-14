# Makefile

CMD_DIR := ./cmd/api
SWAG_MAIN := $(CMD_DIR)/main.go
SWAG_OUT := ./docs
BIN_DIR := ./bin
SWAG := go run github.com/swaggo/swag/cmd/swag@v1.16.6
MOCKGEN := go run go.uber.org/mock/mockgen@v0.6.0
GO_TEST := go test
ATLAS ?= atlas
MIGRATION_DIR := file://migrations
ENV_FILE ?= .env
-include $(ENV_FILE)

DB_SCHEME ?= mysql
DATABASE_URL ?= $(if $(and $(DB_USER),$(DB_PASSWORD),$(DB_HOST),$(DB_PORT),$(DB)),$(DB_SCHEME)://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB),)
GOVULNCHECK_VERSION := v1.6.0
GOVULNCHECK := go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
AIR_VERSION := v1.67.1
AIR := $(shell go env GOPATH)/bin/air

export DB_USER DB_PASSWORD DB_HOST DB_PORT DB PORT JWT_SECRET COOKIE_SECURE

.PHONY: tools dev build swag test test-cover migrate-diff migrate-apply migrate-status integration-test vulncheck clean

tools:
	go install go.uber.org/mock/mockgen@v0.6.0
	go install github.com/swaggo/swag/cmd/swag@v1.16.6
	go install github.com/air-verse/air@$(AIR_VERSION)
	@if command -v "$(ATLAS)" >/dev/null 2>&1; then \
		echo "Atlas is already installed: $$(command -v "$(ATLAS)")"; \
	elif command -v brew >/dev/null 2>&1; then \
		brew install ariga/tap/atlas; \
	else \
		echo "Atlas is not installed. Install Homebrew or run 'brew install ariga/tap/atlas'." >&2; \
		exit 1; \
	fi

dev:
	@test -x "$(AIR)" || (echo "Air is not installed. Run 'make tools' first." >&2; exit 1)
	$(AIR) -c .air.toml

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/main $(SWAG_MAIN)

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
