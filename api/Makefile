# Makefile

CMD_DIR := ./cmd/api
SWAG_MAIN := $(CMD_DIR)/main.go
SWAG_OUT := ./docs
BIN_DIR := ./bin
SWAG := go run github.com/swaggo/swag/cmd/swag@v1.16.4
MOCKGEN := go run go.uber.org/mock/mockgen@v0.6.0
GO_TEST := go test

.PHONY: dev build swag test test-cover clean

dev:
	go run $(SWAG_MAIN)

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

clean:
	rm -rf $(BIN_DIR)
