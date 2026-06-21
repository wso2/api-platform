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

# Makefile for Event-Gateway Controller

VERSION ?= $(shell cat ../VERSION 2>/dev/null || echo "0.0.1-SNAPSHOT")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
VERSION_PKG := github.com/wso2/api-platform/gateway/gateway-controller/pkg/version
LDFLAGS := -X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).GitCommit=$(GIT_COMMIT) -X $(VERSION_PKG).BuildDate=$(BUILD_DATE)

DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
IMAGE_NAME := $(DOCKER_REGISTRY)/event-gateway-controller

.PHONY: help build build-local build-docker build-debug build-and-push-multiarch test clean

help: ## Show this help message
	@echo 'Event Gateway Controller Build System (Version: $(VERSION))'
	@echo ''
	@echo 'Build Targets:'
	@echo '  make build                      - Build Docker image'
	@echo '  make build-local                - Build event-gateway-controller binary locally'
	@echo '  make build-docker               - Build Docker image'
	@echo '  make build-debug                - Build Docker image with Delve remote debugger (port 2345)'
	@echo '  make build-and-push-multiarch   - Build and push multi-arch Docker image'
	@echo ''
	@echo 'Test Targets:'
	@echo '  make test                       - Run all tests'
	@echo ''
	@echo 'Clean Targets:'
	@echo '  make clean                      - Clean build artifacts'

.PHONY: build
build: build-docker ## Build Docker image

.PHONY: build-local
build-local: ## Build event-gateway-controller binary locally
	@echo "Building event-gateway-controller binary..."
	@mkdir -p target
	CGO_ENABLED=1 go build \
		-ldflags "$(LDFLAGS)" \
		-o target/event-gateway-controller \
		./cmd/main
	@echo "Binary: target/event-gateway-controller"

.PHONY: build-docker
build-docker: ## Build Docker image using buildx
	@echo "Building Docker image ($(IMAGE_NAME):$(VERSION))..."
	@mkdir -p ../../target
	@cp ../../LICENSE ../../target/
	docker buildx build -f Dockerfile \
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context gateway-controller=../../gateway/gateway-controller \
		--build-context target=../../target \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--target production \
		-t $(IMAGE_NAME):$(VERSION) \
		--load \
		.
	@echo "Image built: $(IMAGE_NAME):$(VERSION)"

.PHONY: build-debug
build-debug: ## Build Docker image with Delve debugger (port 2345)
	@echo "Building debug Docker image ($(IMAGE_NAME):$(VERSION)-debug)..."
	@mkdir -p ../../target
	@cp ../../LICENSE ../../target/
	docker buildx build -f Dockerfile \
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context gateway-controller=../../gateway/gateway-controller \
		--build-context target=../../target \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg ENABLE_DEBUG=true \
		--target debug \
		-t $(IMAGE_NAME):$(VERSION)-debug \
		--load \
		.

.PHONY: build-and-push-multiarch
build-and-push-multiarch: ## Build and push multi-arch Docker image to registry
	@echo "Building and pushing multi-arch image ($(IMAGE_NAME):$(VERSION))..."
	@mkdir -p ../../target
	@cp ../../LICENSE ../../target/
	docker buildx build -f Dockerfile \
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context gateway-controller=../../gateway/gateway-controller \
		--build-context target=../../target \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--target production \
		--platform linux/amd64,linux/arm64 \
		-t $(IMAGE_NAME):$(VERSION) \
		--push \
		.

.PHONY: test
test: ## Run all tests
	@echo "Running tests..."
	@go test -v ./... -cover -coverprofile=unit-test-coverage.txt

.PHONY: clean
clean: ## Clean build artifacts
	@rm -rf target/
	@rm -f unit-test-coverage.txt
	@echo "Cleaned."
