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

# Makefile for Platform API

# Version management
VERSION ?= $(shell cat VERSION 2>/dev/null || echo "0.0.1-SNAPSHOT")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Docker image configuration
DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
IMAGE_NAME := $(DOCKER_REGISTRY)/platform-api
PORT ?= 9243

.PHONY: help build test push build-and-push-multiarch

help: ## Show this help message
	@echo 'Platform API Build System (Version: $(VERSION))'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-25s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

test: ## Run tests
	@echo "Running tests..."
	@cd src && go test -v ./...

build: ## Build Docker image
	@echo "Building Docker image ($(IMAGE_NAME):$(VERSION))..."
	docker buildx build -f Dockerfile \
		--build-arg VERSION=$(VERSION) \
		--build-arg PORT=$(PORT) \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		--load \
		.

push: ## Push Docker image to registry
	@echo "Pushing Docker images..."
	docker push $(IMAGE_NAME):$(VERSION)
	docker push $(IMAGE_NAME):latest

build-and-push-multiarch: ## Build and push multi-architecture Docker image (linux/amd64, linux/arm64)
	@echo "Building and pushing multi-arch Docker image: $(IMAGE_NAME):$(VERSION)"
	docker buildx build -f Dockerfile \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg PORT=$(PORT) \
		-t $(IMAGE_NAME):$(VERSION) \
		--push \
		.

# Version Management Targets
version-set: ## Set platform-api version and update artifacts
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION required"; \
		echo "Usage: make version-set VERSION=1.0.0"; \
		exit 1; \
	fi
	@echo "$(VERSION)" > VERSION
	@bash ../scripts/update-docker-compose.sh platform-api $(VERSION)
	@bash ../scripts/update-helm.sh platform-api $(VERSION)
	@echo "✓ Set platform-api version to $(VERSION)"

version-bump-patch: ## Bump patch version
	@bash ../scripts/version-bump.sh patch platform-api

version-bump-minor: ## Bump minor version
	@bash ../scripts/version-bump.sh minor platform-api

version-bump-major: ## Bump major version
	@bash ../scripts/version-bump.sh major platform-api

version-bump-next-dev: ## Bump to next minor dev version with SNAPSHOT suffix
	@bash ../scripts/version-bump.sh next-dev platform-api

version-get-release: ## Get release version (strips SNAPSHOT suffix)
	@VERSION=$$(cat VERSION 2>/dev/null | tr -d '[:space:]' | sed 's/-SNAPSHOT//'); \
	if [ -z "$$VERSION" ]; then \
		echo "Error: VERSION is empty or contains only whitespace. Check platform-api/VERSION file." >&2; \
		exit 1; \
	fi; \
	echo "$$VERSION"
