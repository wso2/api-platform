# Copilot Coding Agent Instructions

This document provides instructions for the GitHub Copilot coding agent when working on this repository.

## Project Structure

- `gateway/` - Gateway components (controller, policy-engine, router)
  - `gateway/gateway-controller/` - Gateway controller service
  - `gateway/policy-engine/` - Policy engine service
  - `gateway/router/` - Envoy-based router
  - `gateway/it/` - Integration tests
- `kubernetes/` - Kubernetes operator and Helm charts
  - `kubernetes/gateway-operator/` - Gateway Kubernetes operator

## Running Integration Tests

### When modifying code in `./gateway/**` (excluding `./gateway/it/`)

If you change any code in the gateway components (gateway-controller, policy-engine, router, etc.), you MUST:

1. **Rebuild the Docker images** to include your changes:
   ```bash
   cd gateway && make build-local
   ```

2. **Modify the test configuration** to use the non-coverage images. Edit `gateway/it/docker-compose.test.yaml` line 25 and remove the `-coverage` suffix from the gateway-controller image name:
   - Change: `ghcr.io/wso2/api-platform/gateway-controller-coverage:<version>`
   - To: `ghcr.io/wso2/api-platform/gateway-controller:<version>`
   
   **Important:** Keep the existing version tag (e.g., `0.3.0-SNAPSHOT`) as-is. The version in the file is the current version - do not hardcode a different version.

3. **Run the integration tests**:
   ```bash
   cd gateway/it && make test
   ```

### When modifying only `./gateway/it/**` (integration test files only)

If you are only modifying integration test files (test cases, test utilities, etc.) and not the gateway component source code:

1. **Modify the test configuration** to use the non-coverage images. Edit `gateway/it/docker-compose.test.yaml` line 25 and remove the `-coverage` suffix from the gateway-controller image name:
   - Change: `ghcr.io/wso2/api-platform/gateway-controller-coverage:<version>`
   - To: `ghcr.io/wso2/api-platform/gateway-controller:<version>`
   
   **Important:** Keep the existing version tag as-is. Do not change the version number.

2. **Run the integration tests**:
   ```bash
   cd gateway/it && make test
   ```

Note: The images are pre-built during the setup phase, so you can run tests directly without rebuilding.

## Important Notes

- The `copilot-setup-steps.yml` workflow pre-builds gateway images using `make build-local`
- The docker-compose file by default references coverage images (for CI coverage reporting)
- You must switch to non-coverage images when running tests in the Copilot environment
- **Version tags:** Always use whatever version tag is currently in the docker-compose.test.yaml file. The version may change over time (e.g., `0.3.0-SNAPSHOT`, `0.4.0-SNAPSHOT`, etc.) - never hardcode a specific version

## Build Commands Reference

| Command | Description | Working Directory |
|---------|-------------|-------------------|
| `make build-local` | Build all gateway Docker images locally | `gateway/` |
| `make test` | Run integration tests | `gateway/it/` |

## Unit Tests

To run unit tests for individual components:

```bash
# Gateway Controller
cd gateway/gateway-controller && go test ./...

# Policy Engine
cd gateway/policy-engine && go test ./...

# Router
cd gateway/router && go test ./...

# Gateway Operator
cd kubernetes/gateway-operator && go test ./...
```
