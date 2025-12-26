# Operator Integration Tests

.PHONY: test test-verbose test-tags deps clean help run-local

# Default test timeout
TIMEOUT ?= 45m

# Default tag filter (empty = all)
TAGS ?=

# Project root (2 levels up from kubernetes/it)
PROJECT_ROOT := $(CURDIR)/../..

# Docker Hub username - change this to your Docker Hub username
DOCKER_HUB_USER ?= tharsanan
IMAGE_TAG ?= test

# Full registry path for images
DOCKER_REGISTRY := docker.io/$(DOCKER_HUB_USER)

help:
	@echo "Operator Integration Test Suite"
	@echo ""
	@echo "Usage:"
	@echo "  make deps              - Download Go dependencies"
	@echo "  make test              - Run all integration tests (assumes setup is done)"
	@echo "  make test-verbose      - Run tests with verbose output"
	@echo "  make test-tags         - Run tests with specific tags (TAGS=@gateway-lifecycle)"
	@echo "  make run-local         - Full local test run (builds images, pushes to Docker Hub, runs tests)"
	@echo "  make run-local TAGS=@tag - Run specific tags locally"
	@echo "  make run-local-quick   - Quick run (skip building, for iterating on tests)"
	@echo "  make clean             - Clean test artifacts"
	@echo ""
	@echo "Configuration:"
	@echo "  DOCKER_HUB_USER=$(DOCKER_HUB_USER)  - Set your Docker Hub username"
	@echo "  IMAGE_TAG=$(IMAGE_TAG)              - Image tag to use"
	@echo ""
	@echo "Examples:"
	@echo "  make run-local DOCKER_HUB_USER=myuser"
	@echo "  make run-local TAGS=@gateway-lifecycle"
	@echo ""
	@echo "Available tags:"
	@echo "  @gateway-lifecycle    - Gateway CR create/delete"
	@echo "  @restapi-lifecycle    - RestApi CR create/update/delete"
	@echo "  @jwt-authentication   - JWT policy tests"
	@echo "  @multi-namespace      - Multi-namespace API tests"
	@echo "  @scoped-mode          - Scoped operator mode tests"
	@echo "  @full-lifecycle       - Complete operator lifecycle test"

deps:
	go mod tidy
	go mod download

test: deps
	go test -v -timeout $(TIMEOUT) ./...

test-verbose: deps
	go test -v -timeout $(TIMEOUT) ./... 2>&1 | tee test-output.log

test-tags: deps
ifdef TAGS
	go test -v -timeout $(TIMEOUT) ./... -godog.tags="$(TAGS)"
else
	@echo "Please specify TAGS, e.g., make test-tags TAGS=@gateway-lifecycle"
	@exit 1
endif

# Run a specific feature file
test-feature: deps
ifdef FEATURE
	go test -v -timeout $(TIMEOUT) ./... -godog.paths="features/$(FEATURE)"
else
	@echo "Please specify FEATURE, e.g., make test-feature FEATURE=gateway_lifecycle.feature"
	@exit 1
endif

clean:
	rm -f test-output.log
	go clean -testcache

# Verify prerequisites
check-prereqs:
	@echo "Checking prerequisites..."
	@which kubectl > /dev/null || (echo "kubectl not found" && exit 1)
	@which helm > /dev/null || (echo "helm not found" && exit 1)
	@which docker > /dev/null || (echo "docker not found" && exit 1)
	@kubectl cluster-info > /dev/null 2>&1 || (echo "No Kubernetes cluster available - set kubectl context" && exit 1)
	@echo "All prerequisites met!"

# Check Docker Hub login
check-docker-login:
	@echo "Checking Docker Hub login..."
	@docker login
	@echo "Docker Hub login OK!"

# Deploy OCI registry in-cluster for Helm charts
setup-helm-registry:
	@echo "Setting up in-cluster OCI registry for Helm charts..."
	@kubectl get namespace registry > /dev/null 2>&1 || kubectl create namespace registry
	@kubectl get deployment registry -n registry > /dev/null 2>&1 || kubectl apply -f $(CURDIR)/manifests/registry.yaml
	@echo "Waiting for registry to be ready..."
	@kubectl wait --for=condition=available deployment/registry -n registry --timeout=120s
	@kubectl wait --for=condition=ready pod -l app=registry -n registry --timeout=120s
	@echo "Helm registry is ready!"

# Build all required images using existing Makefiles
build-images:
	@echo "Building images for $(DOCKER_REGISTRY)..."
	@echo "Building Gateway images (controller, policy-engine, router)..."
	$(MAKE) -C $(PROJECT_ROOT)/gateway build-local DOCKER_REGISTRY=$(DOCKER_REGISTRY) VERSION=$(IMAGE_TAG)
	@echo "Building Operator image..."
	$(MAKE) -C $(PROJECT_ROOT)/kubernetes/gateway-operator docker-build IMG=$(DOCKER_REGISTRY)/gateway-operator:$(IMAGE_TAG)
	@echo "Building Mock JWKS..."
	cd $(PROJECT_ROOT)/tests/mock-servers/mock-jwks && \
		docker build -t $(DOCKER_REGISTRY)/mock-jwks:$(IMAGE_TAG) .
	@echo "All images built!"
	cd $(PROJECT_ROOT)/tests/mock-servers/mock-jwks && \
		docker build -t $(DOCKER_REGISTRY)/mock-jwks:$(IMAGE_TAG) .
	@echo "All images built!"

# Push images to Docker Hub
push-images: check-docker-login
	@echo "Pushing images to Docker Hub ($(DOCKER_REGISTRY))..."
	docker push $(DOCKER_REGISTRY)/gateway-controller:$(IMAGE_TAG)
	docker push $(DOCKER_REGISTRY)/policy-engine:$(IMAGE_TAG)
	docker push $(DOCKER_REGISTRY)/gateway-router:$(IMAGE_TAG)
	docker push $(DOCKER_REGISTRY)/gateway-operator:$(IMAGE_TAG)
	docker push $(DOCKER_REGISTRY)/mock-jwks:$(IMAGE_TAG)
	@echo "All images pushed to Docker Hub!"

# Push Gateway Helm chart to in-cluster OCI registry
push-helm-chart: setup-helm-registry
	@echo "Packaging Gateway Helm chart..."
	cd $(PROJECT_ROOT)/kubernetes/helm/gateway-helm-chart && \
		helm package . --version 0.0.0-test
	@echo "Starting port-forward for Helm push..."
	@kubectl port-forward svc/registry -n registry 5000:5000 &
	@sleep 3
	@echo "Pushing Helm chart..."
	cd $(PROJECT_ROOT)/kubernetes/helm/gateway-helm-chart && \
		helm push gateway-0.0.0-test.tgz oci://localhost:5000/charts --plain-http
	@pkill -f "kubectl port-forward.*registry" || true
	@echo "Helm chart pushed!"

# Full local setup: build images, push to Docker Hub, setup Helm registry
# Set SKIP_IMAGES=1 to skip building and pushing images
setup-local: check-prereqs
ifndef SKIP_IMAGES
	$(MAKE) build-images
	$(MAKE) push-images
endif
	$(MAKE) setup-helm-registry
	$(MAKE) push-helm-chart
	@echo ""
	@echo "=== Local environment is ready! ==="
ifndef SKIP_IMAGES
	@echo "Images pushed to: $(DOCKER_REGISTRY)"
endif
	@echo "Helm chart pushed to: oci://registry.registry.svc.cluster.local:5000/charts/gateway"
	@echo ""

# Run tests locally with full setup
# Only assumes: kubectl context is set, docker login done
# Set SKIP_SETUP=1 to skip setup-local and deps
# Set TAGS=@tag to run specific tags (optional)
run-local:
ifndef SKIP_SETUP
	$(MAKE) setup-local
	$(MAKE) deps
endif
	@echo ""
ifdef TAGS
	@echo "=== Running operator integration tests with tags: $(TAGS) ==="
else
	@echo "=== Running operator integration tests ==="
endif
	@echo ""
	cd $(CURDIR)/cmd && \
		DOCKER_REGISTRY=$(DOCKER_REGISTRY) \
		IMAGE_TAG=$(IMAGE_TAG) \
		IMAGE_PULL_POLICY=Always \
		OPERATOR_CHART_PATH=$(PROJECT_ROOT)/kubernetes/helm/operator-helm-chart \
		GATEWAY_CHART_PATH=$(PROJECT_ROOT)/kubernetes/helm/gateway-helm-chart \
		GATEWAY_CHART_NAME=oci://registry.registry.svc.cluster.local:5000/charts/gateway \
		GATEWAY_CHART_VERSION=0.0.0-test \
		TAGS="$(TAGS)" \
		go run .

