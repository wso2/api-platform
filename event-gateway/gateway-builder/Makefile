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

SHELL := /bin/bash

VERSION       ?= $(shell cat ../../VERSION 2>/dev/null || echo "0.1.0-SNAPSHOT")
GIT_COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BINARY_NAME   := event-gateway-builder
OUTPUT_DIR    := output

LDFLAGS := -X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildDate=$(BUILD_DATE)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@echo 'Event Gateway Builder (version: $(VERSION))'
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-30s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the event-gateway-builder binary
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) ./cmd/builder

.PHONY: run
run: build ## Run the builder with default settings (requires GATEWAY_SRC and BUILD_FILE)
	./$(BINARY_NAME) \
		-build-file   $${BUILD_FILE:-build.yaml} \
		-gateway-src  $${GATEWAY_SRC:-../../event-gateway/gateway-runtime} \
		-out-dir      $(OUTPUT_DIR) \
		-log-format   text

.PHONY: test
test: ## Run unit tests
	go test ./...

.PHONY: clean
clean: ## Remove build artifacts
	rm -f $(BINARY_NAME)
	rm -rf $(OUTPUT_DIR)

.PHONY: vet
vet: ## Run go vet
	go vet ./...
