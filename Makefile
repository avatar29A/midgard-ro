# Midgard RO Client - Makefile
# Requires: Go 1.22+, SDL2

.PHONY: all build build-tools run run-debug run-release clean test deps check fmt lint help

# Build settings
BINARY_NAME := midgard
BUILD_DIR := build
CMD_DIR := ./cmd/client

# Go settings
GOFLAGS := -v
LDFLAGS := -s -w

# Default target
all: build

## Build

build: ## Build the client binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)"

build-debug: ## Build with debug symbols
	@echo "Building $(BINARY_NAME) (debug)..."
	@mkdir -p $(BUILD_DIR)
	go build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-debug $(CMD_DIR)
	@echo "Built: $(BUILD_DIR)/$(BINARY_NAME)-debug"

build-tools: ## Build CLI tools (grftool)
	@echo "Building tools..."
	@mkdir -p $(BUILD_DIR)
	go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/grftool ./cmd/grftool
	@echo "Built: $(BUILD_DIR)/grftool"

## Run

run: ## Run the client
	go run $(CMD_DIR)

run-debug: ## Run with debug mode (uses config.yaml + race detector)
	go run $(CMD_DIR) --config config.yaml --debug

run-release: build ## Run optimized release build
	./$(BUILD_DIR)/$(BINARY_NAME)

## Development

deps: ## Install/update dependencies
	go mod download
	go mod tidy

check: ## Verify dependencies
	go mod verify

fmt: ## Format code
	go fmt ./...
	goimports -w .

lint: ## Run linter (requires golangci-lint)
	golangci-lint run ./...

vet: ## Run go vet
	go vet ./...

## Testing

test: ## Run all tests
	go test ./...

test-v: ## Run tests with verbose output
	go test -v ./...

test-cover: ## Run tests with coverage
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## Cleanup

clean: ## Remove build artifacts
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@echo "Cleaned build artifacts"

## Environment Setup

env-check: ## Check if required tools are installed
	@echo "Checking environment..."
	@echo -n "Go: "; go version || echo "NOT INSTALLED"
	@echo -n "SDL2: "; pkg-config --modversion sdl2 2>/dev/null || echo "NOT INSTALLED"
	@echo -n "GCC: "; gcc --version | head -1 || echo "NOT INSTALLED"

env-install-macos: ## Install dependencies on macOS
	brew install go sdl2
	@echo "Dependencies installed. Run 'make deps' to download Go modules."

env-install-linux: ## Install dependencies on Linux (Debian/Ubuntu)
	sudo apt update
	sudo apt install -y golang libsdl2-dev build-essential
	@echo "Dependencies installed. Run 'make deps' to download Go modules."

## Help

help: ## Show this help
	@echo "Midgard RO Client - Available targets:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
	@echo ""
	@echo "Examples:"
	@echo "  make run          # Run the client (go run)"
	@echo "  make run-debug    # Run with debug flags + race detector"
	@echo "  make run-release  # Build and run optimized binary"
	@echo "  make test         # Run all tests"
	@echo "  make env-check    # Check if tools are installed"
