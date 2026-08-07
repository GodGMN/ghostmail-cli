.PHONY: build build-all build-linux build-darwin build-windows test clean install help cover

# Variables
BINARY_NAME=ghostmail
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
# Reproducible build timestamp: prefer SOURCE_DATE_EPOCH (set by release CI),
# fall back to the current time for local development builds.
ifdef SOURCE_DATE_EPOCH
  DATE=$(shell date -u -d "@$(SOURCE_DATE_EPOCH)" +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -r "$(SOURCE_DATE_EPOCH)" +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%SZ")
else
  DATE=$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
endif
# -s -w strip debug info and DWARF tables for smaller release binaries;
# -trimpath removes absolute paths from the binary so builds are reproducible
# across machines.
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"
BUILD_FLAGS=-trimpath

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
	go build $(BUILD_FLAGS) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/ghostmail
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"

## build-linux: Build for Linux (amd64, arm64)
build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 go build $(BUILD_FLAGS) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/ghostmail
	GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/ghostmail
	@echo "Built: $(DIST_DIR)/$(BINARY_NAME)-linux-*"

## build-darwin: Build for macOS (amd64, arm64)
build-darwin:
	@echo "Building for macOS..."
	@mkdir -p $(DIST_DIR)
	GOOS=darwin GOARCH=amd64 go build $(BUILD_FLAGS) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/ghostmail
	GOOS=darwin GOARCH=arm64 go build $(BUILD_FLAGS) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/ghostmail
	@echo "Built: $(DIST_DIR)/$(BINARY_NAME)-darwin-*"

## build-windows: Build for Windows (amd64)
build-windows:
	@echo "Building for Windows amd64..."
	@mkdir -p $(DIST_DIR)
	GOOS=windows GOARCH=amd64 go build $(BUILD_FLAGS) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/ghostmail
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
	go install $(BUILD_FLAGS) $(LDFLAGS) ./cmd/ghostmail
	@echo "Installed to $(GOPATH)/bin/$(BINARY_NAME)"

## run: Run the application (for development)
run:
	go run $(BUILD_FLAGS) $(LDFLAGS) ./cmd/ghostmail

## fmt: Format Go code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	gofmt -s -w .

## vet: Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

## lint: Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install from https://golangci-lint.run/" && exit 1)
	golangci-lint run ./...

## cover: Generate a coverage report
cover:
	@echo "Generating coverage report..."
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

## check: Run all checks (fmt, vet, lint, test)
check: fmt vet lint test
	@echo "All checks passed!"

## ci-check: Run full CI checks locally (tests, vet, lint, vulnerability scan, formatting, module hygiene)
ci-check: test vet lint
	@echo "Checking go.mod/go.sum are tidy..."
	@cp go.mod go.mod.bak && cp go.sum go.sum.bak
	@go mod tidy
	@if ! diff -q go.mod.bak go.mod >/dev/null || ! diff -q go.sum.bak go.sum >/dev/null; then \
		echo "go.mod or go.sum is out of date. Run 'go mod tidy' and commit."; \
		rm -f go.mod.bak go.sum.bak; \
		exit 1; \
	fi
	@rm -f go.mod.bak go.sum.bak
	@echo "Scanning for reachable vulnerabilities..."
	go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...
	@echo "Checking formatting..."
	@if [ -n "$$(gofmt -s -l .)" ]; then \
		echo "Go code is not formatted:"; \
		gofmt -s -l .; \
		exit 1; \
	fi
	@echo "All CI checks passed!"
