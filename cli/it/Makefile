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

.PHONY: test clean deps check-docker build-images help

# Gateway IT compose file location
GATEWAY_COMPOSE_FILE=../../gateway/it/docker-compose.test.yaml
GATEWAY_DIR=../../gateway

# Default target
help:
	@echo "CLI Integration Tests"
	@echo ""
	@echo "Usage:"
	@echo "  make test           - Run integration tests (requires images to be built first)"
	@echo "  make test-all       - Build gateway images and run integration tests"
	@echo "  make build-images   - Build required gateway Docker images"
	@echo "  make clean          - Clean up containers, volumes, and logs"
	@echo "  make deps           - Install Go dependencies"
	@echo ""
	@echo "Prerequisites:"
	@echo "  - Docker must be running"
	@echo "  - Gateway images must be built (run 'make build-images' first)"

# Run integration tests (assumes images are already built)
test: check-docker
	@mkdir -p logs
	@pkgs="$$(go list -f '{{if .TestGoFiles}}{{.ImportPath}}{{end}}' ./... | xargs)" ; \
	if [ -z "$$pkgs" ]; then \
		echo "No packages with tests found under $(PWD)" ; \
	else \
		go test -v -timeout 30m $$pkgs 2>&1 | awk '/^Logs directory:/{print;exit}1' ; \
	fi

# Build images and run tests (full flow)
test-all: build-images test

# Build required gateway Docker images locally
build-images: check-docker
	@echo "==============================================="
	@echo "Building Gateway Docker Images (with coverage)"
	@echo "==============================================="
	@echo "This may take several minutes..."
	@$(MAKE) -C $(GATEWAY_DIR) build-coverage
	@echo "✓ Gateway images built successfully"

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
