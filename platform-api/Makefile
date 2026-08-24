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

# Makefile for Platform API

# Version management
VERSION ?= $(shell cat VERSION 2>/dev/null || echo "0.0.1-SNAPSHOT")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Docker image configuration
DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
IMAGE_SUFFIX ?= platform-api
IMAGE_NAME := $(DOCKER_REGISTRY)/$(IMAGE_SUFFIX)
# Cloud image — built with EXPERIMENTAL=true so the event-gateway plugin
# (WebSub/WebBroker API management) is compiled in. Published under a distinct
# image name so it never collides with the OSS platform-api image.
CLOUD_IMAGE_SUFFIX ?= platform-api-cloud
CLOUD_IMAGE_NAME := $(DOCKER_REGISTRY)/$(CLOUD_IMAGE_SUFFIX)
PORT ?= 9243

# Cloud image tag: defaults to "<branch>-<commit>" computed IDENTICALLY to the
# platform-api-cloud-release workflow's IMAGE_VERSION step — sanitize the branch
# ([^a-zA-Z0-9._-] -> '-'), prefix '_' if it doesn't start with [a-zA-Z0-9_],
# cap the branch at 87 chars, then append the FULL commit SHA. Prefer GitHub
# Actions' GITHUB_REF_NAME/GITHUB_SHA when set (git rev-parse --abbrev-ref HEAD
# returns "HEAD" after actions/checkout's detached checkout, which would yield a
# "HEAD-<commit>" tag); fall back to git for local builds. An explicitly supplied
# VERSION (CI passes PLATFORM_API_VERSION, or `make cloud-build VERSION=x`) takes
# precedence over all of this.
CLOUD_IMAGE_TAG := $(shell \
	b="$${GITHUB_REF_NAME:-$$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)}"; \
	b=$$(printf '%s' "$$b" | sed -e 's/[^a-zA-Z0-9._-]/-/g' -e 's/^[^a-zA-Z0-9_]/_&/' | cut -c1-87); \
	printf '%s-%s' "$$b" "$${GITHUB_SHA:-$$(git rev-parse HEAD 2>/dev/null || echo unknown)}")
ifeq ($(origin VERSION),command line)
CLOUD_IMAGE_TAG := $(VERSION)
endif
ifeq ($(origin VERSION),environment)
CLOUD_IMAGE_TAG := $(VERSION)
endif

# Integration test configuration
IT_PROJECT ?= platform-api-it
MSSQL_PASSWORD ?= Strong!Passw0rd
# On Apple Silicon, override with: MSSQL_IMAGE=mcr.microsoft.com/azure-sql-edge:latest
MSSQL_IMAGE ?= mcr.microsoft.com/mssql/server:2022-latest

# Doc generation
REPO_ROOT := $(shell git rev-parse --show-toplevel)
APIDOCS_TOOLS_DIR := $(REPO_ROOT)/tools/apidocs
DOCS_OUT_DIR := $(REPO_ROOT)/docs/rest-apis/platform-api

.PHONY: help build cloud-build test push build-and-push-multiarch cloud-build-and-push-multiarch it it-sqlite it-postgres it-sqlserver it-all-dbs generate generate-apidocs setup-local-dev run-local

help: ## Show this help message
	@echo 'Platform API Build System (Version: $(VERSION))'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-25s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

setup-local-dev: ## One-time secret provisioning for `make run-local` (TLS cert, JWT keypair, encryption key, admin credentials)
	@./scripts/setup-local-dev.sh

run-local: ## Run Platform API standalone via `go run` against config/config.local.toml (run `make setup-local-dev` first)
	@test -f local-dev.env || (echo "local-dev.env not found — run 'make setup-local-dev' first" >&2 && exit 1)
	@test -f resources/keys/encryption.key || (echo "resources/keys/encryption.key not found — run 'make setup-local-dev' first" >&2 && exit 1)
	export $$(grep -v '^#' local-dev.env | xargs) && \
		export APIP_CP_ENCRYPTION_KEY=$$(cat resources/keys/encryption.key) && \
		go run ./cmd/main.go -config config/config.local.toml

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

it: it-sqlite ## Run cross-database integration tests on SQLite (alias of it-sqlite)

it-sqlite: ## Run cross-database integration tests on SQLite
	@echo "Running integration tests on SQLite..."
	@IT_DB=sqlite go test -tags integration -count=1 -v ./internal/integration/...

it-postgres: ## Run cross-database integration tests on PostgreSQL (spins up a container)
	@echo "Starting PostgreSQL for integration tests..."
	@docker compose -p $(IT_PROJECT) -f it/docker-compose.postgres.yaml up -d --wait
	@IT_DB=postgres IT_DB_HOST=localhost IT_DB_PORT=55432 IT_DB_USER=postgres \
		IT_DB_PASSWORD=postgres IT_DB_NAME=platform_api_it \
		go test -tags integration -count=1 -v ./internal/integration/...; rc=$$?; \
		docker compose -p $(IT_PROJECT) -f it/docker-compose.postgres.yaml down -v >/dev/null 2>&1; \
		exit $$rc

it-sqlserver: ## Run cross-database integration tests on SQL Server (spins up a container)
	@echo "Starting SQL Server ($(MSSQL_IMAGE)) for integration tests..."
	@MSSQL_IMAGE=$(MSSQL_IMAGE) MSSQL_PASSWORD='$(MSSQL_PASSWORD)' \
		docker compose -p $(IT_PROJECT) -f it/docker-compose.sqlserver.yaml up -d
	@IT_DB=sqlserver IT_DB_HOST=localhost IT_DB_PORT=14333 IT_DB_USER=sa \
		IT_DB_PASSWORD='$(MSSQL_PASSWORD)' IT_DB_NAME=platform_api_it \
		go test -tags integration -count=1 -v ./internal/integration/...; rc=$$?; \
		docker compose -p $(IT_PROJECT) -f it/docker-compose.sqlserver.yaml down -v >/dev/null 2>&1; \
		exit $$rc

it-all-dbs: ## Run integration tests across SQLite, PostgreSQL and SQL Server
	@$(MAKE) it-sqlite
	@$(MAKE) it-postgres
	@$(MAKE) it-sqlserver

e2e: ## Run the combined platform-api + gateway Cucumber e2e (E2E_DB=postgres|sqlite|sqlserver)
	@cd ../tests/integration-e2e && E2E_DB=$${E2E_DB:-postgres} go test -run TestFeatures -count=1 -v -timeout 25m ./...

e2e-all-dbs: ## Run the combined Cucumber e2e across all three databases
	@cd ../tests/integration-e2e && E2E_DB=sqlite go test -run TestFeatures -count=1 -v -timeout 25m ./...
	@cd ../tests/integration-e2e && E2E_DB=postgres go test -run TestFeatures -count=1 -v -timeout 25m ./...
	@cd ../tests/integration-e2e && E2E_DB=sqlserver go test -run TestFeatures -count=1 -v -timeout 25m ./...

build: ## Build Docker image
	@echo "Building Docker image ($(IMAGE_NAME):$(VERSION))..."
	docker buildx build -f Dockerfile \
		--build-context common=../common \
		--build-context httpkit=../httpkit \
		--build-arg VERSION=$(VERSION) \
		--build-arg PORT=$(PORT) \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		--load \
		.

cloud-build: VERSION := $(CLOUD_IMAGE_TAG)
cloud-build: ## Build cloud Docker image with the event-gateway plugin (EXPERIMENTAL=true), tagged <branch>-<commit> (override with VERSION=x)
	@echo "Building cloud Docker image ($(CLOUD_IMAGE_NAME):$(VERSION)) with event-gateway plugin..."
	docker buildx build -f Dockerfile \
		--build-context common=../common \
		--build-context httpkit=../httpkit \
		--build-arg VERSION=$(VERSION) \
		--build-arg PORT=$(PORT) \
		--build-arg EXPERIMENTAL=true \
		-t $(CLOUD_IMAGE_NAME):$(VERSION) \
		-t $(CLOUD_IMAGE_NAME):latest \
		--load \
		.

generate: ## Generate API server code from OpenAPI spec
	@echo "Adding bindings to OpenAPI spec..."
	@command -v yq >/dev/null 2>&1 || { echo "Error: yq is not installed. Install it via 'brew install yq' or 'go install github.com/mikefarah/yq/v4@latest'"; exit 1; }
	@yq '.components.schemas[].properties |= (. as $$props | (parent | .required[]) as $$req_field | select($$props | has($$req_field)) | .[$$req_field]."x-oapi-codegen-extra-tags".binding = "required")' resources/openapi.yaml > resources/openapi_with_binding.yaml
	@echo "Generating API server code from OpenAPI spec with bindings..."
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1 --config=oapi-codegen.yaml resources/openapi_with_binding.yaml
	@echo "Generating API models for the eventgateway plugin spec..."
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1 --config=oapi-codegen-eventgateway.yaml plugins/eventgateway/openapi.yaml

generate-apidocs: ## Generate REST API docs (Markdown) from the Platform API OpenAPI spec
	@command -v widdershins >/dev/null 2>&1 || { echo "widdershins not found, installing..."; npm install -g widdershins; }
	@echo "Generating REST API docs..."
	@widdershins resources/openapi.yaml -o /tmp/platform-api-openapi.md \
		--omitHeader --summary \
		--language_tabs 'shell:Shell:curl' \
		--user_templates $(APIDOCS_TOOLS_DIR)/widdershins_templates/
	@bash $(APIDOCS_TOOLS_DIR)/split-openapi.sh /tmp/platform-api-openapi.md $(DOCS_OUT_DIR)
	@rm -f /tmp/platform-api-openapi.md
	@echo "REST API docs generated at $(DOCS_OUT_DIR)"

push: ## Push Docker image to registry
	@echo "Pushing Docker images..."
	docker push $(IMAGE_NAME):$(VERSION)
	docker push $(IMAGE_NAME):latest

build-and-push-multiarch: ## Build and push multi-architecture Docker image (linux/amd64, linux/arm64)
	@echo "Building and pushing multi-arch Docker image: $(IMAGE_NAME):$(VERSION)"
	docker buildx build -f Dockerfile \
		--platform linux/amd64,linux/arm64 \
		--build-context common=../common \
		--build-context httpkit=../httpkit \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg PORT=$(PORT) \
		-t $(IMAGE_NAME):$(VERSION) \
		--push \
		.

cloud-build-and-push-multiarch: VERSION := $(CLOUD_IMAGE_TAG)
cloud-build-and-push-multiarch: ## Build and push multi-arch cloud image with the event-gateway plugin (linux/amd64, linux/arm64), tagged <branch>-<commit> (override with VERSION=x)
	@echo "Building and pushing multi-arch cloud image: $(CLOUD_IMAGE_NAME):$(VERSION) with event-gateway plugin"
	docker buildx build -f Dockerfile \
		--platform linux/amd64,linux/arm64 \
		--build-context common=../common \
		--build-context httpkit=../httpkit \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--build-arg PORT=$(PORT) \
		--build-arg EXPERIMENTAL=true \
		-t $(CLOUD_IMAGE_NAME):$(VERSION) \
		--push \
		.

# Version Management Targets
version-set: ## Set platform-api version and update artifacts
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION required"; \
		echo "Usage: make version-set VERSION=1.0.0"; \
		exit 1; \
	fi
	@echo "$(VERSION)" > VERSION
	@bash ../scripts/update-docker-compose.sh platform-api $(VERSION)
	@bash ../scripts/update-helm.sh platform-api $(VERSION)
	@echo "✓ Set platform-api version to $(VERSION)"

version-bump-patch: ## Bump patch version
	@$(MAKE) version-set VERSION=$$(cd .. && bash scripts/next-version.sh patch platform-api)

version-bump-minor: ## Bump minor version
	@$(MAKE) version-set VERSION=$$(cd .. && bash scripts/next-version.sh minor platform-api)

version-bump-major: ## Bump major version
	@$(MAKE) version-set VERSION=$$(cd .. && bash scripts/next-version.sh major platform-api)

version-bump-next-dev: ## Bump to next minor dev version with SNAPSHOT suffix
	@$(MAKE) version-set VERSION=$$(cd .. && bash scripts/next-version.sh next-dev platform-api)

version-get-release: ## Get release version (strips SNAPSHOT suffix)
	@VERSION=$$(cat VERSION 2>/dev/null | tr -d '[:space:]' | sed 's/-SNAPSHOT//'); \
	if [ -z "$$VERSION" ]; then \
		echo "Error: VERSION is empty or contains only whitespace. Check platform-api/VERSION file." >&2; \
		exit 1; \
	fi; \
	echo "$$VERSION"
