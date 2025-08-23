# Makefile for Proxer
# Use make help to see available targets

.DEFAULT_GOAL := help

# Variables
GO_VERSION := $(shell go version | cut -d' ' -f3)
VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X main.Version=$(VERSION) -X main.GitCommit=$(COMMIT) -X main.BuildDate=$(DATE)

# Build settings
BINARY_NAME := pxc
BUILD_DIR := build
DIST_DIR := dist

# Test settings
TEST_TIMEOUT := 10m
COVERAGE_DIR := test/coverage
REPORTS_DIR := test/reports

# Colors
RED := \033[0;31m
GREEN := \033[0;32m
YELLOW := \033[1;33m
BLUE := \033[0;34m
PURPLE := \033[0;35m
CYAN := \033[0;36m
NC := \033[0m # No Color

##@ General

.PHONY: help
help: ## Display this help message
	@awk 'BEGIN {FS = ":.*##"; printf "Usage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: info
info: ## Show project information
	@echo "$(CYAN)Project Information:$(NC)"
	@echo "  Name:        Proxer"
	@echo "  Version:     $(VERSION)"
	@echo "  Commit:      $(COMMIT)"
	@echo "  Build Date:  $(DATE)"
	@echo "  Go Version:  $(GO_VERSION)"
	@echo "  Binary:      $(BINARY_NAME)"

##@ Development

.PHONY: deps
deps: ## Install dependencies
	@echo "$(BLUE)Installing dependencies...$(NC)"
	@go mod download
	@go mod tidy

.PHONY: deps-dev
deps-dev: deps ## Install development dependencies
	@echo "$(BLUE)Installing development dependencies...$(NC)"
	@which golangci-lint || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@which gosec || go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	@which govulncheck || go install golang.org/x/vuln/cmd/govulncheck@latest

.PHONY: clean
clean: ## Clean build artifacts
	@echo "$(YELLOW)Cleaning build artifacts...$(NC)"
	@rm -rf $(BUILD_DIR) $(DIST_DIR) $(COVERAGE_DIR) $(REPORTS_DIR)
	@rm -f $(BINARY_NAME) coverage.txt

##@ Building

.PHONY: build
build: ## Build the binary
	@echo "$(GREEN)Building $(BINARY_NAME)...$(NC)"
	@mkdir -p $(BUILD_DIR)
	@go build -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/pxc

.PHONY: install
install: build ## Build and install the binary
	@echo "$(GREEN)Installing $(BINARY_NAME)...$(NC)"
	@go install -ldflags="$(LDFLAGS)" ./cmd/pxc

.PHONY: build-all
build-all: clean ## Build binaries for all platforms
	@echo "$(GREEN)Building binaries for all platforms...$(NC)"
	@mkdir -p $(DIST_DIR)
	@echo "  Linux (amd64)..."
	@GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/pxc
	@echo "  Linux (arm64)..."
	@GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 ./cmd/pxc
	@echo "  macOS (amd64)..."
	@GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/pxc
	@echo "  macOS (arm64)..."
	@GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd/pxc
	@echo "  Windows (amd64)..."
	@GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/pxc
	@echo "$(GREEN)All binaries built successfully!$(NC)"

##@ Testing

.PHONY: test
test: ## Run unit tests
	@echo "$(BLUE)Running unit tests...$(NC)"
	@chmod +x ./test/run_tests.sh
	@./test/run_tests.sh unit -v

.PHONY: test-integration
test-integration: ## Run integration tests
	@echo "$(BLUE)Running integration tests...$(NC)"
	@chmod +x ./test/run_tests.sh
	@./test/run_tests.sh integration -v

.PHONY: test-e2e
test-e2e: ## Run end-to-end tests
	@echo "$(BLUE)Running end-to-end tests...$(NC)"
	@chmod +x ./test/run_tests.sh
	@./test/run_tests.sh e2e -v

.PHONY: test-all
test-all: ## Run all tests
	@echo "$(BLUE)Running all tests...$(NC)"
	@chmod +x ./test/run_tests.sh
	@./test/run_tests.sh all -v

.PHONY: test-ci
test-ci: ## Run CI test suite
	@echo "$(BLUE)Running CI test suite...$(NC)"
	@chmod +x ./test/run_tests.sh
	@./test/run_tests.sh ci -v

.PHONY: coverage
coverage: ## Run tests with coverage analysis
	@echo "$(BLUE)Running tests with coverage analysis...$(NC)"
	@chmod +x ./test/run_tests.sh
	@./test/run_tests.sh coverage -v

.PHONY: benchmark
benchmark: ## Run benchmark tests
	@echo "$(BLUE)Running benchmark tests...$(NC)"
	@chmod +x ./test/run_tests.sh
	@./test/run_tests.sh benchmark -v

##@ Quality Assurance

.PHONY: lint
lint: ## Run linters and static analysis
	@echo "$(PURPLE)Running linters and static analysis...$(NC)"
	@chmod +x ./test/run_tests.sh
	@./test/run_tests.sh lint -v

.PHONY: fmt
fmt: ## Format code
	@echo "$(PURPLE)Formatting code...$(NC)"
	@go fmt ./...
	@goimports -w . 2>/dev/null || true

.PHONY: vet
vet: ## Run go vet
	@echo "$(PURPLE)Running go vet...$(NC)"
	@go vet ./...

.PHONY: security
security: ## Run security scans
	@echo "$(RED)Running security scans...$(NC)"
	@chmod +x ./test/run_tests.sh
	@./test/run_tests.sh security -v

.PHONY: check
check: fmt vet lint test-ci ## Run all quality checks (fmt, vet, lint, test-ci)

##@ Documentation

.PHONY: docs
docs: ## Generate documentation
	@echo "$(CYAN)Generating documentation...$(NC)"
	@mkdir -p docs/generated
	@go run ./cmd/pxc help > docs/generated/cli-help.txt 2>&1 || true
	@echo "$(GREEN)Documentation generated in docs/generated/$(NC)"

##@ Development Helpers

.PHONY: run
run: build ## Build and run with example
	@echo "$(GREEN)Running $(BINARY_NAME) --help...$(NC)"
	@$(BUILD_DIR)/$(BINARY_NAME) --help

.PHONY: debug
debug: ## Build and run with debug flags
	@echo "$(YELLOW)Building with debug flags...$(NC)"
	@go build -gcflags="all=-N -l" -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME)-debug ./cmd/pxc
	@echo "$(GREEN)Debug binary built at $(BUILD_DIR)/$(BINARY_NAME)-debug$(NC)"

.PHONY: profile
profile: ## Build with profiling enabled
	@echo "$(YELLOW)Building with profiling enabled...$(NC)"
	@go build -ldflags="$(LDFLAGS)" -tags="profile" -o $(BUILD_DIR)/$(BINARY_NAME)-profile ./cmd/pxc
	@echo "$(GREEN)Profile binary built at $(BUILD_DIR)/$(BINARY_NAME)-profile$(NC)"

##@ Release

.PHONY: release-check
release-check: check ## Check if ready for release
	@echo "$(BLUE)Checking release readiness...$(NC)"
	@git diff --quiet || (echo "$(RED)Error: Working directory is not clean$(NC)" && exit 1)
	@git diff --cached --quiet || (echo "$(RED)Error: Staging area is not clean$(NC)" && exit 1)
	@test -n "$(VERSION)" || (echo "$(RED)Error: No version tag found$(NC)" && exit 1)
	@echo "$(GREEN)✓ Ready for release$(NC)"

.PHONY: tag
tag: ## Create a new git tag (usage: make tag VERSION=v1.0.0)
	@test -n "$(VERSION)" || (echo "$(RED)Error: VERSION is required. Usage: make tag VERSION=v1.0.0$(NC)" && exit 1)
	@git tag -a $(VERSION) -m "Release $(VERSION)"
	@echo "$(GREEN)Tagged $(VERSION). Push with: git push origin $(VERSION)$(NC)"

##@ Docker

.PHONY: docker-build
docker-build: ## Build Docker image
	@echo "$(BLUE)Building Docker image...$(NC)"
	@docker build -t proxer/pxc:$(VERSION) .
	@docker build -t proxer/pxc:latest .
	@echo "$(GREEN)Docker image built: proxer/pxc:$(VERSION)$(NC)"

.PHONY: docker-run
docker-run: docker-build ## Build and run Docker container
	@echo "$(BLUE)Running Docker container...$(NC)"
	@docker run --rm proxer/pxc:latest --help

##@ Utilities

.PHONY: mod-tidy
mod-tidy: ## Tidy go modules
	@echo "$(BLUE)Tidying Go modules...$(NC)"
	@go mod tidy

.PHONY: mod-verify
mod-verify: ## Verify go modules
	@echo "$(BLUE)Verifying Go modules...$(NC)"
	@go mod verify

.PHONY: list-targets
list-targets: ## List all make targets
	@echo "$(CYAN)Available targets:$(NC)"
	@make -qp | awk -F':' '/^[a-zA-Z0-9][^$$#\/\t=]*:([^=]|$$)/ {split($$1,A,/ /);for(i in A)print A[i]}' | sort

##@ Maintenance

.PHONY: update-deps
update-deps: ## Update dependencies
	@echo "$(BLUE)Updating dependencies...$(NC)"
	@go get -u ./...
	@go mod tidy
	@echo "$(GREEN)Dependencies updated$(NC)"

.PHONY: audit
audit: ## Audit dependencies for vulnerabilities
	@echo "$(BLUE)Auditing dependencies...$(NC)"
	@govulncheck ./... || echo "$(YELLOW)govulncheck not installed. Run: make deps-dev$(NC)"

# Hidden targets (internal use)
.PHONY: _check-tools
_check-tools:
	@which go > /dev/null || (echo "$(RED)Error: Go is not installed$(NC)" && exit 1)
	@echo "$(GREEN)✓ Required tools are available$(NC)"

# Make directories if they don't exist
$(BUILD_DIR) $(DIST_DIR) $(COVERAGE_DIR) $(REPORTS_DIR):
	@mkdir -p $@