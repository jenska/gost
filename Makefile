GO ?= go
APP ?= gost
CMD ?= ./cmd/gost
BIN_DIR ?= bin
BIN ?= $(BIN_DIR)/$(APP)
WASM_DIR ?= wasm
WASM_BIN ?= $(WASM_DIR)/gost.wasm
WASM_EXEC ?= $(WASM_DIR)/wasm_exec.js
GO_WASM_EXEC ?= $(shell $(GO) env GOROOT)/lib/wasm/wasm_exec.js
FRAMES ?= 1000
ARGS ?=
ROM ?=
RUN_CONFIG ?= configs/atari-1040ste-mono.json
RUN_FLOPPY_ARGS :=

.PHONY: help ci build test run run-rom  headless headless-rom  wasm clean

help:
	@printf "Available targets:\n"
	@printf "  make ci               Run tests and build native + wasm artifacts\n"
	@printf "  make build            Build the emulator binary\n"
	@printf "  make test             Run the Go test suite\n"
	@printf "  make run              Run the desktop emulator with %s\n" "$(RUN_CONFIG)"
	@printf "  make run-rom          Run with a local ROM via ROM=/path/to/tos.rom\n"
	@printf "  make headless         Run headless for FRAMES=%s\n" "$(FRAMES)"
	@printf "  make headless-rom     Run headless with a local ROM via ROM=/path/to/tos.rom\n"
	@printf "  make wasm             Build wasm/gost.wasm for the browser demo\n"
	@printf "  make clean            Remove built artifacts\n"
	@printf "\n"
	@printf "Examples:\n"
	@printf "  make ci\n"
	@printf "  make build\n"
	@printf "  make run ARGS='--fullscreen'\n"
	@printf "  make run RUN_CONFIG=configs/atari-1040stf-color.json\n"
	@printf "  make run-rom ROM=TOS/TOS104GE.IMG ARGS='--preset mega-st'\n"
	@printf "  make headless FRAMES=1000 ARGS='--trace cpu'\n"
	@printf "  make headless-rom ROM=TOS/TOS102GE.IMG FRAMES=1000 ARGS='--preset mega-st --trace boot'\n"
	@printf "  make run ARGS='--floppy-a /path/to/disk.msa'\n"
	@printf "  make wasm\n"

ci: test build wasm

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN) $(CMD)

test:
	$(GO) test ./...

run:
	$(GO) run $(CMD) --config $(RUN_CONFIG) $(RUN_FLOPPY_ARGS) $(ARGS)

run-rom:
	@test -n "$(ROM)" || (printf "set ROM=/path/to/tos.rom\n" && exit 1)
	@test -f "$(ROM)" || (printf "missing local ROM at %s\n" "$(ROM)" && exit 1)
	$(GO) run $(CMD) --rom "$(ROM)" $(RUN_FLOPPY_ARGS) $(ARGS)

headless:
	$(GO) run $(CMD) --headless --frames $(FRAMES) $(RUN_FLOPPY_ARGS) $(ARGS)

headless-rom:
	@test -n "$(ROM)" || (printf "set ROM=/path/to/tos.rom\n" && exit 1)
	@test -f "$(ROM)" || (printf "missing local ROM at %s\n" "$(ROM)" && exit 1)
	$(GO) run $(CMD) --headless --frames $(FRAMES) --rom "$(ROM)" $(RUN_FLOPPY_ARGS) $(ARGS)

wasm:
	@test -f "$(GO_WASM_EXEC)" || (printf "missing wasm_exec.js at %s\n" "$(GO_WASM_EXEC)" && exit 1)
	@mkdir -p $(WASM_DIR)
	cp "$(GO_WASM_EXEC)" "$(WASM_EXEC)"
	GOOS=js GOARCH=wasm $(GO) build -o $(WASM_BIN) $(CMD)
	@touch $(WASM_DIR)/.nojekyll

clean:
	rm -rf $(BIN_DIR)
	rm -f $(WASM_BIN) $(WASM_EXEC) $(WASM_DIR)/.nojekyll
