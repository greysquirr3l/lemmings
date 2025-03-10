# Makefile for Lemmings project

# Configuration
BINARY_NAME=lemmings
MAIN_PACKAGE=./examples/simple/main.go
MODULE_NAME=github.com/greysquirr3l/lemmings
GO_FILES=$(shell find . -name "*.go" -not -path "./vendor/*")
GO_PACKAGES=$(shell go list ./... | grep -v /vendor/)

# Build-time variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-ldflags "-X '$(MODULE_NAME)/internal/version.Version=$(VERSION)' \
				  -X '$(MODULE_NAME)/internal/version.GitCommit=$(GIT_COMMIT)' \
				  -X '$(MODULE_NAME)/internal/version.BuildTime=$(BUILD_TIME)'"

# Tools
GOLANGCI_LINT=$(shell which golangci-lint 2>/dev/null || echo "${GOPATH}/bin/golangci-lint")
GOCOV=$(shell which gocov 2>/dev/null || echo "${GOPATH}/bin/gocov")

# Colors for terminal output
COLOR_RESET=\033[0m
COLOR_GREEN=\033[32m
COLOR_YELLOW=\033[33m
COLOR_BLUE=\033[34m
COLOR_RED=\033[31m

.PHONY: all
all: lint test build

# Build targets
.PHONY: build
build: ## Build the binary
	@echo "${COLOR_BLUE}Building $(BINARY_NAME)...${COLOR_RESET}"
	@go build $(LDFLAGS) -o bin/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "${COLOR_GREEN}Build successful!${COLOR_RESET}"

.PHONY: install
install: ## Install the binary
	@echo "${COLOR_BLUE}Installing $(BINARY_NAME)...${COLOR_RESET}"
	@go install $(LDFLAGS) $(MAIN_PACKAGE)
	@echo "${COLOR_GREEN}Install successful!${COLOR_RESET}"

.PHONY: run
run: ## Run the application
	@echo "${COLOR_BLUE}Running $(BINARY_NAME)...${COLOR_RESET}"
	@go run $(LDFLAGS) $(MAIN_PACKAGE)

# Build examples
.PHONY: examples
examples: ## Build all examples
	@echo "${COLOR_BLUE}Building examples...${COLOR_RESET}"
	@go build -o bin/simple ./examples/simple/main.go
	@go build -o bin/advanced ./examples/advanced/main.go
	@echo "${COLOR_GREEN}Examples built successfully!${COLOR_RESET}"

# Cross compilation
.PHONY: build-all
build-all: build-linux build-darwin build-windows ## Build for all platforms

.PHONY: build-linux
build-linux: ## Build for Linux
	@echo "${COLOR_BLUE}Building for Linux...${COLOR_RESET}"
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 $(MAIN_PACKAGE)

.PHONY: build-darwin
build-darwin: ## Build for macOS
	@echo "${COLOR_BLUE}Building for macOS...${COLOR_RESET}"
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)

.PHONY: build-windows
build-windows: ## Build for Windows
	@echo "${COLOR_BLUE}Building for Windows...${COLOR_RESET}"
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)

# Test targets
.PHONY: test
test: ## Run tests
	@echo "${COLOR_BLUE}Running tests...${COLOR_RESET}"
	@go test -v $(GO_PACKAGES)

.PHONY: test-race
test-race: ## Run tests with race detection
	@echo "${COLOR_BLUE}Running tests with race detection...${COLOR_RESET}"
	@go test -race -v $(GO_PACKAGES)

.PHONY: test-short
test-short: ## Run only short tests
	@echo "${COLOR_BLUE}Running short tests...${COLOR_RESET}"
	@go test -short -v $(GO_PACKAGES)

.PHONY: test-cover
test-cover: ## Run tests with coverage
	@echo "${COLOR_BLUE}Running tests with coverage...${COLOR_RESET}"
	@go test -cover -v $(GO_PACKAGES)

.PHONY: test-cover-html
test-cover-html: ## Generate HTML coverage report
	@echo "${COLOR_BLUE}Generating coverage report...${COLOR_RESET}"
	@go test -coverprofile=coverage.out $(GO_PACKAGES)
	@go tool cover -html=coverage.out -o coverage.html
	@echo "${COLOR_GREEN}Coverage report generated: coverage.html${COLOR_RESET}"

.PHONY: benchmark
benchmark: ## Run benchmarks
	@echo "${COLOR_BLUE}Running benchmarks...${COLOR_RESET}"
	@go test -bench=. -benchmem $(GO_PACKAGES)

# Lint and formatting targets
.PHONY: lint
lint: ## Run linters
	@echo "${COLOR_BLUE}Running linters...${COLOR_RESET}"
	@golangci-lint run -v ./...

.PHONY: fmt
fmt: ## Format code
	@echo "${COLOR_BLUE}Formatting code...${COLOR_RESET}"
	@gofmt -s -w $(GO_FILES)

.PHONY: vet
vet: ## Run go vet
	@echo "${COLOR_BLUE}Running go vet...${COLOR_RESET}"
	@go vet $(GO_PACKAGES)

.PHONY: tidy
tidy: ## Tidy dependencies
	@echo "${COLOR_BLUE}Tidying dependencies...${COLOR_RESET}"
	@go mod tidy
	@echo "${COLOR_GREEN}Dependencies tidied!${COLOR_RESET}"

# Documentation targets
.PHONY: godoc
godoc: ## Start godoc server
	@echo "${COLOR_BLUE}Starting godoc server on http://localhost:6060${COLOR_RESET}"
	@godoc -http=:6060

.PHONY: docs
docs: ## Generate documentation
	@echo "${COLOR_BLUE}Generating documentation...${COLOR_RESET}"
	@mkdir -p docs
	@go doc -all $(MODULE_NAME) > docs/api.txt
	@echo "${COLOR_GREEN}Documentation generated in docs/api.txt${COLOR_RESET}"

# Clean targets
.PHONY: clean
clean: ## Clean build artifacts
	@echo "${COLOR_BLUE}Cleaning build artifacts...${COLOR_RESET}"
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@rm -f profile.out
	@echo "${COLOR_GREEN}Clean complete!${COLOR_RESET}"

.PHONY: clean-all
clean-all: clean ## Clean everything including dependencies
	@echo "${COLOR_BLUE}Cleaning all artifacts...${COLOR_RESET}"
	@go clean -cache -modcache -i -r
	@echo "${COLOR_GREEN}Clean all complete!${COLOR_RESET}"

# Tool installation targets
.PHONY: tools
tools: $(GOLANGCI_LINT) $(GOCOV) ## Install tools

$(GOLANGCI_LINT): ## Install golangci-lint
	@echo "${COLOR_BLUE}Installing golangci-lint...${COLOR_RESET}"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

$(GOCOV): ## Install gocov
	@echo "${COLOR_BLUE}Installing gocov...${COLOR_RESET}"
	@go install github.com/axw/gocov/gocov@latest

# Dependency management
.PHONY: deps
deps: ## List dependencies
	@echo "${COLOR_BLUE}Listing dependencies...${COLOR_RESET}"
	@go list -m all

.PHONY: deps-update
deps-update: ## Update dependencies
	@echo "${COLOR_BLUE}Updating dependencies...${COLOR_RESET}"
	@go get -u ./...
	@go mod tidy
	@echo "${COLOR_GREEN}Dependencies updated!${COLOR_RESET}"

# Utility targets
.PHONY: version
version: ## Show version information
	@echo "${COLOR_BLUE}Version: ${COLOR_GREEN}$(VERSION)${COLOR_RESET}"
	@echo "${COLOR_BLUE}Git commit: ${COLOR_GREEN}$(GIT_COMMIT)${COLOR_RESET}"
	@echo "${COLOR_BLUE}Build time: ${COLOR_GREEN}$(BUILD_TIME)${COLOR_RESET}"

# Git related targets
.PHONY: git-check
git-check: ## Check Git authentication status
	@echo "${COLOR_BLUE}Checking Git authentication status...${COLOR_RESET}"
	@chmod +x ./scripts/ssh-helper.sh
	@./scripts/ssh-helper.sh status
	@./scripts/ssh-helper.sh test

.PHONY: git-reinit
git-reinit: ## Reinitialize Git authentication
	@echo "${COLOR_BLUE}Reinitializing Git authentication...${COLOR_RESET}"
	@chmod +x ./scripts/ssh-helper.sh
	@./scripts/ssh-helper.sh reinit

.PHONY: git-find-keys
git-find-keys: ## Find available authentication keys
	@echo "${COLOR_BLUE}Finding available authentication keys...${COLOR_RESET}"
	@chmod +x ./scripts/ssh-helper.sh
	@./scripts/ssh-helper.sh find

.PHONY: git-gpg-setup
git-gpg-setup: ## Set up GPG key for Git signing
	@echo "${COLOR_BLUE}Setting up GPG key for Git...${COLOR_RESET}"
	@chmod +x ./scripts/ssh-helper.sh
	@read -p "Enter GPG key ID: " KEYID && \
	./scripts/ssh-helper.sh gpg $$KEYID

.PHONY: git-creds
git-creds: ## Set up Git credentials helper
	@echo "${COLOR_BLUE}Setting up Git credentials helper...${COLOR_RESET}"
	@chmod +x ./scripts/ssh-helper.sh
	@./scripts/ssh-helper.sh credentials

.PHONY: git-tag
git-tag: git-check ## Create a new Git tag
	@echo "${COLOR_BLUE}Current version: ${COLOR_GREEN}$(VERSION)${COLOR_RESET}"
	@read -p "Enter new version tag (e.g. v1.0.0): " TAG && \
	git tag -a $$TAG -m "Release $$TAG" && \
	echo "${COLOR_GREEN}Created tag: $$TAG${COLOR_RESET}" && \
	echo "Use 'git push origin $$TAG' to push the tag."

.PHONY: git-push
git-push: git-check ## Push to Git repository
	@echo "${COLOR_BLUE}Pushing to Git repository...${COLOR_RESET}"
	@git push || (echo "${COLOR_YELLOW}Git push failed. Trying to reinitialize authentication...${COLOR_RESET}" && \
	make git-reinit && git push)

.PHONY: git-pull
git-pull: git-check ## Pull from Git repository
	@echo "${COLOR_BLUE}Pulling from Git repository...${COLOR_RESET}"
	@git pull || (echo "${COLOR_YELLOW}Git pull failed. Trying to reinitialize authentication...${COLOR_RESET}" && \
	make git-reinit && git pull)

.PHONY: help
help: ## Display this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "${COLOR_GREEN}%-20s${COLOR_RESET} %s\n", $$1, $$2}'

# Default to help if no target is specified
.DEFAULT_GOAL := help
