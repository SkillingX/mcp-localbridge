.PHONY: build build-server build-client run run-server run-client docker-build docker-run test fmt clean help

# Binary names
SERVER_BINARY=mcp-server
CLIENT_BINARY=mcp-client
DOCKER_IMAGE=mcp-localbridge
VERSION?=latest

# Build directories
BUILD_DIR=./bin
SERVER_CMD=./cmd/server
CLIENT_CMD=./cmd/client

help: ## Display this help message
	@echo "Available targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'

build: build-server build-client ## Build both server and client binaries

build-server: ## Build the MCP server binary
	@echo "Building $(SERVER_BINARY)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(SERVER_BINARY) $(SERVER_CMD)/main.go
	@echo "Build complete: $(BUILD_DIR)/$(SERVER_BINARY)"

build-client: ## Build the MCP client binary
	@echo "Building $(CLIENT_BINARY)..."
	@mkdir -p $(BUILD_DIR)
	@go build -o $(BUILD_DIR)/$(CLIENT_BINARY) $(CLIENT_CMD)/main.go
	@echo "Build complete: $(BUILD_DIR)/$(CLIENT_BINARY)"

run: run-server ## Run the server (default)

run-server: ## Run the MCP server locally
	@echo "Running MCP server..."
	@go run $(SERVER_CMD)/main.go

run-client: ## Run the MCP client (usage: make run-client ARGS="-list")
	@echo "Running MCP client..."
	@go run $(CLIENT_CMD)/main.go $(ARGS)

docker-build: ## Build Docker image
	@echo "Building Docker image $(DOCKER_IMAGE):$(VERSION)..."
	@docker build -t $(DOCKER_IMAGE):$(VERSION) .
	@echo "Docker image built: $(DOCKER_IMAGE):$(VERSION)"

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	@docker-compose up -d
	@echo "Container started. Use 'docker-compose logs -f' to view logs"

docker-stop: ## Stop Docker container
	@echo "Stopping Docker container..."
	@docker-compose down

docker-update: docker-stop docker-build docker-run ## Update Docker image and restart container
	@echo "Docker update complete"

test: ## Run tests
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...
	@echo "Tests complete"

test-coverage: test ## Run tests with coverage report
	@echo "Generating coverage report..."
	@go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Code formatted"

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...
	@echo "go vet complete"

lint: ## Run golangci-lint (requires golangci-lint installed)
	@echo "Running golangci-lint..."
	@golangci-lint run ./...
	@echo "Linting complete"

tidy: ## Tidy go modules
	@echo "Tidying go modules..."
	@go mod tidy
	@echo "go mod tidy complete"

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.txt coverage.html
	@go clean
	@echo "Clean complete"

# Client usage examples
client-list: build-client ## List all available tools from server
	@echo "Listing available tools..."
	@$(BUILD_DIR)/$(CLIENT_BINARY) -list

client-test: build-client ## Test client with db_table_list (requires server running)
	@echo "Testing client with db_table_list..."
	@$(BUILD_DIR)/$(CLIENT_BINARY) -tool db_table_list -args '{"database":"mysql_main"}'

deps: ## Download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@echo "Dependencies downloaded"

ci: fmt vet test ## Run CI checks (format, vet, test)
	@echo "CI checks complete"

all: clean deps fmt vet test build ## Run all tasks (clean, deps, format, vet, test, build)
	@echo "All tasks complete"
