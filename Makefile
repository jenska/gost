GO ?= go
APP ?= gost
CMD ?= ./cmd/gost
BIN_DIR ?= bin
BIN ?= $(BIN_DIR)/$(APP)
WASM_DIR ?= docs
WASM_BIN ?= $(WASM_DIR)/gost.wasm
WASM_EXEC ?= $(WASM_DIR)/wasm_exec.js
GO_WASM_EXEC ?= $(shell $(GO) env GOROOT)/lib/wasm/wasm_exec.js
FRAMES ?= 300
ARGS ?=
DEFAULT_FLOPPY ?= downloads/atari-st/PDATS321.msa
RUN_FLOPPY_ARGS :=

ifneq ("$(wildcard $(DEFAULT_FLOPPY))","")
RUN_FLOPPY_ARGS += --floppy-a $(DEFAULT_FLOPPY)
endif

.PHONY: help build test run headless wasm clean

help:
	@printf "Available targets:\n"
	@printf "  make build            Build the emulator binary\n"
	@printf "  make test             Run the Go test suite\n"
	@printf "  make run              Run the desktop emulator\n"
	@printf "  make headless         Run headless for FRAMES=%s\n" "$(FRAMES)"
	@printf "  make wasm             Build docs/gost.wasm for the browser demo\n"
	@printf "  make clean            Remove built artifacts\n"
	@printf "\n"
	@printf "Examples:\n"
	@printf "  make build\n"
	@printf "  make run ARGS='--fullscreen'\n"
	@printf "  make headless FRAMES=600 ARGS='--trace cpu'\n"
	@printf "  make run ARGS='--floppy-a /path/to/disk.msa'\n"
	@printf "  make wasm\n"

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN) $(CMD)

test:
	$(GO) test ./...

run:
	$(GO) run $(CMD) $(RUN_FLOPPY_ARGS) $(ARGS)

headless:
	$(GO) run $(CMD) --headless --frames $(FRAMES) $(RUN_FLOPPY_ARGS) $(ARGS)

wasm:
	@test -f "$(GO_WASM_EXEC)" || (printf "missing wasm_exec.js at %s\n" "$(GO_WASM_EXEC)" && exit 1)
	@mkdir -p $(WASM_DIR)
	cp "$(GO_WASM_EXEC)" "$(WASM_EXEC)"
	GOOS=js GOARCH=wasm $(GO) build -o $(WASM_BIN) $(CMD)
	@touch $(WASM_DIR)/.nojekyll

clean:
	rm -rf $(BIN_DIR)
	rm -f $(WASM_BIN) $(WASM_EXEC) $(WASM_DIR)/.nojekyll
