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

# Gateway Makefile - Coordinate all gateway components

SHELL := /bin/bash

# Read shared version from VERSION file
VERSION := $(shell cat VERSION 2>/dev/null || echo "0.0.1-SNAPSHOT")

# Docker registry configuration
DOCKER_REGISTRY ?= ghcr.io/renuka-fernando/api-platform

# Image names
CONTROLLER_IMAGE := $(DOCKER_REGISTRY)/gateway-controller
POLICY_ENGINE_IMAGE := $(DOCKER_REGISTRY)/policy-engine
POLICY_BUILDER_IMAGE := $(DOCKER_REGISTRY)/gateway-builder
ROUTER_IMAGE := $(DOCKER_REGISTRY)/gateway-router

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo 'Gateway Build System (Version: $(VERSION))'
	@echo ''
	@echo 'Build Targets:'
	@echo '  make build                                      - Build all gateway component Docker images'
	@echo '  make build-controller                           - Build gateway-controller Docker image'
	@echo '  make build-policy-engine                        - Build policy-engine Docker images'
	@echo '  make build-router                               - Build router Docker image'
	@echo ''
	@echo 'Multi-arch Build and Push Targets:'
	@echo '  make build-and-push-multiarch                   - Build and push all gateway components for multiple architectures'
	@echo '  make build-and-push-multiarch-controller        - Build and push controller for multiple architectures'
	@echo '  make build-and-push-multiarch-policy-engine     - Build and push policy-engine for multiple architectures'
	@echo '  make build-and-push-multiarch-router            - Build and push router for multiple architectures'
	@echo ''
	@echo 'Test Targets:'
	@echo '  make test                      - Run all tests'
	@echo '  make test-controller           - Test gateway-controller'
	@echo '  make test-policy-engine        - Test policy-engine'
	@echo ''
	@echo 'Clean Targets:'
	@echo '  make clean                     - Clean all build artifacts'

# Build Targets
.PHONY: build
build: build-controller build-policy-engine build-router ## Build all gateway components

.PHONY: build-controller
build-controller: ## Build gateway-controller Docker image
	@echo "Building controller Docker image ($(VERSION))..."
	$(MAKE) -C gateway-controller build VERSION=$(VERSION)

.PHONY: build-policy-engine
build-policy-engine: ## Build policy-engine Docker images
	@echo "Building policy-engine images ($(VERSION))..."
	$(MAKE) -C policy-engine build VERSION=$(VERSION)

.PHONY: build-router
build-router: ## Build router Docker image
	@echo "Building router Docker image ($(VERSION))..."
	$(MAKE) -C router build VERSION=$(VERSION)

# Multi-arch Build and Push Targets
.PHONY: build-and-push-multiarch
build-and-push-multiarch: build-and-push-multiarch-controller build-and-push-multiarch-policy-engine build-and-push-multiarch-router ## Build and push all gateway components for multiple architectures

.PHONY: build-and-push-multiarch-controller
build-and-push-multiarch-controller: ## Build and push gateway-controller Docker image for multiple architectures
	@echo "Building and pushing multi-arch controller Docker image ($(VERSION))..."
	$(MAKE) -C gateway-controller build-and-push-multiarch VERSION=$(VERSION)

.PHONY: build-and-push-multiarch-policy-engine
build-and-push-multiarch-policy-engine: ## Build and push policy-engine Docker images for multiple architectures
	@echo "Building and pushing multi-arch policy-engine images ($(VERSION))..."
	$(MAKE) -C policy-engine build-and-push-multiarch VERSION=$(VERSION)

.PHONY: build-and-push-multiarch-router
build-and-push-multiarch-router: ## Build and push router Docker image for multiple architectures
	@echo "Building and pushing multi-arch router Docker image ($(VERSION))..."
	$(MAKE) -C router build-and-push-multiarch VERSION=$(VERSION)

# Test Targets
.PHONY: test
test: test-controller test-policy-engine ## Run all tests

.PHONY: test-controller
test-controller: ## Test gateway-controller
	$(MAKE) -C gateway-controller test

.PHONY: test-policy-engine
test-policy-engine: ## Test policy-engine
	$(MAKE) -C policy-engine test

# Clean Targets
.PHONY: clean
clean: ## Clean all build artifacts
	$(MAKE) -C gateway-controller clean
	$(MAKE) -C policy-engine clean
	$(MAKE) -C router clean
