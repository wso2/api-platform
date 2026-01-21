# Basic Rate Limiting

## Overview

The Basic Rate Limiting policy offers a simplified configuration for protecting APIs from excessive traffic. It automatically uses the **Route Name** (or API Name if attached at the API level) as the key for rate limiting.

For advanced use cases (e.g., custom keys, cost extraction, multiple quotas), use the **Advanced Rate Limiting** policy instead.

## Features

- **Simple Configuration**: Just define the rate limits.
- **Automatic Keying**: Rates are tracked per-route by default.
- **Shared Infrastructure**: Uses the same high-performance backend (Redis or In-Memory) and algorithms (GCRA or Fixed Window) as the Advanced policy.

## Configuration

This policy separates configuration into System Parameters (admin) and User Parameters (dev).

### User Parameters (API Definition)

The only configuration required is the list of limits.

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `limits` | array | **Yes** | Array of rate limits to enforce. |

#### Limit Object

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `limit` | integer | **Yes** | Maximum number of requests allowed. |
| `duration` | string | **Yes** | Time window (e.g., "1s", "1m", "1h"). |

### System Parameters (config.yaml)

These parameters are shared with the Advanced Rate Limit policy and configured globally.

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `algorithm` | string | `"gcra"` | "gcra" (smooth) or "fixed-window". |
| `backend` | string | `"memory"` | "memory" (local) or "redis" (distributed). |
| `redis` | object | - | Redis connection settings. |
| `memory` | object | - | Memory cleanup settings. |

---

## Examples

### Example 1: Simple Per-Route Request Limit

Allow 1000 requests per minute for this route.

```yaml
policies:
  - name: basic-ratelimit
    version: v0.1.0
    params:
      limits:
        - limit: 1000
          duration: "1m"
```

### Example 2: Multiple Time Windows

Enforce a short-term burst limit and a long-term quota.
- 10 requests per second
- 500 requests per hour

```yaml
policies:
  - name: basic-ratelimit
    version: v0.1.0
    params:
      limits:
        - limit: 10
          duration: "1s"
        - limit: 500
          duration: "1h"
```
