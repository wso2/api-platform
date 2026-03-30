---
title: "Overview"
---
# API Key Authentication

## Overview

The API Key Authentication policy validates API keys to secure APIs by verifying pre-generated keys before allowing access to protected resources. This policy is essential for API security, supporting both header-based and query parameter-based key validation.

## Features

- Validates API keys from request headers or query parameters
- Configurable key extraction from headers (case-insensitive) or query parameters (exact match)
- Pre-generated key validation against gateway-managed key lists
- Request context enrichment with authentication metadata
- Issuer-based key validation via system configuration
- Streaming-compatible request processing (header-phase only)

## Configuration

The API Key Authentication policy uses a two-level configuration model. User parameters are configured per-API/route in the API definition YAML, and system parameters are resolved from gateway configuration.

### User Parameters (API Definition)

These parameters are configured per-API/route by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `key` | string | Yes | `API-Key` | The name of the header or query parameter that contains the API key. For headers: case-insensitive matching is used (e.g., "X-API-Key", "Authorization"). For query parameters: exact name matching is used (e.g., "api_key", "token"). Length: 1-128 characters. |
| `in` | string | Yes | `header` | Specifies where to look for the API key. Must be either "header" or "query". |

### System Parameters (config.toml)

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `issuer` | string | No | `""` | Identifier of the portal associated with this gateway. Resolved from `config.toml` `[api_key] issuer` at startup. When set to a non-empty string, the policy rejects any API key whose issuer field is non-null and does not match this value. When empty or absent, the issuer check is skipped entirely. |

#### Sample System Configuration

```toml
[api_key]
issuer = "https://portal.example.com"
```

**Note:**

Inside the `gateway/build.yaml`, ensure the policy module is added under `policies:`:

```yaml
- name: api-key-auth
  gomodule: github.com/wso2/gateway-controllers/policies/api-key-auth@v0
```

## Reference Scenarios

### Example 1: Basic API Key Authentication (Header)

Apply API key authentication using a custom header

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: weather-api-v1.0
spec:
  displayName: Weather-API
  version: v1.0
  context: /weather/$version
  upstream:
    main:
      url: http://sample-backend:5000/api/v2
  policies:
    - name: api-key-auth
      version: v0
      params:
        key: X-API-Key
        in: header
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 2: Default Header Name

Use the default API-Key header name

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: weather-api-v1.0
spec:
  displayName: Weather-API
  version: v1.0
  context: /weather/$version
  upstream:
    main:
      url: http://sample-backend:5000/api/v2
  policies:
    - name: api-key-auth
      version: v0
      params:
        key: API-Key
        in: header
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 3: Query Parameter Authentication

Extract API key from query parameter

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: weather-api-v1.0
spec:
  displayName: Weather-API
  version: v1.0
  context: /weather/$version
  upstream:
    main:
      url: http://sample-backend:5000/api/v2
  policies:
    - name: api-key-auth
      version: v0
      params:
        key: api_key
        in: query
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 4: Custom Header Name

Use a custom header for API key authentication

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: weather-api-v1.0
spec:
  displayName: Weather-API
  version: v1.0
  context: /weather/$version
  upstream:
    main:
      url: http://sample-backend:5000/api/v2
  policies:
    - name: api-key-auth
      version: v0
      params:
        key: X-Custom-Auth
        in: header
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 5: Route-Specific Authentication

Apply different API key configurations to different routes

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: weather-api-v1.0
spec:
  displayName: Weather-API
  version: v1.0
  context: /weather/$version
  upstream:
    main:
      url: http://sample-backend:5000/api/v2
  policies:
    - name: api-key-auth
      version: v0
      params:
        key: X-Custom-Auth
        in: header
  operations:
    - method: GET
      path: /{country_code}/{city}
      policies:
        - name: api-key-auth
          version: v0
          params:
            key: X-API-Key
            in: header
    - method: GET
      path: /alerts/active
      policies:
        - name: api-key-auth
          version: v0
          params:
            key: Authorization
            in: header
    - method: POST
      path: /alerts/active
```

## How it Works

- On each request, the gateway policy reads `key` and `in` from the policy configuration and validates that required parameters are present.

- Based on `in`, it extracts the API key either from a request header (case-insensitive header lookup) or from a query parameter in the request URL.

- If the key is missing, empty, or the required API context values are unavailable, the policy short-circuits the request and returns `401 Unauthorized` with a JSON error response.

- For valid inputs, the policy calls the API key store validator using API and operation context (`apiId`, operation path, HTTP method) to determine whether the key is allowed for the target operation.

- If an `issuer` is configured at the system level, the policy additionally verifies that the API key's issuer field matches the configured value. Keys with a non-null issuer that does not match are rejected.

- On successful validation, the request continues upstream and authentication metadata is added to the shared context (`auth.success=true`, `auth.method=api-key`). The policy does not modify response traffic.

- Key lifecycle and control-plane capabilities still apply, but are handled outside this gateway runtime policy: quota enforcement (including `remaining_api_key_quota` in key management APIs), key generation/regeneration, key format, secure hashing/storage, masking, access control, and audit logging.


## Notes:

- API keys offer a lightweight, secure authentication mechanism for internal services, partner and third-party integrations, legacy systems, development and testing environments, and service-to-service communication, providing a practical alternative to complex OAuth flows while ensuring controlled access through HTTPS-only transmission, secure hashing, masking, and constant-time validation.

- Store API keys securely, never exposing them in client-side code, logs, or version control systems.

- The platform enforces access control, audit logging, and quota limits to prevent abuse and support traceability. To maintain security over time, keys should be regenerated regularly, handled carefully in logs and query parameters, and revoked immediately if compromised.

- Use clear, descriptive naming and maintain separate keys per environment (development, staging, production) to simplify management.

- Always transmit API keys over HTTPS only and ensure logging practices do not inadvertently expose sensitive key material.

