APP_NAME := imagedupfinder
BIN_DIR := bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: all build clean test run install cross help

all: build

## build: Build for current platform
build:
	go build $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME) .

## clean: Remove build artifacts
clean:
	rm -rf $(BIN_DIR)
	go clean

## test: Run tests
test:
	go test -v ./...

## run: Build and run with arguments (usage: make run ARGS="scan ./photos")
run: build
	$(BIN_DIR)/$(APP_NAME) $(ARGS)

## install: Install to GOPATH/bin
install:
	go install $(LDFLAGS) .

## cross: Build for all platforms
cross: cross-linux cross-darwin cross-windows

cross-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME)_linux_amd64 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME)_linux_arm64 .

cross-darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME)_darwin_amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME)_darwin_arm64 .

cross-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME)_windows_amd64.exe .

## tidy: Run go mod tidy
tidy:
	go mod tidy

## fmt: Format code
fmt:
	go fmt ./...

## lint: Run linter (requires golangci-lint)
lint:
	golangci-lint run

## help: Show this help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
