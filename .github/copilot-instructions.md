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

4. **Revert the gateway-controller image name before committing**. The `-coverage` suffix removal was only for local testing. Before creating a PR, restore the `-coverage` suffix of `gateway/it/docker-compose.test.yaml`:
   - Change: `ghcr.io/wso2/api-platform/gateway-controller:<version>`
   - Back to: `ghcr.io/wso2/api-platform/gateway-controller-coverage:<version>`
   
   **Important:** Do NOT include this image name change in your PR. The CI pipeline requires the coverage image. Other legitimate changes to docker-compose.test.yaml should still be committed.

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

3. **Revert the gateway-controller image name before committing**. Restore the `-coverage` suffix  of `gateway/it/docker-compose.test.yaml`:
   - Change: `ghcr.io/wso2/api-platform/gateway-controller:<version>`
   - Back to: `ghcr.io/wso2/api-platform/gateway-controller-coverage:<version>`
   
   **Important:** Do NOT include this image name change in your PR. Other legitimate changes to docker-compose.test.yaml should still be committed.

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

## Policy Configuration in Integration Tests

When writing integration tests for policies, you MUST understand how policy parameters work:

### Policy Parameter Types

Policies have two types of parameters defined in their `policy-definition.yaml` files (located in `gateway/policies/<policy-name>/v<version>/policy-definition.yaml`):

1. **`parameters`** (User Parameters): User-defined parameters that are specified in the API configuration when deploying an API. These are set by API developers via the REST API or API YAML.

2. **`systemParameters`** (System Parameters): Gateway admin-defined parameters that are configured in the gateway configuration file. These are NOT set in the API definition.

### How System Parameters Work

System parameters use the `"wso2/defaultValue"` attribute to reference values from the gateway config file using a JSON path syntax:

```yaml
# Example from a policy-definition.yaml
systemParameters:
  type: object
  properties:
    keyManagers:
      type: array
      description: List of key manager definitions
      "wso2/defaultValue": "${config.policy_configurations.jwtauth_v010.keymanagers}"
    jwksCacheTtl:
      type: string
      "wso2/defaultValue": "${config.policy_configurations.jwtauth_v010.jwkscachettl}"
```

The `"wso2/defaultValue"` path `${config.policy_configurations.jwtauth_v010.keymanagers}` tells the gateway to fetch the value from:

```yaml
# In gateway/it/test-config.yaml
policy_configurations:
  jwtauth_v010:
    keyManagers:
      - name: test-jwks
        issuer: http://mock-jwks.default.svc.cluster.local:8080/token
        jwks:
          remote:
            uri: http://mock-jwks:8080/jwks
```

### Setting Up Policy Tests

When creating integration tests for a policy:

1. **Check the policy definition** at `gateway/policies/<policy-name>/v<version>/policy-definition.yaml`

2. **Identify systemParameters**: Look for properties with `"wso2/defaultValue"` - these need gateway config entries

3. **Add system parameter values to test-config.yaml**: Add the required configuration under `policy_configurations` in `gateway/it/test-config.yaml` following the JSON path from `"wso2/defaultValue"`:
   - Path: `${config.policy_configurations.<policy_id>.<param_name>}`
   - Location: `gateway/it/test-config.yaml` → `policy_configurations` → `<policy_id>` → `<param_name>`
   - Note: The `<policy_id>` in the path typically follows the pattern `<policyname>_v<version>` with underscores (e.g., `jwtauth_v010` for `jwt-auth` v0.1.0)

4. **Use regular parameters in API definitions**: In your feature file's API YAML, only specify user `params` (not system parameters):

```yaml
# Example API definition in feature file
operations:
  - method: GET
    path: /protected
    policies:
      - name: jwt-auth
        version: v0.1.0
        params:                    # These are user parameters
          issuers:
            - test-jwks
          audiences:
            - my-api
          requiredScopes:
            - read
```

### Example: Adding JWT Auth Policy Test

1. **Policy definition** (`gateway/policies/jwt-auth/v0.1.0/policy-definition.yaml`) shows:
   - `parameters`: `issuers`, `audiences`, `requiredScopes`, etc. (user-defined)
   - `systemParameters`: `keyManagers`, `jwksCacheTtl`, etc. with `"wso2/defaultValue"` references

2. **Add to test-config.yaml**:
```yaml
policy_configurations:
  jwtauth_v010:
    keyManagers:
      - name: test-jwks
        issuer: http://mock-jwks:8080/token
        jwks:
          remote:
            uri: http://mock-jwks:8080/jwks
    jwkscachettl: "5m"
```

3. **In your feature file API definition**, only include user params:
```yaml
policies:
  - name: jwt-auth
    version: v0.1.0
    params:
      issuers:
        - test-jwks
```

### Key Points

- **NEVER** put systemParameters in the API definition - they come from the gateway config
- **ALWAYS** check the policy-definition.yaml for `"wso2/defaultValue"` to know what config entries are needed
- The config path uses the format: `policy_configurations.<policy_id>.<lowercase_param_name>`
- If a test fails with missing system parameters, check that `gateway/it/test-config.yaml` has the required `policy_configurations` entries

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
