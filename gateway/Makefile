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
UNIFIED_GATEWAY_IMAGE := $(DOCKER_REGISTRY)/unified-gateway

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo 'Gateway Build System (Version: $(VERSION))'
	@echo ''
	@echo 'Build Targets:'
	@echo '  make build                                      - Build all gateway component Docker images (buildx)'
	@echo '  make build-local                                - Build all gateway component Docker images locally (faster)'
	@echo '  make build-controller                           - Build gateway-controller Docker image'
	@echo '  make build-gateway-builder                      - Build gateway-builder Docker image'
	@echo '  make build-unified-gateway                      - Build unified-gateway Docker image (Router + Policy Engine)'
	@echo '  make build-local-unified-gateway                - Build unified-gateway Docker image locally (faster)'
	@echo ''
	@echo 'Multi-arch Build and Push Targets:'
	@echo '  make build-and-push-multiarch                   - Build and push all gateway components for multiple architectures'
	@echo '  make build-and-push-multiarch-controller        - Build and push controller for multiple architectures'
	@echo '  make build-and-push-multiarch-gateway-builder   - Build and push gateway-builder for multiple architectures'
	@echo '  make build-and-push-multiarch-unified-gateway   - Build and push unified-gateway for multiple architectures'
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
	@echo '  make build-unified-gateway-coverage - Build coverage-instrumented unified-gateway Docker image'
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
	@echo 'Clean Targets:'
	@echo '  make clean                     - Clean all build artifacts'

# Build Targets
.PHONY: build
build: build-controller build-gateway-builder build-unified-gateway ## Build all gateway components

.PHONY: build-local
build-local: build-local-controller build-local-gateway-builder build-local-unified-gateway ## Build all gateway components locally (faster)

.PHONY: build-coverage
build-coverage: build-gateway-builder build-controller-coverage build-unified-gateway-coverage ## Build all coverage-instrumented gateway components

.PHONY: build-controller
build-controller: ## Build gateway-controller Docker image
	@echo "Building controller Docker image ($(VERSION))..."
	$(MAKE) -C gateway-controller build VERSION=$(VERSION)

.PHONY: build-local-controller
build-local-controller: ## Build gateway-controller Docker image locally (faster)
	@echo "Building controller Docker image locally ($(VERSION))..."
	$(MAKE) -C gateway-controller build-local VERSION=$(VERSION)

.PHONY: build-controller-coverage
build-controller-coverage: ## Build coverage-instrumented gateway-controller Docker image
	@echo "Building coverage-instrumented controller Docker image ($(VERSION))..."
	$(MAKE) -C gateway-controller build-coverage-image VERSION=$(VERSION)

.PHONY: build-gateway-builder
build-gateway-builder: ## Build gateway-builder Docker image
	@echo "Building gateway-builder Docker image ($(VERSION))..."
	$(MAKE) -C gateway-builder build VERSION=$(VERSION)

.PHONY: build-local-gateway-builder
build-local-gateway-builder: ## Build gateway-builder Docker image locally (faster)
	@echo "Building gateway-builder Docker image locally ($(VERSION))..."
	$(MAKE) -C gateway-builder build-local VERSION=$(VERSION)

.PHONY: build-unified-gateway
build-unified-gateway: ## Build unified-gateway Docker image (Router + Policy Engine)
	@echo "Building unified-gateway Docker image ($(VERSION))..."
	$(MAKE) -C unified-gateway build VERSION=$(VERSION)

.PHONY: build-local-unified-gateway
build-local-unified-gateway: ## Build unified-gateway Docker image locally (faster)
	@echo "Building unified-gateway Docker image locally ($(VERSION))..."
	$(MAKE) -C unified-gateway build-local VERSION=$(VERSION)

.PHONY: build-unified-gateway-coverage
build-unified-gateway-coverage: ## Build coverage-instrumented unified-gateway Docker image
	@echo "Building coverage-instrumented unified-gateway Docker image ($(VERSION))..."
	$(MAKE) -C unified-gateway build-coverage-image VERSION=$(VERSION)

# Multi-arch Build and Push Targets
.PHONY: build-and-push-multiarch
build-and-push-multiarch: build-and-push-multiarch-controller build-and-push-multiarch-gateway-builder build-and-push-multiarch-unified-gateway ## Build and push all gateway components for multiple architectures

.PHONY: build-and-push-multiarch-controller
build-and-push-multiarch-controller: ## Build and push gateway-controller Docker image for multiple architectures
	@echo "Building and pushing multi-arch controller Docker image ($(VERSION))..."
	$(MAKE) -C gateway-controller build-and-push-multiarch VERSION=$(VERSION)

.PHONY: build-and-push-multiarch-gateway-builder
build-and-push-multiarch-gateway-builder: ## Build and push gateway-builder Docker image for multiple architectures
	@echo "Building and pushing multi-arch gateway-builder Docker image ($(VERSION))..."
	$(MAKE) -C gateway-builder build-and-push-multiarch VERSION=$(VERSION)

.PHONY: build-and-push-multiarch-unified-gateway
build-and-push-multiarch-unified-gateway: ## Build and push unified-gateway Docker image for multiple architectures
	@echo "Building and pushing multi-arch unified-gateway Docker image ($(VERSION))..."
	$(MAKE) -C unified-gateway build-and-push-multiarch VERSION=$(VERSION)

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
	$(MAKE) -C policy-engine test

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

# Clean Targets
.PHONY: clean
clean: ## Clean all build artifacts
	$(MAKE) -C gateway-controller clean
	$(MAKE) -C gateway-builder clean
	$(MAKE) -C unified-gateway clean
