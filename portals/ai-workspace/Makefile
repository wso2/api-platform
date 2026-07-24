# --------------------------------------------------------------------
#
# WSO2 LLC. licenses this file to you under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
# --------------------------------------------------------------------

# Makefile for AI Workspace

SHELL := /bin/bash

VERSION ?= $(shell cat VERSION 2>/dev/null || echo "0.0.0-SNAPSHOT")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
IMAGE_NAME := $(DOCKER_REGISTRY)/ai-workspace

.PHONY: help run build push build-and-push-multiarch version-set version-bump-patch version-bump-minor version-bump-major version-bump-next-dev version-get-release e2e-deps e2e-test e2e-open e2e-ci bff-build bff-run bff-test bff-tidy

# BFF (Backend-for-Frontend) — Go server that serves the SPA, proxies all
# browser→backend traffic, and owns authentication.
BFF_DIR        := bff
BFF_DEV_PORT   ?= 8081
CONTROL_PLANE_URL ?= https://localhost:9243

help: ## Show this help message
	@echo 'AI Workspace Build System (Version: $(VERSION))'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-25s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

run: ## Start AI Workspace dev server locally (installs deps if needed)
	@if [ ! -d node_modules ]; then \
		echo "node_modules not found — running npm install..."; \
		npm install; \
	fi
	npm run dev

bff-build: ## Build the BFF binary
	cd $(BFF_DIR) && GOWORK=off CGO_ENABLED=0 go build -o ../target/ai-workspace-bff .

# Runs the deployed config — configs/config.toml, the same file the container mounts —
# with the container-shaped defaults pointed at this machine. Each variable below is
# read by that key's '{{ env "APIP_AIW_..." }}' token in the file; the token is the only
# thing that lets an environment variable reach a key, so a key absent from the file
# cannot be set here. To test against an IDP, add APIP_AIW_AUTH_MODE=oidc and the
# APIP_AIW_OIDC_* variables the [oidc] tokens name — see README.md.
bff-run: ## Run the BFF locally (proxies to CONTROL_PLANE_URL; pair with `make run`)
	./setup.sh --certs-only
	cd $(BFF_DIR) && \
		APIP_AIW_CONTROL_PLANE_URL=$(CONTROL_PLANE_URL) \
		APIP_AIW_CONTROL_PLANE_TLS_SKIP_VERIFY=true \
		APIP_AIW_SERVER_HTTPS_CERT_FILE=../resources/certificates/cert.pem \
		APIP_AIW_SERVER_HTTPS_KEY_FILE=../resources/certificates/key.pem \
		APIP_AIW_SERVER_HTTPS_PORT=$(BFF_DEV_PORT) \
		APIP_AIW_LOGGING_LEVEL=debug \
		go run . -config ../configs/config.toml -static-dir ../dist

bff-test: ## Run BFF unit tests
	cd $(BFF_DIR) && GOWORK=off go test ./...

bff-tidy: ## Tidy the BFF module
	cd $(BFF_DIR) && go mod tidy

e2e-deps: ## Install Cypress dependencies for local E2E runs
	npm install

e2e-test: ## Run Cypress headlessly against a locally running docker compose stack
	CYPRESS_BASE_URL=https://host.docker.internal:9643 docker run --rm --add-host=host.docker.internal:host-gateway -v "$$(pwd):/e2e" -w /e2e -e CYPRESS_BASE_URL cypress/included:13.17.0 cypress run --headless --browser electron --config-file cypress.config.js

e2e-open: ## Open Cypress UI against a locally running docker compose stack
	CYPRESS_BASE_URL=https://localhost:9643 npx cypress open --e2e --config-file cypress.config.js

e2e-ci: ## Run AI Workspace E2E against a local quickstart stack
	@set -euo pipefail; \
	backup_file="$$(mktemp)"; \
	cp docker-compose.yaml "$$backup_file"; \
	env_backup_file=""; \
	if [ -f api-platform.env ]; then \
		env_backup_file="$$(mktemp)"; \
		cp api-platform.env "$$env_backup_file"; \
	fi; \
	cleanup() { \
		status=$$?; \
		if [ $$status -ne 0 ]; then \
			docker compose logs --no-color || true; \
		fi; \
		docker compose down -v >/dev/null 2>&1 || true; \
		mv "$$backup_file" docker-compose.yaml; \
		if [ -n "$$env_backup_file" ]; then \
			mv "$$env_backup_file" api-platform.env; \
		else \
			rm -f api-platform.env; \
		fi; \
		exit $$status; \
	}; \
	trap cleanup EXIT; \
	trap 'exit 130' INT; \
	trap 'exit 143' TERM; \
	npm ci; \
	if [ "$(PLATFORM_API_FROM_TAG)" = "true" ]; then \
		platform_api_version="$(PLATFORM_API_VERSION)"; \
		echo "Using released platform-api image $(DOCKER_REGISTRY)/platform-api:$${platform_api_version} (skipping build)"; \
	else \
		platform_api_version="$$(tr -d '[:space:]' < ../../platform-api/VERSION)"; \
		echo "Building platform-api:$${platform_api_version} from source"; \
		$(MAKE) -C ../../platform-api build VERSION="$${platform_api_version}"; \
	fi; \
	echo "Pinning ai-workspace compose to platform-api:$${platform_api_version}"; \
	sed -i.bak -E "s#image: .*/platform-api:.*#image: $(DOCKER_REGISTRY)/platform-api:$${platform_api_version}#" docker-compose.yaml; \
	rm -f docker-compose.yaml.bak; \
	ADMIN_USERNAME=admin ADMIN_PASSWORD=admin ./setup.sh --force; \
	docker compose up -d --wait --wait-timeout 300; \
	$(MAKE) e2e-test

build: ## Build Docker image
	@echo "Building Docker image ($(IMAGE_NAME):$(VERSION))..."
	docker buildx build \
		--build-context common=../../common \
		--build-context httpkit=../../httpkit \
		--build-arg VERSION=$(VERSION) \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		--load \
		.

build-and-push-multiarch: ## Build and push multi-architecture Docker image (linux/amd64, linux/arm64)
	@echo "Building and pushing multi-arch ai-workspace Docker image: $(IMAGE_NAME):$(VERSION)"
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-context common=../../common \
		--build-context httpkit=../../httpkit \
		--build-arg VERSION=$(VERSION) \
		-t $(IMAGE_NAME):$(VERSION) \
		--push \
		.

version-set: ## Set ai-workspace version and update artifacts
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION required"; \
		echo "Usage: make version-set VERSION=1.0.0"; \
		exit 1; \
	fi
	@echo "$(VERSION)" > VERSION
	@(cd ../.. && bash scripts/update-docker-compose.sh ai-workspace $(VERSION))
	@echo "✓ Set ai-workspace version to $(VERSION)"

version-bump-patch: ## Bump patch version
	@$(MAKE) version-set VERSION=$$(cd ../.. && bash scripts/next-version.sh patch portals/ai-workspace)

version-bump-minor: ## Bump minor version
	@$(MAKE) version-set VERSION=$$(cd ../.. && bash scripts/next-version.sh minor portals/ai-workspace)

version-bump-major: ## Bump major version
	@$(MAKE) version-set VERSION=$$(cd ../.. && bash scripts/next-version.sh major portals/ai-workspace)

version-bump-next-dev: ## Bump to next minor dev version with SNAPSHOT suffix
	@$(MAKE) version-set VERSION=$$(cd ../.. && bash scripts/next-version.sh next-dev portals/ai-workspace)

version-get-release: ## Get release version (strips SNAPSHOT suffix)
	@VERSION=$$(cat VERSION 2>/dev/null | tr -d '[:space:]' | sed 's/-SNAPSHOT//'); \
	if [ -z "$$VERSION" ]; then \
		echo "Error: VERSION is empty or contains only whitespace. Check portals/ai-workspace/VERSION file." >&2; \
		exit 1; \
	fi; \
	echo "$$VERSION"

# Distribution Targets
DIST_VERSION  := $(shell echo "$(VERSION)" | sed 's/-SNAPSHOT//')
DIST_NAME     := wso2apip-ai-workspace-$(DIST_VERSION)
DIST_DIR      := target/$(DIST_NAME)
DIST_ZIP      := target/$(DIST_NAME).zip
# The fallback is applied to the captured value rather than with `|| echo`: in a
# `sed … < file` pipeline the exit status is sed's, and sed succeeds on empty
# input, so a missing or empty VERSION file would otherwise yield an empty string
# — which would also propagate into PLATFORM_API_TAG, producing the malformed git
# tag "platform-api/v".
PLATFORM_API_VERSION ?= $(shell v=$$(sed 's/-SNAPSHOT//' ../../platform-api/VERSION 2>/dev/null); echo "$${v:-0.0.0}")
PLATFORM_API_TAG := platform-api/v$(PLATFORM_API_VERSION)
# Version used to pin the optional developer-portal image (the profile-gated
# service in docker-compose.yaml). Its config is not merged into the unified
# config.toml — the profile is opt-in and self-served (see README) — so only the
# image tag is pinned here for a reproducible release.
#
# Supplied explicitly when releasing (`make dist DEVPORTAL_VERSION=1.0.0`, driven
# by the release workflow's developer_portal_version input) so the zip pins a
# *released* developer-portal image rather than whatever the working tree happens
# to carry. For local builds it falls back to the sibling VERSION file. There is
# deliberately no hardcoded version fallback — `dist` fails below if this resolves
# to empty, so a release can never silently ship a placeholder tag.
DEVPORTAL_VERSION ?= $(shell sed 's/-SNAPSHOT//' ../developer-portal/VERSION 2>/dev/null | tr -d '[:space:]')
DEVPORTAL_TAG     := developer-portal/v$(DEVPORTAL_VERSION)

# As with PLATFORM_API_VERSION: an explicitly supplied version means "bundle that
# release", so the [developer_portal] template appended to config-template.toml is
# read from the tag rather than the working tree — otherwise the zip would document
# settings from unreleased branch state against a released image.
ifeq ($(origin DEVPORTAL_VERSION),command line)
DEVPORTAL_FROM_TAG := true
endif
ifeq ($(origin DEVPORTAL_VERSION),environment)
DEVPORTAL_FROM_TAG := true
endif

# When PLATFORM_API_VERSION is supplied explicitly (e.g. `make dist PLATFORM_API_VERSION=0.10.0`),
# the platform-api db-scripts are fetched from the released git tag $(PLATFORM_API_TAG) so the
# distribution is reproducible for that exact release. Otherwise they are taken from the working tree.
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

# Merge an AI Workspace config and a Platform API config into one unified
# multi-component config file: $(call merge_config,<aiw-toml>,<cp-toml>,<out>).
# The components share no top-level keys ([ai_workspace.*] vs [platform_api.*],
# and [developer_portal.*] when append_section adds it below), so the merge is a
# banner-separated concatenation — each service reads only its own root table and
# ignores the other sections and their tokens.
define merge_config
	@printf '%s\n' \
		'# =============================================================================' \
		'# UNIFIED API PLATFORM CONFIGURATION' \
		'# =============================================================================' \
		'# One file, multiple components. Each service reads ONLY its own root table' \
		'# (e.g. [ai_workspace], [platform_api]) and ignores the other sections and' \
		'# their tokens. GENERATED by `make dist` from the per-component source configs.' \
		'' '' \
		'# #############################################################################' \
		'# #  AI WORKSPACE' \
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

.PHONY: dist clean-dist
dist: clean-dist ## Build standalone AI Workspace + Platform API distribution zip
	@if [ -z "$(strip $(DEVPORTAL_VERSION))" ]; then \
		echo "Error: DEVPORTAL_VERSION is empty — the optional developer-portal image tag cannot be resolved." >&2; \
		echo "Pass it explicitly (make dist DEVPORTAL_VERSION=1.0.0) or ensure ../developer-portal/VERSION exists." >&2; \
		exit 1; \
	fi
	@echo "Building distribution $(DIST_NAME)..."
	@echo "  optional developer-portal image pinned to $(DOCKER_REGISTRY)/developer-portal:$(DEVPORTAL_VERSION)"
	@mkdir -p $(DIST_DIR)/configs $(DIST_DIR)/scripts $(DIST_DIR)/resources/certificates $(DIST_DIR)/resources/platform-api/db-scripts
# Stage platform-api configs + db-scripts (from tag or working tree) for the merge below.
ifeq ($(PLATFORM_API_FROM_TAG),true)
	@echo "Fetching platform-api db-scripts from tag $(PLATFORM_API_TAG)..."
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
	@git -C ../.. show "$(PLATFORM_API_TAG):platform-api/config/config-template.toml" \
		> $(DIST_DIR)/configs/.pa-config-template.toml
	@git -C ../.. show "$(PLATFORM_API_TAG):platform-api/config/config.toml" \
		> $(DIST_DIR)/configs/.pa-config.toml
	@git -C ../.. show "$(PLATFORM_API_TAG):platform-api/resources/roles.yaml" \
		> $(DIST_DIR)/resources/roles.yaml
else
	@cp ../../platform-api/internal/database/schema.*.sql $(DIST_DIR)/resources/platform-api/db-scripts/
	@cp ../../platform-api/config/config-template.toml $(DIST_DIR)/configs/.pa-config-template.toml
	@cp ../../platform-api/config/config.toml $(DIST_DIR)/configs/.pa-config.toml
	@cp ../../platform-api/resources/roles.yaml $(DIST_DIR)/resources/roles.yaml
endif
# Require a [platform_api] root table — pre-unified configs would merge into a broken file.
	@if ! grep -q '^\[platform_api' $(DIST_DIR)/configs/.pa-config.toml; then \
		echo "Error: the platform-api config for $(PLATFORM_API_VERSION) has no [platform_api] root table." >&2; \
		echo "That version predates the unified multi-component config and cannot be bundled — pick a newer PLATFORM_API_VERSION." >&2; \
		exit 1; \
	fi
# Merge ai-workspace + platform-api into one config.toml — each service reads only its own root table.
	$(call merge_config,configs/config.toml,$(DIST_DIR)/configs/.pa-config.toml,$(DIST_DIR)/configs/config.toml)
	$(call merge_config,configs/config-template.toml,$(DIST_DIR)/configs/.pa-config-template.toml,$(DIST_DIR)/configs/config-template.toml)
# Developer Portal is opt-in: append its template to config-template.toml only, not the active config.
ifeq ($(DEVPORTAL_FROM_TAG),true)
	$(call stage_optional_template,$(DEVPORTAL_TAG),portals/developer-portal/configs/config-template.toml,../developer-portal/configs/config-template.toml,$(DIST_DIR)/configs/.dp-config-template.toml)
else
	@cp ../developer-portal/configs/config-template.toml $(DIST_DIR)/configs/.dp-config-template.toml
endif
	$(call append_section,DEVELOPER PORTAL (optional),$(DIST_DIR)/configs/.dp-config-template.toml,$(DIST_DIR)/configs/config-template.toml)
	@rm -f $(DIST_DIR)/configs/.pa-config.toml $(DIST_DIR)/configs/.pa-config-template.toml $(DIST_DIR)/configs/.dp-config-template.toml
	@printf '%s\n' \
		'# Generated secrets — never commit these.' \
		'*.env' \
		'resources/certificates/*' \
		> $(DIST_DIR)/.gitignore
	@cp setup.sh $(DIST_DIR)/scripts/setup.sh
	@chmod +x $(DIST_DIR)/scripts/setup.sh
# Point the platform-api mount at the merged config so both containers share one file.
	@sed 's#\.\./\.\./platform-api/config/config\.toml:#./configs/config.toml:#' \
		docker-compose.yaml > $(DIST_DIR)/docker-compose.yaml
	@cp distribution/README.md $(DIST_DIR)/README.md
	@sed -i.bak -E \
	    -e 's|^([[:space:]]*image:[[:space:]]*.*/ai-workspace):[^[:space:]]*|\1:$(DIST_VERSION)|' \
	    -e 's|^([[:space:]]*image:[[:space:]]*.*/platform-api):[^[:space:]]*|\1:$(PLATFORM_API_VERSION)|' \
	    -e 's|^([[:space:]]*image:[[:space:]]*.*/developer-portal):[^[:space:]]*|\1:$(DEVPORTAL_VERSION)|' \
	    $(DIST_DIR)/docker-compose.yaml
	@rm -f $(DIST_DIR)/docker-compose.yaml.bak
	@cd target && zip -rq $(DIST_NAME).zip $(DIST_NAME)
	@echo "✓ Built $(DIST_ZIP)"

.PHONY: clean-dist
clean-dist: ## Remove distribution staging dir and zip
	@rm -rf target/wso2apip-ai-workspace-*
