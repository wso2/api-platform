# CORS Policy

## Overview

The CORS (Cross-Origin Resource Sharing) policy handles cross-origin requests by validating preflight requests and adding appropriate CORS headers to responses. This policy enables controlled access to resources from different origins by configuring allowed origins, methods, headers, and credentials.

## Features

- Handles CORS preflight (OPTIONS) requests with automatic validation
- Configurable allowed origins with wildcard and regex pattern support
- Control over allowed HTTP methods for cross-origin requests
- Request header validation and filtering
- Configurable exposed response headers for client-side access
- Support for credentials in cross-origin requests
- Preflight response caching with configurable max age
- Option to forward non-compliant preflight requests to upstream service
- Validates CORS constraints (e.g., credentials with wildcard origins)

## Configuration

The CORS policy uses a single-level configuration model where all parameters are configured per-API in the API definition YAML. This policy does not require system-level configuration.

> **Important**: The CORS policy MUST be applied at the API level only, not at individual resource/operation level. Additionally, you MUST explicitly define OPTIONS operations for all resources where you expect to handle CORS preflight requests. This is not needed for MCP Proxies as the gateway internally handles the operations for them. The gateway will automatically handle preflight requests (OPTIONS method) when they match the allowed origins and methods.

### User Parameters (API Definition)

These parameters are configured per-API/route by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `allowedOrigins` | array | Yes | - | List of origins allowed to access the resource. Use `"*"` to allow all origins, or specify exact origins (e.g., `"https://example.com"`) or regex patterns. At least one origin must be specified. When using credentials, specific origins must be listed (no wildcards). |
| `allowedMethods` | array | No | `["GET", "POST", "PUT", "DELETE", "OPTIONS"]` | HTTP methods allowed for cross-origin requests. Valid values: GET, POST, PUT, DELETE, PATCH, OPTIONS, HEAD. |
| `allowedHeaders` | array | No | `["*"]` | Request headers that can be used in the actual request. Use `"*"` to allow all headers, or specify exact header names (e.g., `"Content-Type"`, `"Authorization"`). When using credentials, specific headers must be listed (no wildcards). |
| `exposedHeaders` | array | No | `[]` | Response headers that browsers are allowed to access. Only these headers will be exposed to the client-side JavaScript code. |
| `allowCredentials` | boolean | No | `false` | Indicates whether the response can be shared when credentials (cookies, authorization headers, TLS certificates) are included. When true, `allowedOrigins` cannot contain `"*"` and `allowedHeaders` cannot contain `"*"`. |
| `maxAge` | integer | No | `3600` | Maximum time in seconds that a preflight response can be cached by the browser. Valid range: 0 to 86400. Helps reduce the number of preflight requests. |
| `forwardPreflight` | boolean | No | `false` | If true, forwards preflight requests that do not match the CORS policy to the upstream service instead of responding with CORS headers. |

## API Definition Examples

### Example 1: Basic CORS Configuration (Allow All Origins)

Enable CORS for a public API that allows requests from any origin:

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
    - name: cors
      version: v0.1.0
      params:
        allowedOrigins:
          - "*"
        allowedMethods:
          - GET
          - POST
          - OPTIONS
        allowedHeaders:
          - "*"
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: POST
      path: /alerts/active
    - method: OPTIONS
      path: /{country_code}/{city}
    - method: OPTIONS
      path: /alerts/active
```

### Example 2: Specific Origins with Credentials

Allow requests from specific origins with credential support:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: user-api-v1.0
spec:
  displayName: User-API
  version: v1.0
  context: /users/$version
  upstream:
    main:
      url: http://sample-backend:5000/api
  policies:
    - name: cors
      version: v0.1.0
      params:
        allowedOrigins:
          - "https://app.example.com"
          - "https://admin.example.com"
        allowedMethods:
          - GET
          - POST
          - PUT
          - DELETE
          - OPTIONS
        allowedHeaders:
          - "Content-Type"
          - "Authorization"
          - "X-Requested-With"
        exposedHeaders:
          - "X-Total-Count"
          - "X-Page-Number"
        allowCredentials: true
        maxAge: 7200
  operations:
    - method: GET
      path: /profile
    - method: POST
      path: /profile
    - method: PUT
      path: /profile/{id}
    - method: OPTIONS
      path: /profile
    - method: OPTIONS
      path: /profile/{id}
```

### Example 3: Regex Pattern Origins

Allow origins matching a regex pattern:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: api-v1.0
spec:
  displayName: API
  version: v1.0
  context: /api/$version
  upstream:
    main:
      url: http://sample-backend:5000
  policies:
    - name: cors
      version: v0.1.0
      params:
        allowedOrigins:
          - "https://.*\\.example\\.com"  # Matches any subdomain of example.com
          - "http://localhost:3000"
          - "http://localhost:8080"
        allowedMethods:
          - GET
          - POST
          - PUT
          - OPTIONS
        allowedHeaders:
          - "Content-Type"
          - "Authorization"
        maxAge: 3600
  operations:
    - method: GET
      path: /data
    - method: POST
      path: /data
    - method: OPTIONS
      path: /data
```

### Example 4: Limited Headers with Exposed Headers

Control which headers can be sent and which can be accessed:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: public-api-v1.0
spec:
  displayName: Public-API
  version: v1.0
  context: /public/$version
  upstream:
    main:
      url: http://sample-backend:5000
  policies:
    - name: cors
      version: v0.1.0
      params:
        allowedOrigins:
          - "https://example.com"
        allowedMethods:
          - GET
          - POST
          - OPTIONS
        allowedHeaders:
          - "Content-Type"
          - "Accept"
        exposedHeaders:
          - "Content-Type"
          - "X-RateLimit-Limit"
          - "X-RateLimit-Remaining"
          - "X-RateLimit-Reset"
        maxAge: 1800
  operations:
    - method: GET
      path: /products
    - method: POST
      path: /products/search
    - method: OPTIONS
      path: /products
    - method: OPTIONS
      path: /products/search
```

### Example 5: Forward Non-Compliant Preflight Requests

Forward preflight requests that don't match the policy to upstream:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: dynamic-api-v1.0
spec:
  displayName: Dynamic-API
  version: v1.0
  context: /dynamic/$version
  upstream:
    main:
      url: http://sample-backend:5000
  policies:
    - name: cors
      version: v0.1.0
      params:
        allowedOrigins:
          - "https://app.example.com"
        allowedMethods:
          - GET
          - POST
          - PUT
          - OPTIONS
        allowedHeaders:
          - "*"
        forwardPreflight: true
        maxAge: 3600
  operations:
    - method: GET
      path: /resource/{id}
    - method: POST
      path: /resource
    - method: PUT
      path: /resource/{id}
    - method: OPTIONS
      path: /resource
    - method: OPTIONS
      path: /resource/{id}
```

## Important Considerations

### API-Level Configuration Requirement

The CORS policy MUST be applied at the API level in the `policies` section of the API definition, not at individual operation/resource level. Even though the policy framework technically supports operation-level configuration, CORS handling requires API-level application to work correctly with preflight requests.

Additionally, you MUST explicitly define OPTIONS operations for each resource path where you expect to handle CORS preflight requests. For example:

```yaml
operations:
  - method: GET
    path: /users
  - method: POST
    path: /users
  - method: OPTIONS      # Required for CORS preflight
    path: /users
  - method: GET
    path: /users/{id}
  - method: PUT
    path: /users/{id}
  - method: DELETE
    path: /users/{id}
  - method: OPTIONS      # Required for CORS preflight
    path: /users/{id}
```

### CORS Constraints

When `allowCredentials` is set to `true`, the following constraints apply:

- `allowedOrigins` cannot contain `"*"` (wildcard). You must specify explicit origins.
- `allowedHeaders` cannot contain `"*"` (wildcard). You must specify explicit header names.
- `allowedMethods` cannot contain `"*"` (wildcard). You must specify explicit methods.
- `exposedHeaders` cannot contain `"*"` (wildcard). You must specify explicit header names.

These constraints are enforced by the CORS specification to prevent security issues when credentials are involved.

### Regex Pattern Origins

You can use regex patterns for `allowedOrigins` to match multiple origins dynamically. For example:

- `"https://.*\\.example\\.com"` matches `https://app.example.com`, `https://api.example.com`, etc.
- `"http://localhost:(3000|8080)"` matches `http://localhost:3000` and `http://localhost:8080`

### Preflight Caching

The `maxAge` parameter controls how long browsers cache the preflight response. A higher value reduces preflight requests but means changes to CORS configuration take longer to take effect. Recommended values:

- Development: 300-600 seconds (5-10 minutes)
- Production: 3600-86400 seconds (1-24 hours)

### Forward Preflight

When `forwardPreflight` is enabled, preflight requests that don't match the CORS policy are forwarded to the upstream service. This is useful when the upstream service handles CORS validation directly. By default, non-compliant requests receive an empty response.
