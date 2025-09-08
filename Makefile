ifeq ($(OS),Windows_NT)
	CLEAR_COMMAND = @cls
else
	CLEAR_COMMAND = @clear
endif


# Default Port number
PORT ?= 4000
SEEDS ?=
BLOCK_DIR_NAME ?= blocks
FLOOD ?= false
PROPOSER ?= false
ID ?=
TESTER ?= false

build:
	@go build -o ./bin/blockchain

run: build
	@$(CLEAR_COMMAND)
	./bin/blockchain --port=$(PORT) --id=$(ID) --tester=$(TESTER) --block-dir-name=$(BLOCK_DIR_NAME) --flood=$(FLOOD) --proposer=$(PROPOSER)

