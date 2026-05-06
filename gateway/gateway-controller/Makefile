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
VERSION_PKG := github.com/wso2/api-platform/gateway/gateway-controller/pkg/version
LDFLAGS := -X $(VERSION_PKG).Version=$(VERSION) -X $(VERSION_PKG).GitCommit=$(GIT_COMMIT) -X $(VERSION_PKG).BuildDate=$(BUILD_DATE)

# Docker image configuration
DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
IMAGE_NAME := $(DOCKER_REGISTRY)/gateway-controller

# Policy definitions are produced by the gateway-runtime build (policy-compiler stage)
# and extracted via `make -C ../gateway-runtime extract-policies`.
POLICIES_BUILD_CONTEXT ?= ../target/build/gateway-controller/policies

# Doc generation
REPO_ROOT := $(shell git rev-parse --show-toplevel)
APIDOCS_TOOLS_DIR := $(REPO_ROOT)/tools/apidocs
DOCS_OUT_DIR := $(REPO_ROOT)/docs/rest-apis/gateway

.PHONY: help generate generate-server-code generate-apidocs build test generate-listener-certs push build-coverage-image build-debug clean build-and-push-multiarch generate-postman-collection

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

generate-server-code: ## Generate API server code from OpenAPI spec
	@echo "Generating API server code from OpenAPI spec..."
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1 --config=oapi-codegen.yaml api/management-openapi.yaml
	@echo "Generating admin API models from OpenAPI admin spec..."
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1 --config=oapi-codegen-admin.yaml api/admin-openapi.yaml

generate: generate-server-code generate-apidocs ## Generate server code and API docs (run both steps in order)
	@echo ""
	@echo "=========================================================="
	@echo " Generation complete. Two steps were run:"
	@echo "  [1] generate-server-code  — Go server stubs from OpenAPI"
	@echo "  [2] generate-apidocs      — REST API docs (Markdown)"
	@echo "=========================================================="
	@echo ""
	@echo "  Please review the generated API docs at:"
	@echo "    $(DOCS_OUT_DIR)"
	@echo ""
	@echo "  Once satisfied, stage and commit:"
	@echo "    git add $(DOCS_OUT_DIR)"
	@echo "    git commit -m 'update generated api docs'"
	@echo "=========================================================="
test: ## Run unit and integration tests
	@echo "Running tests..."
	@go test -v ./... -cover -coverprofile=unit-test-coverage.txt

build: generate-server-code test ## Build Docker image using buildx
	@echo "Building Docker image ($(IMAGE_NAME):$(VERSION))..."
	docker buildx build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context build-manifest=.. \
		--build-context policies=$(POLICIES_BUILD_CONTEXT) \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--target production \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		--load \
		.
	@echo "Docker image ($(IMAGE_NAME):$(VERSION)) built successfully."

push: ## Push Docker image to registry
	@echo "Pushing Docker images..."
	docker push $(IMAGE_NAME):$(VERSION)
	docker push $(IMAGE_NAME):latest

build-and-push-multiarch: ## Build and push multi-architecture Docker image (linux/amd64, linux/arm64)
	@echo "Building and pushing multi-arch Docker image: $(IMAGE_NAME):$(VERSION)"
	docker buildx build -f Dockerfile \
		--build-context sdk=../../sdk \
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context build-manifest=.. \
		--build-context policies=$(POLICIES_BUILD_CONTEXT) \
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
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context build-manifest=.. \
		--build-context policies=$(POLICIES_BUILD_CONTEXT) \
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
		--build-context sdk-core=../../sdk/core \
		--build-context common=../../common \
		--build-context build-manifest=.. \
		--build-context policies=$(POLICIES_BUILD_CONTEXT) \
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
	@npx openapi-to-postmanv2 -s api/management-openapi.yaml -o api/postman_collection.json -p
	@echo "Postman collection generated at api/postman_collection.json"

generate-apidocs: ## Generate REST API docs from OpenAPI specs
	@command -v widdershins >/dev/null 2>&1 || { echo "widdershins not found, installing..."; npm install -g widdershins; }
	@echo "Generating REST API docs..."
	@widdershins api/management-openapi.yaml -o /tmp/gw-openapi.md \
		--omitHeader --summary \
		--language_tabs 'shell:Shell:curl' \
		--user_templates $(APIDOCS_TOOLS_DIR)/widdershins_templates/
	@bash $(APIDOCS_TOOLS_DIR)/split-openapi.sh /tmp/gw-openapi.md $(DOCS_OUT_DIR)
	@rm -f /tmp/gw-openapi.md
	@echo "REST API docs generated at $(DOCS_OUT_DIR)"

clean: ## Remove SQLite database files from data/
	@echo "Cleaning database files from data/..."
	@rm -f data/*.db data/*.db-shm data/*.db-wal
	@echo "Done."
