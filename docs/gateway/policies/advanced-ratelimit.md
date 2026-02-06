# Advanced Rate Limiting

## Overview

The Advanced Rate Limiting policy applies one or more independent quotas to requests using token-bucket style algorithms. Each quota can define its own limits, key extraction, and optional cost extraction rules, enabling multi-dimensional rate limiting for APIs.

## Features

- Multiple independent quotas per route or API
- Multiple limits per quota (for example, per-second and per-hour windows)
- Key extraction from headers, metadata, IP, API name, route name, constants, or CEL expressions
- Cost extraction from request/response data with optional multipliers
- Distributed (Redis) or in-memory backends
- GCRA (smooth) and Fixed Window algorithms
- Customizable 429 responses
- Standard rate-limit response headers, including IETF `RateLimit` format

## Configuration

Advanced Rate Limiting uses two levels of configuration.

- System parameters live in `gateway/configs/config.toml` under `policy_configurations.ratelimit_v010`.
- User parameters are defined in the API configuration under `policies`.

### System Parameters (config.toml)

| Parameter | Type | Default | Description |
| --- | --- | --- | --- |
| `algorithm` | string | `"gcra"` | Rate limiting algorithm: `"gcra"` or `"fixed-window"`. |
| `backend` | string | `"memory"` | Storage backend: `"memory"` or `"redis"`. |
| `redis` | object | - | Redis configuration (used when `backend=redis`). |
| `memory` | object | - | Memory configuration (used when `backend=memory`). |
| `headers` | object | - | Controls which response headers are emitted. |

#### Redis Configuration

| Parameter | Type | Default | Description |
| --- | --- | --- | --- |
| `host` | string | `"localhost"` | Redis server hostname. |
| `port` | integer | `6379` | Redis server port. |
| `password` | string | `""` | Redis password (optional). |
| `username` | string | `""` | Redis ACL username (optional). |
| `db` | integer | `0` | Redis database number (0-15). |
| `keyPrefix` | string | `"ratelimit:v1:"` | Key prefix to avoid collisions. |
| `failureMode` | string | `"open"` | `"open"` allows requests if Redis is unavailable; `"closed"` denies. |
| `connectionTimeout` | string | `"5s"` | Connection timeout. |
| `readTimeout` | string | `"3s"` | Read timeout. |
| `writeTimeout` | string | `"3s"` | Write timeout. |

#### Memory Configuration

| Parameter | Type | Default | Description |
| --- | --- | --- | --- |
| `maxEntries` | integer | `10000` | Maximum in-memory entries. |
| `cleanupInterval` | string | `"5m"` | Cleanup interval. Use `"0"` to disable. |

#### Headers Configuration

| Parameter | Type | Default | Description |
| --- | --- | --- | --- |
| `includeXRateLimit` | boolean | `true` | Emit `X-RateLimit-*` headers. |
| `includeIETF` | boolean | `true` | Emit IETF `RateLimit` headers. |
| `includeRetryAfter` | boolean | `true` | Emit `Retry-After` on 429 responses. |

### User Parameters (API Definition)

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `quotas` | array | Yes | List of quota definitions. Each quota is enforced independently. |
| `keyExtraction` | array | No | Global key extraction for quotas that do not define their own. Defaults to `routename` when omitted. |
| `onRateLimitExceeded` | object | No | Customize the 429 response. |

#### Quota Configuration

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `name` | string | No | Optional quota name for logs and headers. |
| `limits` | array | Yes | One or more limits for this quota. All limits are enforced; the most restrictive one applies. |
| `keyExtraction` | array | No | Per-quota key extraction. Overrides global `keyExtraction`. |
| `costExtraction` | object | No | Optional dynamic cost extraction configuration. |

#### Limit Configuration

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `limit` | integer | Yes | Maximum tokens or requests in the duration. |
| `duration` | string | Yes | Go duration string (for example `"1s"`, `"1m"`, `"1h"`, `"24h"`). |
| `burst` | integer | No | Burst capacity (GCRA only). Defaults to `limit`. |

#### Key Extraction Configuration

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `type` | string | Yes | `header`, `metadata`, `ip`, `apiname`, `apiversion`, `routename`, `constant`, or `cel`. |
| `key` | string | Conditional | Required for `header`, `metadata`, and `constant`. |
| `expression` | string | Conditional | Required for `cel`. Expression must return a string. |

Key components are concatenated with `:` in the order provided. If you change the order, you change the bucket key.

#### Cost Extraction Configuration

When `costExtraction` is omitted or disabled, each request consumes a cost of `1`.

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `enabled` | boolean | No | Enables dynamic cost extraction. |
| `sources` | array | Conditional | Required when `enabled=true`. All successful extractions are summed. |
| `default` | number | No | Cost to use when extraction fails or no source succeeds. |

**Source types**

| `type` | Required fields | Phase |
| --- | --- | --- |
| `request_header` | `key` | Pre-request |
| `request_metadata` | `key` | Pre-request |
| `request_body` | `jsonPath` | Pre-request |
| `response_header` | `key` | Post-response |
| `response_metadata` | `key` | Post-response |
| `response_body` | `jsonPath` | Post-response |
| `request_cel` | `expression` | Pre-request |
| `response_cel` | `expression` | Post-response |

Each source can also specify `multiplier` (default `1.0`) to weight extracted values.

If you configure only response-phase sources, the gateway performs a pre-check (blocks when remaining quota is already exhausted) and then consumes the extracted cost after the response is received. This means a request can succeed and still exhaust the quota; subsequent requests will be limited based on the post-response consumption.

#### Rate Limit Exceeded Response

| Parameter | Type | Required | Description |
| --- | --- | --- | --- |
| `statusCode` | integer | No | HTTP status code (default `429`). |
| `body` | string | No | Response body string. |
| `bodyFormat` | string | No | `"json"` or `"plain"`. |

## Response Headers

When enabled in `headers`, the policy emits standard headers.

- `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset` for legacy clients.
- `RateLimit` and `RateLimit-Policy` for the IETF structured fields format. Quota names are included when configured.
- `X-RateLimit-Quota` on 429 responses to identify which quota was violated.
- `Retry-After` on 429 responses when enabled.

## Examples

### Example 1: Basic Per-Route Limit

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: ratelimit-basic-api
spec:
  displayName: RateLimit Basic API
  version: v1.0
  context: /ratelimit-basic/$version
  upstream:
    main:
      url: http://sample-backend:9080/api/v1
  operations:
    - method: GET
      path: /resource
      policies:
        - name: advanced-ratelimit
          version: v0
          params:
            quotas:
              - name: request-limit
                limits:
                  - limit: 4
                    duration: "1h"
```

### Example 2: Multi-Dimensional Quotas (Per-Route + Per-API)

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: ratelimit-quotas-api
spec:
  displayName: RateLimit Quotas API
  version: v1.0
  context: /ratelimit-quotas/$version
  upstream:
    main:
      url: http://sample-backend:9080/api/v1
  operations:
    - method: GET
      path: /multi
      policies:
        - name: advanced-ratelimit
          version: v0
          params:
            quotas:
              - name: per-route
                limits:
                  - limit: 5
                    duration: "1h"
                keyExtraction:
                  - type: routename
              - name: per-api
                limits:
                  - limit: 8
                    duration: "1h"
                keyExtraction:
                  - type: apiname
```

### Example 3: Response-Based Cost Extraction (LLM Tokens)

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: ratelimit-llm-api
spec:
  displayName: RateLimit LLM API
  version: v1.0
  context: /ratelimit-llm/$version
  upstream:
    main:
      url: http://echo-backend:80
  operations:
    - method: POST
      path: /chat
      policies:
        - name: advanced-ratelimit
          version: v0
          params:
            quotas:
              - name: prompt-tokens
                limits:
                  - limit: 500
                    duration: "1h"
                keyExtraction:
                  - type: header
                    key: X-User-ID
                costExtraction:
                  enabled: true
                  sources:
                    - type: response_body
                      jsonPath: "$.json.usage.prompt_tokens"
                      multiplier: 1.0
                  default: 0
```

### Example 4: CEL-Based Key Extraction

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: ratelimit-cel-key-api
spec:
  displayName: RateLimit CEL Key API
  version: v1.0
  context: /ratelimit-cel-key/$version
  upstream:
    main:
      url: http://sample-backend:9080/api/v1
  operations:
    - method: GET
      path: /resource
      policies:
        - name: advanced-ratelimit
          version: v0
          params:
            quotas:
              - name: per-user-cel
                limits:
                  - limit: 3
                    duration: "1h"
                keyExtraction:
                  - type: cel
                    expression: 'request.Headers["x-user-id"][0]'
```
