# DIN Caddy Plugins Makefile
# Provides common development commands for building, testing, and running the DIN middleware

# Ensure bash is used for all shell commands to guarantee cross-environment reproducibility
SHELL := /bin/bash

.PHONY: help build run test test-verbose test-coverage test-race clean secure dev-deps lint format check-deps benchmark profile docker-build docker-run docker-compose

# Default target
.DEFAULT_GOAL := help

# Variables
BINARY_NAME=caddy
BUILD_DIR=build
GO_FILES=$(shell find . -name '*.go' -type f -not -path "./vendor/*")
TEST_TIMEOUT=30m
COVERAGE_OUT=coverage.out
PROFILE_OUT=cpu.prof

# Colors for output
GREEN=\033[0;32m
YELLOW=\033[1;33m
RED=\033[0;31m
NC=\033[0m # No Color

## Help
help: ## Show this help message
	@echo "$(GREEN)DIN Caddy Plugins - Available Commands$(NC)"
	@echo "======================================"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "$(YELLOW)%-15s$(NC) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## Development Commands
run: ## Run Caddy with private config (main development command)
	@echo "$(GREEN)Starting Caddy with private configuration...$(NC)"
	go tool xcaddy run -- --config Caddyfile.private --adapter caddyfile

run-dev: ## Run Caddy with development config
	@echo "$(GREEN)Starting Caddy with development configuration...$(NC)"
	go tool xcaddy run -- --config Caddyfile.dev --adapter caddyfile

run-prod: ## Run Caddy with production config
	@echo "$(GREEN)Starting Caddy with production configuration...$(NC)"
	go tool xcaddy run -- --config Caddyfile --adapter caddyfile

## Build Commands
build: ## Build Caddy with DIN plugins
	@echo "$(GREEN)Building Caddy with DIN plugins...$(NC)"
	@mkdir -p $(BUILD_DIR)
	go tool xcaddy build \
		--output $(BUILD_DIR)/$(BINARY_NAME) \
		--with github.com/DIN-center/din-caddy-plugins=. \
		--replace github.com/DIN-center/din-sc/apps/din-go=./upstream/github.com/DIN-center/din-sc/apps/din-go

build-version: ## Build with version info
	@echo "$(GREEN)Building Caddy with version info...$(NC)"
	@mkdir -p $(BUILD_DIR)
	go tool xcaddy build \
		--output $(BUILD_DIR)/$(BINARY_NAME) \
		--with github.com/DIN-center/din-caddy-plugins=. \
		--with github.com/caddyserver/caddy/v2=$(shell go list -m -versions github.com/caddyserver/caddy/v2 | awk '{print $$NF}')

## Testing Commands
test: ## Run all tests
	@echo "$(GREEN)Running all tests...$(NC)"
	go test -timeout $(TEST_TIMEOUT) ./...

test-verbose: ## Run tests with verbose output
	@echo "$(GREEN)Running tests with verbose output...$(NC)"
	go test -v -timeout $(TEST_TIMEOUT) ./...

test-coverage: ## Run tests with coverage report
	@echo "$(GREEN)Running tests with coverage...$(NC)"
	go test -timeout $(TEST_TIMEOUT) -coverprofile=$(COVERAGE_OUT) ./...
	go tool cover -html=$(COVERAGE_OUT) -o coverage.html
	@echo "$(YELLOW)Coverage report generated: coverage.html$(NC)"

test-race: ## Run tests with race detector
	@echo "$(GREEN)Running tests with race detector...$(NC)"
	go test -race -timeout $(TEST_TIMEOUT) ./...

test-modules: ## Run tests for specific modules
	@echo "$(GREEN)Running module tests...$(NC)"
	go test -v -timeout $(TEST_TIMEOUT) ./modules/...

test-lib: ## Run tests for lib packages
	@echo "$(GREEN)Running lib tests...$(NC)"
	go test -v -timeout $(TEST_TIMEOUT) ./lib/...

test-network: ## Run network handler tests
	@echo "$(GREEN)Running network handler tests...$(NC)"
	go test -v -timeout $(TEST_TIMEOUT) ./lib/network/...

test-short: ## Run short tests only
	@echo "$(GREEN)Running short tests...$(NC)"
	go test -short -timeout 5m ./...

benchmark: ## Run benchmark tests
	@echo "$(GREEN)Running benchmark tests...$(NC)"
	go test -bench=. -benchmem ./...

## Code Quality Commands
lint: check-deps ## Run linter
	@echo "$(GREEN)Running linter...$(NC)"
	golangci-lint run ./...

format: ## Format Go code
	@echo "$(GREEN)Formatting Go code...$(NC)"
	gofmt -s -w $(GO_FILES)
	go tool goimports -w $(GO_FILES)

vet: ## Run go vet
	@echo "$(GREEN)Running go vet...$(NC)"
	go vet ./...

secure: ## Run govulncheck
	@echo "$(GREEN)Running govulncheck...$(NC)"
	go tool govulncheck

check: format vet lint test secure ## Run all checks (format, vet, lint, test, secure)

## Performance Commands
profile: ## Run CPU profiling
	@echo "$(GREEN)Running CPU profiling...$(NC)"
	go test -cpuprofile=$(PROFILE_OUT) -bench=. ./...
	@echo "$(YELLOW)Profile saved to: $(PROFILE_OUT)$(NC)"
	@echo "$(YELLOW)View with: go tool pprof $(PROFILE_OUT)$(NC)"

mem-profile: ## Run memory profiling
	@echo "$(GREEN)Running memory profiling...$(NC)"
	go test -memprofile=mem.prof -bench=. ./...
	@echo "$(YELLOW)Memory profile saved to: mem.prof$(NC)"

## Dependency Commands
##
## Note that installing golangci-lint using `go install` is NOT recommended
## per the installation instructions. Instead, prefer installing tools using
## asdf according to the provided .tool-versions file.
##
## See: https://golangci-lint.run/docs/welcome/install/#install-from-sources
dev-deps: ## Install development dependencies
	@echo "$(GREEN)Installing development dependencies...$(NC)"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

check-deps: ## Check if development dependencies are installed
	@echo "$(GREEN)Checking dependencies...$(NC)"
	@command -v golangci-lint >/dev/null || (echo "$(RED)golangci-lint not found. Run 'make dev-deps'$(NC)" && exit 1)
	@command -v go tool goimports >/dev/null || (echo "$(RED)goimports not found. Run 'make dev-deps'$(NC)" && exit 1)
	@command -v go tool xcaddy >/dev/null || (echo "$(RED)xcaddy not found. Run 'make dev-deps'$(NC)" && exit 1)
	@command -v go tool mockgen >/dev/null || (echo "$(RED)mockgen not found. Run 'make dev-deps'$(NC)" && exit 1)
	@echo "$(GREEN)All dependencies are installed$(NC)"

update-deps: ## Update Go dependencies
	@echo "$(GREEN)Updating Go dependencies...$(NC)"
	go get -u ./...
	go mod tidy

## Docker Commands
docker-build: ## Build Docker image
	@echo "$(GREEN)Building Docker image...$(NC)"
	docker build -t localhost/din-caddy .

docker-run: ## Run Docker container
	@echo "$(GREEN)Running Docker container...$(NC)"
	docker run -p 80:80 -p 8443:443 localhost/din-caddy

docker-compose: ## Run Caddy monitored by the LGTM stack
	@echo "$(GREEN)Running Docker container with LGTM stack...$(NC)"
	UID=$(id -u) GID=$(id -g) docker-compose up -d

## Utility Commands
clean: ## Clean build artifacts and test files
	@echo "$(GREEN)Cleaning build artifacts...$(NC)"
	rm -rf $(BUILD_DIR)
	rm -f $(COVERAGE_OUT) coverage.html
	rm -f $(PROFILE_OUT) mem.prof
	rm -f caddy
	go clean -testcache
	go clean -modcache

validate-config: ## Validate Caddyfile configuration
	@echo "$(GREEN)Validating Caddyfile configuration...$(NC)"
	@if [ -f "Caddyfile.private" ]; then \
		xcaddy validate -- --config Caddyfile.private --adapter caddyfile; \
	else \
		echo "$(RED)Caddyfile.private not found$(NC)"; \
	fi

debug: ## Run with debug logging
	@echo "$(GREEN)Running with debug logging...$(NC)"
	xcaddy run -- --config Caddyfile.private --adapter caddyfile --debug

reload: ## Reload Caddy configuration
	@echo "$(GREEN)Reloading Caddy configuration...$(NC)"
	curl -X POST "http://localhost:2019/config/apps/http"

stop: ## Stop running Caddy instance
	@echo "$(GREEN)Stopping Caddy...$(NC)"
	curl -X POST "http://localhost:2019/stop"

## Git Commands
git-hooks: ## Install git hooks
	@echo "$(GREEN)Installing git hooks...$(NC)"
	@mkdir -p .git/hooks
	@echo '#!/bin/sh\nmake check' > .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "$(YELLOW)Pre-commit hook installed$(NC)"

## Documentation Commands
docs: ## Generate documentation
	@echo "$(GREEN)Generating documentation...$(NC)"
	go doc -all ./... > docs/API.md

## Release Commands
tag: ## Create a new git tag (usage: make tag VERSION=v1.0.0)
	@if [ -z "$(VERSION)" ]; then \
		echo "$(RED)Please provide VERSION: make tag VERSION=v1.0.0$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)Creating tag $(VERSION)...$(NC)"
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)

## Quick Commands
quick-test: ## Run quick tests (short + race detector disabled)
	@echo "$(GREEN)Running quick tests...$(NC)"
	go test -short ./...

dev: dev-deps git-hooks ## Set up development environment
	@echo "$(GREEN)Development environment setup complete!$(NC)"
	@echo "$(YELLOW)You can now run 'make run' to start development$(NC)"

ci: check test-race test-coverage ## Run CI checks
	@echo "$(GREEN)All CI checks passed!$(NC)"

# Show current status
status: ## Show project status
	@echo "$(GREEN)DIN Caddy Plugins Status:$(NC)"
	@echo "Go version: $(shell go version)"
	@echo "Working directory: $(shell pwd)"
	@echo "Git branch: $(shell git branch --show-current 2>/dev/null || echo 'N/A')"
	@echo "Go modules: $(shell [ -f go.mod ] && echo 'Enabled' || echo 'Disabled')"
	@echo "Build artifacts: $(shell [ -d $(BUILD_DIR) ] && echo 'Present' || echo 'None')"
	@echo "Test coverage: $(shell [ -f $(COVERAGE_OUT) ] && echo 'Available' || echo 'Not generated')"


# To download mockgen, run: ``
generate-mocks: ## Generate Mock interface
	go tool mockgen -source=./lib/auth/interface.go -package=auth -destination=./lib/auth/interface_mock.go
	go tool mockgen -source=./lib/prometheus/interface.go -package=prometheus -destination=./lib/prometheus/interface_mock.go
	go tool mockgen -source=./lib/auth/siwe/client.go -package=siwe -destination=./lib/auth/siwe/interface_mock.go
	go tool mockgen -source=./lib/watcherscore/interface.go -package=watcherscore -destination=./lib/watcherscore/interface_mock.go
	go tool mockgen -source=./lib/network/handlers.go -package=network -destination=./lib/network/interface_mock.go

.PHONY: tag quick-test dev ci status