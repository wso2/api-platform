# Analytics Header Filter

## Overview

The Analytics Header Filter policy allows you to control which request and response headers are included in analytics data using allow or deny modes. This policy is intended to prevent sensitive, noisy, or irrelevant headers from being sent to analytics backends while preserving the rest of the request and response context.

The policy is only effective when analytics is enabled at the system level and must be explicitly added to the API’s policy chain.

**Operation modes:**
- **"allow"**: Only the specified headers will be included in analytics (whitelist mode)
- **"deny"**: All headers except the specified ones will be included in analytics (blacklist mode)

Request and response headers can have different operation modes, allowing for flexible filtering strategies.


## Features

* Filters request and response headers from analytics data collection using allow or deny modes
* Case-insensitive header matching
* Supports independent configuration with flexible filtering strategies with whitelist (allow) and blacklist (deny) modes
* Operates transparently without affecting request or response processing
* Helps protect sensitive information from being exposed in analytics systems


## Configuration

### Parameters

| Parameter                 | Type   | Required | Default | Description                                                                                                |
| ------------------------- | ------ | -------- | ------- | ---------------------------------------------------------------------------------------------------------- |
| `requestHeadersToFilter`  | object | No       | -       | Configuration for filtering request headers. Contains `operation` and `headers` properties.              |
| `responseHeadersToFilter` | object | No       | -       | Configuration for filtering response headers. Contains `operation` and `headers` properties.              |

### Parameter Structure

Each filter parameter (`requestHeadersToFilter` and `responseHeadersToFilter`) is an object with the following properties:

| Property    | Type   | Required | Description                                                                                                |
| ----------- | ------ | -------- | ---------------------------------------------------------------------------------------------------------- |
| `operation` | string | Yes      | Operation mode: `"allow"` (whitelist) or `"deny"` (blacklist). Header names are matched case-insensitively. |
| `headers`   | array  | Yes      | List of header names to filter. Behavior depends on the operation mode. Each header name must be 1-256 characters. |

> **Note**: This policy only affects analytics data collection. It does not remove or modify headers sent to upstream services or returned to clients.


## System Requirements

* Analytics must be enabled globally via `config.yaml` (`analytics.enabled: true`)
* The policy must be explicitly applied to the API policy chain
* If analytics is disabled at the system level, this policy has no effect


## API Definition Example

The following example demonstrates how to apply the Analytics Header Filter policy to a LlmProvider:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic <base64-credentials>" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: openai-provider
spec:
  displayName: OpenAI Provider
  version: v1.0
  template: openai
  upstream:
    url: https://api.openai.com/v1
    auth:
      type: api-key
      header: Authorization
      value: Bearer <openai-apikey>
  accessControl:
    mode: deny_all
    exceptions:
      - path: /chat/completions
        methods: [POST]
        policies:
          - name: analytics-header-filter
            version: v0.1.0
            params:
              requestHeadersToFilter:
                operation: deny
                headers:
                  - "authorization"
                  - "x-api-key"
              responseHeadersToFilter:
                operation: allow
                headers:
                  - "content-type"
      - path: /models
        methods: [GET]
      - path: /models/{modelId}
        methods: [GET]
EOF
```

## Use Cases

-  **Sensitive Data Protection**: Prevent authentication tokens, internal identifiers, or security-related headers from being sent to analytics systems.

- **Noise Reduction**: Exclude verbose or low-value headers to improve the clarity and usefulness of analytics data.

- **Compliance and Governance**: Support compliance requirements by ensuring certain headers are never exported outside the platform.

- **Cost and Storage Optimization**: Reduce analytics payload size by removing unnecessary headers from published events.


## Notes

* Header name matching is case-insensitive.
* The `operation` field is required and must be either `"allow"` or `"deny"`.
* The `headers` array is required but can be empty. When the array is empty, all original headers are included(if allowed explicitly) in analytics for both `"allow"` and `"deny"` modes (safe fallback behavior).
* Request and response headers can use different operation modes independently.
* This policy does not block requests or responses.
* Filtering applies only to analytics collection, not to runtime request handling.
* The policy must be applied per API and does not operate implicitly.

