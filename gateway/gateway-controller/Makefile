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

# Makefile for Gateway-Controller

# Version management
VERSION ?= $(shell cat ../VERSION 2>/dev/null || echo "0.0.1-SNAPSHOT")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildDate=$(BUILD_DATE)

# Docker image configuration
DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
IMAGE_NAME := $(DOCKER_REGISTRY)/gateway-controller

.PHONY: help generate build build-local test generate-listener-certs push

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

generate: ## Generate API server code from OpenAPI spec
	@echo "Generating API server code from OpenAPI spec..."
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1 --config=oapi-codegen.yaml api/openapi.yaml

test: ## Run unit and integration tests
	@echo "Running tests..."
	@go test -v ./... -cover

copy-policy-definitions: ## Copy policy definitions from policy-manifest.yaml
	@# Check if yq is installed
	@if ! command -v yq >/dev/null 2>&1; then \
		echo "yq is not installed. Please install yq before running this target."; \
		echo "  MacOS: brew install yq"; \
		echo "  Linux: sudo wget https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64 -O /usr/local/bin/yq && sudo chmod +x /usr/local/bin/yq"; \
		exit 1; \
	fi
	@echo "Copying policy definitions..."
	@mkdir -p policies
	@yq e '.policies[] | .name + "|" + .version + "|" + .filePath' ../policies/policy-manifest.yaml | \
	while IFS= read -r line; do \
		name=$$(echo "$$line" | cut -d'|' -f1); \
		version=$$(echo "$$line" | cut -d'|' -f2); \
		filepath=$$(echo "$$line" | cut -d'|' -f3); \
		src="../policies/$${filepath#./}/policy-definition.yaml"; \
		dest="default-policies/$${name}-$${version}.yaml"; \
		if [ -f "$$src" ]; then \
			cp "$$src" "$$dest"; \
			echo "  Copied $$src -> $$dest"; \
		else \
			echo "  Warning: $$src not found"; \
		fi; \
	done

build: generate copy-policy-definitions ## Build Docker image using buildx
	@echo "Building Docker image ($(IMAGE_NAME):$(VERSION))..."
	docker buildx build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context common=../../common \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		--load \
		.

build-local: generate copy-policy-definitions ## Build Docker image locally (faster, no buildx)
	@echo "Building Docker image locally ($(IMAGE_NAME):$(VERSION))..."
	DOCKER_BUILDKIT=1 docker build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context common=../../common \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		.

push: ## Push Docker image to registry
	@echo "Pushing Docker images..."
	docker push $(IMAGE_NAME):$(VERSION)
	docker push $(IMAGE_NAME):latest

build-and-push-multiarch: ## Build and push multi-architecture Docker image (linux/amd64, linux/arm64)
	@echo "Building and pushing multi-arch Docker image: $(IMAGE_NAME):$(VERSION)"
	docker buildx build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context common=../../common \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		--push \
		.

generate-listener-certs: ## Generate listener certificates
	@echo "Generating listener certificates..."
	@mkdir -p listener-certs
	@openssl req -x509 \
		-newkey rsa:4096 \
		-keyout listener-certs/default-listener.key \
		-out listener-certs/default-listener.crt \
		-days 36500 \
		-nodes \
		-subj "/C=US/ST=California/L=San Francisco/O=WSO2/OU=API Gateway/CN=localhost" \
		-addext "subjectAltName=DNS:localhost,DNS:*.localhost,IP:127.0.0.1"
	@echo "Certificate and key generated successfully in listener-certs/"
	@echo "  Certificate: listener-certs/default-listener.crt"
	@echo "  Private Key: listener-certs/default-listener.key"
