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

.PHONY: test test-postgres test-all test-verbose clean deps check-docker coverage-report build-coverage ensure-test-tags

VERSION ?= $(shell cat ../VERSION 2>/dev/null | tr -d '[:space:]' || echo "0.0.1-SNAPSHOT")
DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
COVERAGE_IMAGES := gateway-controller-coverage gateway-runtime-coverage

COMPOSE_FILE ?= docker-compose.test.yaml

# Refresh :test tags from :VERSION images when available.
# If :VERSION is unavailable but :test already exists, keep using :test.
# Tries coverage image first (from make build-coverage), then falls back to
# non-coverage image (from make build-local) for local development convenience.
ensure-test-tags:
	@for img in $(COVERAGE_IMAGES); do \
		base=$${img%-coverage}; \
		if docker image inspect $(DOCKER_REGISTRY)/$$img:$(VERSION) >/dev/null 2>&1; then \
			echo "Tagging $(DOCKER_REGISTRY)/$$img:$(VERSION) -> $(DOCKER_REGISTRY)/$$img:test"; \
			docker tag $(DOCKER_REGISTRY)/$$img:$(VERSION) $(DOCKER_REGISTRY)/$$img:test; \
		elif docker image inspect $(DOCKER_REGISTRY)/$$base:$(VERSION) >/dev/null 2>&1; then \
			echo "Tagging $(DOCKER_REGISTRY)/$$base:$(VERSION) -> $(DOCKER_REGISTRY)/$$img:test"; \
			docker tag $(DOCKER_REGISTRY)/$$base:$(VERSION) $(DOCKER_REGISTRY)/$$img:test; \
		elif docker image inspect $(DOCKER_REGISTRY)/$$img:test >/dev/null 2>&1; then \
			echo "Using existing $(DOCKER_REGISTRY)/$$img:test (no :$(VERSION) image found)"; \
		else \
			echo "Error: No image found for $$img (:$(VERSION) or :test). Run 'make build-coverage' or 'make build-local' in gateway/ first." && exit 1; \
		fi; \
	done

# Run integration tests (always with coverage)
# Requires: make build-coverage-image (run once before tests)
test: ensure-test-tags
	COMPOSE_FILE=$(COMPOSE_FILE) go test -v -timeout 30m ./...

# Build all coverage-instrumented gateway components
build-coverage:
	$(MAKE) -C .. build-coverage

# Build and run tests in one command
test-all: build-coverage test

# Run integration tests against Postgres-backed controller
test-postgres:
	COMPOSE_FILE=docker-compose.test.postgres.yaml $(MAKE) test

# Run tests with verbose output
test-verbose: ensure-test-tags
	COMPOSE_FILE=$(COMPOSE_FILE) go test -v -count=1 -timeout 30m ./...

# Clean up test artifacts
clean:
	docker rm -f $$(docker ps -aq --filter "name=it-") 2>/dev/null || true
	docker compose -f docker-compose.test.yaml -p gateway-it down -v --remove-orphans 2>/dev/null || true
	docker compose -f docker-compose.test.postgres.yaml -p gateway-it down -v --remove-orphans 2>/dev/null || true
	rm -rf coverage/*.out coverage/*.txt coverage/*.html coverage/merged coverage/gateway-controller reports/*.json

# Install dependencies
deps:
	go mod download
	go mod tidy

# Verify Docker is available
check-docker:
	@docker info > /dev/null 2>&1 || (echo "Docker is not running" && exit 1)
	@echo "Docker is available"

# View coverage report (after running test-coverage)
coverage-report:
	@if [ -f coverage/coverage.html ]; then \
		echo "Opening coverage report..."; \
		xdg-open coverage/coverage.html 2>/dev/null || open coverage/coverage.html 2>/dev/null || echo "Coverage report: coverage/coverage.html"; \
	else \
		echo "No coverage report found. Run 'make test-coverage' first."; \
	fi
