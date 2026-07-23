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
#
# Supplied explicitly when releasing (`make dist AIW_VERSION=1.0.0`, driven by the
# release workflow's ai_workspace_version input) so the zip pins a *released*
# ai-workspace image rather than whatever the working tree happens to carry. For
# local builds it falls back to the sibling VERSION file. There is deliberately no
# hardcoded version fallback — `dist` fails below if this resolves to empty, so a
# release can never silently ship a placeholder tag.
AIW_IMAGE   := $(DOCKER_REGISTRY)/ai-workspace
AIW_VERSION ?= $(shell sed 's/-SNAPSHOT//' ../ai-workspace/VERSION 2>/dev/null | tr -d '[:space:]')
AIW_TAG     := portals/ai-workspace/v$(AIW_VERSION)

# As with PLATFORM_API_VERSION below: an explicitly supplied version means "bundle
# that release", so the [ai_workspace] template appended to config-template.toml is
# read from the tag rather than the working tree — otherwise the zip would document
# settings from unreleased branch state against a released image.
ifeq ($(origin AIW_VERSION),command line)
AIW_FROM_TAG := true
endif
ifeq ($(origin AIW_VERSION),environment)
AIW_FROM_TAG := true
endif

# Platform API is a first-class service of this distribution (not profile-gated),
# so both its image tag AND its config sections are pinned to a released version.
#
# The fallback is applied to the captured value rather than with `|| echo`: in a
# `sed … < file` pipeline the exit status is sed's, and sed succeeds on empty
# input, so a missing or empty VERSION file would otherwise yield an empty string
# — which would also propagate into PLATFORM_API_TAG, producing the malformed git
# tag "platform-api/v".
PLATFORM_API_IMAGE   := $(DOCKER_REGISTRY)/platform-api
PLATFORM_API_VERSION ?= $(shell v=$$(sed 's/-SNAPSHOT//' ../../platform-api/VERSION 2>/dev/null); echo "$${v:-0.0.0}")
PLATFORM_API_TAG     := platform-api/v$(PLATFORM_API_VERSION)

# When PLATFORM_API_VERSION is supplied explicitly (e.g. `make dist PLATFORM_API_VERSION=0.13.0`),
# the platform-api configs and db-scripts are fetched from the released git tag $(PLATFORM_API_TAG)
# so the distribution is reproducible for that exact release — the merged config.toml then matches
# the image it is mounted into. Otherwise they are taken from the working tree.
ifeq ($(origin PLATFORM_API_VERSION),command line)
PLATFORM_API_FROM_TAG := true
endif
ifeq ($(origin PLATFORM_API_VERSION),environment)
PLATFORM_API_FROM_TAG := true
endif

# Stage an OPTIONAL component's config template:
# $(call stage_optional_template,<tag>,<repo-relative path>,<working-tree fallback>,<out>).
#
# The optional product does not have to be released when this distribution is cut —
# its version may name a release that is only tagged/published later. So the tag is
# preferred (its template then matches a published image) but a missing tag is a
# warning, not an error: fall back to the working-tree template rather than failing
# the release. CI checks out with actions/checkout's default shallow, tag-less clone,
# so tags are fetched once before deciding.
define stage_optional_template
	@tag="$(1)"; \
	if ! git -C ../.. rev-parse -q --verify "refs/tags/$$tag" >/dev/null 2>&1; then \
		git -C ../.. fetch --tags --quiet || true; \
	fi; \
	if git -C ../.. cat-file -e "$$tag:$(2)" 2>/dev/null; then \
		echo "  using config template from tag $$tag"; \
		git -C ../.. show "$$tag:$(2)" > $(4); \
	else \
		echo "  NOTE: tag $$tag not found (not released yet?) — using the working-tree config template."; \
		echo "        Documented settings may differ from the pinned image if that release diverges."; \
		cp $(3) $(4); \
	fi
endef

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
	@if [ -z "$(strip $(AIW_VERSION))" ]; then \
		echo "Error: AIW_VERSION is empty — the optional ai-workspace image tag cannot be resolved." >&2; \
		echo "Pass it explicitly (make dist AIW_VERSION=1.0.0) or ensure ../ai-workspace/VERSION exists." >&2; \
		exit 1; \
	fi
	@echo "Building distribution $(DIST_NAME)..."
	@echo "  platform-api image pinned to $(PLATFORM_API_IMAGE):$(PLATFORM_API_VERSION)"
	@echo "  optional ai-workspace image pinned to $(AIW_IMAGE):$(AIW_VERSION)"
	@mkdir -p $(DIST_DIR)/configs
	@mkdir -p $(DIST_DIR)/resources/developer-portal/db-scripts
	@cp -R database/* $(DIST_DIR)/resources/developer-portal/db-scripts
	@mkdir -p $(DIST_DIR)/resources/platform-api/db-scripts
	@mkdir -p $(DIST_DIR)/resources/samples
	@cp -R samples/apis $(DIST_DIR)/resources/samples/
	@cp -R samples/mcps $(DIST_DIR)/resources/samples/
	# Stage the Platform API source configs (active + template) and db-scripts into
	# the staging dir so the merge below can fold them into the unified config files.
	# Both the tag and working-tree branches produce the same two temp configs.
ifeq ($(PLATFORM_API_FROM_TAG),true)
	@echo "Fetching platform-api configs and db-scripts from tag $(PLATFORM_API_TAG)..."
	@tag="$(PLATFORM_API_TAG)"; dest="$(DIST_DIR)/resources/platform-api/db-scripts"; \
	if ! git -C ../.. rev-parse -q --verify "refs/tags/$$tag" >/dev/null 2>&1; then \
		git -C ../.. fetch --tags --quiet || true; \
	fi; \
	if ! git -C ../.. rev-parse -q --verify "refs/tags/$$tag" >/dev/null 2>&1; then \
		echo "Error: tag $$tag not found (check PLATFORM_API_VERSION)" >&2; exit 1; \
	fi; \
	files=$$(git -C ../.. ls-tree -r --name-only "$$tag" -- platform-api/internal/database | grep 'schema\.[^.]*\.sql$$'); \
	if [ -z "$$files" ]; then echo "Error: no .sql files found under platform-api/internal/database at $$tag" >&2; exit 1; fi; \
	for f in $$files; do git -C ../.. show "$$tag:$$f" > "$$dest/$$(basename $$f)"; done; \
	echo "✓ Fetched $$(echo "$$files" | wc -l | tr -d ' ') db-script(s) from $$tag"
	@git -C ../.. show "$(PLATFORM_API_TAG):platform-api/config/config.toml" \
		> $(DIST_DIR)/configs/.pa-config.toml
	@git -C ../.. show "$(PLATFORM_API_TAG):platform-api/config/config-template.toml" \
		> $(DIST_DIR)/configs/.pa-config-template.toml
else
	@cp ../../platform-api/internal/database/schema.*.sql $(DIST_DIR)/resources/platform-api/db-scripts/
	@cp ../../platform-api/config/config.toml $(DIST_DIR)/configs/.pa-config.toml
	@cp ../../platform-api/config/config-template.toml $(DIST_DIR)/configs/.pa-config-template.toml
endif
	# The merge below assumes each component's config is namespaced under its own
	# root table. Platform API versions predating the unified config ship
	# un-namespaced tables ([database] rather than [platform_api.database]), which
	# would merge into a silently broken file — fail loudly on that instead.
	@if ! grep -q '^\[platform_api' $(DIST_DIR)/configs/.pa-config.toml; then \
		echo "Error: the platform-api config for $(PLATFORM_API_VERSION) has no [platform_api] root table." >&2; \
		echo "That version predates the unified multi-component config and cannot be bundled — pick a newer PLATFORM_API_VERSION." >&2; \
		exit 1; \
	fi
	# Unified config: the standalone zip ships ONE config.toml carrying both the
	# [developer_portal] and [platform_api] sections (portal on top, platform-api
	# below). Each service reads only its own root table and ignores the other's,
	# so the same file is mounted into both containers. Merged from the monorepo's
	# two per-component source configs — the zip has no platform-api/ sibling.
	$(call merge_config,configs/config.toml,$(DIST_DIR)/configs/.pa-config.toml,$(DIST_DIR)/configs/config.toml)
	$(call merge_config,configs/config-template.toml,$(DIST_DIR)/configs/.pa-config-template.toml,$(DIST_DIR)/configs/config-template.toml)
	# AI Workspace is an opt-in Compose profile (see README) — its section is NOT
	# added to the active config.toml, but its template is appended at the bottom
	# of config-template.toml so a user enabling the profile has the [ai_workspace]
	# settings documented to copy. Sourced from the ai-workspace release tag when a
	# version was supplied, so the documented settings match the pinned image.
ifeq ($(AIW_FROM_TAG),true)
	$(call stage_optional_template,$(AIW_TAG),portals/ai-workspace/configs/config-template.toml,../ai-workspace/configs/config-template.toml,$(DIST_DIR)/configs/.aiw-config-template.toml)
else
	@cp ../ai-workspace/configs/config-template.toml $(DIST_DIR)/configs/.aiw-config-template.toml
endif
	$(call append_section,AI WORKSPACE (optional),$(DIST_DIR)/configs/.aiw-config-template.toml,$(DIST_DIR)/configs/config-template.toml)
	@rm -f $(DIST_DIR)/configs/.pa-config.toml $(DIST_DIR)/configs/.pa-config-template.toml $(DIST_DIR)/configs/.aiw-config-template.toml
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
	    -e 's|image: .*/platform-api:[^[:space:]]*|image: $(PLATFORM_API_IMAGE):$(PLATFORM_API_VERSION)|g' \
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
