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

# Version/image used to pin the optional ai-workspace image (the profile-gated
# service in docker-compose.yaml). Its config is not merged into the active
# config.toml — the profile is opt-in and self-served (see README) — so only the
# image tag is pinned here for a reproducible release.
AIW_IMAGE   := $(DOCKER_REGISTRY)/ai-workspace
AIW_VERSION ?= $(shell cat ../ai-workspace/VERSION 2>/dev/null | sed 's/-SNAPSHOT//' || echo "0.0.0")

# Merge a Developer Portal config and a Platform API config into one unified
# multi-component config file: $(call merge_config,<dp-toml>,<cp-toml>,<out>).
# The components share no top-level keys ([developer_portal.*] vs [platform_api.*],
# and [ai_workspace.*] when append_section adds it below), so the merge is a
# banner-separated concatenation — each service reads only its own root table and
# ignores the other sections and their tokens.
define merge_config
	@printf '%s\n' \
		'# =============================================================================' \
		'# UNIFIED API PLATFORM CONFIGURATION' \
		'# =============================================================================' \
		'# One file, multiple components. Each service reads ONLY its own root table' \
		'# (e.g. [developer_portal], [platform_api]) and ignores the other sections and' \
		'# their tokens. GENERATED by `make dist` from the per-component source configs.' \
		'' '' \
		'# #############################################################################' \
		'# #  DEVELOPER PORTAL' \
		'# #############################################################################' \
		'' > $(3)
	@cat $(1) >> $(3)
	@printf '%s\n' \
		'' '' \
		'# #############################################################################' \
		'# #  PLATFORM API' \
		'# #############################################################################' \
		'' >> $(3)
	@cat $(2) >> $(3)
endef

# Append one more component section (banner + body) to a config file already
# written by merge_config: $(call append_section,<title>,<toml>,<out>). Used for
# the config-template only — it documents an optional component's settings at the
# bottom of the reference file without adding that section to the active config.toml.
define append_section
	@printf '%s\n' \
		'' '' \
		'# #############################################################################' \
		'# #  $(1)' \
		'# #############################################################################' \
		'' >> $(3)
	@cat $(2) >> $(3)
endef

.PHONY: dist
dist: clean-dist ## Build standalone developer portal distribution zip
	@echo "Building distribution $(DIST_NAME)..."
	@mkdir -p $(DIST_DIR)/configs
	@mkdir -p $(DIST_DIR)/resources/developer-portal/db-scripts
	@cp -R database/* $(DIST_DIR)/resources/developer-portal/db-scripts
	@mkdir -p $(DIST_DIR)/resources/samples
	@cp -R samples/apis $(DIST_DIR)/resources/samples/
	@cp -R samples/mcps $(DIST_DIR)/resources/samples/
	# Unified config: the standalone zip ships ONE config.toml carrying both the
	# [developer_portal] and [platform_api] sections (portal on top, platform-api
	# below). Each service reads only its own root table and ignores the other's,
	# so the same file is mounted into both containers. Merged from the monorepo's
	# two per-component source configs — the zip has no platform-api/ sibling.
	$(call merge_config,configs/config.toml,../../platform-api/config/config.toml,$(DIST_DIR)/configs/config.toml)
	$(call merge_config,configs/config-template.toml,../../platform-api/config/config-template.toml,$(DIST_DIR)/configs/config-template.toml)
	# AI Workspace is an opt-in Compose profile (see README) — its section is NOT
	# added to the active config.toml, but its template is appended at the bottom
	# of config-template.toml so a user enabling the profile has the [ai_workspace]
	# settings documented to copy.
	$(call append_section,AI WORKSPACE (optional),../ai-workspace/configs/config-template.toml,$(DIST_DIR)/configs/config-template.toml)
	# Repoint the platform-api mount at the single merged file (the source compose
	# mounts the monorepo's ../../platform-api/config/config.toml); the devportal
	# mount already points at ./configs/config.toml, so both now share one file.
	# The trailing ':' anchors the match to the volume mapping so comments that
	# mention the source path are left untouched.
	@sed 's#\.\./\.\./platform-api/config/config\.toml:#./configs/config.toml:#' \
		docker-compose.yaml > $(DIST_DIR)/docker-compose.yaml
	@cp distribution/README.md $(DIST_DIR)/README.md
	@mkdir -p $(DIST_DIR)/scripts
	@cp scripts/setup.sh $(DIST_DIR)/scripts/setup.sh
	@chmod +x $(DIST_DIR)/scripts/setup.sh
	@cp scripts/seed-samples.sh $(DIST_DIR)/scripts/seed-samples.sh
	@chmod +x $(DIST_DIR)/scripts/seed-samples.sh
	@sed -i.bak \
	    -e 's|image: .*/developer-portal:[^[:space:]]*|image: $(DEVPORTAL_IMAGE):$(DIST_VERSION)|g' \
	    -e 's|image: .*/ai-workspace:[^[:space:]]*|image: $(AIW_IMAGE):$(AIW_VERSION)|g' \
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
