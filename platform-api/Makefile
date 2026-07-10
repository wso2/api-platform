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
IMAGE_NAME := $(DOCKER_REGISTRY)/platform-api
PORT ?= 9243

# Integration test configuration
IT_PROJECT ?= platform-api-it
MSSQL_PASSWORD ?= Strong!Passw0rd
# On Apple Silicon, override with: MSSQL_IMAGE=mcr.microsoft.com/azure-sql-edge:latest
MSSQL_IMAGE ?= mcr.microsoft.com/mssql/server:2022-latest

.PHONY: help build test push build-and-push-multiarch it it-sqlite it-postgres it-sqlserver it-all-dbs

help: ## Show this help message
	@echo 'Platform API Build System (Version: $(VERSION))'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-25s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

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

generate: ## Generate API server code from OpenAPI spec
	@echo "Adding bindings to OpenAPI spec..."
	@command -v yq >/dev/null 2>&1 || { echo "Error: yq is not installed. Install it via 'brew install yq' or 'go install github.com/mikefarah/yq/v4@latest'"; exit 1; }
	@yq '.components.schemas[].properties |= (. as $$props | (parent | .required[]) as $$req_field | select($$props | has($$req_field)) | .[$$req_field]."x-oapi-codegen-extra-tags".binding = "required")' resources/openapi.yaml > resources/openapi_with_binding.yaml
	@echo "Generating API server code from OpenAPI spec with bindings..."
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1 --config=oapi-codegen.yaml resources/openapi_with_binding.yaml
	@echo "Generating API models for the eventgateway plugin spec..."
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1 --config=oapi-codegen-eventgateway.yaml plugins/eventgateway/openapi.yaml

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
