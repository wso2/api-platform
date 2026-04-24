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

SHELL := /bin/bash

MODULE_NAME := event-gateway
BINARY_NAME := event-gateway
VERSION ?= $(shell cat ../VERSION 2>/dev/null || echo "0.1.0-SNAPSHOT")
DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
IMAGE_NAME := $(DOCKER_REGISTRY)/event-gateway-runtime
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo 'Event Gateway Runtime Build System (Version: $(VERSION))'
	@echo ''
	@echo 'Build Targets:'
	@echo '  make build                      - Build Docker image'
	@echo '  make build-local                - Build event-gateway binary locally'
	@echo '  make build-docker               - Build Docker image'
	@echo '  make build-and-push-multiarch   - Build and push Docker image for multiple architectures'
	@echo ''
	@echo 'Test Targets:'
	@echo '  make test                       - Run all tests'
	@echo ''
	@echo 'Clean Targets:'
	@echo '  make clean                      - Clean all build artifacts'

.PHONY: build
build: build-docker ## Build Docker image

.PHONY: build-local
build-local: ## Build event-gateway binary
	mkdir -p bin
	go build -o bin/$(BINARY_NAME) ./cmd/event-gateway/

.PHONY: build-docker
build-docker: ## Build Docker image
	@echo "Building event-gateway-runtime Docker image ($(VERSION))..."
	docker buildx build \
		--load \
		--platform linux/$(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') \
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context policy-engine=../../gateway/gateway-runtime/policy-engine \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(IMAGE_NAME):$(VERSION) \
		-f Dockerfile .
	@echo "✓ Built $(IMAGE_NAME):$(VERSION)"

.PHONY: build-and-push-multiarch
build-and-push-multiarch: ## Build and push Docker image for multiple architectures
	@echo "Building and pushing multi-arch event-gateway-runtime Docker image ($(VERSION))..."
	docker buildx build \
		--push \
		--platform linux/amd64,linux/arm64 \
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context policy-engine=../../gateway/gateway-runtime/policy-engine \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(IMAGE_NAME):$(VERSION) \
		-f Dockerfile .
	@echo "✓ Built and pushed $(IMAGE_NAME):$(VERSION) for linux/amd64,linux/arm64"

.PHONY: test
test: ## Run all tests
	go test ./...

.PHONY: clean
clean: ## Clean all build artifacts
	rm -rf bin/
