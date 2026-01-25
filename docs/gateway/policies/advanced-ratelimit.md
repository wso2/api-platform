# Rate Limiting (Advanced)

## Overview

The Advanced Rate Limiting policy provides a powerful, multi-dimensional token bucket implementation for API rate limiting. It supports multiple independent rate limit quotas per API, sophisticated key extraction strategies, and dynamic cost extraction for AI/LLM workloads.

## Features

- **Multi-dimensional Quotas**: Define multiple independent rate limit buckets (e.g., one per user, another per organization).
- **Multiple Algorithms**: Choose between GCRA (Leaky Bucket variant for smooth shaping) and Fixed Window.
- **Dynamic Cost Extraction**: Extract request costs from headers, metadata, or JSON bodies (e.g., for LLM token usage).
- **Flexible Key Extraction**: Rate limit by Header, Metadata, IP, API Name, or Route Name.
- **Distributed & Local**: Support for both In-Memory (local) and Redis (distributed) backends.
- **Fail-Open Support**: Configurable behavior when Redis is unavailable.
- **Comprehensive Headers**: Supports `X-RateLimit-*`, IETF `RateLimit`, and `Retry-After` headers.

## Configuration

The Advanced Rate Limiting policy uses a structure based on **Quotas**. 

- **System Parameters**: Configured by the administrator in `config.toml`.
- **User Parameters**: Configured per-API/route via the `quotas` array.

### System Parameters (config.toml)

These parameters are set globally by the administrator.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `algorithm` | string | `"gcra"` | Rate limiting algorithm: `"gcra"` (smooth, burst-capable) or `"fixed-window"` (simple counter). |
| `backend` | string | `"memory"` | Storage backend: `"memory"` (single-instance) or `"redis"` (distributed). |
| `redis` | object | - | Redis configuration (host, port, auth, timeouts). Used when `backend: redis`. |
| `memory` | object | - | Memory backend configuration (max entries, cleanup interval). |
| `headers` | object | - | Controls which response headers (X-RateLimit, IETF, Retry-After) are included. |

### User Parameters (API Definition)

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `quotas` | array | **Yes** | Array of independent rate limit quotas. Each quota tracks usage separately. |
| `keyExtraction` | array | No | **Global** key extraction config. Used as a default for quotas that don't specify their own. |
| `onRateLimitExceeded` | object | No | Customize the 429 response (status code, body). |

#### Quota Configuration

Each item in the `quotas` array represents an independent dimension of rate limiting.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `name` | string | No | Name of the quota (e.g. "per-user", "per-ip"). Useful for debugging and logs. |
| `limits` | array | **Yes** | List of limits for this quota (e.g. 10/s AND 100/m). All must be satisfied. |
| `keyExtraction` | array | No | Per-quota key extraction. Overrides global `keyExtraction`. Defaults to `routename`. |
| `costExtraction` | object | No | Configuration for dynamic cost extraction (e.g. LLM tokens). |

#### Limit Configuration

Each item in the `limits` array:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `limit` | integer | **Yes** | Maximum number of requests/tokens allowed. |
| `duration` | string | **Yes** | Time window (e.g., "1s", "1m", "1h"). |
| `burst` | integer | No | Maximum burst capacity (GCRA only). Defaults to `limit`. |

#### Key Extraction Configuration

Define how to identify the bucket (e.g., by User ID or IP).

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | **Yes** | One of: `header`, `metadata`, `ip`, `apiname`, `apiversion`, `routename`. |
| `key` | string | Conditional | Header name or metadata key. Required for `header`/`metadata`. |

**Note**: Multiple components are joined with `:` (e.g., `header:X-User-ID` + `ip` -> "user123:1.2.3.4").

#### Cost Extraction Configuration

Used for weighting requests dynamically (e.g. based on response body content).

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `enabled` | boolean | `false` | Enable cost extraction. |
| `default` | integer | `1` | Default cost if extraction fails. |
| `sources` | array | - | List of sources to extract cost from (summed if multiple succeed). |

**Sources**:
- `request_header`, `request_metadata`, `request_body` (JsonPath)
- `response_header`, `response_metadata`, `response_body` (JsonPath)

---

## Examples

### Example 1: Basic Per-Route Rate Limit

Simplest configuration: 10 requests per minute per route.

```yaml
policies:
  - name: advanced-ratelimit
    version: v0.1.0
    params:
      quotas:
        - limits:
            - limit: 10
              duration: "1m"
```

### Example 2: Per-User and Per-IP Limits (Multi-Dimensional)

Enforce two independent quotas simultaneously:
1. **User Quota**: 1000 requests/hour per User ID.
2. **IP Quota**: 60 requests/minute per IP address.

```yaml
policies:
  - name: advanced-ratelimit
    version: v0.1.0
    params:
      quotas:
        - name: "user_quota"
          limits:
            - limit: 1000
              duration: "1h"
          keyExtraction:
            - type: header
              key: X-User-ID
        - name: "ip_quota"
          limits:
            - limit: 60
              duration: "1m"
          keyExtraction:
            - type: ip
```

### Example 3: Multiple Time Windows (Bursts)

Allow 10 req/sec (bursty) AND 1000 req/hour (sustained) for the same key.

```yaml
policies:
  - name: advanced-ratelimit
    version: v0.1.0
    params:
      quotas:
        - limits:
            - limit: 10
              duration: "1s"
              burst: 20
            - limit: 1000
              duration: "1h"
          keyExtraction:
            - type: header
              key: "X-API-Key"
```

### Example 4: LLM Token-Based Rate Limiting

Rate limit based on total tokens consumed, extracted from the response body.

```yaml
policies:
  - name: advanced-ratelimit
    version: v0.1.0
    params:
      quotas:
        - limits:
            - limit: 50000
              duration: "1h"
          keyExtraction:
            - type: header
              key: X-Org-ID
          costExtraction:
            enabled: true
            default: 1
            sources:
              - type: response_body
                jsonPath: "$.usage.total_tokens"
```

### Example 5: Custom Error Response

```yaml
policies:
  - name: advanced-ratelimit
    version: v0.1.0
    params:
      quotas:
        - limits:
            - limit: 5
              duration: "1m"
      onRateLimitExceeded:
        statusCode: 429
        body: '{"error": "Too Many Requests", "retry_after": "1m"}'
```

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

The Rate Limiting policy uses a two-level configuration model:

- **System Parameters**: Configured by the administrator in `config.toml` under `policy_configurations.ratelimit_v010`
- **User Parameters**: Configured per-API/route in the API definition YAML

### System Parameters (config.toml)

These parameters are set by the administrator and apply globally to all rate limiting policies:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `algorithm` | string | No | `"gcra"` | Rate limiting algorithm: `"gcra"` (smooth rate limiting with burst support) or `"fixed-window"` (simple counter per time window). |
| `backend` | string | No | `"memory"` | Storage backend: `"memory"` for single-instance or `"redis"` for distributed rate limiting. |
| `redis` | object | No | - | Redis configuration (only used when `backend=redis`). |
| `memory` | object | No | - | In-memory storage configuration (only used when `backend=memory`). |
| `headers` | object | No | - | Control which rate limit headers are included in responses. |

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
| `limits` | array | Yes | - | Array of rate limit policies to enforce. Multiple limits can be specified for different time windows. |
| `cost` | integer | No | `1` | Number of tokens this operation consumes per request (weighted rate limiting). Ignored when `costExtraction` is enabled. |
| `costExtraction` | object | No | - | Configuration for extracting cost from response data (post-response rate limiting). |
| `keyExtraction` | array | No | `[{type: "routename"}]` | Array of components to extract and combine for the rate limit key. |
| `onRateLimitExceeded` | object | No | - | Customize the 429 response when rate limit is exceeded. |

#### Limit Configuration

Each item in the `limits` array supports the following structure:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `limit` | integer | Yes | - | Maximum number of requests allowed in the duration (1-1,000,000,000). |
| `duration` | string | Yes | - | Time window for the limit (Go duration format: "1s", "1m", "1h", "24h"). |
| `burst` | integer | No | Same as `limit` | Maximum burst capacity (GCRA only). Number of requests that can accumulate. |

#### Key Extraction Configuration

Each item in the `keyExtraction` array supports the following structure:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | Yes | Type of component: `"header"`, `"metadata"`, `"ip"`, `"apiname"`, `"apiversion"`, `"routename"`. |
| `key` | string | Conditional | Header name or metadata key (required for `header` and `metadata` types). |

**Key extraction types:**
- `header`: Extract from HTTP header (requires `key` field)
- `metadata`: Extract from SharedContext.Metadata (requires `key` field)
- `ip`: Extract client IP from X-Forwarded-For/X-Real-IP headers
- `apiname`: Use API name from context
- `apiversion`: Use API version from context
- `routename`: Use route name from metadata (default)

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

#### Cost Extraction Configuration

The `costExtraction` object enables post-response rate limiting, where the cost is extracted from the response data instead of using a static value. This is useful for scenarios where the actual resource consumption is only known after the request completes (e.g., LLM token usage, compute units).

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `enabled` | boolean | No | `false` | Enable post-response cost extraction. |
| `sources` | array | Yes (if enabled) | - | Ordered list of sources to extract cost from. Sources are tried in order until one succeeds. |
| `default` | integer | No | `1` | Default cost to use if extraction fails from all sources. |

**Source Configuration:**

Each item in the `sources` array supports the following structure:

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | Yes | Type of source: `"response_header"`, `"metadata"`, or `"response_body"`. |
| `key` | string | Conditional | Header name (for `response_header`) or metadata key (for `metadata`). Required for these types. |
| `jsonPath` | string | Conditional | JSON path expression for extracting cost from response body (for `response_body`). Required for this type. Example: `"$.usage.total_tokens"`. |

**Source Types:**

- `response_header`: Extract cost from a response header (must be an integer value)
- `metadata`: Extract cost from shared metadata (set by other policies)
- `response_body`: Extract cost from JSON response body using JSONPath expression

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

#### Rate Limit Exceeded Response

The `onRateLimitExceeded` object supports:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `statusCode` | integer | No | `429` | HTTP status code for rate limit response (400-599). |
| `body` | string | No | `{"error": "Too Many Requests", "message": "Rate limit exceeded. Please try again later."}` | Custom error message body. |
| `bodyFormat` | string | No | `"json"` | Response body content type: `"json"` or `"plain"`. |

## Algorithm Comparison

### GCRA (Generic Cell Rate Algorithm)

- **Best for**: Smooth traffic shaping, burst handling, consistent rate enforcement
- **How it works**: Token bucket semantics with smooth rate limiting
- **Advantages**: Prevents traffic bursts at window boundaries, supports burst capacity
- **Use when**: You need consistent rate enforcement and burst tolerance

### Fixed Window

- **Best for**: Simple counting, lower computational overhead
- **How it works**: Divides time into fixed intervals and counts requests per window
- **Advantages**: Simple to understand, lower memory overhead
- **Limitation**: Can allow up to 2x burst at window boundaries
- **Use when**: Simplicity is preferred and boundary bursts are acceptable

## System Configuration Example

Add the following to your `gateway/configs/config.toml` file under `policy_configurations`:

```toml
[policy_configurations.ratelimit_v010]
algorithm = "gcra"
backend = "memory"

[policy_configurations.ratelimit_v010.memory]
max_entries = 10000
cleanup_interval = "5m"

[policy_configurations.ratelimit_v010.headers]
include_x_rate_limit = true
include_ietf = true
include_retry_after = true
```

### Redis Backend Configuration

For distributed rate limiting across multiple gateway instances:

```toml
[policy_configurations.ratelimit_v010]
algorithm = "gcra"
backend = "redis"

[policy_configurations.ratelimit_v010.redis]
host = "redis.example.com"
port = 6379
password = "your-redis-password"
db = 0
key_prefix = "ratelimit:v1:"
failure_mode = "open"
connection_timeout = "5s"
read_timeout = "3s"
write_timeout = "3s"

[policy_configurations.ratelimit_v010.headers]
include_x_rate_limit = true
include_ietf = true
include_retry_after = true
```

## API Definition Examples

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
        - name: ratelimit
          version: v0.1.0
          params:
            cost: 1
            limits:
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
        - name: ratelimit
          version: v0.1.0
          params:
            cost: 1
            limits:
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
        - name: ratelimit
          version: v0.1.0
          params:
            cost: 1
            limits:
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
    - name: ratelimit
      version: v0.1.0
      params:
        limits:
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
    - name: ratelimit
      version: v0.1.0
      params:
        limits:
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

### Example 6: Weighted Rate Limiting (Cost-Based)

Apply different costs to different operations:

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
    - name: ratelimit
      version: v0.1.0
      params:
        limits:
          - limit: 1000
            duration: "1h"
  operations:
    - method: GET
      path: /simple-query
      policies:
        - name: ratelimit
          version: v0.1.0
          params:
            limits:
              - limit: 1000
                duration: "1h"
            cost: 1
    - method: POST
      path: /complex-report
      policies:
        - name: ratelimit
          version: v0.1.0
          params:
            limits:
              - limit: 1000
                duration: "1h"
            cost: 10
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
    - name: ratelimit
      version: v0.1.0
      params:
        limits:
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
    - name: ratelimit
      version: v0.1.0
      params:
        limits:
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
    - name: ratelimit
      version: v0.1.0
      params:
        limits:
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
    - name: ratelimit
      version: v0.1.0
      params:
        limits:
          - limit: 1000
            duration: "1h"
        costExtraction:
          enabled: true
          sources:
            - type: response_header
              key: X-Compute-Units
            - type: metadata
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

## Response Headers

When rate limiting is applied, the following headers may be included in responses:

### X-RateLimit Headers (Industry Standard)

| Header | Description |
|--------|-------------|
| `X-RateLimit-Limit` | Maximum requests allowed in the current window |
| `X-RateLimit-Remaining` | Remaining requests in the current window |
| `X-RateLimit-Reset` | Unix timestamp when the rate limit resets |

### IETF RateLimit Headers (Draft Standard)

| Header | Description |
|--------|-------------|
| `RateLimit-Limit` | Maximum requests allowed in the current window |
| `RateLimit-Remaining` | Remaining requests in the current window |
| `RateLimit-Reset` | Seconds until the rate limit resets |
| `RateLimit-Policy` | Rate limit policy description |

### Retry-After Header (RFC 7231)

| Header | Description |
|--------|-------------|
| `Retry-After` | Seconds to wait before retrying (only on 429 responses) |

## Use Cases

1. **API Protection**: Prevent backend services from being overwhelmed by excessive requests.

2. **Fair Usage**: Ensure fair resource allocation among different clients or tenants.

3. **Cost Control**: Use weighted rate limiting to control expensive operations like complex queries or report generation.

4. **DDoS Mitigation**: Apply per-IP rate limiting to mitigate denial-of-service attacks.

5. **Tiered API Access**: Implement different rate limits for different subscription tiers by using header-based key extraction.

6. **Multi-Tenant Rate Limiting**: Rate limit per tenant in multi-tenant applications using composite keys.

7. **Distributed Rate Limiting**: Use Redis backend to enforce consistent rate limits across multiple gateway instances.

8. **Graceful Degradation**: Configure fail-open mode to maintain availability when the rate limiting backend is unavailable.

9. **LLM Token Budgeting**: Use post-response cost extraction to rate limit based on actual token usage from LLM APIs, enabling accurate quota management for AI workloads.

10. **Usage-Based Billing**: Track and limit resource consumption based on actual usage metrics extracted from responses (e.g., compute units, storage operations).
