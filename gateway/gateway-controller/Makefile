# Makefile for Gateway-Controller

.PHONY: help generate build test docker clean run

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

generate: ## Generate API server code from OpenAPI spec
	@echo "Generating API server code from OpenAPI spec..."
	@go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest
	@oapi-codegen --config=oapi-codegen.yaml api/openapi.yaml

build: ## Build the Gateway-Controller binary
	@echo "Building Gateway-Controller..."
	@go build -o bin/controller cmd/controller/main.go

test: ## Run unit and integration tests
	@echo "Running tests..."
	@go test -v ./... -cover

docker: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t wso2/gateway-controller:latest .

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@go clean

run: ## Run the Gateway-Controller locally
	@echo "Running Gateway-Controller..."
	@go run cmd/controller/main.go
