`# Rate Limit Policy: Enhanced Cost Extraction & Multi-Dimensional Limits

## Summary

The current rate limit policy has limitations for advanced use cases like **token-based rate limiting for LLM APIs** where:
- Costs need to be dynamically extracted from request/response data
- Different token types (prompt vs completion) need different weights/limits
- Multiple independent rate limit dimensions are needed

## Current Limitations

### 1. Static Cost Only
```yaml
params:
  limits:
    - limit: 1000
      duration: "1m"
  cost: 1  # Static value only - cannot extract from request/response
```

### 2. Single Key Extraction (Global)
```yaml
params:
  keyExtraction:        # Global - applies to ALL limits
    - type: header
      key: X-User-ID
  limits:
    - limit: 100
      duration: "1m"
    - limit: 1000
      duration: "1h"
```

Cannot have different keys for different limits (e.g., per-user request limit + per-org token limit).

---

## Proposed Solution

### Core Idea: Move `keyExtraction` and `costExtraction` INTO Each Limit

Rename `limits` to `quotas` (or `dimensions`) and make each quota self-contained:

```yaml
params:
  quotas:
    - name: "request-count"
      limit: 100
      duration: "1m"
      keyExtraction:
        - type: header
          key: X-User-ID
      # No costExtraction = cost is 1 per request

    - name: "prompt-tokens"
      limit: 50000
      duration: "1h"
      keyExtraction:
        - type: header
          key: X-User-ID
      costExtraction:
        enabled: true
        sources:
          - type: response_body
            jsonPath: "$.usage.prompt_tokens"
            multiplier: 1.0
        default: 0

    - name: "completion-tokens"
      limit: 25000
      duration: "1h"
      keyExtraction:
        - type: header
          key: X-User-ID
      costExtraction:
        enabled: true
        sources:
          - type: response_body
            jsonPath: "$.usage.completion_tokens"
            multiplier: 1.0
        default: 0
```

This naturally enables multi-dimensional rate limiting without adding new top-level concepts.

---

## Cost Extraction Feature

### Schema

```yaml
costExtraction:
  enabled: boolean       # Enable/disable dynamic extraction

  sources:               # List of sources (values are summed)
    - type: string       # request_body, response_body, request_metadata,
                         # response_metadata, request_header, response_header

      jsonPath: string   # For body types: JSONPath expression
      key: string        # For metadata/header types: key name

      multiplier: number # Multiplier for extracted value (default: 1.0)
                         # Can be negative for refund scenarios

  default: number        # Fallback if all extractions fail
```

### Supported Extraction Sources

| Type | Description | Phase |
|------|-------------|-------|
| `request_body` | Extract from request JSON body | Request |
| `response_body` | Extract from response JSON body | Response |
| `request_metadata` | Extract from request context metadata | Request |
| `response_metadata` | Extract from response context metadata | Response |
| `request_header` | Extract from request header | Request |
| `response_header` | Extract from response header | Response |

### Cost Calculation

When multiple sources are defined:
```
total_cost = Σ (extracted_value[i] * multiplier[i])
```

If a source extraction fails, it contributes `0`. If ALL sources fail, `default` is used.

---

## Configuration Examples

### Example 1: Simple Response Body Extraction

```yaml
params:
  quotas:
    - name: "custom-cost"
      limit: 100
      duration: "1m"
      costExtraction:
        enabled: true
        sources:
          - type: response_body
            jsonPath: "$.json.custom_cost"
        default: 1
```

### Example 2: LLM Token-Based with Weighted Multipliers

```yaml
params:
  quotas:
    - name: "weighted-tokens"
      limit: 10000
      duration: "1h"
      keyExtraction:
        - type: header
          key: X-User-ID
      costExtraction:
        enabled: true
        sources:
          # Prompt tokens weighted at 0.1 (cheaper)
          - type: response_body
            jsonPath: "$.usage.prompt_tokens"
            multiplier: 0.1

          # Completion tokens weighted at 0.3 (more expensive)
          - type: response_body
            jsonPath: "$.usage.completion_tokens"
            multiplier: 0.3
        default: 1
```

**Calculation:**
```
Response: {"usage": {"prompt_tokens": 500, "completion_tokens": 200}}
Cost = (500 * 0.1) + (200 * 0.3) = 50 + 60 = 110 weighted tokens
```

### Example 3: Multi-Dimensional (Requests + Separate Token Limits)

```yaml
params:
  quotas:
    # Dimension 1: Request count
    - name: "requests"
      limit: 100
      duration: "1m"
      keyExtraction:
        - type: header
          key: X-User-ID
      # No costExtraction = 1 per request

    # Dimension 2: Prompt tokens
    - name: "prompt-tokens"
      limit: 50000
      duration: "1h"
      keyExtraction:
        - type: header
          key: X-User-ID
      costExtraction:
        enabled: true
        sources:
          - type: response_body
            jsonPath: "$.usage.prompt_tokens"
            multiplier: 1.0
        default: 0

    # Dimension 3: Completion tokens (separate limit)
    - name: "completion-tokens"
      limit: 25000
      duration: "1h"
      keyExtraction:
        - type: header
          key: X-User-ID
      costExtraction:
        enabled: true
        sources:
          - type: response_body
            jsonPath: "$.usage.completion_tokens"
            multiplier: 1.0
        default: 0
```

### Example 4: Metadata-Based Cost

```yaml
params:
  quotas:
    - name: "computed-cost"
      limit: 1000
      duration: "1m"
      costExtraction:
        enabled: true
        sources:
          # Cost computed by upstream policy and stored in metadata
          - type: request_metadata
            key: "computed_cost"
            multiplier: 1.0
        default: 1
```

### Example 5: Different Keys per Quota

```yaml
params:
  quotas:
    # Per-user request limit
    - name: "user-requests"
      limit: 100
      duration: "1m"
      keyExtraction:
        - type: header
          key: X-User-ID

    # Per-organization token limit
    - name: "org-tokens"
      limit: 1000000
      duration: "1d"
      keyExtraction:
        - type: header
          key: X-Org-ID
      costExtraction:
        enabled: true
        sources:
          - type: response_body
            jsonPath: "$.usage.total_tokens"
            multiplier: 1.0
        default: 0
```

---

## Schema Changes Summary

### Before (Current)

```yaml
params:
  keyExtraction:           # Global
    - type: header
      key: X-User-ID
  limits:                  # Array of limit/duration/burst only
    - limit: 100
      duration: "1m"
      burst: 150
  cost: 1                  # Global static cost
```

### After (Proposed)

```yaml
params:
  quotas:                  # Renamed from 'limits'
    - name: "quota-name"   # NEW: Optional name for logging/headers
      limit: 100
      duration: "1m"
      burst: 150
      keyExtraction:       # MOVED: Per-quota (optional, defaults to routename)
        - type: header
          key: X-User-ID
      costExtraction:      # NEW: Per-quota dynamic cost extraction
        enabled: true
        sources:
          - type: response_body
            jsonPath: "$.usage.tokens"
            multiplier: 1.0
        default: 1
```
`