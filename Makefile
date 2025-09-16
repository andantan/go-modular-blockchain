ifeq ($(OS),Windows_NT)
	CLEAR_COMMAND = @cls
else
	CLEAR_COMMAND = @clear
endif


PORT ?= 4000
DOMAIN ?=
VALIDATOR ?= false
PROPOSER ?= false
TESTER ?= false

ARGS = --port=$(PORT) --domain=$(DOMAIN)

ifeq ($(VALIDATOR), true)
    ARGS += --validator
endif

ifeq ($(PROPOSER), true)
	ARGS += --proposer
endif

ifeq ($(TESTER), true)
    ARGS += --tester
endif

build:
	@go build -o ./bin/blockchain

run: build
	@$(CLEAR_COMMAND)
	./bin/blockchain $(ARGS)

