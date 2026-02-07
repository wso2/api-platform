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

# Gateway Builder Makefile

SHELL := /bin/bash

# Version from VERSION file or default
VERSION ?= $(shell cat ../VERSION 2>/dev/null || echo "0.0.1-SNAPSHOT")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Docker configuration
DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
IMAGE_NAME := $(DOCKER_REGISTRY)/gateway-builder
IMAGE_TAG := $(VERSION)
FULL_IMAGE := $(IMAGE_NAME):$(IMAGE_TAG)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo 'Gateway Builder Build System (Version: $(VERSION))'
	@echo ''
	@echo 'Build Targets:'
	@echo '  make build                     - Build Docker image using buildx'
	@echo '  make build-local               - Build Docker image locally (faster)'
	@echo '  make push                      - Push Docker image to registry'
	@echo '  make build-and-push-multiarch  - Build and push multi-arch Docker image'
	@echo ''
	@echo 'Development Targets:'
	@echo '  make test                      - Run tests'
	@echo '  make tidy                      - Run go mod tidy'
	@echo '  make clean                     - Clean build artifacts'
	@echo ''
	@echo 'Version Information:'
	@echo '  Version:    $(VERSION)'
	@echo '  Git Commit: $(GIT_COMMIT)'
	@echo '  Build Date: $(BUILD_DATE)'

# Build Targets
.PHONY: build
build: ## Build Docker image using buildx
	@echo "Building Docker image ($(IMAGE_NAME):$(VERSION))..."
	@docker buildx build -f Dockerfile \
		--build-context policy-engine=../gateway-runtime/policy-engine \
		--build-context system-policies=../system-policies \
		--build-context sdk=../../sdk \
		--build-context common=../../common \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		--load \
		.

.PHONY: build-local
build-local: ## Build Docker image locally (faster, no buildx)
	@echo "Building Docker image locally ($(IMAGE_NAME):$(VERSION))..."
	@DOCKER_BUILDKIT=1 docker build -f Dockerfile \
		--build-context policy-engine=../gateway-runtime/policy-engine \
		--build-context system-policies=../system-policies \
		--build-context sdk=../../sdk \
		--build-context common=../../common \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		.

.PHONY: push
push: ## Push Docker image to registry
	@echo "Pushing Docker images..."
	docker push $(IMAGE_NAME):$(VERSION)
	docker push $(IMAGE_NAME):latest

.PHONY: build-and-push-multiarch
build-and-push-multiarch: ## Build and push multi-architecture Docker image (linux/amd64, linux/arm64)
	@echo "Building and pushing multi-arch Docker image: $(IMAGE_NAME):$(VERSION)"
	docker buildx build -f Dockerfile \
		--build-context policy-engine=../gateway-runtime/policy-engine \
		--build-context system-policies=../system-policies \
		--build-context sdk=../../sdk \
		--build-context common=../../common \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		--push \
		.

# Development Targets
.PHONY: test
test: ## Run tests
	@echo "Running tests..."
	@go test -v $$(go list ./... | grep -v /testutils) -cover -coverprofile=unit-test-coverage.txt

.PHONY: tidy
tidy: ## Run go mod tidy
	@echo "Running go mod tidy..."
	@go mod tidy

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@go clean

.PHONY: version
version: ## Show version information
	@echo "Gateway Builder Version Information"
	@echo "===================================="
	@echo "Version:    $(VERSION)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"
	@echo "Image:      $(FULL_IMAGE)"
