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

# Makefile for ap CLI

# Binary name
BINARY_NAME=ap

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Build directory
BUILD_DIR=build

# Version information
VERSION ?= $(shell cat ../VERSION 2>/dev/null || echo "0.0.1-SNAPSHOT")
BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# Linker flags
LDFLAGS=-ldflags "-X github.com/wso2/api-platform/cli/utils.Version=$(VERSION) -X github.com/wso2/api-platform/cli/utils.BuildTime=$(BUILD_TIME)"

.PHONY: all build clean test deps help install build-all build-macos build-linux build-windows version-set version-bump-patch version-bump-minor version-bump-major version-bump-next-dev

all: clean test build


## build: Build the binary for current platform (runs tests first)
build:
	@bash scripts/run-tests-with-summary.sh
	@echo ""
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

## build-skip-tests: Build the binary without running tests
build-skip-tests:
	@echo "Building $(BINARY_NAME) (skipping tests)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

## build-coverage: Build the binary with coverage instrumentation (skipping tests)
build-coverage:
	@echo "Building $(BINARY_NAME) with coverage instrumentation..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -cover -o $(BUILD_DIR)/$(BINARY_NAME) main.go
	@echo "Coverage-instrumented build complete: $(BUILD_DIR)/$(BINARY_NAME)"

## build-all: Build binaries for all platforms (macOS, Linux, Windows) - runs tests first
build-all: test build-macos build-linux build-windows
	@echo "All platform builds complete"

## build-all-skip-tests: Build binaries for all platforms without running tests
build-all-skip-tests: build-macos build-linux build-windows
	@echo "All platform builds complete (tests skipped)"

## build-macos: Build for macOS (Intel and Apple Silicon)
build-macos:
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 main.go
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 main.go
	@echo "macOS builds complete"

## build-linux: Build for Linux (amd64 and arm64)
build-linux:
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 main.go
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 main.go
	@echo "Linux builds complete"

## build-windows: Build for Windows (amd64 and arm64)
build-windows:
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe main.go
	GOOS=windows GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-arm64.exe main.go
	@echo "Windows builds complete"

## clean: Clean build files
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
	@echo "Clean complete"

## test: Run tests with coverage report
test:
	@echo "Running tests..."
	@$(GOTEST) -v ./... -cover -coverprofile=unit-test-coverage.txt || (echo "Tests failed. Build aborted." && exit 1)

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOGET) -v ./...
	$(GOMOD) tidy
	@echo "Dependencies downloaded"

## install: Install the binary to $GOPATH/bin
install: build
	@echo "Installing $(BINARY_NAME)..."
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)
	@echo "Installed to $(GOPATH)/bin/$(BINARY_NAME)"

## run: Build and run the binary
run: build
	@echo "Running $(BINARY_NAME)..."
	@$(BUILD_DIR)/$(BINARY_NAME)

## version-set: Set CLI version (Usage: make version-set VERSION=X.Y.Z)
version-set:
	@if [ -z "$(VERSION)" ]; then \
		echo "Error: VERSION required"; \
		echo "Usage: make version-set VERSION=1.0.0"; \
		exit 1; \
	fi
	@echo "$(VERSION)" > ../VERSION
	@echo "✓ Set CLI version to $(VERSION)"

## version-bump-patch: Bump patch version
version-bump-patch:
	@bash ../../scripts/version-bump.sh patch cli

## version-bump-minor: Bump minor version
version-bump-minor:
	@bash ../../scripts/version-bump.sh minor cli

## version-bump-major: Bump major version
version-bump-major:
	@bash ../../scripts/version-bump.sh major cli

## version-bump-next-dev: Bump to next minor dev version with SNAPSHOT suffix
version-bump-next-dev:
	@bash ../../scripts/version-bump.sh next-dev cli

## version-get-release: Get release version (strips SNAPSHOT suffix)
version-get-release:
	@VERSION=$$(cat ../VERSION 2>/dev/null | tr -d '[:space:]' | sed 's/-SNAPSHOT//'); \
	if [ -z "$$VERSION" ]; then \
		echo "Error: VERSION is empty or contains only whitespace. Check cli/VERSION file." >&2; \
		exit 1; \
	fi; \
	echo "$$VERSION"

## help: Display this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
