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
VERSION?=0.0.1
BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# Linker flags
LDFLAGS=-ldflags "-X github.com/wso2/api-platform/cli/utils.Version=$(VERSION) -X github.com/wso2/api-platform/cli/utils.BuildTime=$(BUILD_TIME)"

.PHONY: all build clean test deps help install build-all build-macos build-linux build-windows

all: clean test build

## build: Build the binary for current platform (runs tests first)
build:
	@echo "==============================================="
	@echo "Running Tests"
	@echo "==============================================="
	@mkdir -p tests/logs
	@$(GOTEST) -v ./tests/... > tests/logs/test-results.log 2>&1 || true
	@echo ""
	@echo "==============================================="
	@echo "Test Results Summary"
	@echo "==============================================="
	@printf "%-40s | %s\n" "Test Name" "Status"
	@printf "%-40s-+---------\n" "----------------------------------------"
	@grep -E "^=== RUN|^--- PASS|^--- FAIL" tests/logs/test-results.log | \
		awk 'BEGIN { test="" } \
		/^=== RUN/ { test=$$3 } \
		/^--- PASS/ { printf "%-40s | ✓ PASS\n", test } \
		/^--- FAIL/ { printf "%-40s | ✗ FAIL\n", test }' || true
	@echo "==============================================="
	@echo "ℹ Full test logs: tests/logs/test-results.log"
	@if grep -q "^--- FAIL" tests/logs/test-results.log 2>/dev/null; then \
		echo "⚠ Some tests failed - check logs for details"; \
	fi
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

## test: Run tests
test:
	@echo "Running tests..."
	@$(GOTEST) -v ./tests/... || (echo "Tests failed. Build aborted." && exit 1)

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

## help: Display this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
