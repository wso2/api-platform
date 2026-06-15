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

.PHONY: test test-devportal clean deps check-docker build-cli build-cli-coverage build-images build-devportal-image test-all help

# Directory locations
CLI_SRC_DIR=../src
GATEWAY_COMPOSE_FILE=../../gateway/it/docker-compose.test.yaml
GATEWAY_DIR=../../gateway
DEVPORTAL_DIR=../../portals/developer-portal

# Default target
help:
	@echo "CLI Integration Tests"
	@echo ""
	@echo "Usage:"
	@echo "  make test           - Run integration tests (requires CLI and images pre-built)"
	@echo "  make test-devportal - Build the devportal image and run ONLY the devportal suite"
	@echo "  make test-all       - Build CLI (coverage), gateway + devportal images, and run the full suite"
	@echo "  make build-cli      - Build the CLI binary (without coverage)"
	@echo "  make build-cli-coverage - Build the CLI binary with coverage instrumentation"
	@echo "  make build-images   - Build required gateway Docker images"
	@echo "  make build-devportal-image - Build the developer-portal :test image (for devportal tests)"
	@echo "  make clean          - Clean up containers, volumes, and logs"
	@echo "  make deps           - Install Go dependencies"
	@echo ""
	@echo "Prerequisites:"
	@echo "  - Docker must be running"
	@echo "  - CLI must be built (run 'make build-cli' first)"
	@echo "  - Gateway images must be built (run 'make build-images' first)"
	@echo "  - For devportal tests, the developer-portal :test image must be built"
	@echo "    (run 'make build-devportal-image' first)"

# Run integration tests (assumes CLI and images are already built)
test: check-docker
	@mkdir -p logs
	@pkgs="$$(go list -f '{{if .TestGoFiles}}{{.ImportPath}}{{end}}' ./... | xargs)" ; \
	if [ -z "$$pkgs" ]; then \
		echo "No packages with tests found under $(PWD)" ; \
	else \
		go test -v -timeout 30m $$pkgs 2>&1 | awk '/^Logs directory:/{print;exit}1' ; \
	fi

# Run ONLY the developer-portal suite (no gateway images required). Builds the
# developer-portal :test image first, then scopes the run with IT_SUITES.
test-devportal: check-docker build-cli build-devportal-image
	@mkdir -p logs
	IT_SUITES=devportal go test -v -timeout 30m . 2>&1 | awk '/^Logs directory:/{print;exit}1'

# Build CLI binary
build-cli:
	@echo "==============================================="
	@echo "Building CLI Binary"
	@echo "==============================================="
	@$(MAKE) -C $(CLI_SRC_DIR) build-skip-tests
	@echo "CLI binary built successfully"

# Build CLI (with coverage), gateway images, devportal image, and run the full
# suite (gateway + devportal).
test-all: build-cli-coverage build-images build-devportal-image test

# Build required gateway Docker images locally
build-images: check-docker
	@echo "==============================================="
	@echo "Building Gateway Docker Images (with coverage)"
	@echo "==============================================="
	@echo "This may take several minutes..."
	@$(MAKE) -C $(GATEWAY_DIR) build-coverage
	@echo "✓ Gateway images built successfully"

# Build the developer-portal Docker image and tag it :test for devportal tests.
# Required before running the DEVPORTAL-backed scenarios.
build-devportal-image: check-docker
	@echo "==============================================="
	@echo "Building Developer Portal Docker Image"
	@echo "==============================================="
	@echo "This may take several minutes..."
	@$(MAKE) -C $(DEVPORTAL_DIR) build
	@$(MAKE) -C $(DEVPORTAL_DIR)/it ensure-test-tag
	@echo "✓ Developer portal :test image ready"

# Clean up containers, volumes, and logs
clean:
	@echo "Cleaning up..."
	@docker compose -f $(GATEWAY_COMPOSE_FILE) -p cli-it down -v --remove-orphans 2>/dev/null || true
	@docker rm -f $$(docker ps -aq --filter "name=cli-it-") 2>/dev/null || true
	@rm -rf logs/*.log
	@echo "Clean complete"

# Install dependencies
deps:
	go mod download
	go mod tidy

# Verify Docker is available
check-docker:
	@docker info > /dev/null 2>&1 || (echo "\033[31m✗ Error: Docker is not running. Please start Docker and try again.\033[0m" && exit 1)
	@echo "\033[32m✓ Docker is available\033[0m"

# Build CLI binary with coverage instrumentation
build-cli-coverage:
	@echo "==============================================="
	@echo "Building CLI Binary (with coverage)"
	@echo "==============================================="
	@$(MAKE) -C $(CLI_SRC_DIR) build-coverage
	@echo "Coverage-instrumented CLI binary built successfully"
