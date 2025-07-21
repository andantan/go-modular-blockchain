ifeq ($(OS),Windows_NT)
	CLEAR_COMMAND = @cls
else
	CLEAR_COMMAND = @clear
endif

test-race:
	@go test ./... --race

test:
	@go test ./...

build:
	@go build -o ./bin/blockchain-node

run: build
	@$(CLEAR_COMMAND)
	./bin/blockchain-node
