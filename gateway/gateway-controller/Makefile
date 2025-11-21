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

# Makefile for Gateway-Controller

.PHONY: help generate build test docker clean run

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

generate: ## Generate API server code from OpenAPI spec
	@echo "Generating API server code from OpenAPI spec..."
	@go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.5.1 --config=oapi-codegen.yaml api/openapi.yaml

build: ## Build the Gateway-Controller binary
	@echo "Building Gateway-Controller..."
	@go build -o bin/controller cmd/controller/main.go

test: ## Run unit and integration tests
	@echo "Running tests..."
	@go test -v ./... -cover

docker: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t wso2/gateway-controller:latest .

clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@go clean

run: ## Run the Gateway-Controller locally
	@echo "Running Gateway-Controller..."
	@go run cmd/controller/main.go
