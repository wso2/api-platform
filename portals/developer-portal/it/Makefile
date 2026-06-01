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

.PHONY: all test open clean deps ensure-test-tag

VERSION ?= $(shell cat ../VERSION 2>/dev/null | tr -d '[:space:]' || echo "0.0.1-SNAPSHOT")
DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
IMAGE_NAME      := developer-portal

# Auto-detect host CPU so Cypress runs natively (no QEMU).
# arm64 = Apple Silicon → pull linux/arm64 Cypress image.
# Everything else (CI, Intel) defaults to linux/amd64.
CYPRESS_PLATFORM ?= $(shell [ "$$(uname -m)" = "arm64" ] && echo "linux/arm64" || echo "linux/amd64")

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

# Run Cypress tests headlessly inside Docker Compose (CI-friendly).
# Requires the developer-portal image to be built first: make build (from portals/developer-portal/).
test: ensure-test-tag
	CYPRESS_PLATFORM=$(CYPRESS_PLATFORM) DOCKER_REGISTRY=$(DOCKER_REGISTRY) docker compose -f docker-compose.test.yaml up --abort-on-container-exit --exit-code-from cypress
	$(MAKE) _collect-exit

# Capture the exit code from the cypress container even after compose exits.
_collect-exit:
	@EXIT=$$(docker compose -f docker-compose.test.yaml ps -q cypress | xargs docker inspect --format='{{.State.ExitCode}}' 2>/dev/null || echo 0); \
	docker compose -f docker-compose.test.yaml down -v --remove-orphans; \
	exit $$EXIT

# Open Cypress interactive UI — runs against a LOCALLY running devportal (not in Docker).
# Start your devportal with `docker compose up` in portals/developer-portal/ first.
open: deps
	CYPRESS_BASE_URL=https://localhost:3000 npx cypress open --e2e --config-file cypress.config.js

# Install Node dependencies (only needed for `make open`).
deps:
	npm install

# Tear down any leftover test containers and volumes.
clean:
	docker compose -f docker-compose.test.yaml down -v --remove-orphans 2>/dev/null || true
	rm -rf reports/videos reports/screenshots reports/*.json reports/*.html node_modules/.cache
