# Content Length Guardrail

## Overview

The Content Length Guardrail validates the byte length of request or response body content against configurable minimum and maximum thresholds. This guardrail is essential for controlling payload sizes, preventing resource exhaustion, and ensuring efficient data transfer.

## Features

- Validates byte length against minimum and maximum thresholds
- Supports JSONPath extraction to validate specific fields within JSON payloads
- Configurable inverted logic to pass when content length is outside the range
- Separate configuration for request and response phases
- Optional detailed assessment information in error responses

## Configuration

### Parameters

#### Request Phase

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `min` | integer | Yes | - | Minimum allowed byte length (inclusive). Must be >= 0. |
| `max` | integer | Yes | - | Maximum allowed byte length (inclusive). Must be >= 1. |
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, validates the entire payload as a string. |
| `invert` | boolean | No | `false` | If `true`, validation passes when content length is NOT within the min-max range. If `false`, validation passes when content length is within the range. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information in error responses. |

#### Response Phase

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `min` | integer | Yes | - | Minimum allowed byte length (inclusive). Must be >= 0. |
| `max` | integer | Yes | - | Maximum allowed byte length (inclusive). Must be >= 1. |
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, validates the entire payload as a string. |
| `invert` | boolean | No | `false` | If `true`, validation passes when content length is NOT within the min-max range. If `false`, validation passes when content length is within the range. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information in error responses. |

## JSONPath Support

The guardrail supports JSONPath expressions to extract and validate specific fields within JSON payloads. Common examples:

- `$.message` - Extracts the `message` field from the root object
- `$.data.content` - Extracts nested content from `data.content`
- `$.items[0].text` - Extracts text from the first item in an array
- `$.messages[0].content` - Extracts content from the first message in a messages array

If `jsonPath` is empty or not specified, the entire payload is treated as a string and validated.

## Examples

### Example 1: Basic Content Length Validation

Deploy an LLM provider that limits request payloads to between 100 bytes and 1MB:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  --data-binary @- <<'EOF'
version: ai.api-platform.wso2.com/v1
kind: llm/provider
spec:
  name: content-length-provider
  version: v1.0
  template: openai
  vhost: openai
  upstream:
    url: https://api.openai.com/v1
    auth:
      type: api-key
      header: Authorization
      value: <openai-apikey>
  accessControl:
    mode: deny_all
    exceptions:
      - path: /chat/completions
        methods: [POST]
      - path: /models
        methods: [GET]
      - path: /models/{modelId}
        methods: [GET]
  policies:
    - name: ContentLengthGuardrail
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            request:
              min: 100
              max: 1048576
EOF
```

**Test the guardrail:**

**Note**: Ensure that "openai" is mapped to the appropriate IP address (e.g., 127.0.0.1) in your `/etc/hosts` file. or remove the vhost from the llm provider configuration and use localhost to invoke.

```bash
# Valid request (should pass)
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Please explain artificial intelligence in simple terms for beginners"
      }
    ]
  }'

# Invalid request - too small (should fail with HTTP 422)
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Hi"
      }
    ]
  }'
```

### Additional Configuration Options

You can customize the guardrail behavior by modifying the `policies` section:

- **Request and Response Validation**: Configure both `request` and `response` parameters to validate byte lengths in both directions. Use `showAssessment: true` to include detailed assessment information in error responses.

- **Inverted Logic**: Set `invert: true` to allow only content *outside* the specified byte range. This is useful for blocking content that falls within a prohibited size range.

- **Full Payload Validation**: Omit the `jsonPath` parameter to validate the entire request body without JSONPath extraction.

- **Field-Specific Validation**: Use `jsonPath` to extract and validate specific fields within JSON payloads (e.g., `"$.messages[0].content"` for message content or `"$.choices[0].message.content"` for response content).

## Use Cases

1. **Resource Protection**: Prevent excessively large payloads that could exhaust system resources or cause performance degradation.

2. **Network Optimization**: Control payload sizes to optimize network transfer times and reduce bandwidth costs.

3. **Storage Management**: Limit content sizes to manage storage requirements effectively.

4. **API Rate Limiting**: Enforce size constraints as part of rate limiting strategies.

5. **Quality Assurance**: Ensure responses meet minimum size requirements for completeness.

## Error Response

When validation fails, the guardrail returns an HTTP 422 status code with the following structure:

```json
{
  "type": "CONTENT_LENGTH_GUARDRAIL",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "ContentLengthGuardrail",
    "actionReason": "Violation of applied content length constraints detected.",
    "direction": "REQUEST"
  }
}
```

If `showAssessment` is enabled, additional details are included:

```json
{
  "type": "CONTENT_LENGTH_GUARDRAIL",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "ContentLengthGuardrail",
    "actionReason": "Violation of applied content length constraints detected.",
    "assessments": "Violation of content length detected. Expected between 10 and 100 bytes.",
    "direction": "REQUEST"
  }
}
```

## Notes

- Byte length is calculated on the UTF-8 encoded representation of the content.
- When using JSONPath, if the path does not exist or the extracted value is not a string, validation will fail.
- Inverted logic is useful for blocking content that falls outside acceptable size ranges.
- Consider network and storage constraints when setting maximum values.
- Minimum values help ensure content quality and completeness.
