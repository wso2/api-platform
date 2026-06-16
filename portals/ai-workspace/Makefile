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

VERSION ?= $(shell cat VERSION 2>/dev/null || echo "0.0.0-SNAPSHOT")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
IMAGE_NAME := $(DOCKER_REGISTRY)/ai-workspace

.PHONY: help run build push build-and-push-multiarch version-set version-bump-patch version-bump-minor version-bump-major version-bump-next-dev version-get-release e2e-deps e2e-test e2e-open

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

e2e-deps: ## Install Cypress dependencies for local E2E runs
	npm install

e2e-test: ## Run Cypress headlessly against a locally running docker compose stack
	CYPRESS_BASE_URL=https://host.docker.internal:5380 docker run --rm -v "$$(pwd):/e2e" -w /e2e -e CYPRESS_BASE_URL cypress/included:13.17.0 cypress run --headless --browser electron --config-file cypress.config.js

e2e-open: ## Open Cypress UI against a locally running docker compose stack
	CYPRESS_BASE_URL=https://localhost:5380 npx cypress open --e2e --config-file cypress.config.js

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

.PHONY: dist clean-dist
dist: clean-dist ## Build standalone AI Workspace + Platform API distribution zip
	@echo "Building distribution $(DIST_NAME)..."
	@mkdir -p $(DIST_DIR)/configs $(DIST_DIR)/resources/certificates
	@cp -R configs/. $(DIST_DIR)/configs/
	@cp ../../platform-api/src/resources/roles.yaml $(DIST_DIR)/resources/roles.yaml
	@cp docker-compose.yaml $(DIST_DIR)/docker-compose.yaml
	@sed -i.bak \
	    -e 's|/ai-workspace:[^[:space:]]*|/ai-workspace:$(DIST_VERSION)|g' \
	    -e 's|/platform-api:[^[:space:]]*|/platform-api:$(PLATFORM_API_VERSION)|g' \
	    $(DIST_DIR)/docker-compose.yaml
	@rm -f $(DIST_DIR)/docker-compose.yaml.bak
	@cd target && zip -rq $(DIST_NAME).zip $(DIST_NAME)
	@echo "✓ Built $(DIST_ZIP)"

.PHONY: clean-dist
clean-dist: ## Remove distribution staging dir and zip
	@rm -rf target/wso2apip-ai-workspace-*
