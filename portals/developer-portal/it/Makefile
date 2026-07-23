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

.PHONY: all test test-postgres test-rest-api test-rest-api-postgres open clean deps ensure-test-tag ensure-certs

VERSION ?= $(shell cat ../VERSION 2>/dev/null | tr -d '[:space:]' || echo "0.0.1-SNAPSHOT")
DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
IMAGE_NAME      := developer-portal
CERT_DIR        := .certs

# Auto-detect host CPU so Cypress runs natively (no QEMU).
# arm64 = Apple Silicon → pull linux/arm64 Cypress image.
# Everything else (CI, Intel) defaults to linux/amd64.
CYPRESS_PLATFORM ?= $(shell [ "$$(uname -m)" = "arm64" ] && echo "linux/arm64" || echo "linux/amd64")

# One-time, idempotent TLS cert generation for platform-api's HTTPS listener —
# mirrors ../scripts/setup.sh's openssl step, scoped to this test fixture and run
# automatically as a `make test`/`test-postgres`/`test-rest-api*` prerequisite
# (unlike setup.sh, which a person runs by hand before `docker compose up`) so CI
# and a fresh checkout need no manual pre-step. Bind-mounted read-only into
# platform-api (docker-compose.test*.yaml) instead of the old certgen init
# container + named volume. 644, not 600, so the container's non-root user can
# read a file owned by the host user (same tradeoff as setup.sh's CERT_FILE_MODE).
#
# Also provisions the RS256 JWT signing keypair platform-api's auth.jwt config
# requires (config-platform-api-it.toml reads it via {{ file }}) — tokens are
# signed asymmetrically, there is no shared HMAC secret to pass via env anymore.
# Written into the same dir so one bind mount covers both TLS and JWT material.
ensure-certs:
	@mkdir -p $(CERT_DIR)
	@if [ -f $(CERT_DIR)/cert.pem ] && [ -f $(CERT_DIR)/key.pem ]; then \
		echo "Using existing $(CERT_DIR)/{cert,key}.pem"; \
	else \
		echo "Generating platform-api TLS certificate at $(CERT_DIR) ..."; \
		openssl req -x509 -newkey rsa:2048 -sha256 -days 365 -nodes \
			-keyout $(CERT_DIR)/key.pem -out $(CERT_DIR)/cert.pem \
			-subj "/O=WSO2 API Platform/CN=platform-api" \
			-addext "subjectAltName=DNS:localhost,DNS:platform-api,IP:127.0.0.1" 2>/dev/null; \
		chmod 644 $(CERT_DIR)/key.pem $(CERT_DIR)/cert.pem; \
	fi
	@if [ -f $(CERT_DIR)/jwt_private.pem ] && [ -f $(CERT_DIR)/jwt_public.pem ]; then \
		echo "Using existing $(CERT_DIR)/jwt_{private,public}.pem"; \
	else \
		echo "Generating platform-api RS256 JWT keypair at $(CERT_DIR) ..."; \
		openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 \
			-out $(CERT_DIR)/jwt_private.pem 2>/dev/null; \
		openssl rsa -in $(CERT_DIR)/jwt_private.pem -pubout \
			-out $(CERT_DIR)/jwt_public.pem 2>/dev/null; \
		chmod 644 $(CERT_DIR)/jwt_private.pem $(CERT_DIR)/jwt_public.pem; \
	fi

# Tag the pre-built developer-portal image as :test so docker-compose.test.yaml can
# reference a stable :test tag regardless of whether the local build is :VERSION.
ensure-test-tag:
	@if docker image inspect $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(VERSION) >/dev/null 2>&1; then \
		echo "Tagging $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(VERSION) -> $(DOCKER_REGISTRY)/$(IMAGE_NAME):test"; \
		docker tag $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(VERSION) $(DOCKER_REGISTRY)/$(IMAGE_NAME):test; \
	elif docker image inspect $(DOCKER_REGISTRY)/$(IMAGE_NAME):test >/dev/null 2>&1; then \
		echo "Using existing $(DOCKER_REGISTRY)/$(IMAGE_NAME):test (no :$(VERSION) image found)"; \
	else \
		echo "Error: No image found for $(IMAGE_NAME) (:$(VERSION) or :test). Run 'make build' in portals/developer-portal/ first." && exit 1; \
	fi

# Run Cypress tests headlessly against SQLite (CI-friendly, no external DB required).
# Requires the developer-portal image to be built first: make build (from portals/developer-portal/).
# Service list is explicit — the compose file also defines rest-api-tests, which
# `up` with no args would otherwise start too.
# Each target captures the exit code of `docker compose up` itself — which
# `--exit-code-from <svc>` sets to that service's container exit code, and which
# is also non-zero when a dependency fails to start — then always tears down and
# re-exits with it. Do NOT recover the code via `docker compose ps -q <svc> |
# docker inspect`: `ps -q` lists only RUNNING containers, so once the service has
# exited it returns nothing and the code is silently lost (the target then passes
# even when tests fail). Keep the capture in the same shell as `up`.
test: ensure-test-tag ensure-certs
	@CYPRESS_PLATFORM=$(CYPRESS_PLATFORM) DOCKER_REGISTRY=$(DOCKER_REGISTRY) docker compose -f docker-compose.test.yaml up devportal cypress --abort-on-container-exit --exit-code-from cypress; \
	EXIT=$$?; \
	docker compose -f docker-compose.test.yaml down -v --remove-orphans; \
	exit $$EXIT

# Run Cypress tests headlessly against PostgreSQL.
# Requires the developer-portal image to be built first: make build (from portals/developer-portal/).
test-postgres: ensure-test-tag ensure-certs
	@CYPRESS_PLATFORM=$(CYPRESS_PLATFORM) DOCKER_REGISTRY=$(DOCKER_REGISTRY) docker compose -f docker-compose.test.postgres.yaml up postgres devportal cypress --abort-on-container-exit --exit-code-from cypress; \
	EXIT=$$?; \
	docker compose -f docker-compose.test.postgres.yaml down -v --remove-orphans; \
	exit $$EXIT

# Run the REST API integration test suite (Jest + Supertest) against SQLite.
# Requires the developer-portal image to be built first: make build (from portals/developer-portal/).
test-rest-api: ensure-test-tag ensure-certs
	@DOCKER_REGISTRY=$(DOCKER_REGISTRY) docker compose -f docker-compose.test.yaml up devportal rest-api-tests --abort-on-container-exit --exit-code-from rest-api-tests; \
	EXIT=$$?; \
	docker compose -f docker-compose.test.yaml down -v --remove-orphans; \
	exit $$EXIT

# Run the REST API integration test suite (Jest + Supertest) against PostgreSQL.
test-rest-api-postgres: ensure-test-tag ensure-certs
	@DOCKER_REGISTRY=$(DOCKER_REGISTRY) docker compose -f docker-compose.test.postgres.yaml up postgres devportal rest-api-tests --abort-on-container-exit --exit-code-from rest-api-tests; \
	EXIT=$$?; \
	docker compose -f docker-compose.test.postgres.yaml down -v --remove-orphans; \
	exit $$EXIT

# Open Cypress interactive UI — runs against a LOCALLY running devportal (not in Docker).
# Start your devportal with `docker compose up` in portals/developer-portal/ first.
open: deps
	cd ui && CYPRESS_BASE_URL=https://localhost:3000 npx cypress open --e2e --config-file cypress.config.js

# Install Node dependencies (only needed for `make open`).
deps:
	cd ui && npm install

# Tear down any leftover test containers and volumes.
clean:
	docker compose -f docker-compose.test.yaml down -v --remove-orphans 2>/dev/null || true
	docker compose -f docker-compose.test.postgres.yaml down -v --remove-orphans 2>/dev/null || true
	rm -rf reports/videos reports/screenshots reports/*.json reports/*.html ui/node_modules/.cache rest-api/node_modules
