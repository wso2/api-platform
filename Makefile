# Makefile for Envoy Policy Engine

# Variables
BUILDER_IMAGE := policy-engine-builder
RUNTIME_IMAGE := policy-engine
BUILDER_VERSION := latest
RUNTIME_VERSION := latest

# Docker image tags
BUILDER_TAG := $(BUILDER_IMAGE):$(BUILDER_VERSION)
RUNTIME_TAG := $(RUNTIME_IMAGE):$(RUNTIME_VERSION)

# Directories
SRC_DIR := src
BUILD_DIR := build
POLICIES_DIR := policies
CONFIGS_DIR := configs
OUTPUT_DIR := output

# Go variables
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod

# Binary name
BINARY_NAME := policy-engine

.PHONY: all build-builder build-runtime build test clean docker-clean run-builder run help

# Default target
all: help

## help: Display this help message
help:
	@echo "Envoy Policy Engine - Build Targets"
	@echo ""
	@echo "Builder Image Targets:"
	@echo "  make build-builder          - Build the Policy Engine Builder Docker image"
	@echo "  make run-builder           - Run the Builder to compile policies into runtime binary"
	@echo ""
	@echo "Runtime Image Targets:"
	@echo "  make build-runtime         - Build the Policy Engine Runtime Docker image"
	@echo "  make run-runtime          - Run the Policy Engine with docker-compose"
	@echo "  make stop-runtime         - Stop the running Policy Engine services"
	@echo ""
	@echo "Development Targets:"
	@echo "  make build                - Build the binary locally (without Docker)"
	@echo "  make test                 - Run tests"
	@echo "  make clean                - Clean build artifacts"
	@echo "  make docker-clean         - Remove all Docker images and containers"
	@echo ""
	@echo "Complete Workflow:"
	@echo "  make full-build           - Build builder, compile policies, and build runtime"
	@echo ""
	@echo "Other Targets:"
	@echo "  make tidy                 - Run go mod tidy"
	@echo "  make lint                 - Run Go linter (if installed)"
	@echo ""

## build-builder: Build the Policy Engine Builder Docker image
build-builder:
	@echo "Building Policy Engine Builder image: $(BUILDER_TAG)"
	docker build -f Dockerfile.builder -t $(BUILDER_TAG) .
	@echo "✅ Builder image built successfully: $(BUILDER_TAG)"

## run-builder: Run the Builder to compile policies
run-builder:
	@echo "Running Policy Engine Builder to compile policies..."
	@mkdir -p $(OUTPUT_DIR)
	docker run --rm \
		-v $(PWD)/$(POLICIES_DIR):/policies:ro \
		-v $(PWD)/$(OUTPUT_DIR):/output \
		$(BUILDER_TAG)
	@echo "✅ Policies compiled successfully. Output in $(OUTPUT_DIR)/"

## build-runtime: Build the Policy Engine Runtime Docker image
build-runtime:
	@echo "Building Policy Engine Runtime image: $(RUNTIME_TAG)"
	docker build -f Dockerfile.runtime -t $(RUNTIME_TAG) .
	@echo "✅ Runtime image built successfully: $(RUNTIME_TAG)"

## run-runtime: Start the Policy Engine with docker-compose
run-runtime:
	@echo "Starting Policy Engine services..."
	docker-compose up -d
	@echo "✅ Services started. Access Envoy at http://localhost:10000"
	@echo "   - Envoy Admin: http://localhost:9901"
	@echo "   - Policy Engine ext_proc: localhost:9001"
	@echo ""
	@echo "View logs with: docker-compose logs -f"

## stop-runtime: Stop the Policy Engine services
stop-runtime:
	@echo "Stopping Policy Engine services..."
	docker-compose down
	@echo "✅ Services stopped"

## build: Build the binary locally (for development)
build:
	@echo "Building $(BINARY_NAME) locally..."
	cd $(SRC_DIR) && CGO_ENABLED=0 $(GOBUILD) -o ../$(BINARY_NAME) -v
	@echo "✅ Binary built: ./$(BINARY_NAME)"

## test: Run tests
test:
	@echo "Running tests..."
	cd $(SRC_DIR) && $(GOTEST) -v ./...
	@echo "✅ Tests completed"

## clean: Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	cd $(SRC_DIR) && $(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf $(OUTPUT_DIR)
	@echo "✅ Clean completed"

## docker-clean: Remove all Docker images and containers
docker-clean:
	@echo "Removing Docker containers..."
	-docker-compose down -v
	@echo "Removing Docker images..."
	-docker rmi $(BUILDER_TAG) 2>/dev/null || true
	-docker rmi $(RUNTIME_TAG) 2>/dev/null || true
	@echo "✅ Docker cleanup completed"

## full-build: Complete workflow - build builder, compile policies, build runtime
full-build: build-builder run-builder build-runtime
	@echo "✅ Full build completed!"
	@echo "   - Builder image: $(BUILDER_TAG)"
	@echo "   - Runtime image: $(RUNTIME_TAG)"
	@echo "   - Compiled output: $(OUTPUT_DIR)/"
	@echo ""
	@echo "Next steps:"
	@echo "   make run-runtime    - Start the services"

## tidy: Run go mod tidy
tidy:
	@echo "Running go mod tidy..."
	cd $(SRC_DIR) && $(GOMOD) tidy
	@echo "✅ go mod tidy completed"

## lint: Run Go linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		cd $(SRC_DIR) && golangci-lint run ./...; \
		echo "✅ Linting completed"; \
	else \
		echo "⚠️  golangci-lint not installed. Install with:"; \
		echo "   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

## logs: View docker-compose logs
logs:
	@echo "Viewing service logs (Ctrl+C to exit)..."
	docker-compose logs -f

## ps: Show running services
ps:
	@echo "Running services:"
	docker-compose ps

## restart: Restart all services
restart: stop-runtime run-runtime

## rebuild: Rebuild and restart runtime
rebuild: build-runtime restart

# Development helpers

## dev-build: Quick development build (local binary only)
dev-build:
	@echo "Quick development build..."
	cd $(SRC_DIR) && $(GOBUILD) -o ../$(BINARY_NAME)
	@echo "✅ Built: ./$(BINARY_NAME)"

## dev-run: Run the binary locally (without Docker)
dev-run: dev-build
	@echo "Running Policy Engine locally..."
	./$(BINARY_NAME) -extproc-port=9001 -config-file=configs/policy-chains.yaml

## check-deps: Check if required tools are installed
check-deps:
	@echo "Checking dependencies..."
	@command -v docker >/dev/null 2>&1 || { echo "❌ docker is required but not installed"; exit 1; }
	@command -v docker-compose >/dev/null 2>&1 || { echo "❌ docker-compose is required but not installed"; exit 1; }
	@command -v go >/dev/null 2>&1 || { echo "❌ go is required but not installed"; exit 1; }
	@echo "✅ All required dependencies are installed"

# Sample policy builds

## build-sample-setheader: Build SetHeader sample policy
build-sample-setheader:
	@echo "Building SetHeader sample policy..."
	cd policies/set-header/v1.0.0 && $(GOMOD) tidy
	@echo "✅ SetHeader policy ready"

# Testing targets

## test-integration: Run integration tests (requires docker-compose)
test-integration: run-runtime
	@echo "Running integration tests..."
	@sleep 5  # Wait for services to be ready
	@echo "Testing Envoy endpoint..."
	@curl -s -o /dev/null -w "Status: %{http_code}\n" http://localhost:10000/ || true
	@echo "Testing Envoy admin..."
	@curl -s -o /dev/null -w "Status: %{http_code}\n" http://localhost:9901/ || true
	@echo "✅ Integration tests completed"

## test-envoy: Test Envoy configuration
test-envoy:
	@echo "Testing Envoy configuration..."
	docker run --rm -v $(PWD)/configs/envoy.yaml:/etc/envoy/envoy.yaml \
		envoyproxy/envoy:v1.36.2 \
		--mode validate -c /etc/envoy/envoy.yaml
	@echo "✅ Envoy configuration is valid"

# Cleanup targets

## clean-all: Clean everything (build artifacts + Docker)
clean-all: clean docker-clean
	@echo "✅ Complete cleanup finished"

# Version and info

## version: Show version information
version:
	@echo "Policy Engine Build Information"
	@echo "================================"
	@echo "Builder Image: $(BUILDER_TAG)"
	@echo "Runtime Image: $(RUNTIME_TAG)"
	@echo "Go Version: $(shell go version)"
	@echo "Docker Version: $(shell docker --version)"
	@echo "Docker Compose Version: $(shell docker-compose --version)"
