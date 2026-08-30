# Midgard RO Client - Makefile
# Requires: Go 1.22+, SDL2

.PHONY: all build build-tools run run-debug run-release play config clean test deps check fmt lint help \
	server-up server-down server-reset server-rebuild server-logs server-status server-shell-db

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

build-tools: ## Build CLI tools (grftool, grfbrowser)
	@echo "Building tools..."
	@mkdir -p $(BUILD_DIR)
	go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/grftool ./cmd/grftool
	go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BUILD_DIR)/grfbrowser ./cmd/grfbrowser
	@echo "Built: $(BUILD_DIR)/grftool, $(BUILD_DIR)/grfbrowser"

## Run

run: ## Run the client
	go run $(CMD_DIR)

run-debug: ## Run with debug mode (uses config.yaml + race detector)
	go run $(CMD_DIR) --config config.yaml --debug

run-release: build ## Run optimized release build
	./$(BUILD_DIR)/$(BINARY_NAME)

play: server-up config ## Start server (if needed) and launch client against it
	@echo "Launching client against localhost:6900 ..."
	go run $(CMD_DIR) --config config.yaml

config: ## Create config.yaml from config.example.yaml if missing
	@if [ ! -f config.yaml ]; then \
		echo "Creating config.yaml from config.example.yaml ..."; \
		cp config.example.yaml config.yaml; \
		echo "Edit config.yaml to set your GRF paths."; \
	fi

## Self-hosted rAthena server (RFC #49 / Track A — see docker/rathena/README.md)

server-up: ## Bring up the local rAthena stack (clones rAthena at pin on first run)
	@cd docker/rathena && \
		[ -d build/rathena/.git ] || ./setup.sh && \
		docker compose up -d
	@echo "rAthena listening on localhost:6900 (login), 6121 (char), 5121 (map)"
	@echo "Test accounts: midgard-test / midgard-sword / midgard-mage (password = login)"
	@echo "See docs/TEST_ACCOUNTS.md"

server-down: ## Stop the rAthena stack (preserves DB volume)
	cd docker/rathena && docker compose down

server-reset: ## Stop and wipe the DB volume (next server-up re-seeds)
	cd docker/rathena && docker compose down --volumes

server-rebuild: ## Wipe everything (DB + cloned rAthena), force re-clone and re-compile
	cd docker/rathena && docker compose down --volumes && rm -rf build/

server-logs: ## Tail logs from all rAthena services
	cd docker/rathena && docker compose logs -f --tail=50

server-status: ## Show status of rAthena containers
	@cd docker/rathena && docker compose ps

server-shell-db: ## Open a mariadb shell against the running DB
	docker exec -it midgard-rathena-db mariadb -uragnarok -pragnarok ragnarok

# GM level lives on the login account, not the character, and the seed only
# runs on first DB init — so flipping it in the seed alone would need a
# server-reset and lose the character. These work on a running database. The
# change takes effect at the next login.
MIDGARD_ACCOUNT ?= midgard-test
DB_EXEC = docker exec midgard-rathena-db mariadb -uragnarok -pragnarok ragnarok -e

server-gm: ## Make the test account a GM (group 99 — all 313 commands)
	@$(DB_EXEC) "UPDATE login SET group_id=99 WHERE userid='$(MIDGARD_ACCOUNT)';"
	@echo "$(MIDGARD_ACCOUNT) is group 99 (Admin). Log in again for it to apply."

server-player: ## Put the test account back to an ordinary player (group 0)
	@$(DB_EXEC) "UPDATE login SET group_id=0 WHERE userid='$(MIDGARD_ACCOUNT)';"
	@echo "$(MIDGARD_ACCOUNT) is group 0 (Player). Log in again for it to apply."

server-group: ## Show the test account's current group
	@$(DB_EXEC) "SELECT userid, group_id FROM login WHERE userid='$(MIDGARD_ACCOUNT)';"

# Killing a client leaves char.online set and the account authenticated on the
# login server, so the next run is refused with "Too many connections. Please
# wait." on the login screen — which looks like a client bug and is not one.
# Clearing the flag needs the server restart too: the login server holds its
# own in-memory session list that the database does not reach.
server-kick: ## Clear a stranded login session after a client was killed
	@$(DB_EXEC) "UPDATE \`char\` SET online=0 WHERE account_id IN \
		(SELECT account_id FROM login WHERE userid='$(MIDGARD_ACCOUNT)');"
	@docker restart midgard-rathena-map midgard-rathena-char midgard-rathena-login >/dev/null
	@echo "session cleared; give the map server ~20s to come back up"

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
	@printf "Go:             "; go version 2>/dev/null || echo "NOT INSTALLED"
	@printf "pkg-config:     "; pkg-config --version 2>/dev/null || echo "NOT INSTALLED"
	@printf "SDL2:           "; pkg-config --modversion sdl2 2>/dev/null || echo "NOT INSTALLED"
	@printf "GCC:            "; gcc --version 2>/dev/null | head -1 || echo "NOT INSTALLED"
	@printf "Docker:         "; docker --version 2>/dev/null || echo "NOT INSTALLED"
	@printf "Docker Compose: "; docker compose version 2>/dev/null || echo "NOT INSTALLED"
	@printf "Colima:         "; colima version 2>/dev/null | head -1 || echo "NOT INSTALLED"

env-install-macos: ## Install dependencies on macOS (build + server)
	brew install go pkg-config sdl2 colima docker docker-compose
	@echo
	@echo "If 'docker compose' isn't found, ensure ~/.docker/config.json contains:"
	@echo "  \"cliPluginsExtraDirs\": [\"/opt/homebrew/lib/docker/cli-plugins\"]"
	@echo
	@echo "Next: 'colima start --memory 8 --cpu 4' then 'make deps' then 'make play'"

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
	@echo "  make play         # Start server + run client (one-shot end-to-end)"
	@echo "  make server-up    # Start rAthena Docker stack (login/char/map + DB)"
	@echo "  make server-logs  # Tail server logs"
	@echo "  make server-down  # Stop server"
	@echo "  make run          # Run the client only (server already running)"
	@echo "  make run-debug    # Run with debug flags"
	@echo "  make test         # Run all tests"
	@echo "  make env-check    # Check if tools are installed"
