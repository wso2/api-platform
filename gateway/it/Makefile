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

.PHONY: test test-all test-verbose clean deps check-docker coverage-report build-coverage

# Run integration tests (always with coverage)
# Requires: make build-coverage-image (run once before tests)
test:
	go test -v -timeout 30m ./...

# Build all coverage-instrumented gateway components
build-coverage:
	$(MAKE) -C .. build-coverage

# Build and run tests in one command
test-all: build-coverage test

# Run tests with verbose output
test-verbose:
	go test -v -count=1 -timeout 30m ./...

# Clean up test artifacts
clean:
	docker rm -f $$(docker ps -aq --filter "name=it-") 2>/dev/null || true
	docker compose -f docker-compose.test.yaml -p gateway-it down -v --remove-orphans 2>/dev/null || true
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
