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

# Makefile for Event-Gateway-Controller (gateway-controller core + WebSub/WebBroker support)

VERSION ?= $(shell cat ../VERSION 2>/dev/null || echo "0.0.1-SNAPSHOT")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
IMAGE_NAME := $(DOCKER_REGISTRY)/event-gateway-controller

# Default policy manifests bundled into the image (control-plane default-policy
# metadata, not compiled plugin code — unrelated to gateway/build.yaml).
POLICIES_BUILD_CONTEXT ?= ../default-policies

.PHONY: help generate-server-code test build push build-and-push-multiarch build-coverage-image build-debug clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

generate-server-code: ## Generate WebSub/WebBroker API server code from OpenAPI spec
	@echo "Generating event-gateway API server code from OpenAPI spec..."
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1 --config=oapi-codegen.yaml api/eventgateway-openapi.yaml

test: ## Run unit tests
	@echo "Running tests..."
	@go test -v ./... -cover -coverprofile=unit-test-coverage.txt

build: generate-server-code test ## Build Docker image using buildx
	@echo "Building Docker image ($(IMAGE_NAME):$(VERSION))..."
	@mkdir -p target
	@cp ../../LICENSE target/
	docker buildx build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context httpkit=../../httpkit \
		--build-context gateway-controller=../../gateway/gateway-controller \
		--build-context policies=$(POLICIES_BUILD_CONTEXT) \
		--build-context target=target \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--target production \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		--load \
		.
	@rm -rf target
	@echo "Docker image ($(IMAGE_NAME):$(VERSION)) built successfully."

push: ## Push Docker image to registry
	docker push $(IMAGE_NAME):$(VERSION)
	docker push $(IMAGE_NAME):latest

build-and-push-multiarch: ## Build and push multi-architecture Docker image (linux/amd64, linux/arm64)
	@mkdir -p target
	@cp ../../LICENSE target/
	docker buildx build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context httpkit=../../httpkit \
		--build-context gateway-controller=../../gateway/gateway-controller \
		--build-context policies=$(POLICIES_BUILD_CONTEXT) \
		--build-context target=target \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--target production \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		--push \
		.
	@rm -rf target

build-coverage-image: test ## Build coverage-instrumented event-gateway-controller image
	@mkdir -p target
	@cp ../../LICENSE target/
	docker buildx build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context httpkit=../../httpkit \
		--build-context gateway-controller=../../gateway/gateway-controller \
		--build-context policies=$(POLICIES_BUILD_CONTEXT) \
		--build-context target=target \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg ENABLE_COVERAGE=true \
		--target production \
		-t $(IMAGE_NAME)-coverage:$(VERSION) \
		--load \
		.
	@rm -rf target

build-debug: ## Build debug image for remote debugging with dlv (VS Code attach on port 2345)
	@mkdir -p target
	@cp ../../LICENSE target/
	docker buildx build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context httpkit=../../httpkit \
		--build-context gateway-controller=../../gateway/gateway-controller \
		--build-context policies=$(POLICIES_BUILD_CONTEXT) \
		--build-context target=target \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg ENABLE_DEBUG=true \
		--target debug \
		-t $(IMAGE_NAME)-debug:$(VERSION) \
		-t $(IMAGE_NAME)-debug:latest \
		--load \
		.
	@rm -rf target

clean: ## Remove SQLite database files from data/
	@rm -f data/*.db data/*.db-shm data/*.db-wal
