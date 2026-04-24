# --------------------------------------------------------------------
# Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

# Event Gateway Makefile - Coordinate all event gateway components

SHELL := /bin/bash

# Read shared version from VERSION file
VERSION := $(shell cat VERSION 2>/dev/null || echo "0.1.0-SNAPSHOT")

# Docker registry configuration
DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform

# Image names
EVENT_GATEWAY_CONTROLLER_IMAGE := $(DOCKER_REGISTRY)/event-gateway-controller
EVENT_GATEWAY_RUNTIME_IMAGE := $(DOCKER_REGISTRY)/event-gateway-runtime
WEBHOOK_LISTENER_IMAGE := $(DOCKER_REGISTRY)/webhook-listener

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo 'Event Gateway Build System (Version: $(VERSION))'
	@echo ''
	@echo 'Build Targets:'
	@echo '  make build                                      - Build all event gateway component Docker images'
	@echo '  make build-event-gateway-controller             - Build event-gateway-controller Docker image'
	@echo '  make build-gateway-runtime                      - Build event-gateway-runtime Docker image'
	@echo '  make build-webhook-listener                     - Build webhook-listener Docker image'
	@echo ''
	@echo 'Multi-arch Build and Push Targets:'
	@echo '  make build-and-push-multiarch                   - Build and push all event gateway components for multiple architectures'
	@echo '  make build-and-push-multiarch-event-gateway-controller - Build and push event-gateway-controller for multiple architectures'
	@echo '  make build-and-push-multiarch-gateway-runtime   - Build and push event-gateway-runtime for multiple architectures'
	@echo '  make build-and-push-multiarch-webhook-listener  - Build and push webhook-listener for multiple architectures'
	@echo ''
	@echo 'Test Targets:'
	@echo '  make test                      - Run all tests'
	@echo '  make test-event-gateway-controller - Test event-gateway-controller'
	@echo '  make test-gateway-runtime      - Test event-gateway-runtime'
	@echo '  make test-webhook-listener     - Test webhook-listener'
	@echo ''
	@echo 'Version Management:'
	@echo '  make version-set VERSION=X              - Set event gateway version and update artifacts'
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
build: build-event-gateway-controller build-gateway-runtime build-webhook-listener ## Build all event gateway components

.PHONY: build-event-gateway-controller
build-event-gateway-controller: ## Build event-gateway-controller Docker image
	@echo "Building event-gateway-controller Docker image ($(VERSION))..."
	@cd ../gateway/gateway-controller && \
		docker buildx build -f Dockerfile \
			--build-context sdk=../../sdk \
			--build-context sdk-core=../../sdk/core \
			--build-context common=../../common \
			--build-context build-manifest=.. \
			--build-arg VERSION=$(VERSION) \
			--build-arg GIT_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") \
			--target production \
			-t $(EVENT_GATEWAY_CONTROLLER_IMAGE):$(VERSION) \
			--load \
			.

.PHONY: build-gateway-runtime
build-gateway-runtime: ## Build event-gateway-runtime Docker image
	@echo "Building event-gateway-runtime Docker image ($(VERSION))..."
	$(MAKE) -C gateway-runtime build VERSION=$(VERSION)

.PHONY: build-webhook-listener
build-webhook-listener: ## Build webhook-listener Docker image
	@echo "Building webhook-listener Docker image ($(VERSION))..."
	$(MAKE) -C webhook-listener build VERSION=$(VERSION)

# Multi-arch Build and Push Targets
.PHONY: build-and-push-multiarch
build-and-push-multiarch: build-and-push-multiarch-event-gateway-controller build-and-push-multiarch-gateway-runtime build-and-push-multiarch-webhook-listener ## Build and push all event gateway components for multiple architectures

.PHONY: build-and-push-multiarch-event-gateway-controller
build-and-push-multiarch-event-gateway-controller: ## Build and push event-gateway-controller Docker image for multiple architectures
	@echo "Building and pushing multi-arch event-gateway-controller Docker image ($(VERSION))..."
	@cd ../gateway/gateway-controller && \
		docker buildx build -f Dockerfile \
			--build-context sdk=../../sdk \
			--build-context sdk-core=../../sdk/core \
			--build-context common=../../common \
			--build-context build-manifest=.. \
			--platform linux/amd64,linux/arm64 \
			--build-arg VERSION=$(VERSION) \
			--build-arg GIT_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") \
			--target production \
			-t $(EVENT_GATEWAY_CONTROLLER_IMAGE):$(VERSION) \
			--push \
			.

.PHONY: build-and-push-multiarch-gateway-runtime
build-and-push-multiarch-gateway-runtime: ## Build and push event-gateway-runtime Docker image for multiple architectures
	@echo "Building and pushing multi-arch event-gateway-runtime Docker image ($(VERSION))..."
	$(MAKE) -C gateway-runtime build-and-push-multiarch VERSION=$(VERSION)

.PHONY: build-and-push-multiarch-webhook-listener
build-and-push-multiarch-webhook-listener: ## Build and push webhook-listener Docker image for multiple architectures
	@echo "Building and pushing multi-arch webhook-listener Docker image ($(VERSION))..."
	$(MAKE) -C webhook-listener build-and-push-multiarch VERSION=$(VERSION)

# Test Targets
.PHONY: test
test: test-event-gateway-controller test-gateway-runtime test-webhook-listener ## Run all tests

.PHONY: test-event-gateway-controller
test-event-gateway-controller: ## Test event-gateway-controller
	$(MAKE) -C ../gateway/gateway-controller test

.PHONY: test-gateway-runtime
test-gateway-runtime: ## Test event-gateway-runtime
	$(MAKE) -C gateway-runtime test

.PHONY: test-webhook-listener
test-webhook-listener: ## Test webhook-listener
	$(MAKE) -C webhook-listener test

# Version Management Targets
.PHONY: version-set
version-set: ## Set event gateway version and update artifacts
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION required"; \
		echo "Usage: make version-set VERSION=1.0.0"; \
		exit 1; \
	fi
	@echo "$(VERSION)" > VERSION
	@(cd .. && bash scripts/update-docker-compose.sh event-gateway $(VERSION))
	@echo "✓ Set event gateway version to $(VERSION)"

.PHONY: version-bump-patch
version-bump-patch: ## Bump patch version
	@$(MAKE) version-set VERSION=$$(cd .. && bash scripts/next-version.sh patch event-gateway)

.PHONY: version-bump-minor
version-bump-minor: ## Bump minor version
	@$(MAKE) version-set VERSION=$$(cd .. && bash scripts/next-version.sh minor event-gateway)

.PHONY: version-bump-major
version-bump-major: ## Bump major version
	@$(MAKE) version-set VERSION=$$(cd .. && bash scripts/next-version.sh major event-gateway)

.PHONY: version-bump-next-dev
version-bump-next-dev: ## Bump to next minor dev version with SNAPSHOT suffix
	@$(MAKE) version-set VERSION=$$(cd .. && bash scripts/next-version.sh next-dev event-gateway)

.PHONY: version-get-release
version-get-release: ## Get release version (strips SNAPSHOT suffix)
	@VERSION=$$(cat VERSION 2>/dev/null | tr -d '[:space:]' | sed 's/-SNAPSHOT//'); \
	if [ -z "$$VERSION" ]; then \
		echo "Error: VERSION is empty or contains only whitespace. Check event-gateway/VERSION file." >&2; \
		exit 1; \
	fi; \
	echo "$$VERSION"

# Clean Targets
.PHONY: clean
clean: ## Clean all build artifacts
	$(MAKE) -C ../gateway/gateway-controller clean
	$(MAKE) -C gateway-runtime clean
	$(MAKE) -C webhook-listener clean
