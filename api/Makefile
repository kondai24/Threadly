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
DATABASE_URL ?=

.PHONY: tools dev build swag test test-cover migrate-diff migrate-apply migrate-status integration-test clean

tools:
	go install go.uber.org/mock/mockgen@v0.6.0
	go install github.com/swaggo/swag/cmd/swag@v1.16.6
	go install github.com/air-verse/air@v1.64.5

dev:
	air -c .air.toml

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
	@test -n "$(DATABASE_URL)" || (echo "DATABASE_URL must be set" >&2; exit 1)
	$(ATLAS) migrate apply --url "$(DATABASE_URL)" --dir "$(MIGRATION_DIR)"

migrate-status:
	@test -n "$(DATABASE_URL)" || (echo "DATABASE_URL must be set" >&2; exit 1)
	$(ATLAS) migrate status --url "$(DATABASE_URL)" --dir "$(MIGRATION_DIR)"

integration-test: migrate-apply
	$(GO_TEST) -tags=integration ./internal/infra/database ./internal/infra/database/repositories

clean:
	rm -rf $(BIN_DIR)
