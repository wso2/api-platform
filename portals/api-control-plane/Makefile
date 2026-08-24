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

# Makefile for API Control Plane

SHELL := /bin/bash

VERSION ?= $(shell cat VERSION 2>/dev/null || echo "0.0.0-SNAPSHOT")

DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
IMAGE_NAME := $(DOCKER_REGISTRY)/api-control-plane

.PHONY: help run build bff-build bff-run bff-test bff-tidy test build-and-push-multiarch version-set version-bump-patch version-bump-minor version-bump-major version-bump-next-dev version-get-release

# BFF (Backend-for-Frontend) — Go server that serves the SPA, proxies all
# browser->backend traffic, and owns authentication. Local dev runs it over
# plain HTTP (no cert generation set up for this portal yet); `make run`'s
# Vite dev server proxies same-origin BFF paths to it (see vite.config.ts).
BFF_DIR           := bff
BFF_DEV_PORT      ?= 8082
CONTROL_PLANE_URL ?= https://localhost:9243
# The default CONTROL_PLANE_URL above points at platform-api's local dev
# listener, which uses a self-signed certificate (see
# platform-api/scripts/setup-local-dev.sh) — no real CA can validate it, so
# this defaults to true for that specific case. It is an explicit,
# overridable opt-out, not forced: point CONTROL_PLANE_URL at a real
# deployment and pass CONTROL_PLANE_TLS_SKIP_VERIFY=false alongside it.
CONTROL_PLANE_TLS_SKIP_VERIFY ?= true

help: ## Show this help message
	@echo 'API Control Plane Build System (Version: $(VERSION))'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-25s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

run: ## Start the console dev server locally (installs deps if needed; pair with `make bff-run`)
	@if [ ! -d node_modules ]; then \
		echo "node_modules not found — running npm install..."; \
		npm install; \
	fi
	npm run dev

# Runs against configs/config.toml — the same file the container mounts, with
# container-shaped defaults pointed at this machine instead. Each variable
# below is read by that key's '{{ env "APIP_ACP_..." }}' token in the file;
# the token is the only thing that lets an environment variable reach a key,
# so a key absent from the file cannot be set here. To test against an
# external IdP, add APIP_ACP_AUTH_MODE=oidc and the matching
# APIP_ACP_AUTH_OIDC_* variables the [auth.oidc] tokens name.
bff-run: ## Run the BFF locally (proxies to CONTROL_PLANE_URL; pair with `make run`)
	cd $(BFF_DIR) && \
		APIP_ACP_CONTROL_PLANE_URL=$(CONTROL_PLANE_URL) \
		APIP_ACP_CONTROL_PLANE_TLS_SKIP_VERIFY=$(CONTROL_PLANE_TLS_SKIP_VERIFY) \
		APIP_ACP_SERVER_HTTP_PORT=$(BFF_DEV_PORT) \
		APIP_ACP_LOGGING_LEVEL=debug \
		GOWORK=off go run . -config ../configs/config.toml

bff-build: ## Build the BFF binary
	cd $(BFF_DIR) && GOWORK=off CGO_ENABLED=0 go build -o ../target/api-control-plane-bff .

bff-test: ## Run BFF unit tests
	cd $(BFF_DIR) && GOWORK=off go test ./...

bff-tidy: ## Tidy the BFF module
	cd $(BFF_DIR) && GOWORK=off go mod tidy

test: ## Run frontend and BFF tests
	npm test
	$(MAKE) bff-test

build: ## Build the production Docker image
	docker build -t $(IMAGE_NAME):$(VERSION) -t api-control-plane:$(VERSION) .

build-and-push-multiarch: ## Build and push multi-architecture Docker image (linux/amd64, linux/arm64)
	@echo "Building and pushing multi-arch api-control-plane Docker image: $(IMAGE_NAME):$(VERSION)"
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--build-arg VERSION=$(VERSION) \
		-t $(IMAGE_NAME):$(VERSION) \
		--push \
		.

version-set: ## Set api-control-plane version
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION required"; \
		echo "Usage: make version-set VERSION=1.0.0"; \
		exit 1; \
	fi
	@echo "$(VERSION)" > VERSION
	@echo "✓ Set api-control-plane version to $(VERSION)"

version-bump-patch: ## Bump patch version
	@$(MAKE) version-set VERSION=$$(cd ../.. && bash scripts/next-version.sh patch portals/api-control-plane)

version-bump-minor: ## Bump minor version
	@$(MAKE) version-set VERSION=$$(cd ../.. && bash scripts/next-version.sh minor portals/api-control-plane)

version-bump-major: ## Bump major version
	@$(MAKE) version-set VERSION=$$(cd ../.. && bash scripts/next-version.sh major portals/api-control-plane)

version-bump-next-dev: ## Bump to next minor dev version with SNAPSHOT suffix
	@$(MAKE) version-set VERSION=$$(cd ../.. && bash scripts/next-version.sh next-dev portals/api-control-plane)

version-get-release: ## Get release version (strips SNAPSHOT suffix)
	@VERSION=$$(cat VERSION 2>/dev/null | tr -d '[:space:]' | sed 's/-SNAPSHOT//'); \
	if [ -z "$$VERSION" ]; then \
		echo "Error: VERSION is empty or contains only whitespace. Check portals/api-control-plane/VERSION file." >&2; \
		exit 1; \
	fi; \
	echo "$$VERSION"
