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

# Makefile for Gateway Runtime (Router + Policy Engine)

# Version management
VERSION ?= $(shell cat ../VERSION 2>/dev/null || echo "0.0.1-SNAPSHOT")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Docker image configuration
DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
IMAGE_NAME := $(DOCKER_REGISTRY)/gateway-runtime

.PHONY: help build build-coverage-image build-debug push clean build-and-push-multiarch test

help: ## Show this help message
	@echo 'Gateway Runtime Build (Version: $(VERSION))'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-25s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

build: ## Build Gateway Runtime Docker image using buildx
	@echo "Building Gateway Runtime Docker image ($(IMAGE_NAME):$(VERSION))..."
	@mkdir -p target && cp ../build.yaml target/
	docker buildx build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context common=../../common \
		--build-context gateway-builder=../gateway-builder \
		--build-context system-policies=../system-policies \
		--build-context dev-policies=../dev-policies \
		--build-context target=target \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--target production \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		--load \
		.
	@rm -rf target
	@docker run --rm --entrypoint cat $(IMAGE_NAME):$(VERSION) /app/build-lock.yaml > ../build-lock.yaml
	@echo "  ✓ Extracted build-lock.yaml"
	@echo "Docker image built successfully: $(IMAGE_NAME):$(VERSION)"


build-debug: ## Build debug Gateway Runtime image for remote debugging with dlv (VS Code attach on port 2346)
	@echo "Building Gateway Runtime debug image ($(IMAGE_NAME)-debug:$(VERSION))..."
	@mkdir -p target && cp ../build.yaml target/
	docker buildx build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context common=../../common \
		--build-context gateway-builder=../gateway-builder \
		--build-context system-policies=../system-policies \
		--build-context dev-policies=../dev-policies \
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
	@docker run --rm --entrypoint cat $(IMAGE_NAME)-debug:$(VERSION) /app/build-lock.yaml > ../build-lock.yaml
	@echo "  ✓ Extracted build-lock.yaml"
	@echo "Debug image built successfully: $(IMAGE_NAME)-debug:$(VERSION)"

build-coverage-image: test ## Build Gateway Runtime Docker image with coverage instrumentation
	@echo "Building Gateway Runtime coverage image ($(IMAGE_NAME)-coverage:$(VERSION))..."
	@mkdir -p target && cp ../build.yaml target/
	DOCKER_BUILDKIT=1 docker build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context common=../../common \
		--build-context gateway-builder=../gateway-builder \
		--build-context system-policies=../system-policies \
		--build-context dev-policies=../dev-policies \
		--build-context target=target \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg ENABLE_COVERAGE=true \
		--target production \
		-t $(IMAGE_NAME)-coverage:$(VERSION) \
		.
	@rm -rf target
	@echo "Coverage image built successfully: $(IMAGE_NAME)-coverage:$(VERSION)"

push: ## Push Docker image to registry
	@echo "Pushing Docker images..."
	docker push $(IMAGE_NAME):$(VERSION)
	docker push $(IMAGE_NAME):latest

build-and-push-multiarch: ## Build and push Gateway Runtime Docker image for multiple architectures (amd64, arm64)
	@echo "Building and pushing multi-arch Gateway Runtime Docker image ($(VERSION))..."
	@mkdir -p target && cp ../build.yaml target/
	docker buildx build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context common=../../common \
		--build-context gateway-builder=../gateway-builder \
		--build-context system-policies=../system-policies \
		--build-context dev-policies=../dev-policies \
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
	@docker pull --platform linux/amd64 $(IMAGE_NAME):$(VERSION)
	@docker run --rm --entrypoint cat $(IMAGE_NAME):$(VERSION) /app/build-lock.yaml > ../build-lock.yaml
	@echo "  ✓ Extracted build-lock.yaml"
	@echo "Multi-arch Docker image built and pushed: $(IMAGE_NAME):$(VERSION)"

test: ## Run policy-engine tests
	cd policy-engine && go test -v $$(go list ./... | grep -v /testutils) -cover -coverprofile=unit-test-coverage.txt

clean: ## Clean Docker images
	@echo "Cleaning Docker images..."
	@docker rmi $(IMAGE_NAME):$(VERSION) 2>/dev/null || true
	@docker rmi $(IMAGE_NAME):latest 2>/dev/null || true
	@docker rmi $(IMAGE_NAME)-coverage:$(VERSION) 2>/dev/null || true
	@rm -rf target
	@echo "Clean completed"
