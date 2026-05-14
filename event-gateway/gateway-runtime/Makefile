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
	@echo '  make generate                   - Run event-gateway-builder locally to generate plugin_registry.go (required before build-local)'
	@echo '  make build-local                - Build event-gateway binary locally (runs generate first)'
	@echo '  make build-docker               - Build Docker image'
	@echo '  make build-debug                - Build Docker image with Delve remote debugger (port 2345)'
	@echo '  make debug-local                - Build locally with debug symbols and launch Delve on :2345'
	@echo '  make build-and-push-multiarch   - Build and push Docker image for multiple architectures'
	@echo ''
	@echo 'Test Targets:'
	@echo '  make test                       - Run all tests'
	@echo ''
	@echo 'Clean Targets:'
	@echo '  make clean                      - Clean all build artifacts'

.PHONY: build
build: build-docker ## Build Docker image

.PHONY: generate
generate: ## Run event-gateway-builder locally to generate plugin_registry.go with all policies from build.yaml
	@echo "Generating plugin_registry.go from ../build.yaml..."
	cd ../gateway-builder && go run ./cmd/builder \
		-build-file ../build.yaml \
		-gateway-src ../gateway-runtime \
		-generate-only \
		-log-format text
	@echo "✓ plugin_registry.go generated — policies from build.yaml are now compiled in"

.PHONY: build-local
build-local: generate ## Build event-gateway binary locally with all policies from build.yaml
	mkdir -p bin
	go build -o bin/$(BINARY_NAME) ./cmd/event-gateway/

.PHONY: debug-local
debug-local: generate ## Build locally with debug symbols and launch Delve on :2345 (attach via VS Code or dlv)
	@command -v dlv >/dev/null 2>&1 || { echo "ERROR: dlv not found. Install with: go install github.com/go-delve/delve/cmd/dlv@latest"; exit 1; }
	mkdir -p bin
	go build -gcflags "all=-N -l" -o bin/$(BINARY_NAME)-debug ./cmd/event-gateway/
	@echo "Starting Delve on :2345 — attach your debugger now"
	dlv exec bin/$(BINARY_NAME)-debug \
		--headless \
		--listen=:2345 \
		--accept-multiclient \
		--continue \
		-- -config configs/config.toml

.PHONY: build-docker
build-docker: ## Build Docker image
	@echo "Building event-gateway-runtime Docker image ($(VERSION))..."
	@mkdir -p target
	@cp ../build.yaml target/
	@cp ../../LICENSE target/
	docker buildx build \
		--load \
		--platform linux/$(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') \
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context policy-engine=../../gateway/gateway-runtime/policy-engine \
		--build-context analytics=../../gateway/system-policies/analytics \
		--build-context gateway-builder=../gateway-builder \
		--build-context target=target \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(IMAGE_NAME):$(VERSION) \
		-f Dockerfile .
	@rm -rf target
	@echo "✓ Built $(IMAGE_NAME):$(VERSION)"

.PHONY: build-debug
build-debug: ## Build Docker image with Delve remote debugger (port 2345)
	@echo "Building event-gateway-runtime debug Docker image ($(VERSION))..."
	@mkdir -p target
	@cp ../build.yaml target/
	@cp ../../LICENSE target/
	docker buildx build \
		--load \
		--platform linux/$(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') \
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context policy-engine=../../gateway/gateway-runtime/policy-engine \
		--build-context analytics=../../gateway/system-policies/analytics \
		--build-context gateway-builder=../gateway-builder \
		--build-context target=target \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(IMAGE_NAME)-debug:$(VERSION) \
		-f Dockerfile.debug .
	@rm -rf target
	@echo "✓ Built $(IMAGE_NAME)-debug:$(VERSION) (Delve on :2345)"

.PHONY: build-and-push-multiarch
build-and-push-multiarch: ## Build and push Docker image for multiple architectures
	@echo "Building and pushing multi-arch event-gateway-runtime Docker image ($(VERSION))..."
	@mkdir -p target
	@cp ../build.yaml target/
	@cp ../../LICENSE target/
	docker buildx build \
		--push \
		--platform linux/amd64,linux/arm64 \
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context policy-engine=../../gateway/gateway-runtime/policy-engine \
		--build-context analytics=../../gateway/system-policies/analytics \
		--build-context gateway-builder=../gateway-builder \
		--build-context target=target \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(IMAGE_NAME):$(VERSION) \
		-f Dockerfile .
	@rm -rf target
	@echo "✓ Built and pushed $(IMAGE_NAME):$(VERSION) for linux/amd64,linux/arm64"

.PHONY: test
test: ## Run all tests
	go test ./...

.PHONY: clean
clean: ## Clean all build artifacts
	rm -rf bin/
