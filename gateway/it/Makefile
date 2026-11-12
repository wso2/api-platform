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

.PHONY: all test test-load test-faultflow test-postgres test-sqlserver test-vhosts-single test-vhosts-multi test-dp-to-cp-nosync test-all test-verbose clean deps check-docker coverage-report build-coverage ensure-test-tags

VERSION ?= $(shell cat ../VERSION 2>/dev/null | tr -d '[:space:]' || echo "0.0.1-SNAPSHOT")
DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
COVERAGE_IMAGES := gateway-controller-coverage gateway-runtime-coverage

COMPOSE_FILE ?= docker-compose.test.yaml

# Refresh :test tags from :VERSION images when available.
# If :VERSION is unavailable but :test already exists, keep using :test.
# Tries coverage image first (from make build-coverage), then falls back to
# non-coverage image (from make build) for local development convenience.
ensure-test-tags:
	@for img in $(COVERAGE_IMAGES); do \
		base=$${img%-coverage}; \
		if docker image inspect $(DOCKER_REGISTRY)/$$img:$(VERSION) >/dev/null 2>&1; then \
			echo "Tagging $(DOCKER_REGISTRY)/$$img:$(VERSION) -> $(DOCKER_REGISTRY)/$$img:test"; \
			docker tag $(DOCKER_REGISTRY)/$$img:$(VERSION) $(DOCKER_REGISTRY)/$$img:test; \
		elif docker image inspect $(DOCKER_REGISTRY)/$$base:$(VERSION) >/dev/null 2>&1; then \
			echo "Tagging $(DOCKER_REGISTRY)/$$base:$(VERSION) -> $(DOCKER_REGISTRY)/$$img:test"; \
			docker tag $(DOCKER_REGISTRY)/$$base:$(VERSION) $(DOCKER_REGISTRY)/$$img:test; \
		elif docker image inspect $(DOCKER_REGISTRY)/$$img:test >/dev/null 2>&1; then \
			echo "Using existing $(DOCKER_REGISTRY)/$$img:test (no :$(VERSION) image found)"; \
		else \
			echo "Error: No image found for $$img (:$(VERSION) or :test). Run 'make build-coverage' or 'make build' in gateway/ first." && exit 1; \
		fi; \
	done

# Run integration tests (always with coverage)
# Requires: make build-coverage-image (run once before tests)
test: ensure-test-tags
	COMPOSE_FILE=$(COMPOSE_FILE) go test -v -timeout 50m ./...

# Run ONLY the load scenarios (@load), which every other target excludes.
#
# LOCAL ONLY — not run by CI. These drive tens of thousands of requests and take
# minutes; in CI they would dominate the suite's runtime and turn an unrelated
# failure into a long wait. Run this before changing anything that affects how
# often a target is attempted, or the cost of an attempt.
# IT_FEATURE_PATHS is set explicitly: the tag filter only selects WITHIN the files
# godog loads, and the load feature is not in suite_test.go's default list, so
# filtering alone would run zero scenarios and still report PASS. The warm-up
# features come first for the same reason every other targeted run uses them —
# a cold gateway answers 503 for the first few seconds.
LOAD_FEATURES ?= features/health.feature,features/api_deploy.feature,features/model-failover-load.feature

test-load: ensure-test-tags
	COMPOSE_FILE=$(COMPOSE_FILE) IT_TAGS=@load IT_FEATURE_PATHS=$(LOAD_FEATURES) go test -v -timeout 90m ./...

# Run the failover features with the fault flow handling upstream failures.
#
# Why this needs its own target rather than a tag: handle_upstream_faults is a
# process-level setting read from the policy engine's config, with no env override and
# no per-API form, so selecting it means starting a differently-configured stack.
#
# Why it is worth a stack of its own: with the flag on, the gateway runs a failing
# response through the fault chain INSTEAD of the response flow. For model-failover
# that is exactly the set of responses it grades, so a regression there leaves the
# policy inert — nothing recorded, nothing suspended, the upstream's own error
# reaching the client — while every scenario in the default run still passes. The
# failure mode is silence, so the default suite cannot catch it.
#
# IT_FEATURE_PATHS is set explicitly for the same reason test-load sets it: the
# feature is not in suite_test.go's default list, and a filter alone would select
# zero scenarios and still report PASS.
FAULTFLOW_FEATURES ?= features/health.feature,features/api_deploy.feature,features/model-failover.feature

test-faultflow: ensure-test-tags
	COMPOSE_FILE=docker-compose.test.yaml:docker-compose.test.faultflow.override.yaml \
		IT_TAGS='~@load' IT_FEATURE_PATHS=$(FAULTFLOW_FEATURES) go test -v -timeout 50m ./...

# Build all coverage-instrumented gateway components
build-coverage:
	$(MAKE) -C .. build-coverage

# Build and run tests in one command
test-all: build-coverage test

# Run integration tests against Postgres-backed controller.
# This compose runs two gateway-controllers: the management one (REST) and
# gateway-controller-xds (feeds xDS to gateway-runtime). IT_GATEWAY_CONTROLLER_HA
# tells the suite to probe the runtime controller (host port 9093) for the
# policy-chain xDS-sync version, since that is the version the policy engine echoes.
test-postgres:
	COMPOSE_FILE=docker-compose.test.postgres.yaml IT_GATEWAY_CONTROLLER_HA=true $(MAKE) test

# Run integration tests against SQL Server-backed controller
test-sqlserver:
	COMPOSE_FILE=docker-compose.test.sqlserver.yaml $(MAKE) test

# Run vhost integration tests with single-domain gateway vhost config
test-vhosts-single:
	COMPOSE_FILE=docker-compose.test.vhosts-single.yaml IT_FEATURE_PATHS=features/vhost-routing-single.feature $(MAKE) test

# Run vhost integration tests with multi-domain gateway vhost config
test-vhosts-multi:
	COMPOSE_FILE=docker-compose.test.vhosts-multi.yaml IT_FEATURE_PATHS=features/vhost-routing-multi.feature $(MAKE) test

# Run the DP->CP push tests with control-plane sync DISABLED. Layers a tiny override on the
# base compose (deployment_sync_enabled=false via COMPOSE_FILE=base:override) and asserts
# gateway-originated artifacts are NOT pushed. Not part of the default suite (which runs with
# sync enabled). Verifies dp-to-cp-testing.md items 1-2.
test-dp-to-cp-nosync:
	COMPOSE_FILE=docker-compose.test.yaml:docker-compose.test.dp-nosync.override.yaml \
	IT_FEATURE_PATHS=features/health.feature,features/dp-to-cp-sync-disabled.feature $(MAKE) test

# Run tests with verbose output
test-verbose: ensure-test-tags
	COMPOSE_FILE=$(COMPOSE_FILE) go test -v -count=1 -timeout 50m ./...

# Clean up test artifacts
clean:
	docker rm -f $$(docker ps -aq --filter "name=it-") 2>/dev/null || true
	docker compose -f docker-compose.test.yaml -p gateway-it down -v --remove-orphans 2>/dev/null || true
	docker compose -f docker-compose.test.postgres.yaml -p gateway-it down -v --remove-orphans 2>/dev/null || true
	docker compose -f docker-compose.test.sqlserver.yaml -p gateway-it down -v --remove-orphans 2>/dev/null || true
	docker compose -f docker-compose.test.vhosts-single.yaml -p gateway-it down -v --remove-orphans 2>/dev/null || true
	docker compose -f docker-compose.test.vhosts-multi.yaml -p gateway-it down -v --remove-orphans 2>/dev/null || true
	docker compose -f docker-compose.test.postgres.vhosts-single.yaml -p gateway-it down -v --remove-orphans 2>/dev/null || true
	docker compose -f docker-compose.test.postgres.vhosts-multi.yaml -p gateway-it down -v --remove-orphans 2>/dev/null || true
	rm -rf coverage/*.out coverage/*.txt coverage/*.html coverage/merged coverage/gateway-controller reports/*.json

# Install dependencies
deps:
	go mod download
	go mod tidy

# Verify Docker is available
check-docker:
	@docker info > /dev/null 2>&1 || (echo "Docker is not running" && exit 1)
	@echo "Docker is available"

# View coverage report (after running test-coverage)
coverage-report:
	@if [ -f coverage/coverage.html ]; then \
		echo "Opening coverage report..."; \
		xdg-open coverage/coverage.html 2>/dev/null || open coverage/coverage.html 2>/dev/null || echo "Coverage report: coverage/coverage.html"; \
	else \
		echo "No coverage report found. Run 'make test-coverage' first."; \
	fi
