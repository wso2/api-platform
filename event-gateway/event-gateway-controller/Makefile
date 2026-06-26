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

VERSION ?= 0.1.0-SNAPSHOT
DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
IMAGE := $(DOCKER_REGISTRY)/event-gateway-controller

.DEFAULT_GOAL := help

.PHONY: help
help:
	@echo 'event-gateway-controller build targets:'
	@echo '  make test                - Run unit tests'
	@echo '  make build               - Build Docker image'
	@echo '  make build-coverage-image - Build coverage-instrumented image'

.PHONY: test
test: ## Run unit tests
	GOWORK=off go test ./...

.PHONY: build
build: ## Build Docker image
	@echo "Building event-gateway-controller Docker image ($(VERSION))..."
	docker buildx build -f Dockerfile \
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context gateway-controller=../../gateway/gateway-controller \
		--build-context build-manifest=.. \
		--build-context policies=../default-policies \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") \
		--target production \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):latest \
		--load \
		.

.PHONY: build-coverage-image
build-coverage-image: test ## Build coverage-instrumented image
	@echo "Building coverage-instrumented event-gateway-controller image ($(VERSION))..."
	docker buildx build -f Dockerfile \
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context gateway-controller=../../gateway/gateway-controller \
		--build-context build-manifest=.. \
		--build-context policies=../default-policies \
		--build-arg VERSION=$(VERSION) \
		--build-arg ENABLE_COVERAGE=true \
		--build-arg GIT_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") \
		--target production \
		-t $(IMAGE):$(VERSION)-coverage \
		--load \
		.

.PHONY: build-and-push-multiarch
build-and-push-multiarch: ## Build and push multi-arch image
	docker buildx build -f Dockerfile \
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context gateway-controller=../../gateway/gateway-controller \
		--build-context build-manifest=.. \
		--build-context policies=../default-policies \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") \
		--target production \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):latest \
		--push \
		.
