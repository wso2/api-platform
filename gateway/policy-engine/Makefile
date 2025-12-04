# Makefile for Envoy Policy Engine

# Variables
VERSION ?= $(shell cat ../VERSION 2>/dev/null || echo "0.0.1-SNAPSHOT")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildDate=$(BUILD_DATE)
DOCKER_REGISTRY ?= ghcr.io/renuka-fernando/api-platform
BUILDER_IMAGE := $(DOCKER_REGISTRY)/gateway-builder
RUNTIME_IMAGE := $(DOCKER_REGISTRY)/policy-engine
BUILDER_VERSION := $(VERSION)
RUNTIME_VERSION := $(VERSION)

# Docker image tags
BUILDER_TAG := $(BUILDER_IMAGE):$(BUILDER_VERSION)
RUNTIME_TAG := $(RUNTIME_IMAGE):$(RUNTIME_VERSION)

# Directories
RUNTIME_DIR := policy-engine
CMD_DIR := $(RUNTIME_DIR)/cmd/policy-engine
INTERNAL_DIR := $(RUNTIME_DIR)/internal
BUILDER_DIR := policy-builder
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

.PHONY: all build build-builder build-runtime test clean docker-clean run-builder help push push-builder push-runtime \
	run-runtime stop-runtime tidy lint logs ps restart rebuild dev-build dev-run check-deps \
	build-sample-setheader test-integration test-envoy clean-all version build-and-push-multiarch \
	build-and-push-multiarch-builder build-and-push-multiarch-runtime

# Default target
all: help

## help: Display this help message
help:
	@echo "Envoy Policy Engine - Build Targets (Version: $(VERSION))"
	@echo ""
	@echo "Build Targets:"
	@echo "  make build                - Build all Docker images (builder + runtime)"
	@echo "  make build-builder          - Build the Gateway Builder Docker image"
	@echo "  make build-runtime         - Build the Policy Engine Runtime Docker image"
	@echo "  make run-builder           - Run the Builder to compile policies into runtime binary"
	@echo ""
	@echo "Multi-arch Build and Push Targets:"
	@echo "  make build-and-push-multiarch         - Build and push all images for multiple architectures (amd64, arm64)"
	@echo "  make build-and-push-multiarch-builder - Build and push builder image for multiple architectures"
	@echo "  make build-and-push-multiarch-runtime - Build and push runtime image for multiple architectures"
	@echo ""
	@echo "Push Targets:"
	@echo "  make push                 - Push all images to registry"
	@echo "  make push-builder         - Push builder image"
	@echo "  make push-runtime         - Push runtime image"
	@echo ""
	@echo "Runtime Targets:"
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

## build: Build all Docker images (builder + runtime)
build: build-builder build-runtime

## build-builder: Build the Gateway Builder Docker image
build-builder:
	@echo "Building Gateway Builder image: $(BUILDER_TAG)"
	docker build -f Dockerfile.builder \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(BUILDER_TAG) \
		-t $(BUILDER_IMAGE):latest \
		.
	@echo "Builder image built successfully: $(BUILDER_TAG)"

## run-builder: Run the Builder to compile policies
run-builder:
	@echo "Running Gateway Builder to compile policies..."
	@mkdir -p $(OUTPUT_DIR)
	docker run --rm \
		-v $(PWD)/$(POLICIES_DIR):/policies:ro \
		-v $(PWD)/$(OUTPUT_DIR):/output \
		$(BUILDER_TAG)
	@echo "Policies compiled successfully. Output in $(OUTPUT_DIR)/"

## build-runtime: Build the Policy Engine Runtime Docker image
build-runtime:
	@echo "Building Policy Engine Runtime image: $(RUNTIME_TAG)"
	docker build -f Dockerfile.runtime \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(RUNTIME_TAG) \
		-t $(RUNTIME_IMAGE):latest \
		.
	@echo "Runtime image built successfully: $(RUNTIME_TAG)"

## push: Push all images to registry
push: push-builder push-runtime

## push-builder: Push builder image to registry
push-builder:
	@echo "Pushing $(BUILDER_TAG)..."
	docker push $(BUILDER_TAG)
	docker push $(BUILDER_IMAGE):latest

## push-runtime: Push runtime image to registry
push-runtime:
	@echo "Pushing $(RUNTIME_TAG)..."
	docker push $(RUNTIME_TAG)
	docker push $(RUNTIME_IMAGE):latest

## build-and-push-multiarch: Build and push all Docker images for multiple architectures (amd64, arm64)
build-and-push-multiarch: build-and-push-multiarch-builder build-and-push-multiarch-runtime

## build-and-push-multiarch-builder: Build and push the Gateway Builder Docker image for multiple architectures
build-and-push-multiarch-builder:
	@echo "Building and pushing multi-arch Gateway Builder image: $(BUILDER_TAG)"
	docker buildx build -f Dockerfile.builder \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(BUILDER_TAG) \
		-t $(BUILDER_IMAGE):latest \
		--push \
		.
	@echo "Multi-arch builder image built and pushed: $(BUILDER_TAG)"

## build-and-push-multiarch-runtime: Build and push the Policy Engine Runtime Docker image for multiple architectures
build-and-push-multiarch-runtime:
	@echo "Building and pushing multi-arch Policy Engine Runtime image: $(RUNTIME_TAG)"
	docker buildx build -f Dockerfile.runtime \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(RUNTIME_TAG) \
		-t $(RUNTIME_IMAGE):latest \
		--push \
		.
	@echo "Multi-arch runtime image built and pushed: $(RUNTIME_TAG)"

## run-runtime: Start the Policy Engine with docker-compose
run-runtime:
	@echo "Starting Policy Engine services..."
	docker-compose up -d
	@echo "Services started. Access Envoy at http://localhost:10000"
	@echo "   - Envoy Admin: http://localhost:9901"
	@echo "   - Policy Engine ext_proc: localhost:9001"
	@echo ""
	@echo "View logs with: docker-compose logs -f"

## stop-runtime: Stop the Policy Engine services
stop-runtime:
	@echo "Stopping Policy Engine services..."
	docker-compose down
	@echo "Services stopped"

## test: Run tests
test:
	@echo "Running tests..."
	cd $(RUNTIME_DIR) && $(GOTEST) -v ./...
	@echo "Tests completed"

## clean: Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	cd $(RUNTIME_DIR) && $(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf $(OUTPUT_DIR)
	@echo "Clean completed"

## docker-clean: Remove all Docker images and containers
docker-clean:
	@echo "Removing Docker containers..."
	-docker-compose down -v
	@echo "Removing Docker images..."
	-docker rmi $(BUILDER_TAG) 2>/dev/null || true
	-docker rmi $(RUNTIME_TAG) 2>/dev/null || true
	@echo "Docker cleanup completed"

## full-build: Complete workflow - build builder, compile policies, build runtime
full-build: build-builder run-builder build-runtime
	@echo "Full build completed!"
	@echo "   - Builder image: $(BUILDER_TAG)"
	@echo "   - Runtime image: $(RUNTIME_TAG)"
	@echo "   - Compiled output: $(OUTPUT_DIR)/"
	@echo ""
	@echo "Next steps:"
	@echo "   make run-runtime    - Start the services"

## tidy: Run go mod tidy
tidy:
	@echo "Running go mod tidy..."
	cd $(RUNTIME_DIR) && $(GOMOD) tidy
	cd sdk && $(GOMOD) tidy
	cd $(BUILDER_DIR) && $(GOMOD) tidy
	@echo "go mod tidy completed for all modules"

## lint: Run Go linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		cd $(RUNTIME_DIR) && golangci-lint run ./...; \
		echo "Linting completed"; \
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

## check-deps: Check if required tools are installed
check-deps:
	@echo "Checking dependencies..."
	@command -v docker >/dev/null 2>&1 || { echo "❌ docker is required but not installed"; exit 1; }
	@command -v docker-compose >/dev/null 2>&1 || { echo "❌ docker-compose is required but not installed"; exit 1; }
	@command -v go >/dev/null 2>&1 || { echo "❌ go is required but not installed"; exit 1; }
	@echo "All required dependencies are installed"

# Sample policy builds

## build-sample-setheader: Build SetHeader sample policy
build-sample-setheader:
	@echo "Building SetHeader sample policy..."
	cd policies/set-header/v1.0.0 && $(GOMOD) tidy
	@echo "SetHeader policy ready"

# Testing targets

## test-integration: Run integration tests (requires docker-compose)
test-integration: run-runtime
	@echo "Running integration tests..."
	@sleep 5  # Wait for services to be ready
	@echo "Testing Envoy endpoint..."
	@curl -s -o /dev/null -w "Status: %{http_code}\n" http://localhost:10000/ || true
	@echo "Testing Envoy admin..."
	@curl -s -o /dev/null -w "Status: %{http_code}\n" http://localhost:9901/ || true
	@echo "Integration tests completed"

## test-envoy: Test Envoy configuration
test-envoy:
	@echo "Testing Envoy configuration..."
	docker run --rm -v $(PWD)/configs/envoy.yaml:/etc/envoy/envoy.yaml \
		envoyproxy/envoy:v1.36.2 \
		--mode validate -c /etc/envoy/envoy.yaml
	@echo "Envoy configuration is valid"

# Cleanup targets

## clean-all: Clean everything (build artifacts + Docker)
clean-all: clean docker-clean
	@echo "Complete cleanup finished"

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
