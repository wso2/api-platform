---
title: "Overview"
---
# Respond

## Overview

The Respond policy returns an immediate HTTP response to the client without forwarding the request to the upstream backend. This policy short-circuits the request processing pipeline in the request phase and is useful for API mocking, maintenance-mode responses, feature gating, and controlled error handling.

Because the response is generated at the gateway, no upstream call is made and response-phase policies are not involved for that request path.

## Features

- Returns immediate responses from the gateway request phase
- Skips upstream backend invocation entirely
- Configurable HTTP status code (`100`-`599`, default `200`)
- Configurable response body for JSON, text, XML, or other content
- Configurable custom response headers
- Useful for mocks, fallback responses, and temporary endpoint stubs
- Fail-fast validation for malformed `headers` parameter entries
- Deterministic response behavior independent of upstream availability

## Configuration

The Respond policy uses single-level API definition parameters and does not require any system-level configuration.

### User Parameters (API Definition)

These parameters are configured per-API/route by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `statusCode` | integer | No | `200` | HTTP status code for the immediate response. Valid range: 100-599. |
| `body` | string | No | `""` | Response body content returned to the client. Maximum length: 1048576 characters. |
| `headers` | array | No | `[]` | Array of response headers to include in the immediate response. |

### Headers Array Item

Each item in `headers` supports:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | Header name. Must match pattern `^[a-zA-Z0-9-_]+$`. Length: 1-256 characters. |
| `value` | string | Yes | Header value. Maximum length: 8192 characters. |

**Note:**

Inside the `gateway/build.yaml`, ensure the policy module is added under `policies:`:

```yaml
- name: respond
  gomodule: github.com/wso2/gateway-controllers/policies/respond@v0
```

## Reference Scenarios:

### Example 1: Simple Static Success Response

Return a static success message directly from the gateway:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: health-api-v1.0
spec:
  displayName: Health-API
  version: v1.0
  context: /health/$version
  upstream:
    main:
      url: http://sample-backend:5000
  policies:
    - name: respond
      version: v0
      params:
        statusCode: 200
        body: '{"status":"ok","source":"gateway"}'
        headers:
          - name: content-type
            value: application/json
  operations:
    - method: GET
      path: /status
```

**Response behavior:**

Incoming client request
```http
GET /health/v1.0/status HTTP/1.1
Host: api-gateway.company.com
```

Immediate gateway response
```http
HTTP/1.1 200 OK
content-type: application/json

{"status":"ok","source":"gateway"}
```

### Example 2: Maintenance Mode Response

Return `503 Service Unavailable` during planned downtime:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: maintenance-api-v1.0
spec:
  displayName: Maintenance-API
  version: v1.0
  context: /service/$version
  upstream:
    main:
      url: http://sample-backend:5000
  policies:
    - name: respond
      version: v0
      params:
        statusCode: 503
        body: '{"error":"Service Unavailable","message":"Scheduled maintenance in progress"}'
        headers:
          - name: content-type
            value: application/json
          - name: retry-after
            value: "300"
  operations:
    - method: GET
      path: /orders
    - method: POST
      path: /orders
```

**Response behavior:**

Incoming client request
```http
GET /service/v1.0/orders HTTP/1.1
Host: api-gateway.company.com
```

Immediate gateway response
```http
HTTP/1.1 503 Service Unavailable
content-type: application/json
retry-after: 300

{"error":"Service Unavailable","message":"Scheduled maintenance in progress"}
```

### Example 3: Route-Specific Mock Responses

Use route-level policy attachment for endpoint-specific mocks:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: mock-api-v1.0
spec:
  displayName: Mock-API
  version: v1.0
  context: /mock/$version
  upstream:
    main:
      url: http://sample-backend:5000
  operations:
    - method: GET
      path: /users
      policies:
        - name: respond
          version: v0
          params:
            statusCode: 200
            body: '[{"id":1,"name":"Alex"},{"id":2,"name":"Sam"}]'
            headers:
              - name: content-type
                value: application/json
    - method: GET
      path: /users/{id}
      policies:
        - name: respond
          version: v0
          params:
            statusCode: 404
            body: '{"error":"Not Found","message":"User not found"}'
            headers:
              - name: content-type
                value: application/json
```

**Response behavior:**

For `GET /users`
```http
HTTP/1.1 200 OK
content-type: application/json

[{"id":1,"name":"Alex"},{"id":2,"name":"Sam"}]
```

For `GET /users/{id}`
```http
HTTP/1.1 404 Not Found
content-type: application/json

{"error":"Not Found","message":"User not found"}
```

### Example 4: Plain Text and Custom Headers

Return plain text with custom diagnostic headers:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: text-response-api-v1.0
spec:
  displayName: Text-Response-API
  version: v1.0
  context: /text/$version
  upstream:
    main:
      url: http://sample-backend:5000
  policies:
    - name: respond
      version: v0
      params:
        statusCode: 202
        body: Accepted by gateway
        headers:
          - name: content-type
            value: text/plain
          - name: x-response-source
            value: gateway-respond-policy
  operations:
    - method: POST
      path: /submit
```

**Response behavior:**

Immediate gateway response
```http
HTTP/1.1 202 Accepted
content-type: text/plain
x-response-source: gateway-respond-policy

Accepted by gateway
```

### Example 5: Default Behavior (Minimal Configuration)

When parameters are omitted, status defaults to `200`, body is empty, and no headers are set:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: minimal-respond-api-v1.0
spec:
  displayName: Minimal-Respond-API
  version: v1.0
  context: /minimal/$version
  upstream:
    main:
      url: http://sample-backend:5000
  policies:
    - name: respond
      version: v0
  operations:
    - method: GET
      path: /ping
```

**Response behavior:**

Immediate gateway response
```http
HTTP/1.1 200 OK

```

## How it Works

* The policy executes in request phase and immediately returns a `policy.ImmediateResponse` to the client.
* If `statusCode` is omitted, the response status defaults to `200`.
* If `body` is omitted, the response body is empty.
* If `headers` are configured, they are added to the immediate response as provided.
* Header entries are validated at runtime; malformed `headers` entries return a `500 Configuration Error` response.
* Because response is produced in request phase, the upstream backend is not called.

## Limitations

1. **Short-Circuit Behavior**: Upstream services are never invoked when this policy executes.
2. **Request-Phase Only**: Response-phase policy logic is not applicable for requests terminated by this policy.
3. **Static Response Content**: Response body and headers are configuration-driven and not derived from upstream runtime data.
4. **Header Validation Strictness**: Invalid `headers` structure or field types result in `500 Configuration Error` responses.
5. **No Built-in Templating**: Body generation does not include dynamic templating logic by default.

## Notes

**Operational Usage**

Use this policy for controlled stubbing, maintenance windows, and temporary feature shutdowns without changing backend deployments. It is especially useful when you need deterministic responses even if upstream systems are unstable or unavailable. Apply route-level attachment when only specific operations should be short-circuited.

**Security and Governance**

Do not expose sensitive internal details in static response bodies or headers. Ensure custom error payloads are consistent with your API error contract and avoid leaking stack traces or internal identifiers. Restrict who can change policy parameters, since this policy can fully bypass backend enforcement paths.

**Performance and Reliability**

This policy reduces backend load and latency by terminating requests at the gateway. It can improve resilience during incidents by isolating failing upstreams while still returning predictable client responses. Monitor policy usage carefully to ensure temporary mock/maintenance rules are removed when no longer needed.
