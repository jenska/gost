GO ?= go
APP ?= gost
CMD ?= ./cmd/gost
BIN_DIR ?= bin
BIN ?= $(BIN_DIR)/$(APP)
FRAMES ?= 300
ARGS ?=

.PHONY: help build test run headless clean

help:
	@printf "Available targets:\n"
	@printf "  make build            Build the emulator binary\n"
	@printf "  make test             Run the Go test suite\n"
	@printf "  make run              Run the desktop emulator\n"
	@printf "  make headless         Run headless for FRAMES=%s\n" "$(FRAMES)"
	@printf "  make clean            Remove built artifacts\n"
	@printf "\n"
	@printf "Examples:\n"
	@printf "  make build\n"
	@printf "  make run ARGS='--fullscreen'\n"
	@printf "  make headless FRAMES=600 ARGS='--trace cpu'\n"

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN) $(CMD)

test:
	$(GO) test ./...

run:
	$(GO) run $(CMD) $(ARGS)

headless:
	$(GO) run $(CMD) --headless --frames $(FRAMES) $(ARGS)

clean:
	rm -rf $(BIN_DIR)
