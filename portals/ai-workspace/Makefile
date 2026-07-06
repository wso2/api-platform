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
BFF_DEV_ADDR   ?= :8081
PLATFORM_API_URL ?= https://localhost:9243

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

bff-build: ## Build the BFF binary (stdlib-only Go module)
	cd $(BFF_DIR) && GOWORK=off CGO_ENABLED=0 go build -o ../target/ai-workspace-bff .

bff-run: ## Run the BFF locally (proxies to PLATFORM_API_URL; pair with `make run`)
	cd $(BFF_DIR) && \
		PLATFORM_API_URL=$(PLATFORM_API_URL) \
		PLATFORM_API_TLS_SKIP_VERIFY=true \
		BFF_ADDR=$(BFF_DEV_ADDR) \
		STATIC_DIR=../dist \
		go run .

bff-test: ## Run BFF unit tests
	cd $(BFF_DIR) && GOWORK=off go test ./...

bff-tidy: ## Tidy the BFF module
	cd $(BFF_DIR) && go mod tidy

e2e-deps: ## Install Cypress dependencies for local E2E runs
	npm install

e2e-test: ## Run Cypress headlessly against a locally running docker compose stack
	CYPRESS_BASE_URL=https://host.docker.internal:5380 docker run --rm --add-host=host.docker.internal:host-gateway -v "$$(pwd):/e2e" -w /e2e -e CYPRESS_BASE_URL cypress/included:13.17.0 cypress run --headless --browser electron --config-file cypress.config.js

e2e-open: ## Open Cypress UI against a locally running docker compose stack
	CYPRESS_BASE_URL=https://localhost:5380 npx cypress open --e2e --config-file cypress.config.js

e2e-ci: ## Run AI Workspace E2E against a local quickstart stack
	@set -euo pipefail; \
	backup_file="$$(mktemp)"; \
	cp docker-compose.yaml "$$backup_file"; \
	cleanup() { \
		status=$$?; \
		if [ $$status -ne 0 ]; then \
			docker compose logs --no-color || true; \
		fi; \
		docker compose down -v >/dev/null 2>&1 || true; \
		mv "$$backup_file" docker-compose.yaml; \
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
	docker compose up -d --wait; \
	$(MAKE) e2e-test

build: ## Build Docker image
	@echo "Building Docker image ($(IMAGE_NAME):$(VERSION))..."
	docker buildx build \
		--build-arg VERSION=$(VERSION) \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		--load \
		.

build-and-push-multiarch: ## Build and push multi-architecture Docker image (linux/amd64, linux/arm64)
	@echo "Building and pushing multi-arch ai-workspace Docker image: $(IMAGE_NAME):$(VERSION)"
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
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
PLATFORM_API_VERSION ?= $(shell cat ../../platform-api/VERSION 2>/dev/null | sed 's/-SNAPSHOT//' || echo "0.0.0")
PLATFORM_API_TAG := platform-api/v$(PLATFORM_API_VERSION)

# When PLATFORM_API_VERSION is supplied explicitly (e.g. `make dist PLATFORM_API_VERSION=0.10.0`),
# the platform-api db-scripts are fetched from the released git tag $(PLATFORM_API_TAG) so the
# distribution is reproducible for that exact release. Otherwise they are taken from the working tree.
ifeq ($(origin PLATFORM_API_VERSION),command line)
PLATFORM_API_FROM_TAG := true
endif
ifeq ($(origin PLATFORM_API_VERSION),environment)
PLATFORM_API_FROM_TAG := true
endif

.PHONY: dist clean-dist
dist: clean-dist ## Build standalone AI Workspace + Platform API distribution zip
	@echo "Building distribution $(DIST_NAME)..."
	@mkdir -p $(DIST_DIR)/configs $(DIST_DIR)/resources/certificates $(DIST_DIR)/resources/platform-api/db-scripts
	@cp -R configs/. $(DIST_DIR)/configs/
	@cp ../../platform-api/resources/roles.yaml $(DIST_DIR)/resources/roles.yaml
ifeq ($(PLATFORM_API_FROM_TAG),true)
	@echo "Fetching platform-api db-scripts from tag $(PLATFORM_API_TAG)..."
	@tag="$(PLATFORM_API_TAG)"; dest="$(DIST_DIR)/resources/platform-api/db-scripts"; \
	if ! git -C ../.. rev-parse -q --verify "refs/tags/$$tag" >/dev/null 2>&1; then \
		git -C ../.. fetch --tags --quiet || true; \
	fi; \
	if ! git -C ../.. rev-parse -q --verify "refs/tags/$$tag" >/dev/null 2>&1; then \
		echo "Error: tag $$tag not found (check PLATFORM_API_VERSION)" >&2; exit 1; \
	fi; \
	files=$$(git -C ../.. ls-tree -r --name-only "$$tag" -- platform-api/internal/database | grep '\.sql$$'); \
	if [ -z "$$files" ]; then echo "Error: no .sql files found under platform-api/internal/database at $$tag" >&2; exit 1; fi; \
	for f in $$files; do git -C ../.. show "$$tag:$$f" > "$$dest/$$(basename $$f)"; done; \
	echo "✓ Fetched $$(echo "$$files" | wc -l | tr -d ' ') db-script(s) from $$tag"
else
	@cp ../../platform-api/internal/database/*.sql $(DIST_DIR)/resources/platform-api/db-scripts/
endif
	@cp docker-compose.yaml $(DIST_DIR)/docker-compose.yaml
	@sed -i.bak -E \
	    -e 's|^([[:space:]]*image:[[:space:]]*.*/ai-workspace):[^[:space:]]*|\1:$(DIST_VERSION)|' \
	    -e 's|^([[:space:]]*image:[[:space:]]*.*/platform-api):[^[:space:]]*|\1:$(PLATFORM_API_VERSION)|' \
	    $(DIST_DIR)/docker-compose.yaml
	@rm -f $(DIST_DIR)/docker-compose.yaml.bak
	@cd target && zip -rq $(DIST_NAME).zip $(DIST_NAME)
	@echo "✓ Built $(DIST_ZIP)"

.PHONY: clean-dist
clean-dist: ## Remove distribution staging dir and zip
	@rm -rf target/wso2apip-ai-workspace-*
