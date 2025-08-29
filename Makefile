ifeq ($(OS),Windows_NT)
	CLEAR_COMMAND = @cls
else
	CLEAR_COMMAND = @clear
endif

test:
	@go test ./...

test-verbose:
	@go test -v ./...

test-race:
	@go test ./... --race

build:
	@go build -o ./bin/blockchain

run: build
	@$(CLEAR_COMMAND)
	./bin/blockchain