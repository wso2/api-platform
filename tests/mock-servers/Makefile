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

DOCKER_REGISTRY ?= ghcr.io/wso2/api-platform
VERSION ?= latest
MOCKS ?= mock-jwks mock-azure-content-safety mock-aws-bedrock-guardrail mock-embedding-provider mock-analytics-collector

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo 'Usage: make [target] [DOCKER_REGISTRY=...] [VERSION=...] [MOCKS=...]'
	@echo ''
	@echo 'Targets:'
	@echo '  build   - Build Docker images for all mock servers in MOCKS'
	@echo ''
	@echo 'Variables:'
	@echo '  DOCKER_REGISTRY  Registry prefix (default: ghcr.io/wso2/api-platform)'
	@echo '  VERSION          Image tag       (default: latest)'
	@echo '  MOCKS            Space-separated list of mock server directories to build'

.PHONY: build
build: ## Build Docker images for mock servers
	@for mock in $(MOCKS); do \
		echo "Building $$mock..."; \
		docker build -t $(DOCKER_REGISTRY)/$$mock:$(VERSION) $$mock; \
	done
