# Kubernetes Operator Integration Tests

This directory contains BDD-style integration tests for the WSO2 API Platform Gateway Operator. The tests use [Godog](https://github.com/cucumber/godog) (Cucumber for Go) to run Gherkin feature files against a real Kubernetes cluster.

## Prerequisites

Before running tests, ensure you have the following installed:

- **Go 1.25+** - For running the test suite
- **kubectl** - Configured with access to a Kubernetes cluster
- **Helm** - For installing operators and charts
- **Docker** - For building and pushing images
- **Docker Hub account** - For pushing test images (or modify registry)
- **A running Kubernetes cluster** - Kind, Minikube, or a remote cluster

### Verify Prerequisites

```bash
make check-prereqs
```

## Quick Start

### Full Local Test Run

The easiest way to run all tests locally with complete setup:

```bash
# Set your Docker Hub username (required for pushing images)
export DOCKER_HUB_USER=your-username

# Login to Docker Hub
docker login

# Run all tests (builds images, pushes to registry, runs tests)
make run-local
```

### Skip Image Building (Faster Iterations)

If images are already pushed, skip the build step:

```bash
make run-local SKIP_IMAGES=1
```

Or use the quick variant:

```bash
make run-local-quick
```

## Running Specific Tests

### By Tags

Run tests with specific tags:

```bash
# Run specific tag (includes full setup)
make run-local TAGS=@gateway-lifecycle

# Skip setup and run specific tag
make run-local SKIP_SETUP=1 TAGS=@jwt-authentication

# Skip image building and run specific tag
make run-local SKIP_IMAGES=1 TAGS=@full-lifecycle
```

### Multiple Tags

Combine tags using Cucumber expression syntax:

```bash
# Run scenarios with either tag
make run-local TAGS="@gateway-lifecycle or @restapi-lifecycle"

# Run scenarios with both tags
make run-local TAGS="@full-lifecycle and @setup"
```

## Available Tags

| Tag | Description |
|-----|-------------|
| `@gateway-lifecycle` | Gateway CR create/delete operations |
| `@restapi-lifecycle` | RestApi CR create/update/delete operations |
| `@jwt-authentication` | JWT authentication policy tests |
| `@multi-namespace` | Multi-namespace API tests |
| `@scoped-mode` | Scoped operator mode tests |
| `@full-lifecycle` | Complete operator lifecycle test |
| `@setup` | Prerequisite setup scenarios |
| `@create` | Resource creation scenarios |
| `@delete` | Resource deletion scenarios |
| `@api-create` | API creation scenarios |
| `@api-update` | API update scenarios |

## Feature Files

| Feature File | Description |
|--------------|-------------|
| [full_lifecycle.feature](features/full_lifecycle.feature) | Complete end-to-end operator lifecycle |
| [gateway_lifecycle.feature](features/gateway_lifecycle.feature) | Gateway CR management |
| [restapi_lifecycle.feature](features/restapi_lifecycle.feature) | RestApi CR management |
| [jwt_authentication.feature](features/jwt_authentication.feature) | JWT authentication policy |
| [multi_namespace.feature](features/multi_namespace.feature) | Multi-namespace API support |
| [scoped_mode.feature](features/scoped_mode.feature) | Scoped operator mode |

## Make Targets

| Target | Description |
|--------|-------------|
| `make help` | Show all available targets |
| `make deps` | Download Go dependencies |
| `make run-local` | Full local run (build + push + test) |
| `make run-local TAGS=<tag>` | Run specific tags with full setup |
| `make run-local SKIP_IMAGES=1` | Local run skipping image build/push |
| `make run-local SKIP_SETUP=1` | Run tests only (skip all setup) |
| `make run-local-quick` | Quick run (skip building images) |
| `make run-local-quick TAGS=<tag>` | Quick run with specific tags |
| `make clean` | Clean test artifacts |
| `make check-prereqs` | Verify all prerequisites are met |
| `make build-images` | Build all required Docker images |
| `make push-images` | Push images to Docker Hub |
| `make setup-helm-registry` | Deploy in-cluster OCI registry |

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DOCKER_HUB_USER` | `tharsanan` | Docker Hub username for image registry |
| `IMAGE_TAG` | `test` | Tag for built images |
| `TIMEOUT` | `45m` | Test timeout duration |
| `TAGS` | (empty) | Filter tests by tags |
| `SKIP_IMAGES` | (unset) | Skip image build/push when set to 1 |
| `SKIP_SETUP` | (unset) | Skip setup-local and deps when set to 1 |

### Run with Custom Registry

```bash
make run-local DOCKER_HUB_USER=myuser IMAGE_TAG=v1.0.0
```

## Project Structure

```
kubernetes/it/
├── Makefile           # Build and test commands
├── go.mod             # Go module definition
├── state.go           # Test state management
├── cmd/
│   ├── main.go        # Test runner entry point
│   └── state.go       # State for test runner
├── features/          # Gherkin feature files
│   ├── full_lifecycle.feature
│   ├── gateway_lifecycle.feature
│   ├── restapi_lifecycle.feature
│   ├── jwt_authentication.feature
│   ├── multi_namespace.feature
│   └── scoped_mode.feature
├── steps/             # Step definitions
│   ├── k8s_steps.go   # Kubernetes resource operations
│   ├── http_steps.go  # HTTP request operations
│   ├── helm_steps.go  # Helm and operator lifecycle
│   └── assert_steps.go # Assertion operations
└── manifests/         # Test manifests
    └── registry.yaml  # In-cluster OCI registry
```

## Running Directly

You can also run the test suite directly from the `cmd/` directory:

```bash
cd cmd

# Run all tests
DOCKER_REGISTRY=docker.io/myuser \
IMAGE_TAG=test \
OPERATOR_CHART_PATH=../../helm/operator-helm-chart \
GATEWAY_CHART_PATH=../../helm/gateway-helm-chart \
GATEWAY_CHART_NAME=oci://registry.registry.svc.cluster.local:5000/charts/gateway \
GATEWAY_CHART_VERSION=0.0.0-test \
go run .

# Run with specific tags
DOCKER_REGISTRY=docker.io/myuser \
IMAGE_TAG=test \
OPERATOR_CHART_PATH=../../helm/operator-helm-chart \
GATEWAY_CHART_PATH=../../helm/gateway-helm-chart \
GATEWAY_CHART_NAME=oci://registry.registry.svc.cluster.local:5000/charts/gateway \
GATEWAY_CHART_VERSION=0.0.0-test \
TAGS=@gateway-lifecycle \
go run .
```

## Debugging Tips

### View Test Logs

```bash
make test-verbose
# Logs are saved to test-output.log
cat test-output.log
```

### Check Kubernetes Resources

```bash
# Check operator status
kubectl get pods -n operator

# Check Gateway status
kubectl get gateways -A

# Check RestApi status
kubectl get restapis -A

# View operator logs
kubectl logs -n operator -l app.kubernetes.io/name=gateway-operator
```

### Clean Up Test Resources

```bash
# Delete test namespaces
kubectl delete namespace test-ns scoped-test --ignore-not-found

# Uninstall operator
helm uninstall gateway-operator -n operator

# Clean test artifacts
make clean
```

### Port Forward Issues

If tests fail due to port forwarding:

```bash
# Kill any existing port forwards
pkill -f "kubectl port-forward"
```

## Writing New Tests

### 1. Create a Feature File

Add a new `.feature` file in the `features/` directory:

```gherkin
@my-feature
Feature: My New Feature
  As a user
  I want to test something
  So that it works correctly

  Background:
    Given namespace "default" exists
    And the operator is installed in namespace "operator"

  @my-scenario
  Scenario: Test something specific
    When I apply the following CR:
      """
      apiVersion: gateway.api-platform.wso2.com/v1alpha1
      kind: Gateway
      metadata:
        name: my-gateway
        namespace: default
      spec:
        gatewayClassName: "test"
      """
    Then Gateway "my-gateway" should be Programmed within 180 seconds
```

### 2. Add Step Definitions (if needed)

If your feature requires new steps, add them to the appropriate file in `steps/`:

- `k8s_steps.go` - Kubernetes resource operations
- `http_steps.go` - HTTP request operations
- `helm_steps.go` - Helm and operator operations
- `assert_steps.go` - Assertion operations

### 3. Run Your Tests

```bash
make test-tags TAGS=@my-feature
```

## Common Issues

### "No Kubernetes cluster available"

Ensure your kubectl context is set:
```bash
kubectl config use-context <your-context>
kubectl cluster-info
```

### "Image pull errors"

Ensure images are pushed to your registry:
```bash
make push-images DOCKER_HUB_USER=myuser
```

### "Helm chart not found"

Ensure the in-cluster registry is set up:
```bash
make setup-helm-registry
make push-helm-chart
```

### "Timeout waiting for resource"

Increase the timeout or check resource status:
```bash
make test TIMEOUT=60m
kubectl describe gateway <gateway-name> -n <namespace>
```
