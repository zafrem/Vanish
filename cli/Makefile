.PHONY: help build test lint fmt clean install coverage bench run-tests

# Default target
.DEFAULT_GOAL := help

# Binary name
BINARY_NAME=vanish
VERSION?=dev
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build the binary
	@echo "Building ${BINARY_NAME}..."
	go build ${LDFLAGS} -o ${BINARY_NAME} .
	@echo "Build complete: ${BINARY_NAME}"

install: build ## Install binary to GOPATH/bin
	@echo "Installing ${BINARY_NAME} to $(shell go env GOPATH)/bin..."
	go install ${LDFLAGS} .
	@echo "Installation complete"

test: ## Run tests
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...

test-short: ## Run tests without verbose output
	@echo "Running tests (short)..."
	go test -race ./...

coverage: test ## Run tests with coverage report
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

lint: ## Run linter
	@echo "Running linter..."
	@if ! command -v golangci-lint &> /dev/null; then \
		echo "golangci-lint not found. Installing..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	golangci-lint run --timeout 5m

lint-fix: ## Run linter and auto-fix issues
	@echo "Running linter with auto-fix..."
	@if ! command -v golangci-lint &> /dev/null; then \
		echo "golangci-lint not found. Installing..."; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; \
	fi
	golangci-lint run --fix --timeout 5m

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	@if command -v goimports &> /dev/null; then \
		goimports -w .; \
	else \
		echo "goimports not found, skipping import formatting"; \
	fi

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

clean: ## Remove build artifacts
	@echo "Cleaning..."
	rm -f ${BINARY_NAME}
	rm -f coverage.out coverage.html
	go clean
	@echo "Clean complete"

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod verify

tidy: ## Tidy go modules
	@echo "Tidying go modules..."
	go mod tidy

run-tests: fmt vet lint test ## Run all checks (format, vet, lint, test)
	@echo "All checks passed!"

ci: deps run-tests ## Run CI checks (download deps, format, vet, lint, test)
	@echo "CI checks complete!"

# Development helpers
dev-setup: ## Install development tools
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "Development tools installed"

# Quick test during development
quick: ## Quick test (no race detector, no coverage)
	@echo "Running quick tests..."
	go test ./...

# Show test coverage percentage
coverage-percent: test ## Show coverage percentage
	@go tool cover -func=coverage.out | grep total | awk '{print "Total coverage: " $$3}'
