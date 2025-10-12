# Makefile for Router (Envoy Proxy)

.PHONY: help docker clean

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

docker: ## Build Router Docker image
	@echo "Building Router Docker image..."
	@docker build -t wso2/gateway-router:latest .

clean: ## Clean Docker images
	@echo "Cleaning Docker images..."
	@docker rmi wso2/gateway-router:latest || true
