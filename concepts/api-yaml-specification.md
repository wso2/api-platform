# API.yaml Specification

## Overview

The `api.yaml` format is a declarative specification for defining APIs that can be deployed to the API Platform Gateway. It follows a Kubernetes-like CR (Custom Resource) style but is not a Kubernetes CR itself.

> **Note**: This format looks like a Kubernetes CR but is platform-agnostic. A K8s CR version may be introduced in the future.

---

## Basic Structure

```yaml
version: api-platform.wso2.com/v1
kind: http/rest | graphql | grpc | asyncapi
data:
  # API metadata and configuration
```

---

## Schema Reference

### Top-Level Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `version` | string | Yes | API specification version (currently `api-platform.wso2.com/v1`) |
| `kind` | string | Yes | API type: `http/rest`, `graphql`, `grpc`, `asyncapi` |
| `data` | object | Yes | API configuration and metadata |

### Data Object Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | API name (display name) |
| `version` | string | Yes | API version (e.g., `v1.0`, `v2.0`) |
| `context` | string | Yes | Base path for the API (e.g., `/weather`, `/api/v1`) |
| `upstream` | array | Yes | Backend service configuration |
| `operations` | array | Yes | API operations/endpoints |

### Upstream Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `url` | string | Yes | Backend service URL (e.g., `https://api.example.com/v2`) |

### Operation Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `method` | string | Yes | HTTP method: `GET`, `POST`, `PUT`, `PATCH`, `DELETE` |
| `path` | string | Yes | Operation path with optional path parameters (e.g., `/{id}`, `/{country}/{city}`) |
| `requestPolicies` | array | No | Policies applied to requests |
| `responsePolicies` | array | No | Policies applied to responses |

### Policy Configuration

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Policy name (e.g., `apiKey`, `jwt`, `rateLimit`) |
| `version` | string | Yes | Policy version; must be **major-only** (e.g., `v0`, `v1`). Full semantic versions (e.g., `v1.0.0`) are not allowed. |
| `params` | object | No | Policy-specific parameters |

---

## Complete Example

### REST API with API Key Authentication

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
data:
  name: Weather API
  version: v1.0
  context: /weather
  upstream:
    - url: https://api.weather.com/v2
  operations:
    - method: GET
      path: /{country_code}/{city}
      requestPolicies:
        - name: apiKey
          params:
            header: api-key
```

### Multiple Operations Example

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
data:
  name: User Management API
  version: v1.0
  context: /api/users
  upstream:
    - url: https://backend.example.com
  operations:
    - method: GET
      path: /
      requestPolicies:
        - name: jwt
          params:
            header: Authorization

    - method: GET
      path: /{id}
      requestPolicies:
        - name: jwt
          params:
            header: Authorization

    - method: POST
      path: /
      requestPolicies:
        - name: jwt
          params:
            header: Authorization
        - name: rateLimit
          params:
            limit: 100
            window: 60s

    - method: PUT
      path: /{id}
      requestPolicies:
        - name: jwt
          params:
            header: Authorization

    - method: DELETE
      path: /{id}
      requestPolicies:
        - name: jwt
          params:
            header: Authorization
```

---

## Common Policy Types

### Authentication Policies

#### API Key Authentication
```yaml
requestPolicies:
  - name: apiKey
    params:
      header: api-key  # or X-API-Key, etc.
```

#### JWT Authentication
```yaml
requestPolicies:
  - name: jwt
    params:
      header: Authorization
      issuer: https://auth.example.com
```

#### OAuth 2.0
```yaml
requestPolicies:
  - name: oauth2
    params:
      tokenEndpoint: https://auth.example.com/token
      scopes: read write
```

### Rate Limiting Policies

#### Request Rate Limit
```yaml
requestPolicies:
  - name: rateLimit
    params:
      limit: 100        # requests
      window: 60s       # time window
```

#### Quota Management
```yaml
requestPolicies:
  - name: quota
    params:
      limit: 10000
      period: daily
```

### Other Common Policies

#### CORS
```yaml
requestPolicies:
  - name: cors
    params:
      allowOrigins: "*"
      allowMethods: GET, POST, PUT, DELETE
      allowHeaders: Content-Type, Authorization
```

#### Request Transformation
```yaml
requestPolicies:
  - name: headerModify
    params:
      add:
        X-Custom-Header: value
      remove:
        - X-Unwanted-Header
```

---

## Path Parameters

Path parameters are denoted using curly braces `{}`:

```yaml
operations:
  - method: GET
    path: /{country_code}/{city}  # Two path parameters

  - method: GET
    path: /users/{userId}/orders/{orderId}  # Nested resources
```

---

## Best Practices

### 1. Versioning
- Always include a version in your API name or context
- Use semantic versioning (e.g., `v1.0`, `v2.1`)

### 2. Context Paths
- Use clear, descriptive context paths
- Avoid deep nesting (keep it simple)
- Examples: `/weather`, `/api/users`, `/v1/products`

### 3. Policy Organization
- Apply authentication policies at the operation level for flexibility
- Use consistent policy naming across APIs
- Document custom policy requirements

### 4. Path Parameters
- Use descriptive parameter names (e.g., `{userId}` instead of `{id}`)
- Keep path parameter count reasonable (3-4 max)

### 5. Documentation
- Include clear, descriptive API names
- Version your APIs consistently
- Document expected request/response formats separately

---

## Future Enhancements

The following features are under consideration:

- [ ] Import from OpenAPI/Swagger specifications
- [ ] GraphQL schema integration
- [ ] AsyncAPI event definitions
- [ ] Kubernetes Custom Resource (CR) version
- [ ] Advanced routing and traffic splitting
- [ ] Service mesh integration

---

## Related Documentation

- [Gateway Specification](../gateway/spec/spec.md) - Deploy API.yaml to the gateway
- [CLI Documentation](../cli/spec/spec.md) - Commands for deploying and managing APIs

---

**Version**: 1.0
**Last Updated**: 2025-10-06
**Status**: Draft
