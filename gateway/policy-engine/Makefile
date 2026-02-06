# --------------------------------------------------------------------
# Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
# http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

# Makefile for API Platform Policy Engine

# Variables
VERSION ?= $(shell cat ../VERSION 2>/dev/null || echo "0.0.1-SNAPSHOT")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildDate=$(BUILD_DATE)
DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
RUNTIME_IMAGE := $(DOCKER_REGISTRY)/policy-engine
RUNTIME_VERSION := $(VERSION)

# Docker image tags
RUNTIME_TAG := $(RUNTIME_IMAGE):$(RUNTIME_VERSION)

# Go variables
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOMOD := $(GOCMD) mod

# Binary name
BINARY_NAME := policy-engine

.PHONY: all build build-local build-coverage-image test clean docker-clean help push \
	run stop tidy lint logs ps restart rebuild dev-build dev-run check-deps \
	build-sample-setheader test-integration test-envoy clean-all version build-and-push-multiarch

# Default target
all: help

## help: Display this help message
help:
	@echo "API Platform Policy Engine - Build Targets (Version: $(VERSION))"
	@echo ""
	@echo "Build Targets:"
	@echo "  make build                - Build the Policy Engine Docker image using buildx"
	@echo "  make build-local          - Build the Policy Engine Docker image locally (faster)"
	@echo ""
	@echo "Multi-arch Build and Push Targets:"
	@echo "  make build-and-push-multiarch - Build and push image for multiple architectures (amd64, arm64)"
	@echo ""
	@echo "Push Targets:"
	@echo "  make push                 - Push image to registry"
	@echo ""
	@echo "Runtime Targets:"
	@echo "  make run                  - Run the Policy Engine with docker-compose"
	@echo "  make stop                 - Stop the running Policy Engine services"
	@echo "  make restart              - Restart all services"
	@echo "  make rebuild              - Rebuild and restart"
	@echo ""
	@echo "Development Targets:"
	@echo "  make test                 - Run tests"
	@echo "  make clean                - Clean build artifacts"
	@echo "  make docker-clean         - Remove all Docker images and containers"
	@echo ""
	@echo "Other Targets:"
	@echo "  make tidy                 - Run go mod tidy"
	@echo "  make lint                 - Run Go linter (if installed)"
	@echo "  make logs                 - View docker-compose logs"
	@echo "  make ps                   - Show running services"
	@echo ""

## build: Build the Policy Engine Docker image using buildx
build: test
	@echo "Building Policy Engine image: $(RUNTIME_TAG)"
	@mkdir -p target && cp ../build.yaml target/
	docker buildx build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context common=../../common \
		--build-context gateway-builder=../gateway-builder \
		--build-context policy-engine=. \
		--build-context system-policies=../system-policies \
		--build-context dev-policies=../dev-policies \
		--build-context target=target \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(RUNTIME_TAG) \
		-t $(RUNTIME_IMAGE):latest \
		--load \
		.
	@rm -rf target
	@echo "Image built successfully: $(RUNTIME_TAG)"

## build-local: Build the Policy Engine Docker image locally (faster, no buildx)
build-local:
	@echo "Building Policy Engine image locally: $(RUNTIME_TAG)"
	@mkdir -p target && cp ../build.yaml target/
	DOCKER_BUILDKIT=1 docker build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context common=../../common \
		--build-context gateway-builder=../gateway-builder \
		--build-context policy-engine=. \
		--build-context system-policies=../system-policies \
		--build-context dev-policies=../dev-policies \
		--build-context target=target \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(RUNTIME_TAG) \
		-t $(RUNTIME_IMAGE):latest \
		.
	@rm -rf target
	@echo "Image built successfully: $(RUNTIME_TAG)"

## build-coverage-image: Build the Policy Engine Docker image with coverage instrumentation
build-coverage-image: test
	@echo "Building Policy Engine coverage image: $(RUNTIME_IMAGE)-coverage:$(RUNTIME_VERSION)"
	@mkdir -p target && cp ../build.yaml target/
	DOCKER_BUILDKIT=1 docker build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context common=../../common \
		--build-context gateway-builder=../gateway-builder \
		--build-context policy-engine=. \
		--build-context system-policies=../system-policies \
		--build-context dev-policies=../dev-policies \
		--build-context target=target \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg ENABLE_COVERAGE=true \
		-t $(RUNTIME_IMAGE)-coverage:$(RUNTIME_VERSION) \
		.
	@rm -rf target
	@echo "Coverage image built successfully: $(RUNTIME_IMAGE)-coverage:$(RUNTIME_VERSION)"

## push: Push image to registry
push:
	@echo "Pushing $(RUNTIME_TAG)..."
	docker push $(RUNTIME_TAG)
	docker push $(RUNTIME_IMAGE):latest

## build-and-push-multiarch: Build and push the Policy Engine Docker image for multiple architectures (amd64, arm64)
build-and-push-multiarch:
	@echo "Building and pushing multi-arch Policy Engine image: $(RUNTIME_TAG)"
	@mkdir -p target && cp ../build.yaml target/
	docker buildx build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context common=../../common \
		--build-context gateway-builder=../gateway-builder \
		--build-context policy-engine=. \
		--build-context system-policies=../system-policies \
		--build-context dev-policies=../dev-policies \
		--build-context target=target \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(RUNTIME_TAG) \
		-t $(RUNTIME_IMAGE):latest \
		--push \
		.
	@rm -rf target
	@echo "Multi-arch image built and pushed: $(RUNTIME_TAG)"
	
## run: Start the Policy Engine with docker-compose
run:
	@echo "Starting Policy Engine services..."
	docker-compose up -d
	@echo "Services started. Access Envoy at http://localhost:10000"
	@echo "   - Envoy Admin: http://localhost:9901"
	@echo "   - Policy Engine ext_proc: localhost:9001"
	@echo ""
	@echo "View logs with: make logs"

## stop: Stop the Policy Engine services
stop:
	@echo "Stopping Policy Engine services..."
	docker-compose down
	@echo "Services stopped"

## test: Run tests
test:
	@echo "Running tests..."
	$(GOTEST) -v $$(go list ./... | grep -v /testutils) -cover -coverprofile=unit-test-coverage.txt
	@echo "Tests completed"

## clean: Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -rf target
	@echo "Clean completed"

## docker-clean: Remove all Docker images and containers
docker-clean:
	@echo "Removing Docker containers..."
	-docker-compose down -v
	@echo "Removing Docker images..."
	-docker rmi $(RUNTIME_TAG) 2>/dev/null || true
	@echo "Docker cleanup completed"

## tidy: Run go mod tidy
tidy:
	@echo "Running go mod tidy..."
	$(GOMOD) tidy
	cd ../../sdk && $(GOMOD) tidy
	@echo "go mod tidy completed for all modules"

## lint: Run Go linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
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
restart: stop run

## rebuild: Rebuild and restart
rebuild: build restart

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
	cd sample-policies/set-header/v1.0.0 && $(GOMOD) tidy
	@echo "SetHeader policy ready"

# Testing targets

## test-integration: Run integration tests (requires docker-compose)
test-integration: run
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
	@echo "Image: $(RUNTIME_TAG)"
	@echo "Go Version: $(shell go version)"
	@echo "Docker Version: $(shell docker --version)"
	@echo "Docker Compose Version: $(shell docker-compose --version)"
