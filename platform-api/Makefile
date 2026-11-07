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

# Doc generation
REPO_ROOT := $(shell git rev-parse --show-toplevel)
APIDOCS_TOOLS_DIR := $(REPO_ROOT)/tools/apidocs
DOCS_OUT_DIR := $(REPO_ROOT)/docs/rest-apis/platform-api

# Pinned so every developer and CI run generates byte-identical models.
OAPI_CODEGEN_VERSION := v2.8.0

.PHONY: help build cloud-build test push build-and-push-multiarch cloud-build-and-push-multiarch generate generate-apidocs setup-local-dev run-local

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

test: ## Run tests with coverage report
	@echo "Running tests..."
	@go test -v ./... -cover -coverprofile=unit-test-coverage.txt

build: binary ## Build Docker image (from the binary zip, for this machine's CPU; needs a multi-platform builder, see `binary`)
	@echo "Building Docker image ($(IMAGE_NAME):$(VERSION))..."
	@rm -rf target/.binary-ctx && mkdir -p target/.binary-ctx
	@unzip -q target/$(BINARY_NAME).zip -d target/.binary-ctx
	docker buildx build \
		--build-arg PORT=$(PORT) \
		-t $(IMAGE_NAME):$(VERSION) \
		-t $(IMAGE_NAME):latest \
		--load \
		target/.binary-ctx/platform-api
	@rm -rf target/.binary-ctx

# Binary zip: the Platform API binaries for linux/amd64 and linux/arm64, the files
# they read at runtime, and the image Dockerfile, which builds the image from them.
# Customers download it to build their own image, for example on a different base
# image. The images built by `build` and `build-and-push-multiarch` come from this
# zip itself (unzipped into target/.binary-ctx, since Docker can't build from a
# .zip), so a published image and the zip of the same version hold the same
# binaries.
#
# The binaries are compiled by Dockerfile.build and exported with its `binary` stage
# (at linux-<arch>/platform-api). Its builder stage runs on the build machine's own
# platform and cross-compiles, so no emulation is needed.
#
# Building both platforms in one go needs a builder that supports multi-platform
# builds: a `docker-container` builder, or the default `docker` builder with the
# containerd image store turned on. `binary` checks this first, so `build` and
# `build-and-push-multiarch` (which run it) fail with the fix instead of a Docker error.
BINARY_NAME      := wso2apip-platform-api-binary-$(VERSION)
BINARY_PLATFORMS := linux/amd64,linux/arm64

.PHONY: binary
binary: ## Build target/$(BINARY_NAME).zip (needs a multi-platform builder)
	@if [ "$$(docker buildx inspect | awk '/^Driver:/ {print $$2; exit}')" = "docker" ] && \
	    ! docker info --format '{{json .DriverStatus}}' | grep -q 'io.containerd.snapshotter'; then \
		echo "Error: the binary zip is built for $(BINARY_PLATFORMS) in one build, which the" >&2; \
		echo "current 'docker' builder can't do without the containerd image store." >&2; \
		echo "Either turn on the containerd image store, or create and use a docker-container builder:" >&2; \
		echo "  docker buildx create --name multiarch --driver docker-container --use" >&2; \
		exit 1; \
	fi
	@rm -rf target/.binary-out target/.binary-stage target/$(BINARY_NAME).zip
	docker buildx build -f Dockerfile.build \
		--platform $(BINARY_PLATFORMS) \
		--target binary \
		--build-context common=../common \
		--build-context httpkit=../httpkit \
		--build-arg VERSION=$(VERSION) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		--output type=local,dest=target/.binary-out \
		.
	@mkdir -p target/.binary-stage/platform-api/bin target/.binary-stage/platform-api/internal/database target/.binary-stage/platform-api/resources
	@for p in $$(echo "$(BINARY_PLATFORMS)" | tr ',' ' '); do \
		src="target/.binary-out/$$(echo $$p | tr '/' '_')/$$(echo $$p | tr '/' '-')/platform-api"; \
		dst="target/.binary-stage/platform-api/bin/$$(echo $$p | tr '/' '-')"; \
		test -f "$$src" || { echo "Error: no binary exported for $$p ($$src)" >&2; exit 1; }; \
		mkdir -p "$$dst" && cp "$$src" "$$dst/platform-api"; \
	done
	@cp Dockerfile target/.binary-stage/platform-api/
	@cp internal/database/schema.sql internal/database/schema.postgres.sql \
		internal/database/schema.sqlserver.sql internal/database/schema.sqlite.sql \
		target/.binary-stage/platform-api/internal/database/
	@cp -R resources/default-llm-provider-templates resources/openapi.yaml target/.binary-stage/platform-api/resources/
	@cd target/.binary-stage && zip -rq ../$(BINARY_NAME).zip platform-api
	@rm -rf target/.binary-out target/.binary-stage
	@echo "✓ Built target/$(BINARY_NAME).zip"

cloud-build: VERSION := $(CLOUD_IMAGE_TAG)
cloud-build: ## Build cloud Docker image with the event-gateway plugin (EXPERIMENTAL=true), tagged <branch>-<commit> (override with VERSION=x)
	@echo "Building cloud Docker image ($(CLOUD_IMAGE_NAME):$(VERSION)) with event-gateway plugin..."
	VERSION=$(VERSION) GIT_COMMIT=$(GIT_COMMIT) PORT=$(PORT) EXPERIMENTAL=true \
	docker buildx bake -f docker-bake.hcl --allow=fs.read=.. \
		--set image.tags=$(CLOUD_IMAGE_NAME):$(VERSION) \
		--set image.tags=$(CLOUD_IMAGE_NAME):latest \
		--load \
		image

generate: ## Generate API server code from OpenAPI spec
	@echo "Generating API models from the Platform API OpenAPI spec..."
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) --config=oapi-codegen.yaml resources/openapi.yaml
	@echo "Generating API models for the eventgateway plugin spec..."
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) --config=oapi-codegen-eventgateway.yaml plugins/eventgateway/openapi.yaml

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

build-and-push-multiarch: binary ## Build and push multi-architecture Docker image (linux/amd64, linux/arm64) from the binary zip
	@echo "Building and pushing multi-arch Docker image: $(IMAGE_NAME):$(VERSION)"
	@rm -rf target/.binary-ctx && mkdir -p target/.binary-ctx
	@unzip -q target/$(BINARY_NAME).zip -d target/.binary-ctx
	docker buildx build \
		--platform $(BINARY_PLATFORMS) \
		--build-arg PORT=$(PORT) \
		-t $(IMAGE_NAME):$(VERSION) \
		--push \
		target/.binary-ctx/platform-api
	@rm -rf target/.binary-ctx

cloud-build-and-push-multiarch: VERSION := $(CLOUD_IMAGE_TAG)
cloud-build-and-push-multiarch: ## Build and push multi-arch cloud image with the event-gateway plugin (linux/amd64, linux/arm64), tagged <branch>-<commit> (override with VERSION=x)
	@echo "Building and pushing multi-arch cloud image: $(CLOUD_IMAGE_NAME):$(VERSION) with event-gateway plugin"
	VERSION=$(VERSION) GIT_COMMIT=$(GIT_COMMIT) PORT=$(PORT) EXPERIMENTAL=true \
	docker buildx bake -f docker-bake.hcl --allow=fs.read=.. \
		--set compile.platform=linux/amd64,linux/arm64 \
		--set image.platform=linux/amd64,linux/arm64 \
		--set image.tags=$(CLOUD_IMAGE_NAME):$(VERSION) \
		--push \
		image

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

# ---------------------------------------------------------------------------
# v1 → v2 database migration client (cmd/migrate) — operator bundles
# ---------------------------------------------------------------------------
# Static (CGO_ENABLED=0), -trimpath, VCS-stamping off: the binary depends only
# on the source at MIGRATE_COMMIT plus the ldflags below, so a rebuild at the
# same commit with the same Go toolchain yields the same bytes. BuildDate is
# the COMMIT date (not "now") for the same reason.
MIGRATE_DIST_DIR   ?= dist/migrate
MIGRATE_PLATFORMS  ?= linux/amd64 linux/arm64 darwin/amd64 darwin/arm64
MIGRATE_COMMIT     := $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo unknown)
MIGRATE_VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null | tr '/' '-' || echo dev)
MIGRATE_BUILD_DATE := $(shell git log -1 --format=%cI 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)
MIGRATE_PKG        := github.com/wso2/api-platform/platform-api/internal/migration
MIGRATE_LDFLAGS    := -s -w -X $(MIGRATE_PKG).Version=$(MIGRATE_VERSION) -X $(MIGRATE_PKG).Commit=$(MIGRATE_COMMIT) -X $(MIGRATE_PKG).BuildDate=$(MIGRATE_BUILD_DATE)
MIGRATE_GOBUILD    := CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags "$(MIGRATE_LDFLAGS)"

migrate-build: ## Build the v1→v2 migration client for the host platform (./migrate) and print its --version
	$(MIGRATE_GOBUILD) -o migrate ./cmd/migrate
	@./migrate --version

migrate-dist: ## Build operator bundles of the migration client (binary + README + SHA256SUMS) for MIGRATE_PLATFORMS into MIGRATE_DIST_DIR
	@rm -rf $(MIGRATE_DIST_DIR) && mkdir -p $(MIGRATE_DIST_DIR)
	@for p in $(MIGRATE_PLATFORMS); do \
	  os=$${p%/*}; arch=$${p#*/}; name=platform-api-migrate_$(MIGRATE_VERSION)_$${os}_$${arch}; \
	  stage=$(MIGRATE_DIST_DIR)/$$name; mkdir -p $$stage; \
	  echo "building $$name"; \
	  GOOS=$$os GOARCH=$$arch $(MIGRATE_GOBUILD) -o $$stage/migrate ./cmd/migrate || exit 1; \
	  printf 'platform-api v1 -> v2 database migration client\nversion %s, commit %s, built %s, %s\n\n  ./migrate --version   # must print the version/commit above\n  ./migrate -h          # all flags; passwords and the key come from the environment only\n\nSHA256SUMS  checksums of the files in this bundle (sha256sum -c SHA256SUMS).\n' \
	    "$(MIGRATE_VERSION)" "$(MIGRATE_COMMIT)" "$(MIGRATE_BUILD_DATE)" "$$p" > $$stage/README.txt; \
	  (cd $$stage && (sha256sum migrate README.txt 2>/dev/null || shasum -a 256 migrate README.txt) > SHA256SUMS); \
	  COPYFILE_DISABLE=1 tar --no-xattrs -C $(MIGRATE_DIST_DIR) -czf $(MIGRATE_DIST_DIR)/$$name.tar.gz $$name && rm -rf $$stage; \
	done
	@cd $(MIGRATE_DIST_DIR) && (sha256sum *.tar.gz 2>/dev/null || shasum -a 256 *.tar.gz) > SHA256SUMS
	@echo; echo "bundles in $(MIGRATE_DIST_DIR) (version $(MIGRATE_VERSION), commit $(MIGRATE_COMMIT)):"; cat $(MIGRATE_DIST_DIR)/SHA256SUMS
