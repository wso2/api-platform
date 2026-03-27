---
title: "Overview"
---
# Rate Limiting (Advanced)

## Overview

The Rate Limiting policy controls the rate of requests to your APIs by enforcing configurable limits based on various criteria. This policy is essential for protecting backend services from overload, ensuring fair usage, and maintaining service availability.

## Features

- Multiple rate limiting algorithms (GCRA, Fixed Window)
- Weighted rate limiting via cost parameter
- Post-response cost extraction for dynamic rate limiting (e.g., LLM token usage)
- Multiple concurrent limits (e.g., 10/second AND 1000/hour)
- Flexible key extraction (headers, metadata, IP, API name, route name)
- Dual backends: in-memory (single instance) or Redis (distributed)
- Graceful degradation with fail-open/fail-closed modes for Redis failures
- Comprehensive rate limit headers (X-RateLimit-*, IETF RateLimit, Retry-After)
- Customizable error responses

## Configuration

The Rate Limiting policy uses a two-level configuration

### System Parameters (From config.toml)

These parameters are set by the administrator and apply globally to all rate limiting policies:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `algorithm` | string | No | `"fixed-window"` | Rate limiting algorithm: `"gcra"` (smooth rate limiting with burst support) or `"fixed-window"` (simple counter per time window). |
| `backend` | string | No | `"memory"` | Storage backend: `"memory"` for single-instance or `"redis"` for distributed rate limiting. |
| `redis` | ```Redis``` object | No | - | Redis configuration (only used when `backend=redis`). |
| `memory` | ```Memory``` object | No | - | In-memory storage configuration (only used when `backend=memory`). |
| `headers` | ```Headers``` object | No | - | Control which rate limit headers are included in responses. |

#### Redis Configuration

When using Redis backend, the following parameters can be configured under `redis`:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `host` | string | No | `"localhost"` | Redis server hostname or IP address. |
| `port` | integer | No | `6379` | Redis server port. |
| `password` | string | No | `""` | Redis authentication password (optional). |
| `username` | string | No | `""` | Redis ACL username (optional, Redis 6+). |
| `db` | integer | No | `0` | Redis database number (0-15). |
| `keyPrefix` | string | No | `"ratelimit:v1:"` | Prefix for all Redis keys to avoid conflicts. |
| `failureMode` | string | No | `"open"` | Behavior when Redis is unavailable: `"open"` allows requests through, `"closed"` denies requests. |
| `connectionTimeout` | string | No | `"5s"` | Redis connection timeout (Go duration string). |
| `readTimeout` | string | No | `"3s"` | Redis read timeout (Go duration string). |
| `writeTimeout` | string | No | `"3s"` | Redis write timeout (Go duration string). |

#### Memory Configuration

When using in-memory backend, the following parameters can be configured under `memory`:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `maxEntries` | integer | No | `10000` | Maximum number of rate limit entries to store. Oldest entries are evicted when limit is reached. |
| `cleanupInterval` | string | No | `"5m"` | Interval for cleaning up expired entries. Use `"0"` to disable periodic cleanup. |

#### Headers Configuration

Control which rate limit headers are included in responses under `headers`:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `includeXRateLimit` | boolean | No | `true` | Include X-RateLimit-* headers (X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset). |
| `includeIETF` | boolean | No | `true` | Include IETF RateLimit headers (RateLimit-Limit, RateLimit-Remaining, RateLimit-Reset, RateLimit-Policy). |
| `includeRetryAfter` | boolean | No | `true` | Include Retry-After header when rate limited (RFC 7231). Only set on 429 responses. |

### User Parameters (API Definition)

These parameters are configured per-API/route by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `quotas` | `Quota` array | Yes | - | Defines quota entries (1-10). Each quota applies independent limits with its own `keyExtraction` and `costExtraction` settings. |
| `keyExtraction` | `KeyExtraction` array | No | `[{type: "routename"}]` | Global key extraction applied to quotas that do not define their own `keyExtraction`. Defaults to `routename` if omitted. |
| `onRateLimitExceeded` | `onRateLimitExceeded` object | No | - | Customizes the response returned when a request exceeds rate limits. |

#### Quota Configuration

Each entry in the `quotas` array defines an independent rate limit bucket:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | string | No | - | Quota name used in logs and `X-RateLimit-Quota` response headers (1-128 chars). |
| `limits` | `Limit` array | Yes | - | One or more limit windows for the quota (1-10). The strictest limit is enforced. |
| `keyExtraction` | `KeyExtraction` array | No | - | Per-quota key extraction. Overrides the global `keyExtraction` when set. |
| `costExtraction` | `CostExtraction` object | No | - | Per-quota dynamic cost extraction from request or response data. |

#### Limit Configuration

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `limit` | integer | Yes | - | Maximum number of requests or tokens allowed in the duration (1-1,000,000,000). |
| `duration` | string | Yes | - | Time window for the limit (Go duration format: `"1s"`, `"1m"`, `"1h"`, `"24h"`). |
| `burst` | integer | No | Same as `limit` | Maximum burst capacity (GCRA only). Number of requests that can accumulate (1-1,000,000,000). |

#### KeyExtraction Configuration

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | Yes | Type of component: `"header"`, `"metadata"`, `"ip"`, `"apiname"`, `"apiversion"`, `"routename"`, `"cel"`, `"constant"`. |
| `key` | string | Conditional | Header name or metadata key. Required for `header` and `metadata` types (1-256 chars). |
| `expression` | string | Conditional | CEL expression returning a string. Required for `cel` type (1-1024 chars). |

**Key extraction types:**
- `header`: Extract from HTTP header (requires `key` field)
- `metadata`: Extract from SharedContext.Metadata (requires `key` field)
- `ip`: Extract client IP from X-Forwarded-For/X-Real-IP headers
- `apiname`: Use API name from context
- `apiversion`: Use API version from context
- `routename`: Use route name from metadata (default)
- `cel`: Evaluate a CEL expression returning a string (requires `expression` field)
- `constant`: Use a fixed constant string value (requires `key` field)

> **Important: Component Order Matters**
>
> The order of components in the `keyExtraction` array affects the generated rate limit key. Components are joined with `:` separator in the exact order specified:
>
> ```yaml
> # Example 1: User ID then IP
> keyExtraction:
>   - type: header
>     key: X-User-ID
>   - type: ip
> # Generates key: "user123:192.168.1.1"
> ```
>
> ```yaml
> # Example 2: IP then User ID (different from Example 1!)
> keyExtraction:
>   - type: ip
>   - type: header
>     key: X-User-ID
> # Generates key: "192.168.1.1:user123"
> ```
>
> These are treated as **different rate limit buckets** with separate counters. If you change the component order in your configuration, it will effectively reset all rate limit counters for that policy.
>
> **Best Practice:** Maintain consistent component ordering across all environments and configuration updates to avoid unexpected rate limit resets.

#### CostExtraction Configuration

Configures per-quota dynamic cost extraction from request or response data. Values from all successful sources are summed (after applying multipliers). If all sources fail, the `default` cost is used.

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `enabled` | boolean | No | `false` | Enables dynamic cost extraction for the quota. |
| `sources` | `Source` array | Yes (if enabled) | - | Cost extraction sources (1-10). Successful values are summed with multipliers applied. |
| `default` | number | No | `1` | Fallback cost when all source extractions fail (0 to 1,000,000,000). |

**Source Configuration:**

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | Yes | Source type. See **Source Types** below. |
| `key` | string | Conditional | Header name or metadata key. Required for `request_header`, `request_metadata`, `response_header`, `response_metadata` types (1-256 chars). |
| `jsonPath` | string | Conditional | JSONPath expression. Required for `request_body` and `response_body` types (1-512 chars). |
| `expression` | string | Conditional | CEL expression returning a numeric result. Required for `request_cel` and `response_cel` types (1-1024 chars). |
| `multiplier` | number | No | `1.0` | Multiplier applied to the extracted value (minimum 0). |

**Source Types:**

- `request_header`: Extract cost from a request header
- `request_metadata`: Extract cost from request shared metadata
- `request_body`: Extract cost from request JSON body using JSONPath
- `response_header`: Extract cost from a response header
- `response_metadata`: Extract cost from response shared metadata
- `response_body`: Extract cost from response JSON body using JSONPath
- `request_cel`: Evaluate a CEL expression on request data (numeric result)
- `response_cel`: Evaluate a CEL expression on response data (numeric result)

> **Important: Post-Response Rate Limiting Behavior**
>
> When `costExtraction.enabled: true`:
> - A **pre-flight quota check** is performed: if the key's remaining quota is already exhausted (≤ 0), the request is blocked with a 429 response
> - If quota is available, the request proceeds to upstream without consuming tokens
> - Cost is extracted from the response and consumed **after** the response is received
> - If the rate limit is exceeded post-response, the **current request has already succeeded**, but headers indicate quota exhaustion
> - **Subsequent requests** using the same key will be impacted by the consumed quota
>
> This model is appropriate for:
> - Use cases where cost is only known after the operation completes (e.g., LLM token usage)
> - Usage tracking with pre-flight protection against fully exhausted quotas

#### RateLimitExceeded Configuration

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `statusCode` | integer | No | `429` | HTTP status code for rate limit response (400-599). |
| `body` | string | No | `{"error": "Too Many Requests", "message": "Rate limit exceeded. Please try again later."}` | Custom error message body. |
| `bodyFormat` | string | No | `"json"` | Response body content type: `"json"` or `"plain"`. |

#### Sample System Configuration

##### Memory Backend Configuration

```toml
[policy_configurations.ratelimit_v0]
algorithm = "fixed-window"
backend = "memory"

[policy_configurations.ratelimit_v0.memory]
max_entries = 10000
cleanup_interval = "5m"

[policy_configurations.ratelimit_v0.headers]
include_x_rate_limit = true
include_ietf = true
include_retry_after = true
```

##### Redis Backend Configuration

For distributed rate limiting across multiple gateway instances:

```toml
[policy_configurations.ratelimit_v0]
algorithm = "fixed-window"
backend = "redis"

[policy_configurations.ratelimit_v0.redis]
host = "redis.example.com"
port = 6379
password = "your-redis-password"
db = 0
key_prefix = "ratelimit:v1:"
failure_mode = "open"
connection_timeout = "5s"
read_timeout = "3s"
write_timeout = "3s"

[policy_configurations.ratelimit_v0.headers]
include_x_rate_limit = true
include_ietf = true
include_retry_after = true
```

**Note:**

Inside the `gateway/build.yaml`, ensure the policy module is added under `policies:`:

```yaml
- name: advanced-ratelimit
  gomodule: github.com/wso2/gateway-controllers/policies/advanced-ratelimit@v0
```

## Reference Scenarios

### Example 1: Basic Rate Limiting

Apply a simple rate limit to an API:

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
        - name: advanced-ratelimit
          version: v0
          params:
            quotas:
              - limits:
                  - limit: 10
                    duration: "1m"
    - method: GET
      path: /alerts/active
```

### Example 2: Multiple Time Windows

Enforce multiple rate limits simultaneously (e.g., per-second and per-hour):

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
        - name: advanced-ratelimit
          version: v0
          params:
            quotas:
              - limits:
                  - limit: 10
                    duration: "1m"
                  - limit: 20
                    duration: "1h"
    - method: GET
      path: /alerts/active
```

### Example 3: Per-User Rate Limiting

Rate limit based on user identity from a header:

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
        - name: advanced-ratelimit
          version: v0
          params:
            quotas:
              - limits:
                  - limit: 10
                    duration: "1m"
                keyExtraction:
                  - type: header
                    key: X-User-ID
    - method: GET
      path: /alerts/active
```

### Example 4: Per-IP Rate Limiting

Rate limit based on client IP address:

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
spec:
  name: public-api
  version: v1.0
  context: /public
  upstream:
    main:
      url: https://public-service:8080
  policies:
    - name: advanced-ratelimit
      version: v0
      params:
        quotas:
          - limits:
              - limit: 60
                duration: "1m"
            keyExtraction:
              - type: ip
  operations:
    - method: GET
      path: /data
    - method: POST
      path: /submit
```

### Example 5: Composite Key Rate Limiting

Rate limit based on multiple factors (API name + user ID):

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
spec:
  name: multi-tenant-api
  version: v1.0
  context: /tenant
  upstream:
    main:
      url: https://tenant-service:8080
  policies:
    - name: advanced-ratelimit
      version: v0
      params:
        quotas:
          - limits:
              - limit: 500
                duration: "1h"
            keyExtraction:
              - type: apiname
              - type: header
                key: X-Tenant-ID
  operations:
    - method: GET
      path: /resources
    - method: POST
      path: /resources
```

### Example 6: Multi-Quota Rate Limiting

Apply multiple independent quotas with separate limits and key extraction:

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
spec:
  name: analytics-api
  version: v1.0
  context: /analytics
  upstream:
    main:
      url: https://analytics-service:8080
  policies:
    - name: advanced-ratelimit
      version: v0
      params:
        quotas:
          - name: per-user
            limits:
              - limit: 1000
                duration: "1h"
            keyExtraction:
              - type: header
                key: X-User-ID
          - name: global
            limits:
              - limit: 10000
                duration: "1h"
            keyExtraction:
              - type: routename
  operations:
    - method: GET
      path: /query
    - method: POST
      path: /report
```

### Example 7: Burst Rate Limiting with GCRA

Allow burst traffic with GCRA algorithm:

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
spec:
  name: burst-api
  version: v1.0
  context: /burst
  upstream:
    main:
      url: https://burst-service:8080
  policies:
    - name: advanced-ratelimit
      version: v0
      params:
        quotas:
          - limits:
              - limit: 10
                duration: "1s"
                burst: 20
  operations:
    - method: GET
      path: /data
    - method: POST
      path: /data
```

### Example 8: Custom Error Response

Customize the rate limit exceeded response:

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
spec:
  name: custom-error-api
  version: v1.0
  context: /custom
  upstream:
    main:
      url: https://backend-service:8080
  policies:
    - name: advanced-ratelimit
      version: v0
      params:
        quotas:
          - limits:
              - limit: 100
                duration: "1m"
        onRateLimitExceeded:
          statusCode: 429
          body: '{"code": "RATE_LIMIT_EXCEEDED", "message": "You have exceeded the rate limit. Please wait before making more requests.", "retryAfter": "60s"}'
          bodyFormat: json
  operations:
    - method: GET
      path: /resource
```

### Example 9: LLM Token-Based Rate Limiting (Post-Response Cost Extraction)

Rate limit based on actual token usage from an LLM API response:

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
spec:
  name: llm-api
  version: v1.0
  context: /llm
  upstream:
    main:
      url: https://llm-service:8080
  policies:
    - name: advanced-ratelimit
      version: v0
      params:
        quotas:
          - limits:
              - limit: 100000
                duration: "24h"
            keyExtraction:
              - type: header
                key: X-User-ID
            costExtraction:
              enabled: true
              sources:
                - type: response_header
                  key: X-Token-Usage
                - type: response_body
                  jsonPath: "$.usage.total_tokens"
              default: 100
  operations:
    - method: POST
      path: /chat/completions
    - method: POST
      path: /completions
```

### Example 10: Compute Unit Rate Limiting with Fallback Sources

Rate limit based on compute units with multiple extraction sources:

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
spec:
  name: compute-api
  version: v1.0
  context: /compute
  upstream:
    main:
      url: https://compute-service:8080
  policies:
    - name: advanced-ratelimit
      version: v0
      params:
        quotas:
          - limits:
              - limit: 1000
                duration: "1h"
            costExtraction:
              enabled: true
              sources:
                - type: response_header
                  key: X-Compute-Units
                - type: response_metadata
                  key: compute_units
                - type: response_body
                  jsonPath: "$.metrics.compute_units"
              default: 1
  operations:
    - method: POST
      path: /process
    - method: POST
      path: /analyze
```

## How it Works

#### GCRA (Generic Cell Rate Algorithm)

- Token bucket semantics with smooth rate limiting
- **Best for**: Smooth traffic shaping, burst handling, consistent rate enforcement
- **Advantages**: Prevents traffic bursts at window boundaries, supports burst capacity
- **Use when**: You need consistent rate enforcement and burst tolerance

#### Fixed Window

- Divides time into fixed intervals and counts requests per window
- **Best for**: Simple counting, lower computational overhead
- **Advantages**: Simple to understand, lower memory overhead
- **Limitation**: Can allow up to 2x burst at window boundaries
- **Use when**: Simplicity is preferred and boundary bursts are acceptable


## Notes:

When rate limiting is applied, the following headers may be included in responses:

##### X-RateLimit Headers (Industry Standard)

| Header | Description |
|--------|-------------|
| `X-RateLimit-Limit` | Maximum requests allowed in the current window |
| `X-RateLimit-Remaining` | Remaining requests in the current window |
| `X-RateLimit-Reset` | Unix timestamp when the rate limit resets |

##### IETF RateLimit Headers (Draft Standard)

| Header | Description |
|--------|-------------|
| `RateLimit-Limit` | Maximum requests allowed in the current window |
| `RateLimit-Remaining` | Remaining requests in the current window |
| `RateLimit-Reset` | Seconds until the rate limit resets |
| `RateLimit-Policy` | Rate limit policy description |

##### Retry-After Header (RFC 7231)

| Header | Description |
|--------|-------------|
| `Retry-After` | Seconds to wait before retrying (only on 429 responses) |
