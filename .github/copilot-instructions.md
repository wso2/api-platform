# Copilot Coding Agent Instructions

This document provides instructions for the GitHub Copilot coding agent when working on this repository.

## Project Structure

- `gateway/` - Gateway components (controller, policy-engine, router)
  - `gateway/gateway-controller/` - Gateway controller service
  - `gateway/gateway-runtime/policy-engine/` - Policy engine service
  - `gateway/gateway-runtime/router/` - Envoy-based router
- `kubernetes/` - Kubernetes operator and Helm charts
  - `kubernetes/gateway-operator/` - Gateway Kubernetes operator
- `tests/framework/` - Integration test framework; the gateway integration tests are in `tests/framework/suites/it/`

## Running Integration Tests

The gateway integration tests run on the integration framework in `tests/framework` (see
`tests/framework/README.md`, and `tests/framework/CLAUDE.md` for how to write features and steps).
The framework builds the gateway images it tests from source, so a run always tests the current
code, whether you changed gateway components or only test files.

1. **Build the testbench image** (the mock backends), once and after changing `tests/framework/testbench/**`:
   ```bash
   make -C tests/framework testbench
   ```

2. **Run the tests**. The `gateway-core` block on SQLite:
   ```bash
   make -C gateway test-integration
   ```
   Or the block and feature you changed. Find the block in `tests/framework/suites/it/it-suite.yaml`
   and select the feature by its tag:
   ```bash
   cd tests/framework
   go test ./suites/it -count=1 -v -timeout=60m -blocks <block>/sqlite -feature-tags "@<feature-tag>"
   ```

3. **Restore `gateway/build-manifest.yaml` before committing.** A source build rewrites it with the
   newest policy versions:
   ```bash
   git restore gateway/build-manifest.yaml
   ```

## Build Commands Reference

| Command | Description | Working Directory |
|---------|-------------|-------------------|
| `make build` | Build all gateway Docker images locally | `gateway/` |
| `make test-integration` | Run the gateway-core integration tests on SQLite | `gateway/` |
| `make test-integration-all` | Run the whole integration framework suite | `gateway/` |

## Policy Configuration in Integration Tests

When writing integration tests for policies, you MUST understand how policy parameters work:

### Policy Parameter Types

Policies have two types of parameters defined in their `policy-definition.yaml` files (in the [wso2/gateway-controllers](https://github.com/wso2/gateway-controllers) repository, at `policies/<policy-name>/policy-definition.yaml`). Most policies' own integration tests live there too, in `policies/<policy-name>/it/`; the tests in this repository cover policies only as part of gateway behaviour:

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
      "wso2/defaultValue": "${config.policy_configurations.jwtauth_v1.keymanagers}"
    jwksCacheTtl:
      type: string
      "wso2/defaultValue": "${config.policy_configurations.jwtauth_v1.jwkscachettl}"
```

The `"wso2/defaultValue"` path `${config.policy_configurations.jwtauth_v1.keymanagers}` tells the gateway to fetch the value from:

```toml
# In the gateway configuration of the block under test, e.g. an overlay in
# tests/framework/core/catalog/overlays/
[[policy_configurations.jwtauth_v1.keymanagers]]
name = "mock-jwks"
issuer = "http://testbench:3001/token"

[policy_configurations.jwtauth_v1.keymanagers.jwks.remote]
uri = "http://testbench:3001/jwks"
```

### Setting Up Policy Tests

When creating integration tests for a policy:

1. **Check the policy definition** at `policies/<policy-name>/policy-definition.yaml` in wso2/gateway-controllers

2. **Identify systemParameters**: Look for properties with `"wso2/defaultValue"` - these need gateway config entries

3. **Add system parameter values to the block's gateway configuration**: Add the required configuration under `policy_configurations` in the overlay the block uses (`tests/framework/core/catalog/overlays/<name>.toml`, referenced by the block's `overlay:` in `tests/framework/suites/it/it-suite.yaml`), following the JSON path from `"wso2/defaultValue"`:
   - Path: `${config.policy_configurations.<policy_id>.<param_name>}`
   - Location: the overlay → `[policy_configurations.<policy_id>]` → `<param_name>`
   - Note: The `<policy_id>` in the path typically follows the pattern `<policyname>_v<version>` with underscores (e.g., `jwtauth_v1` for `jwt-auth` v1)

4. **Use regular parameters in API definitions**: In your feature file's API YAML, only specify user `params` (not system parameters):

```yaml
# Example API definition in feature file
operations:
  - method: GET
    path: /protected
    policies:
      - name: jwt-auth
        version: v1
        params:                    # These are user parameters
          issuers:
            - mock-jwks
          audiences:
            - my-api
          requiredScopes:
            - read
```

### Example: Adding JWT Auth Policy Test

1. **Policy definition** (`policies/jwt-auth/policy-definition.yaml` in wso2/gateway-controllers) shows:
   - `parameters`: `issuers`, `audiences`, `requiredScopes`, etc. (user-defined)
   - `systemParameters`: `keyManagers`, `jwksCacheTtl`, etc. with `"wso2/defaultValue"` references

2. **Add to the block's overlay**:
```toml
[[policy_configurations.jwtauth_v1.keymanagers]]
name = "mock-jwks"
issuer = "http://testbench:3001/token"

[policy_configurations.jwtauth_v1.keymanagers.jwks.remote]
uri = "http://testbench:3001/jwks"

[policy_configurations.jwtauth_v1]
jwkscachettl = "5m"
```

3. **In your feature file API definition**, only include user params:
```yaml
policies:
  - name: jwt-auth
    version: v1
    params:
      issuers:
        - mock-jwks
```

### Key Points

- **NEVER** put systemParameters in the API definition - they come from the gateway config
- **ALWAYS** check the policy-definition.yaml for `"wso2/defaultValue"` to know what config entries are needed
- The config path uses the format: `policy_configurations.<policy_id>.<lowercase_param_name>`
- If a test fails with missing system parameters, check that the block's overlay has the required `policy_configurations` entries

## Unit Tests

To run unit tests for individual components:

```bash
# Gateway Controller
cd gateway/gateway-controller && go test ./...

# Policy Engine
cd gateway/gateway-runtime/policy-engine && go test ./...

# Gateway Operator
cd kubernetes/gateway-operator && go test ./...
```
