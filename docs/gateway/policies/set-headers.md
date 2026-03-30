---
title: "Overview"
---
# Set Headers

## Overview

The Set Headers policy dynamically sets HTTP headers on incoming requests before they are forwarded to upstream services, and/or sets headers on outgoing responses before they are returned to clients. **Headers are set/replaced instead of appended**, which means existing headers with the same name will be overwritten with the new value.

## Features

- Sets custom headers on requests before forwarding to upstream services
- Sets custom headers on responses before returning to clients
- Supports both request and response phases independently or simultaneously
- **Overwrites headers instead of appending**: Existing headers with the same name are replaced
- Proper header name normalization (lowercase conversion for HTTP/2 compatibility)
- Static value assignment with support for special characters and complex values
- Works with any HTTP method and request type
- Last-value-wins behavior for duplicate header names in configuration
- Comprehensive validation of header configurations

## Configuration

The Set Headers policy can be configured for request phase, response phase, or both.
This policy does not require system-level configuration and operates entirely based on the configured header arrays.


### User Parameters (API Definition)

These parameters are configured per-API/route by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `request` | object | No | - | Specifies request-phase header settings. Must contain a `headers` array. At least one of `request` or `response` must be specified. |
| `response` | object | No | - | Specifies response-phase header settings. Must contain a `headers` array. At least one of `request` or `response` must be specified. |

### Request / Response Header Configuration

Each header entry in the `request.headers` or `response.headers` array must contain:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | The name of the HTTP header to set. Header names are automatically normalized to lowercase for consistency. Must match pattern `^[a-zA-Z0-9-_]+$` and be between 1 and 256 characters. |
| `value` | string | Yes | The value of the HTTP header to set. Can be static text, empty string, or contain special characters and complex values. Maximum length is 8192 characters. |

**Note:**
At least one of `request` or `response` must be specified in the policy configuration. The policy will fail validation if both are omitted.

Inside the `gateway/build.yaml`, ensure the policy module is added under `policies:`:

```yaml
- name: set-headers
  gomodule: github.com/wso2/gateway-controllers/policies/set-headers@v0
```

## Reference Scenarios:

### Example 1: Setting Request Headers for Authentication

Set authentication headers on all requests sent to upstream:

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
    - name: set-headers
      version: v0
      params:
        request:
          headers:
            - name: X-API-Key
              value: "12345-abcde-67890-fghij"
            - name: X-Client-Version
              value: "1.2.3"
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

**Request transformation (header set):**

Original client request
```http
GET /weather/v1.0/US/NewYork HTTP/1.1
Host: api-gateway.company.com
Accept: application/json
User-Agent: WeatherApp/1.0
```

Resulting upstream request
```http
GET /api/v2/US/NewYork HTTP/1.1
Host: sample-backend:5000
Accept: application/json
User-Agent: WeatherApp/1.0
x-api-key: 12345-abcde-67890-fghij
x-client-version: 1.2.3
```

### Example 2: Setting Response Headers for Security

Set security headers on all responses returned to clients:

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
    - name: set-headers
      version: v0
      params:
        response:
          headers:
            - name: X-Content-Type-Options
              value: "nosniff"
            - name: X-Frame-Options
              value: "DENY"
            - name: X-XSS-Protection
              value: "1; mode=block"
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

**Response transformation (header set):**

Original upstream response
```http
HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 256

{"temperature": 22, "humidity": 65}
```

Resulting client response
```http
HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 256
x-content-type-options: nosniff
x-frame-options: DENY
x-xss-protection: 1; mode=block

{"temperature": 22, "humidity": 65}
```

### Example 3: Setting Headers on Both Request and Response

Set headers on both requests (for upstream) and responses (for clients):

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
    - name: set-headers
      version: v0
      params:
        request:
          headers:
            - name: X-Source
              value: "api-gateway"
            - name: X-Request-ID
              value: "req-12345"
        response:
          headers:
            - name: X-Cache-Status
              value: "HIT"
            - name: X-Server-Version
              value: "2.1.0"
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

**Bidirectional transformation sample:**

Incoming client request headers
```http
GET /weather/v1.0/US/NewYork HTTP/1.1
Host: api-gateway.company.com
Accept: application/json
```

Forwarded upstream request headers
```http
GET /api/v2/US/NewYork HTTP/1.1
Host: sample-backend:5000
Accept: application/json
x-source: api-gateway
x-request-id: req-12345
```

Returned upstream response headers
```http
HTTP/1.1 200 OK
Content-Type: application/json
```

Final client response headers
```http
HTTP/1.1 200 OK
Content-Type: application/json
x-cache-status: HIT
x-server-version: 2.1.0
```

### Example 4: Route-Specific Headers

Apply different headers to different routes:

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
  operations:
    - method: GET
      path: /{country_code}/{city}
      policies:
        - name: set-headers
          version: v0
          params:
            request:
              headers:
                - name: X-Operation-Type
                  value: "weather-query"
            response:
              headers:
                - name: X-Data-Source
                  value: "weather-service"
    - method: GET
      path: /alerts/active
      policies:
        - name: set-headers
          version: v0
          params:
            request:
              headers:
                - name: X-Operation-Type
                  value: "alert-query"
            response:
              headers:
                - name: X-Real-Time
                  value: "true"
    - method: POST
      path: /alerts/active
      policies:
        - name: set-headers
          version: v0
          params:
            request:
              headers:
                - name: X-Operation-Type
                  value: "alert-create"
            response:
              headers:
                - name: X-Processing-Mode
                  value: "sync"
```

**Route-level transformation sample:**

For `GET /{country_code}/{city}`
```http
Request to upstream includes: x-operation-type: weather-query
Response to client includes: x-data-source: weather-service
```

For `GET /alerts/active`
```http
Request to upstream includes: x-operation-type: alert-query
Response to client includes: x-real-time: true
```

For `POST /alerts/active`
```http
Request to upstream includes: x-operation-type: alert-create
Response to client includes: x-processing-mode: sync
```

### Example 5: Overwriting Existing Headers (Set Behavior)

Demonstrate header overwriting behavior where existing headers with same name are replaced:

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
    - name: set-headers
      version: v0
      params:
        response:
          headers:
            - name: Cache-Control
              value: "public, max-age=3600"
            - name: Server
              value: "API-Gateway/2.1.0"
            - name: Content-Type
              value: "application/json; charset=utf-8"
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

**Response transformation (header overwrite):**

Original upstream response
```http
HTTP/1.1 200 OK
Content-Type: text/plain
Server: Apache/2.4.41
Cache-Control: no-cache
Content-Length: 256

{"temperature": 22, "humidity": 65}
```

Resulting client response (headers overwritten)
```http
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
Server: API-Gateway/2.1.0
Cache-Control: public, max-age=3600
Content-Length: 256

{"temperature": 22, "humidity": 65}
```


## How it Works

* The policy reads `request.headers` and `response.headers` arrays and applies them independently on request and response flows.
* Header names are normalized (trimmed and lowercased) before application to ensure consistent behavior across HTTP versions.
* Headers are applied using set semantics: if a header already exists, its value is replaced rather than appended.
* If the same header is configured multiple times in the same array, the last configured value wins.
* Request flow modifies outbound headers to upstream; response flow modifies outbound headers to clients.
* If no headers are configured for a flow, that flow passes through without header modification.


## Limitations

1. **Set-Only Behavior**: This policy replaces existing header values and does not append additional values.
2. **No Conditional Logic**: Header setting is static per policy configuration and cannot be conditional on runtime content.
3. **Configuration Dependency**: At least one of `request` or `response` must be configured; omitting both fails validation.
4. **Ordering Sensitivity**: Policy order affects final header values when combined with other header manipulation policies.
5. **Header Constraints Apply**: Header names and values must satisfy schema constraints (name pattern `^[a-zA-Z0-9-_]+$`, name max length 256, value max length 8192).


## Notes

**Security and Data Handling**

Avoid placing secrets, credentials, or personally identifiable data in headers unless strictly necessary, since headers can be logged or forwarded across multiple intermediaries. Validate and sanitize dynamic header values before injecting them to reduce header injection risks. For response headers exposed to clients, ensure values do not reveal sensitive internal topology or implementation details.

**Performance and Operational Impact**

Header setting is lightweight and local, but excessive or oversized headers increase request/response size and can impact proxy or load balancer limits. Keep header sets minimal and purposeful, especially on high-throughput APIs. Monitor for rejected requests caused by upstream or intermediary header-size constraints.

**Operational Best Practices**

Use clear naming conventions and document which headers are enforced at API level versus operation level to avoid conflicts. Apply route-specific policies when different operations require different header contracts. Test overwrite behavior explicitly—especially for standard headers like `content-type`, `cache-control`, and `server`—to ensure downstream systems and clients behave as expected.
