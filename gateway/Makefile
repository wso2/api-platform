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

# Gateway Makefile - Coordinate all gateway components

SHELL := /bin/bash

# Read shared version from VERSION file
VERSION := $(shell cat VERSION 2>/dev/null || echo "0.0.1-SNAPSHOT")

# Docker registry configuration
DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform

# Image names
CONTROLLER_IMAGE := $(DOCKER_REGISTRY)/gateway-controller
GATEWAY_BUILDER_IMAGE := $(DOCKER_REGISTRY)/gateway-builder
GATEWAY_RUNTIME_IMAGE := $(DOCKER_REGISTRY)/gateway-runtime

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo 'Gateway Build System (Version: $(VERSION))'
	@echo ''
	@echo 'Build Targets:'
	@echo '  make build                                      - Build all gateway component Docker images (buildx)'
	@echo '  make build-controller                           - Build gateway-controller Docker image'
	@echo '  make build-controller-debug                     - Build debug gateway-controller image for remote debugging (dlv on port 2345)'
	@echo '  make build-gateway-builder                      - Build gateway-builder Docker image'
	@echo '  make build-gateway-runtime                      - Build gateway-runtime Docker image (Router + Policy Engine)'
	@echo '  make build-debug                                - Build all debug images (controller + runtime) for remote debugging'
	@echo '  make build-gateway-runtime-debug                - Build debug gateway-runtime image for remote debugging (dlv on port 2346)'
	@echo ''
	@echo 'Multi-arch Build and Push Targets:'
	@echo '  make build-and-push-multiarch                   - Build and push all gateway components for multiple architectures'
	@echo '  make build-and-push-multiarch-controller        - Build and push controller for multiple architectures'
	@echo '  make build-and-push-multiarch-gateway-builder   - Build and push gateway-builder for multiple architectures'
	@echo '  make build-and-push-multiarch-gateway-runtime   - Build and push gateway-runtime for multiple architectures'
	@echo ''
	@echo 'Test Targets:'
	@echo '  make test                      - Run all tests'
	@echo '  make test-controller           - Test gateway-controller'
	@echo '  make test-gateway-builder      - Test gateway-builder'
	@echo '  make test-policy-engine        - Test policy-engine'
	@echo ''
	@echo 'Integration Test Targets:'
	@echo '  make test-integration-all      - Build coverage images and run integration tests'
	@echo '  make build-coverage            - Build all coverage-instrumented gateway components'
	@echo '  make build-controller-coverage - Build coverage-instrumented gateway-controller Docker image'
	@echo '  make build-gateway-runtime-coverage - Build coverage-instrumented gateway-runtime Docker image'
	@echo '  make test-integration          - Run integration tests'
	@echo ''
	@echo 'Version Management:'
	@echo '  make version-set VERSION=X              - Set gateway version and update artifacts'
	@echo '  make version-bump-patch                 - Bump patch version'
	@echo '  make version-bump-minor                 - Bump minor version'
	@echo '  make version-bump-major                 - Bump major version'
	@echo '  make version-bump-next-dev              - Bump to next minor dev version with SNAPSHOT'
	@echo '  make version-get-release                - Get release version (strips SNAPSHOT suffix)'
	@echo ''
	@echo 'Distribution Targets:'
	@echo '  make dist                      - Build standalone gateway distribution zip (target/wso2apip-api-gateway-<VERSION>.zip)'
	@echo '  make clean-dist                - Remove distribution staging dir and zip'
	@echo ''
	@echo 'Clean Targets:'
	@echo '  make clean                     - Clean all build artifacts'
	@echo '  make clean-db                  - Remove local dev SQLite database files'

# Build Targets
# Note: Controller must be built last since it depends on the build-manifest.yaml
# generated during the gateway-runtime build.
.PHONY: build
build: build-gateway-runtime build-gateway-builder build-controller ## Build all gateway components

# Note: Controller must be built last since it depends on the build-manifest.yaml
# generated during the gateway-runtime build.
.PHONY: build-debug
build-debug: build-gateway-runtime-debug build-controller-debug ## Build all debug images (controller + runtime) for remote debugging

# Note: Controller must be built last since it depends on the build-manifest.yaml
# generated during the gateway-runtime build.
.PHONY: build-coverage
build-coverage: build-gateway-builder build-gateway-runtime-coverage build-controller-coverage ## Build all coverage-instrumented gateway components

.PHONY: build-controller
build-controller: ## Build gateway-controller Docker image
	@echo "Building controller Docker image ($(VERSION))..."
	$(MAKE) -C gateway-controller build VERSION=$(VERSION)


.PHONY: build-controller-coverage
build-controller-coverage: ## Build coverage-instrumented gateway-controller Docker image
	@echo "Building coverage-instrumented controller Docker image ($(VERSION))..."
	$(MAKE) -C gateway-controller build-coverage-image VERSION=$(VERSION)

.PHONY: build-controller-debug
build-controller-debug: ## Build debug gateway-controller image for remote debugging
	@echo "Building debug controller Docker image ($(VERSION))..."
	$(MAKE) -C gateway-controller build-debug VERSION=$(VERSION)

.PHONY: build-gateway-builder
build-gateway-builder: ## Build gateway-builder Docker image
	@echo "Building gateway-builder Docker image ($(VERSION))..."
	$(MAKE) -C gateway-builder build VERSION=$(VERSION)


.PHONY: build-gateway-runtime
build-gateway-runtime: ## Build gateway-runtime Docker image (Router + Policy Engine)
	@echo "Building gateway-runtime Docker image ($(VERSION))..."
	$(MAKE) -C gateway-runtime build VERSION=$(VERSION)


.PHONY: build-gateway-runtime-coverage
build-gateway-runtime-coverage: ## Build coverage-instrumented gateway-runtime Docker image
	@echo "Building coverage-instrumented gateway-runtime Docker image ($(VERSION))..."
	$(MAKE) -C gateway-runtime build-coverage-image VERSION=$(VERSION)

.PHONY: build-gateway-runtime-debug
build-gateway-runtime-debug: ## Build debug gateway-runtime image for remote debugging
	@echo "Building debug gateway-runtime Docker image ($(VERSION))..."
	$(MAKE) -C gateway-runtime build-debug VERSION=$(VERSION)

# Multi-arch Build and Push Targets
# Note: Controller must be built last since it depends on the build-manifest.yaml
# generated during the gateway-runtime build.
.PHONY: build-and-push-multiarch
build-and-push-multiarch: build-and-push-multiarch-gateway-runtime build-and-push-multiarch-gateway-builder build-and-push-multiarch-controller ## Build and push all gateway components for multiple architectures

.PHONY: build-and-push-multiarch-controller
build-and-push-multiarch-controller: ## Build and push gateway-controller Docker image for multiple architectures
	@echo "Building and pushing multi-arch controller Docker image ($(VERSION))..."
	$(MAKE) -C gateway-controller build-and-push-multiarch VERSION=$(VERSION)

.PHONY: build-and-push-multiarch-gateway-builder
build-and-push-multiarch-gateway-builder: ## Build and push gateway-builder Docker image for multiple architectures
	@echo "Building and pushing multi-arch gateway-builder Docker image ($(VERSION))..."
	$(MAKE) -C gateway-builder build-and-push-multiarch VERSION=$(VERSION)

.PHONY: build-and-push-multiarch-gateway-runtime
build-and-push-multiarch-gateway-runtime: ## Build and push gateway-runtime Docker image for multiple architectures
	@echo "Building and pushing multi-arch gateway-runtime Docker image ($(VERSION))..."
	$(MAKE) -C gateway-runtime build-and-push-multiarch VERSION=$(VERSION)

# Test Targets
.PHONY: test
test: test-controller test-gateway-builder test-policy-engine ## Run all tests

.PHONY: test-controller
test-controller: ## Test gateway-controller
	$(MAKE) -C gateway-controller test

.PHONY: test-gateway-builder
test-gateway-builder: ## Test gateway-builder
	$(MAKE) -C gateway-builder test

.PHONY: test-policy-engine
test-policy-engine: ## Test policy-engine
	$(MAKE) -C gateway-runtime test

.PHONY: test-integration
test-integration: ## Run integration tests
	$(MAKE) -C it test

.PHONY: test-integration-all
test-integration-all: build-coverage test-integration ## Build coverage images and run integration tests

# Version Management Targets
.PHONY: version-set
version-set: ## Set gateway version and update artifacts
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION required"; \
		echo "Usage: make version-set VERSION=1.0.0"; \
		exit 1; \
	fi
	@echo "$(VERSION)" > VERSION
	@sed -i.bak 's/^  version: .*/  version: $(VERSION)/' build.yaml && rm -f build.yaml.bak
	@sed -i.bak 's/^  version: .*/  version: $(VERSION)/' sample-policies/build.yaml && rm -f sample-policies/build.yaml.bak
	@(cd .. && bash scripts/update-docker-compose.sh gateway $(VERSION))
	@(cd .. && bash scripts/update-helm.sh gateway $(VERSION))
	@echo "✓ Set gateway version to $(VERSION)"

.PHONY: version-bump-patch
version-bump-patch: ## Bump patch version
	@$(MAKE) version-set VERSION=$$(cd .. && bash scripts/next-version.sh patch gateway)

.PHONY: version-bump-minor
version-bump-minor: ## Bump minor version
	@$(MAKE) version-set VERSION=$$(cd .. && bash scripts/next-version.sh minor gateway)

.PHONY: version-bump-major
version-bump-major: ## Bump major version
	@$(MAKE) version-set VERSION=$$(cd .. && bash scripts/next-version.sh major gateway)

.PHONY: version-bump-next-dev
version-bump-next-dev: ## Bump to next minor dev version with SNAPSHOT suffix
	@$(MAKE) version-set VERSION=$$(cd .. && bash scripts/next-version.sh next-dev gateway)

.PHONY: version-get-release
version-get-release: ## Get release version (strips SNAPSHOT suffix)
	@VERSION=$$(cat VERSION 2>/dev/null | tr -d '[:space:]' | sed 's/-SNAPSHOT//'); \
	if [ -z "$$VERSION" ]; then \
		echo "Error: VERSION is empty or contains only whitespace. Check gateway/VERSION file." >&2; \
		exit 1; \
	fi; \
	echo "$$VERSION"

# Distribution Targets
DIST_VERSION := $(shell echo "$(VERSION)" | sed 's/-SNAPSHOT//')
DIST_NAME    := wso2apip-api-gateway-$(DIST_VERSION)
DIST_DIR     := target/$(DIST_NAME)
DIST_ZIP     := target/$(DIST_NAME).zip
AI_DIST_NAME := wso2apip-ai-gateway-$(DIST_VERSION)
AI_DIST_DIR  := target/$(AI_DIST_NAME)
AI_DIST_ZIP  := target/$(AI_DIST_NAME).zip

.PHONY: dist
dist: clean-dist ## Build standalone gateway distribution zip
	@echo "Building distribution $(DIST_NAME)..."
	@mkdir -p $(DIST_DIR)/configs $(DIST_DIR)/resources/certificates \
	          $(DIST_DIR)/resources/listener-certs $(DIST_DIR)/resources/secure-backend
	@cp build.yaml build-manifest.yaml $(DIST_DIR)/
	@cp -R configs/. $(DIST_DIR)/configs/
	@cp -R observability $(DIST_DIR)/
	@cp gateway-controller/certificates/default-listener.crt   $(DIST_DIR)/resources/certificates/
	@cp gateway-controller/listener-certs/default-listener.crt $(DIST_DIR)/resources/listener-certs/
	@cp gateway-controller/listener-certs/default-listener.key $(DIST_DIR)/resources/listener-certs/
	@cp -R resources/secure-backend/. $(DIST_DIR)/resources/secure-backend/
	@cp distribution/docker-compose.yaml $(DIST_DIR)/docker-compose.yaml
	@sed -i.bak \
	    -e 's|/gateway-controller:[^[:space:]]*|/gateway-controller:$(DIST_VERSION)|g' \
	    -e 's|/gateway-runtime:[^[:space:]]*|/gateway-runtime:$(DIST_VERSION)|g' \
	    $(DIST_DIR)/docker-compose.yaml
	@rm -f $(DIST_DIR)/docker-compose.yaml.bak
	@sed -i.bak 's/^  version: .*/  version: $(DIST_VERSION)/' $(DIST_DIR)/build.yaml
	@rm -f $(DIST_DIR)/build.yaml.bak
	@cd target && zip -rq $(DIST_NAME).zip $(DIST_NAME)
	@mv $(DIST_DIR) $(AI_DIST_DIR)
	@cd target && zip -rq $(AI_DIST_NAME).zip $(AI_DIST_NAME)
	@rm -rf $(AI_DIST_DIR)
	@echo "✓ Built $(DIST_ZIP)"
	@echo "✓ Built $(AI_DIST_ZIP)"

.PHONY: clean-dist
clean-dist: ## Remove distribution staging dir and zip
	@rm -rf target/wso2apip-api-gateway-* target/wso2apip-ai-gateway-*

# Clean Targets
.PHONY: clean-db
clean-db: ## Remove local dev SQLite database files
	rm -f gateway-controller/data/gateway.db gateway-controller/data/gateway.db-shm gateway-controller/data/gateway.db-wal
	@echo "Cleaned local dev database files"

.PHONY: clean
clean: clean-dist ## Clean all build artifacts
	$(MAKE) -C gateway-controller clean
	$(MAKE) -C gateway-builder clean
	$(MAKE) -C gateway-runtime clean
