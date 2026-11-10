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

# API Portal Makefile

SHELL := /bin/bash

# Read shared version from VERSION file
VERSION := $(shell cat VERSION 2>/dev/null || echo "0.0.1-SNAPSHOT")

# Docker registry configuration
DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform

# Image name
API_PORTAL_IMAGE := $(DOCKER_REGISTRY)/api-portal

# API docs tooling
REPO_ROOT         := $(shell git rev-parse --show-toplevel)
APIDOCS_TOOLS_DIR := $(REPO_ROOT)/tools/apidocs
DOCS_OUT_DIR      := $(REPO_ROOT)/docs/rest-apis/api-portal
OPENAPI_SPEC      := resources/openapi/api-portal-openapi-spec-v0.9.yaml

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo 'API Portal Build System (Version: $(VERSION))'
	@echo ''
	@echo 'Build Targets:'
	@echo '  make build                      - Build API Portal Docker image (buildx)'
	@echo '  make build-and-push-multiarch   - Build and push multi-arch image to GHCR'
	@echo '  make binary                     - Build the binary zip (target/wso2apip-api-portal-binary-<VERSION>.zip)'
	@echo '  make binary-image               - Build the binary zip as an image (PUSH=true to push it)'
	@echo '  make product-binary PLATFORM_API_BINARY_ZIP=<zip>'
	@echo '                                  - Build the product binary zip (API Portal + Platform API) in target/product/'
	@echo ''
	@echo 'Distribution Targets:'
	@echo '  make dist                        - Build standalone distribution zip (target/wso2apip-api-portal-<VERSION>.zip)'
	@echo '  make clean-dist                  - Remove distribution staging dir and zip'
	@echo ''
	@echo 'Version Management:'
	@echo '  make version-set VERSION=X       - Set API Portal version and update artifacts'
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
	@echo '  make it-open                     - Open Cypress interactive UI (needs the API Portal running locally)'

# Build Targets

.PHONY: build
# ENABLE_COVERAGE=true builds the integration suite's coverage image instead: the
# release image plus instrumented browser scripts and /coverage (Dockerfile.coverage,
# via docker-bake.coverage.hcl), under the same tag. Off by default: an instrumented
# image is for measuring, never for shipping.
ENABLE_COVERAGE ?= false

build: ## Build API Portal Docker image (ENABLE_COVERAGE=true to instrument it)
	@echo "Building API Portal Docker image ($(VERSION), coverage=$(ENABLE_COVERAGE))..."
ifeq ($(ENABLE_COVERAGE),true)
	docker buildx bake -f docker-bake.coverage.hcl \
		--allow=fs.read=.. \
		--allow=fs.read=../../tests/framework/tools \
		--set coverage.tags=$(API_PORTAL_IMAGE):$(VERSION) \
		--load \
		coverage
else
	$(MAKE) --no-print-directory binary
	@rm -rf target/.binary-ctx && mkdir -p target/.binary-ctx
	@unzip -q target/$(BINARY_NAME).zip -d target/.binary-ctx
	docker buildx build -f Dockerfile \
		--tag $(API_PORTAL_IMAGE):$(VERSION) \
		--load \
		target/.binary-ctx
	@rm -rf target/.binary-ctx
endif

.PHONY: build-and-push-multiarch
build-and-push-multiarch: binary ## Build and push multi-arch API Portal image to GHCR
	@echo "Building and pushing multi-arch API Portal Docker image ($(VERSION))..."
	@rm -rf target/.binary-ctx && mkdir -p target/.binary-ctx
	@unzip -q target/$(BINARY_NAME).zip -d target/.binary-ctx
	docker buildx build -f Dockerfile \
		--platform linux/amd64,linux/arm64 \
		--tag $(API_PORTAL_IMAGE):$(VERSION) \
		--push \
		target/.binary-ctx
	@rm -rf target/.binary-ctx

# Binary zip: the API Portal files the image is built from (source, database scripts,
# resources and package files), for customers building their own image, for example
# on a different base image. It holds no Dockerfile; the Dockerfile is handed over
# separately. The images above are built from this zip itself, with this directory's
# Dockerfile: Docker can't build from a .zip, so it's unzipped into target/.binary-ctx
# (giving target/.binary-ctx/api-portal/, the layout the Dockerfile reads) for the
# build and removed afterwards. A published image and the zip of the same
# version therefore always hold the same files.
#
# Copies the files the Dockerfile's COPY lines read: tracked files plus untracked
# ones .gitignore doesn't exclude (so a new, not-yet-added file is built too, while
# local *.db files, node_modules, keys and certificates stay out). The copy is the
# working-tree content, so a version bump made before the build is included. Tests
# (*.test.js) are left out; the image never runs them. pipefail makes a failed git
# or tar step (e.g. a tracked file deleted on disk) fail the build instead of
# producing a partial folder. Binary zips of other versions left in target/ by
# earlier builds are removed first, so target/ only ever holds the current one.
.PHONY: binary
binary: ## Build target/$(BINARY_NAME).zip
	@rm -rf target/.binary-stage
	@rm -f target/wso2apip-api-portal-binary-*.zip
	@mkdir -p target/.binary-stage/api-portal
	@set -o pipefail; git ls-files -z --cached --others --exclude-standard -- $(SOURCE_FILES) ':(exclude)*.test.js' \
		| tar --null -T - -cf - | tar -xf - -C target/.binary-stage/api-portal
	@cd target/.binary-stage && zip -rq ../$(BINARY_NAME).zip api-portal
	@rm -rf target/.binary-stage
	@echo "✓ Built target/$(BINARY_NAME).zip"

# Binary image: the binary zip's files as an image, so registries can store, copy and
# sign them and a Dockerfile can COPY --from them (none of which works with a zip).
# It holds only /api-portal/ (no OS, no entrypoint), so it can't be run. It's built
# for linux/amd64 and linux/arm64 with identical files in both entries, so a plain
# FROM works on any machine. Built from the zip itself, so the image and the zip hold
# the same files. PUSH=true pushes it; otherwise it's only built (a check).
BINARY_IMAGE_REGISTRY ?= $(DOCKER_REGISTRY)
BINARY_IMAGE          := $(BINARY_IMAGE_REGISTRY)/binaries/api-portal:$(VERSION)
PUSH                  ?= false

.PHONY: binary-image
binary-image: binary ## Build $(BINARY_IMAGE) from the binary zip (PUSH=true to push it)
	@rm -rf target/.binary-ctx && mkdir -p target/.binary-ctx
	@unzip -q target/$(BINARY_NAME).zip -d target/.binary-ctx
	printf 'FROM scratch\nCOPY api-portal/ /api-portal/\n' | docker buildx build -f - \
		--platform linux/amd64,linux/arm64 \
		--tag $(BINARY_IMAGE) \
		$(if $(filter true,$(PUSH)),--push,) \
		target/.binary-ctx
	@rm -rf target/.binary-ctx
	@echo "✓ Built $(BINARY_IMAGE)$(if $(filter true,$(PUSH)), and pushed it,; PUSH=true pushes it)"

# Product binary zip: the API Portal product's files in one zip, api-portal/ next to
# platform-api/ (the Platform API binary zip given by PLATFORM_API_BINARY_ZIP, the
# version this release ships with). The API Portal release attaches it. It has the
# same file name as the component zip, so it's written to its own folder,
# target/product/; the component zip in target/ is left as it is.
.PHONY: product-binary
product-binary: binary ## Build target/product/$(BINARY_NAME).zip (API Portal + PLATFORM_API_BINARY_ZIP)
	@if [ -z "$(PLATFORM_API_BINARY_ZIP)" ] || [ ! -f "$(PLATFORM_API_BINARY_ZIP)" ]; then \
		echo "Error: set PLATFORM_API_BINARY_ZIP to the Platform API binary zip to include." >&2; \
		exit 1; \
	fi
	@rm -rf target/.product-stage target/product
	@mkdir -p target/.product-stage target/product
	@unzip -q target/$(BINARY_NAME).zip -d target/.product-stage
	@unzip -qo "$(PLATFORM_API_BINARY_ZIP)" -d target/.product-stage
	@if [ "$$(ls -A target/.product-stage | tr '\n' ' ')" != "api-portal platform-api " ]; then \
		echo "Error: expected only api-portal/ and platform-api/ in the two zips, got: $$(ls -A target/.product-stage | tr '\n' ' ')" >&2; \
		echo "Check that PLATFORM_API_BINARY_ZIP is a Platform API binary zip." >&2; \
		rm -rf target/.product-stage; \
		exit 1; \
	fi
	@cd target/.product-stage && zip -rq ../product/$(BINARY_NAME).zip api-portal platform-api
	@rm -rf target/.product-stage
	@echo "✓ Built target/product/$(BINARY_NAME).zip"

# Version Management Targets

.PHONY: version-set
version-set: ## Set API Portal version and update artifacts
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION required"; \
		echo "Usage: make version-set VERSION=1.0.0"; \
		exit 1; \
	fi
	@echo "$(VERSION)" > VERSION
	@npm version $(VERSION) --no-git-tag-version --allow-same-version >/dev/null
	@(cd ../.. && bash scripts/update-docker-compose.sh api-portal $(VERSION))
	@echo "✓ Set API Portal version to $(VERSION)"

.PHONY: version-bump-patch
version-bump-patch: ## Bump patch version
	@$(MAKE) version-set VERSION=$$(cd ../.. && bash scripts/next-version.sh patch portals/api-portal)

.PHONY: version-bump-minor
version-bump-minor: ## Bump minor version
	@$(MAKE) version-set VERSION=$$(cd ../.. && bash scripts/next-version.sh minor portals/api-portal)

.PHONY: version-bump-major
version-bump-major: ## Bump major version
	@$(MAKE) version-set VERSION=$$(cd ../.. && bash scripts/next-version.sh major portals/api-portal)

.PHONY: version-bump-next-dev
version-bump-next-dev: ## Bump to next minor dev version with SNAPSHOT suffix
	@$(MAKE) version-set VERSION=$$(cd ../.. && bash scripts/next-version.sh next-dev portals/api-portal)

.PHONY: version-get-release
version-get-release: ## Get release version (strips SNAPSHOT suffix)
	@VERSION=$$(cat VERSION 2>/dev/null | tr -d '[:space:]' | sed 's/-SNAPSHOT//'); \
	if [ -z "$$VERSION" ]; then \
		echo "Error: VERSION is empty or contains only whitespace. Check portals/api-portal/VERSION file." >&2; \
		exit 1; \
	fi; \
	echo "$$VERSION"

# Distribution Targets

DIST_VERSION := $(shell echo "$(VERSION)" | sed 's/-SNAPSHOT//')
DIST_NAME    := wso2apip-api-portal-$(DIST_VERSION)
DIST_DIR     := target/$(DIST_NAME)
# Everything the Docker image build reads from its context (see the COPY lines in
# Dockerfile), staged by `make binary`.
SOURCE_FILES := .npmrc package.json package-lock.json src database resources
BINARY_NAME  := wso2apip-api-portal-binary-$(VERSION)
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

# Merge an API Portal config and a Platform API config into one unified
# multi-component config file: $(call merge_config,<dp-toml>,<cp-toml>,<out>).
# The components share no top-level keys ([api_portal.*] vs [platform_api.*],
# and [ai_workspace.*] when append_section adds it below), so the merge is a
# banner-separated concatenation — each service reads only its own root table and
# ignores the other sections and their tokens.
define merge_config
	@printf '%s\n' \
		'# =============================================================================' \
		'# UNIFIED API PLATFORM CONFIGURATION' \
		'# =============================================================================' \
		'# One file, multiple components. Each service reads ONLY its own root table' \
		'# (e.g. [api_portal], [platform_api]) and ignores the other sections and' \
		'# their tokens. GENERATED by `make dist` from the per-component source configs.' \
		'' '' \
		'# #############################################################################' \
		'# #  API PORTAL' \
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
dist: clean-dist ## Build standalone API Portal distribution zip
	@if [ -z "$(strip $(AIW_VERSION))" ]; then \
		echo "Error: AIW_VERSION is empty — the optional ai-workspace image tag cannot be resolved." >&2; \
		echo "Pass it explicitly (make dist AIW_VERSION=1.0.0) or ensure ../ai-workspace/VERSION exists." >&2; \
		exit 1; \
	fi
	@echo "Building distribution $(DIST_NAME)..."
	@echo "  platform-api image pinned to $(PLATFORM_API_IMAGE):$(PLATFORM_API_VERSION)"
	@echo "  optional ai-workspace image pinned to $(AIW_IMAGE):$(AIW_VERSION)"
	@mkdir -p $(DIST_DIR)/configs
	@mkdir -p $(DIST_DIR)/resources/api-portal/db-scripts
	@cp -R database/* $(DIST_DIR)/resources/api-portal/db-scripts
# Both components ship a role-to-scope-mapping.yaml, so each goes under its own
# resources/<component>/ dir (matching db-scripts) rather than colliding on one
# resources/role-to-scope-mapping.yaml. The in-container mount paths are unchanged.
	@cp resources/role-to-scope-mapping.yaml $(DIST_DIR)/resources/api-portal/role-to-scope-mapping.yaml
	@mkdir -p $(DIST_DIR)/resources/platform-api/db-scripts
	@mkdir -p $(DIST_DIR)/resources/samples
	@cp -R samples/apis $(DIST_DIR)/resources/samples/
	@cp -R samples/mcps $(DIST_DIR)/resources/samples/
# subscription-plans.yaml and applications.yaml complete the design-mode sample
# set (seed-samples.sh only needs apis/ + mcps/, but design mode reads these too).
	@cp samples/subscription-plans.yaml $(DIST_DIR)/resources/samples/
	@cp samples/applications.yaml $(DIST_DIR)/resources/samples/
# Stage platform-api configs + db-scripts (from tag or working tree) for the merge below.
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
	@git -C ../.. show "$(PLATFORM_API_TAG):platform-api/resources/role-to-scope-mapping.yaml" \
		> $(DIST_DIR)/resources/platform-api/role-to-scope-mapping.yaml
else
	@cp ../../platform-api/internal/database/schema.*.sql $(DIST_DIR)/resources/platform-api/db-scripts/
	@cp ../../platform-api/config/config.toml $(DIST_DIR)/configs/.pa-config.toml
	@cp ../../platform-api/config/config-template.toml $(DIST_DIR)/configs/.pa-config-template.toml
	@cp ../../platform-api/resources/role-to-scope-mapping.yaml $(DIST_DIR)/resources/platform-api/role-to-scope-mapping.yaml
endif
# Require a [platform_api] root table — pre-unified configs would merge into a broken file.
	@if ! grep -q '^\[platform_api' $(DIST_DIR)/configs/.pa-config.toml; then \
		echo "Error: the platform-api config for $(PLATFORM_API_VERSION) has no [platform_api] root table." >&2; \
		echo "That version predates the unified multi-component config and cannot be bundled — pick a newer PLATFORM_API_VERSION." >&2; \
		exit 1; \
	fi
# Merge both components into one config.toml — each service reads only its own root table.
	$(call merge_config,configs/config.toml,$(DIST_DIR)/configs/.pa-config.toml,$(DIST_DIR)/configs/config.toml)
	$(call merge_config,configs/config-template.toml,$(DIST_DIR)/configs/.pa-config-template.toml,$(DIST_DIR)/configs/config-template.toml)
# AI Workspace is opt-in: append its template to config-template.toml only, not the active config.
ifeq ($(AIW_FROM_TAG),true)
	$(call stage_optional_template,$(AIW_TAG),portals/ai-workspace/configs/config-template.toml,../ai-workspace/configs/config-template.toml,$(DIST_DIR)/configs/.aiw-config-template.toml)
else
	@cp ../ai-workspace/configs/config-template.toml $(DIST_DIR)/configs/.aiw-config-template.toml
endif
	$(call append_section,AI WORKSPACE (optional),$(DIST_DIR)/configs/.aiw-config-template.toml,$(DIST_DIR)/configs/config-template.toml)
	@rm -f $(DIST_DIR)/configs/.pa-config.toml $(DIST_DIR)/configs/.pa-config-template.toml $(DIST_DIR)/configs/.aiw-config-template.toml
# Point the platform-api mount at the merged config so both containers share one file.
	@sed -e 's#\.\./\.\./platform-api/config/config\.toml:#./configs/config.toml:#' \
	     -e 's#\.\./\.\./platform-api/resources/role-to-scope-mapping\.yaml:#./resources/platform-api/role-to-scope-mapping.yaml:#' \
	     -e 's#\./resources/role-to-scope-mapping\.yaml:#./resources/api-portal/role-to-scope-mapping.yaml:#' \
	     -e 's#\./samples:/app/samples:#./resources/samples:/app/samples:#' \
		docker-compose.yaml > $(DIST_DIR)/docker-compose.yaml
	@cp distribution/README.md $(DIST_DIR)/README.md
	@mkdir -p $(DIST_DIR)/scripts
	@cp ../scripts/setup.sh $(DIST_DIR)/scripts/setup.sh
	@chmod +x $(DIST_DIR)/scripts/setup.sh
	@cp scripts/seed-samples.sh $(DIST_DIR)/scripts/seed-samples.sh
	@chmod +x $(DIST_DIR)/scripts/seed-samples.sh
# Windows counterpart of seed-samples.sh — same behavior, same generated output.
	@cp scripts/seed-samples.ps1 $(DIST_DIR)/scripts/seed-samples.ps1
# Bake this pack's default COMPOSE_PROFILES into setup.sh so `docker compose
# up -d` (no --profile flag) starts API Portal + Platform API out of
# the box once setup.sh has run. setup.sh writes it to .env at setup time —
# .env is not shipped in the zip itself, only this pack's default value for it.
	@sed -i.bak \
	    -e 's|^DEFAULT_COMPOSE_PROFILES=.*|DEFAULT_COMPOSE_PROFILES="api-portal,platform-api"|' \
	    -e 's|^DEFAULT_PACK_NAME=.*|DEFAULT_PACK_NAME="api-portal"|' \
	    -e 's|^DEFAULT_DIST_VERSION=.*|DEFAULT_DIST_VERSION="$(DIST_VERSION)"|' \
	    $(DIST_DIR)/scripts/setup.sh
	@rm -f $(DIST_DIR)/scripts/setup.sh.bak
# Windows counterpart of setup.sh — same flags, same generated files. The same
# three values are baked in the same way (the PowerShell assignment spells them
# `$VAR = '...'`).
	@cp ../scripts/setup.ps1 $(DIST_DIR)/scripts/setup.ps1
	@sed -i.bak \
	    -e "s|^\$$DEFAULT_COMPOSE_PROFILES = .*|\$$DEFAULT_COMPOSE_PROFILES = 'api-portal,platform-api'|" \
	    -e "s|^\$$DEFAULT_PACK_NAME = .*|\$$DEFAULT_PACK_NAME = 'api-portal'|" \
	    -e "s|^\$$DEFAULT_DIST_VERSION = .*|\$$DEFAULT_DIST_VERSION = '$(DIST_VERSION)'|" \
	    $(DIST_DIR)/scripts/setup.ps1
	@rm -f $(DIST_DIR)/scripts/setup.ps1.bak
	@sed -i.bak \
	    -e 's|image: .*/api-portal:[^[:space:]]*|image: $(API_PORTAL_IMAGE):$(DIST_VERSION)|g' \
	    -e 's|image: .*/platform-api:[^[:space:]]*|image: $(PLATFORM_API_IMAGE):$(PLATFORM_API_VERSION)|g' \
	    -e 's|image: .*/ai-workspace:[^[:space:]]*|image: $(AIW_IMAGE):$(AIW_VERSION)|g' \
	    $(DIST_DIR)/docker-compose.yaml
	@rm -f $(DIST_DIR)/docker-compose.yaml.bak
	@cd target && zip -rq $(DIST_NAME).zip $(DIST_NAME)
	@rm -rf $(DIST_DIR)
	@echo "✓ Built $(DIST_ZIP)"

.PHONY: clean-dist
clean-dist: ## Remove distribution staging dir and zip (binary and product zips are left alone; their targets replace their own)
	@find -H target -maxdepth 1 -name 'wso2apip-api-portal-*' ! -name 'wso2apip-api-portal-binary-*' -exec rm -rf {} + 2>/dev/null || true

# Clean Targets
# `clean` also removes the binary zip and the folders the binary, image and product
# zip targets work in, including any a failed build left behind.

.PHONY: clean
clean: clean-dist ## Clean all build artifacts
	@rm -rf target/.binary-stage target/.binary-ctx target/.product-stage target/product
	@rm -f target/wso2apip-api-portal-binary-*.zip
	@echo "Cleaned API Portal build artifacts"

# Docs Targets

.PHONY: generate-apidocs
generate-apidocs: ## Generate REST API docs from the API Portal OpenAPI spec
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

it-open: ## Open Cypress interactive UI against a locally running API Portal
	$(MAKE) -C it open DOCKER_REGISTRY=$(DOCKER_REGISTRY)
