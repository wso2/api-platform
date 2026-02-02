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

SHELL := /bin/bash

# Version files
ROOT_VERSION := $(shell cat VERSION)
GATEWAY_VERSION := $(shell cat gateway/VERSION)
PLATFORM_API_VERSION := $(shell cat platform-api/VERSION)
CLI_VERSION := $(shell cat cli/VERSION)

# Docker registry configuration
DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform

# Component names
COMPONENT ?=

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo 'WSO2 API Platform - Build & Release System'
	@echo ''
	@echo 'Version Management:'
	@echo '  make version                          - Show all component versions'
	@echo '  cd <component> && make help           - See component-specific version commands'
	@echo ''
	@echo 'Build Targets:'
	@echo '  make build-gateway                    - Build all gateway Docker images'
	@echo '  make build-and-push-gateway-multiarch - Build and push all gateway images for multiple architectures'
	@echo '  make build-and-push-platform-api-multiarch VERSION=X - Build and push platform-api images for multiple architectures'
	@echo '  make build-cli                        - Build CLI binaries for all platforms'
	@echo '  make test-gateway                     - Run gateway tests'
	@echo '  make test-platform-api                - Run platform-api tests'
	@echo '  make test-cli                         - Run CLI tests'
	@echo ''
	@echo 'Push Targets:'
	@echo '  make push-gateway                     - Push gateway images to registry'
	@echo ''
	@echo 'Utility Targets:'
	@echo '  make update-images COMPONENT=X VERSION=Y - Update docker-compose and Helm images'
	@echo '  make validate-versions                - Validate version consistency'
	@echo '  make clean-gateway                    - Clean gateway build artifacts'

# Version Management Targets
.PHONY: version
version: ## Display current versions
	@echo "Platform Version:     $(ROOT_VERSION)"
	@echo "Gateway Version:      $(GATEWAY_VERSION)"
	@echo "Platform API Version: $(PLATFORM_API_VERSION)"
	@echo "CLI Version:          $(CLI_VERSION)"


# Build Targets
.PHONY: build-gateway
build-gateway: ## Build all gateway Docker images
	@echo "Building gateway components ($(GATEWAY_VERSION))..."
	$(MAKE) -C gateway build
	@echo "Successfully built all gateway components"

.PHONY: build-and-push-gateway-multiarch
build-and-push-gateway-multiarch: ## Build and push all gateway Docker images for multiple architectures (amd64, arm64)
	@echo "Building and pushing multi-arch gateway components ($(GATEWAY_VERSION))..."
	$(MAKE) -C gateway build-and-push-multiarch
	@echo "Successfully built and pushed all multi-arch gateway components"

.PHONY: build-and-push-platform-api-multiarch
build-and-push-platform-api-multiarch: ## Build and push platform-api Docker image for multiple architectures (amd64, arm64)
	@echo "Building and pushing multi-arch platform-api ($(PLATFORM_API_VERSION))..."
	$(MAKE) -C platform-api build-and-push-multiarch VERSION=$(PLATFORM_API_VERSION)
	@echo "Successfully built and pushed multi-arch platform-api"

# Test Targets
.PHONY: test-gateway
test-gateway: ## Run gateway tests
	@echo "Running gateway tests..."
	$(MAKE) -C gateway test

.PHONY: test-platform-api
test-platform-api: ## Run platform-api tests
	@echo "Running platform-api tests..."
	$(MAKE) -C platform-api test

.PHONY: build-cli
build-cli: ## Build CLI binaries for all platforms
	@echo "Building CLI ($(CLI_VERSION))..."
	$(MAKE) -C cli/src build-all
	@echo "Successfully built CLI binaries"

.PHONY: test-cli
test-cli: ## Run CLI tests
	@echo "Running CLI tests..."
	$(MAKE) -C cli/src test

# Push Targets
.PHONY: push-gateway
push-gateway: ## Push gateway images to registry
	@echo "Pushing gateway images to registry..."
	$(MAKE) -C gateway push

# Update Targets
.PHONY: update-images
update-images: ## Update docker-compose and Helm chart images
	@if [ -z "$(COMPONENT)" ] || [ -z "$(VERSION)" ]; then \
		echo "Error: COMPONENT and VERSION required"; \
		echo "Usage: make update-images COMPONENT=gateway VERSION=1.0.0"; \
		exit 1; \
	fi
	@echo "Updating docker-compose and Helm charts..."
	@bash scripts/update-docker-compose.sh $(COMPONENT) $(VERSION)
	@bash scripts/update-helm.sh $(COMPONENT) $(VERSION)
	@echo "Updated all image references to version $(VERSION)"

.PHONY: update-versions
update-versions: ## Update docker-compose and Helm charts (alias for update-images)
	@$(MAKE) update-images COMPONENT=$(COMPONENT) VERSION=$(VERSION)

# Validation Targets
.PHONY: validate-versions
validate-versions: ## Validate version consistency
	@bash scripts/validate-versions.sh

# Clean Targets
.PHONY: clean-gateway
clean-gateway: ## Clean gateway build artifacts
	$(MAKE) -C gateway clean
