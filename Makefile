.PHONY: build build-all build-linux build-darwin build-windows test clean install help

# Variables
BINARY_NAME=ghostmail
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Build directories
BUILD_DIR=build
DIST_DIR=dist

# Default target
.DEFAULT_GOAL := help

## help: Show this help message
help:
	@echo "Available targets:"
	@awk '/^##/{if(NR>1)print ""} /^##/{gsub(/^## /,"");printf "  \033[36m%-20s\033[0m",$$0;next} /^[a-zA-Z_-]+:/{gsub(/:.*/,"");printf " %s\n",$$0}' $(MAKEFILE_LIST)

## build: Build binary for current platform
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/ghostmail
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"

## build-linux: Build for Linux (amd64)
build-linux:
	@echo "Building for Linux amd64..."
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/ghostmail
	@echo "Built: $(DIST_DIR)/$(BINARY_NAME)-linux-amd64"

## build-darwin: Build for macOS (amd64, arm64)
build-darwin:
	@echo "Building for macOS..."
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/ghostmail
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/ghostmail
	@echo "Built: $(DIST_DIR)/$(BINARY_NAME)-darwin-*"

## build-windows: Build for Windows (amd64)
build-windows:
	@echo "Building for Windows amd64..."
	@mkdir -p $(DIST_DIR)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/ghostmail
	@echo "Built: $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe"

## build-all: Build for all platforms
build-all: clean build-linux build-darwin build-windows
	@echo "All builds complete"
	@ls -la $(DIST_DIR)/

## test: Run tests
test:
	@echo "Running tests..."
	go test -v -race ./...

## clean: Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)/ $(DIST_DIR)/
	@echo "Clean complete"

## install: Install to GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	go install $(LDFLAGS) ./cmd/ghostmail
	@echo "Installed to $(GOPATH)/bin/$(BINARY_NAME)"

## run: Run the application (for development)
run:
	go run $(LDFLAGS) ./cmd/ghostmail

## fmt: Format Go code
fmt:
	@echo "Formatting code..."
	go fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

## lint: Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install from https://golangci-lint.run/" && exit 1)
	golangci-lint run

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

## check: Run all checks (fmt, vet, test)
check: fmt vet test
	@echo "All checks passed!"
