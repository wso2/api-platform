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

.PHONY: help generate build test generate-listener-certs push build-coverage-image build-debug

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

generate: ## Generate API server code from OpenAPI spec
	@echo "Generating API server code from OpenAPI spec..."
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1 --config=oapi-codegen.yaml api/openapi.yaml
	@echo "Generating admin API models from OpenAPI admin spec..."
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1 --config=oapi-codegen-admin.yaml api/openapi-admin.yaml
test: ## Run unit and integration tests
	@echo "Running tests..."
	@go test -v ./... -cover -coverprofile=unit-test-coverage.txt

build: generate test ## Build Docker image using buildx
	@echo "Building Docker image ($(IMAGE_NAME):$(VERSION))..."
	docker buildx build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context common=../../common \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--target production \
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
		--build-context sdk=../../sdk \
		--build-context common=../../common \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--target production \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		--push \
		.

build-coverage-image: test ## Build coverage-instrumented gateway-controller image
	docker buildx build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context common=../../common \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg ENABLE_COVERAGE=true \
		--target production \
		-t $(IMAGE_NAME)-coverage:$(VERSION) \
		--load \
		.

build-debug: ## Build debug image for remote debugging with dlv (VS Code attach on port 2345)
	docker buildx build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context common=../../common \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg ENABLE_DEBUG=true \
		--target debug \
		-t $(IMAGE_NAME)-debug:$(VERSION) \
		-t $(IMAGE_NAME)-debug:latest \
		--load \
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

generate-postman-collection: ## Generate Postman collection from OpenAPI spec
	@echo "Generating Postman collection from OpenAPI spec..."
	@npx openapi-to-postmanv2 -s api/openapi.yaml -o api/postman_collection.json -p
	@echo "Postman collection generated at api/postman_collection.json"
