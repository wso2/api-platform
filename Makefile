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
ROOT_VERSION := $(shell cat VERSION 2>/dev/null || echo "0.0.1-SNAPSHOT")
GATEWAY_VERSION := $(shell cat gateway/VERSION 2>/dev/null || echo "0.0.1-SNAPSHOT")
PLATFORM_API_VERSION := $(shell cat platform-api/VERSION 2>/dev/null || echo "0.0.1-SNAPSHOT")

# Docker registry configuration
DOCKER_REGISTRY ?= ghcr.io/renuka-fernando/api-platform

# Component names
COMPONENT ?=
VERSION_ARG ?=

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo 'WSO2 API Platform - Build & Release System'
	@echo ''
	@echo 'Version Management:'
	@echo '  make version                          - Show all component versions'
	@echo '  make version-set COMPONENT=X VERSION=Y - Set component version'
	@echo '  make version-bump-patch COMPONENT=X   - Bump patch version'
	@echo '  make version-bump-minor COMPONENT=X   - Bump minor version'
	@echo '  make version-bump-major COMPONENT=X   - Bump major version'
	@echo '  make version-bump-next-dev COMPONENT=X - Bump to next minor dev version with SNAPSHOT'
	@echo ''
	@echo 'Build Targets:'
	@echo '  make build-gateway                    - Build all gateway Docker images'
	@echo '  make build-and-push-gateway-multiarch - Build and push all gateway images for multiple architectures'
	@echo '  make test-gateway                     - Run gateway tests'
	@echo ''
	@echo 'Push Targets:'
	@echo '  make push-gateway                     - Push gateway images to registry'
	@echo ''
	@echo 'Release Targets:'
	@echo '  make release-gateway VERSION=X        - Release gateway components'
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

.PHONY: version-set
version-set: ## Set specific version for a component
	@if [ -z "$(COMPONENT)" ] || [ -z "$(VERSION_ARG)" ]; then \
		echo "Error: COMPONENT and VERSION_ARG required"; \
		echo "Usage: make version-set COMPONENT=gateway VERSION_ARG=1.2.0"; \
		exit 1; \
	fi
	@if [ "$(COMPONENT)" = "root" ]; then \
		echo "$(VERSION_ARG)" > VERSION; \
		echo " Set root version to $(VERSION_ARG)"; \
	elif [ "$(COMPONENT)" = "gateway" ]; then \
		echo "$(VERSION_ARG)" > gateway/VERSION; \
		echo " Set gateway version to $(VERSION_ARG)"; \
	elif [ "$(COMPONENT)" = "platform-api" ]; then \
		echo "$(VERSION_ARG)" > platform-api/VERSION; \
		echo " Set platform-api version to $(VERSION_ARG)"; \
	else \
		echo "Error: Unknown component: $(COMPONENT)"; \
		echo "Valid components: root, gateway, platform-api"; \
		exit 1; \
	fi

.PHONY: version-bump-patch
version-bump-patch: ## Bump patch version
	@if [ -z "$(COMPONENT)" ]; then \
		echo "Error: COMPONENT required"; \
		echo "Usage: make version-bump-patch COMPONENT=gateway"; \
		exit 1; \
	fi
	@bash scripts/version-bump.sh patch $(COMPONENT)

.PHONY: version-bump-minor
version-bump-minor: ## Bump minor version
	@if [ -z "$(COMPONENT)" ]; then \
		echo "Error: COMPONENT required"; \
		echo "Usage: make version-bump-minor COMPONENT=gateway"; \
		exit 1; \
	fi
	@bash scripts/version-bump.sh minor $(COMPONENT)

.PHONY: version-bump-major
version-bump-major: ## Bump major version
	@if [ -z "$(COMPONENT)" ]; then \
		echo "Error: COMPONENT required"; \
		echo "Usage: make version-bump-major COMPONENT=gateway"; \
		exit 1; \
	fi
	@bash scripts/version-bump.sh major $(COMPONENT)

.PHONY: version-bump-next-dev
version-bump-next-dev: ## Bump to next minor dev version with SNAPSHOT suffix
	@if [ -z "$(COMPONENT)" ]; then \
		echo "Error: COMPONENT required"; \
		echo "Usage: make version-bump-next-dev COMPONENT=gateway"; \
		exit 1; \
	fi
	@bash scripts/version-bump.sh next-dev $(COMPONENT)

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

# Test Targets
.PHONY: test-gateway
test-gateway: ## Run gateway tests
	@echo "Running gateway tests..."
	$(MAKE) -C gateway test

# Push Targets
.PHONY: push-gateway
push-gateway: ## Push gateway images to registry
	@echo "Pushing gateway images to registry..."
	$(MAKE) -C gateway push

# Release Targets
.PHONY: release-gateway
release-gateway: ## Release gateway components
	@if [ -z "$(VERSION_ARG)" ]; then \
		echo "Error: VERSION_ARG required"; \
		echo "Usage: make release-gateway VERSION_ARG=1.0.0"; \
		exit 1; \
	fi
	@bash scripts/release.sh gateway $(VERSION_ARG)

# Update Targets
.PHONY: update-images
update-images: ## Update docker-compose and Helm chart images
	@if [ -z "$(COMPONENT)" ] || [ -z "$(VERSION_ARG)" ]; then \
		echo "Error: COMPONENT and VERSION_ARG required"; \
		echo "Usage: make update-images COMPONENT=gateway VERSION_ARG=1.0.0"; \
		exit 1; \
	fi
	@echo "Updating docker-compose and Helm charts..."
	@bash scripts/update-docker-compose.sh $(COMPONENT) $(VERSION_ARG)
	@bash scripts/update-helm.sh $(COMPONENT) $(VERSION_ARG)
	@echo "✅ Updated all image references to version $(VERSION_ARG)"

.PHONY: update-versions
update-versions: ## Update docker-compose and Helm charts (alias for update-images)
	@$(MAKE) update-images COMPONENT=$(COMPONENT) VERSION_ARG=$(VERSION_ARG)

# Validation Targets
.PHONY: validate-versions
validate-versions: ## Validate version consistency
	@bash scripts/validate-versions.sh

# Clean Targets
.PHONY: clean-gateway
clean-gateway: ## Clean gateway build artifacts
	$(MAKE) -C gateway clean
