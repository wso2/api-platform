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

# Developer Portal Makefile

SHELL := /bin/bash

# Read shared version from VERSION file
VERSION := $(shell cat VERSION 2>/dev/null || echo "0.0.1-SNAPSHOT")

# Docker registry configuration
DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform

# Image name
DEVPORTAL_IMAGE := $(DOCKER_REGISTRY)/developer-portal

# API docs tooling
REPO_ROOT         := $(shell git rev-parse --show-toplevel)
APIDOCS_TOOLS_DIR := $(REPO_ROOT)/tools/apidocs
DOCS_OUT_DIR      := $(REPO_ROOT)/docs/rest-apis/devportal
OPENAPI_SPEC      := docs/devportal-openapi-spec-v0.9.yaml

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo 'Developer Portal Build System (Version: $(VERSION))'
	@echo ''
	@echo 'Build Targets:'
	@echo '  make build                      - Build developer-portal Docker image (buildx)'
	@echo '  make build-and-push-multiarch   - Build and push multi-arch image to GHCR'
	@echo ''
	@echo 'Distribution Targets:'
	@echo '  make dist                        - Build standalone distribution zip (target/wso2apip-developer-portal-<VERSION>.zip)'
	@echo '  make clean-dist                  - Remove distribution staging dir and zip'
	@echo ''
	@echo 'Version Management:'
	@echo '  make version-set VERSION=X       - Set developer portal version and update artifacts'
	@echo '  make version-bump-patch          - Bump patch version'
	@echo '  make version-bump-minor          - Bump minor version'
	@echo '  make version-bump-major          - Bump major version'
	@echo '  make version-bump-next-dev       - Bump to next minor dev version with SNAPSHOT'
	@echo '  make version-get-release         - Get release version (strips SNAPSHOT suffix)'
	@echo ''
	@echo 'Docs Targets:'
	@echo '  make generate-apidocs            - Generate REST API docs from OpenAPI spec'
	@echo ''
	@echo 'Clean Targets:'
	@echo '  make clean                       - Clean all build artifacts'
	@echo ''
	@echo 'Integration Tests:'
	@echo '  make it                          - Run Cypress integration tests against SQLite (headless, in Docker)'
	@echo '  make it-postgres                 - Run Cypress integration tests against PostgreSQL (headless, in Docker)'
	@echo '  make it-open                     - Open Cypress interactive UI (needs devportal running locally)'

# Build Targets

.PHONY: build
build: ## Build developer-portal Docker image
	@echo "Building developer-portal Docker image ($(VERSION))..."
	docker buildx build \
		--tag $(DEVPORTAL_IMAGE):$(VERSION) \
		--load \
		.

.PHONY: build-and-push-multiarch
build-and-push-multiarch: ## Build and push multi-arch developer-portal image to GHCR
	@echo "Building and pushing multi-arch developer-portal Docker image ($(VERSION))..."
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--tag $(DEVPORTAL_IMAGE):$(VERSION) \
		--push \
		.

# Version Management Targets

.PHONY: version-set
version-set: ## Set developer portal version and update artifacts
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION required"; \
		echo "Usage: make version-set VERSION=1.0.0"; \
		exit 1; \
	fi
	@echo "$(VERSION)" > VERSION
	@npm version $(VERSION) --no-git-tag-version --allow-same-version >/dev/null
	@(cd ../.. && bash scripts/update-docker-compose.sh developer-portal $(VERSION))
	@echo "✓ Set developer portal version to $(VERSION)"

.PHONY: version-bump-patch
version-bump-patch: ## Bump patch version
	@$(MAKE) version-set VERSION=$$(cd ../.. && bash scripts/next-version.sh patch portals/developer-portal)

.PHONY: version-bump-minor
version-bump-minor: ## Bump minor version
	@$(MAKE) version-set VERSION=$$(cd ../.. && bash scripts/next-version.sh minor portals/developer-portal)

.PHONY: version-bump-major
version-bump-major: ## Bump major version
	@$(MAKE) version-set VERSION=$$(cd ../.. && bash scripts/next-version.sh major portals/developer-portal)

.PHONY: version-bump-next-dev
version-bump-next-dev: ## Bump to next minor dev version with SNAPSHOT suffix
	@$(MAKE) version-set VERSION=$$(cd ../.. && bash scripts/next-version.sh next-dev portals/developer-portal)

.PHONY: version-get-release
version-get-release: ## Get release version (strips SNAPSHOT suffix)
	@VERSION=$$(cat VERSION 2>/dev/null | tr -d '[:space:]' | sed 's/-SNAPSHOT//'); \
	if [ -z "$$VERSION" ]; then \
		echo "Error: VERSION is empty or contains only whitespace. Check portals/developer-portal/VERSION file." >&2; \
		exit 1; \
	fi; \
	echo "$$VERSION"

# Distribution Targets

DIST_VERSION := $(shell echo "$(VERSION)" | sed 's/-SNAPSHOT//')
DIST_NAME    := wso2apip-developer-portal-$(DIST_VERSION)
DIST_DIR     := target/$(DIST_NAME)
DIST_ZIP     := target/$(DIST_NAME).zip

.PHONY: dist
dist: clean-dist ## Build standalone developer portal distribution zip
	@echo "Building distribution $(DIST_NAME)..."
	@mkdir -p $(DIST_DIR)/configs
	@mkdir -p $(DIST_DIR)/resources/developer-portal/db-scripts
	@cp -R database/* $(DIST_DIR)/resources/developer-portal/db-scripts
	@mkdir -p $(DIST_DIR)/resources/samples
	@cp -R samples/apis $(DIST_DIR)/resources/samples/
	@cp -R samples/mcps $(DIST_DIR)/resources/samples/
	@cp configs/config.toml $(DIST_DIR)/configs/config.toml
	@cp configs/config-template.toml $(DIST_DIR)/configs/config-template.toml
	# The source docker-compose.yaml mounts ../../platform-api/config/config.toml
	# directly (the monorepo's single source-of-truth Platform API config, no
	# per-portal copy) — the standalone zip has no platform-api/ sibling, so bake
	# a real copy into its own configs/ dir and repoint the compose file at it.
	@cp ../../platform-api/config/config.toml $(DIST_DIR)/configs/config-platform-api.toml
	@cp ../../platform-api/config/config-template.toml $(DIST_DIR)/configs/config-platform-api-template.toml
	@sed 's#\.\./\.\./platform-api/config/config\.toml#./configs/config-platform-api.toml#' \
		docker-compose.yaml > $(DIST_DIR)/docker-compose.yaml
	@cp distribution/README.md $(DIST_DIR)/README.md
	@mkdir -p $(DIST_DIR)/scripts
	@cp scripts/setup.sh $(DIST_DIR)/scripts/setup.sh
	@chmod +x $(DIST_DIR)/scripts/setup.sh
	@cp scripts/seed-samples.sh $(DIST_DIR)/scripts/seed-samples.sh
	@chmod +x $(DIST_DIR)/scripts/seed-samples.sh
	@sed -i.bak \
	    -e 's|image: .*/developer-portal:[^[:space:]]*|image: $(DEVPORTAL_IMAGE):$(DIST_VERSION)|g' \
	    $(DIST_DIR)/docker-compose.yaml
	@rm -f $(DIST_DIR)/docker-compose.yaml.bak
	@cd target && zip -rq $(DIST_NAME).zip $(DIST_NAME)
	@rm -rf $(DIST_DIR)
	@echo "✓ Built $(DIST_ZIP)"

.PHONY: clean-dist
clean-dist: ## Remove distribution staging dir and zip
	@rm -rf target/wso2apip-developer-portal-*

# Clean Targets

.PHONY: clean
clean: clean-dist ## Clean all build artifacts
	@echo "Cleaned developer portal build artifacts"

# Docs Targets

.PHONY: generate-apidocs
generate-apidocs: ## Generate REST API docs from devportal OpenAPI spec
	@command -v widdershins >/dev/null 2>&1 || { echo "widdershins not found, installing..."; npm install -g widdershins; }
	@widdershins $(OPENAPI_SPEC) -o /tmp/dp-openapi.md \
		--omitHeader --summary --language_tabs 'shell:Shell:curl' \
		--user_templates $(APIDOCS_TOOLS_DIR)/widdershins_templates/
	@bash $(APIDOCS_TOOLS_DIR)/split-openapi.sh /tmp/dp-openapi.md $(DOCS_OUT_DIR)
	@echo "REST API docs generated at $(DOCS_OUT_DIR)"

# Integration Test Targets

.PHONY: it it-postgres it-open
it: ## Run Cypress integration tests against SQLite (headless, in Docker)
	$(MAKE) -C it test DOCKER_REGISTRY=$(DOCKER_REGISTRY)

it-postgres: ## Run Cypress integration tests against PostgreSQL (headless, in Docker)
	$(MAKE) -C it test-postgres DOCKER_REGISTRY=$(DOCKER_REGISTRY)

it-open: ## Open Cypress interactive UI against a locally running devportal
	$(MAKE) -C it open DOCKER_REGISTRY=$(DOCKER_REGISTRY)
