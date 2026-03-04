# Makefile

CMD_DIR := ./cmd/api
SWAG_MAIN := $(CMD_DIR)/main.go
SWAG_OUT := ./docs
BIN_DIR := ./bin

.PHONY: dev build swag clean

dev:
	go run $(CMD_DIR)/main.go

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/main $(CMD_DIR)/main.go

swag:
	go run github.com/swaggo/swag/cmd/swag@latest init \
	  -g $(SWAG_MAIN) \
	  -o $(SWAG_OUT) \
	  --parseDependency \
	  --parseInternal

clean:
	rm -rf $(BIN_DIR)

