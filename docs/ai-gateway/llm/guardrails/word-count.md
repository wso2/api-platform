# Word Count Guardrail

## Overview

The Word Count Guardrail validates the word count of request or response body content against configurable minimum and maximum thresholds. This guardrail is useful for enforcing content length policies, ensuring responses meet quality standards, or preventing excessively long inputs that could impact system performance.

## Features

- Validates word count against minimum and maximum thresholds
- Supports JSONPath extraction to validate specific fields within JSON payloads
- Configurable inverted logic to pass when word count is outside the range
- Separate configuration for request and response phases
- Optional detailed assessment information in error responses

## Configuration

### Parameters

#### Request Phase

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `min` | integer | Yes | - | Minimum allowed word count (inclusive). Must be >= 0. |
| `max` | integer | Yes | - | Maximum allowed word count (inclusive). Must be >= 1. |
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, validates the entire payload as a string. |
| `invert` | boolean | No | `false` | If `true`, validation passes when word count is NOT within the min-max range. If `false`, validation passes when word count is within the range. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information in error responses. |

#### Response Phase

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `min` | integer | Yes | - | Minimum allowed word count (inclusive). Must be >= 0. |
| `max` | integer | Yes | - | Maximum allowed word count (inclusive). Must be >= 1. |
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, validates the entire payload as a string. |
| `invert` | boolean | No | `false` | If `true`, validation passes when word count is NOT within the min-max range. If `false`, validation passes when word count is within the range. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information in error responses. |

## JSONPath Support

The guardrail supports JSONPath expressions to extract and validate specific fields within JSON payloads. Common examples:

- `$.message` - Extracts the `message` field from the root object
- `$.data.content` - Extracts nested content from `data.content`
- `$.items[0].text` - Extracts text from the first item in an array
- `$.messages[0].content` - Extracts content from the first message in a messages array

If `jsonPath` is empty or not specified, the entire payload is treated as a string and validated.

## Examples

### Example 1: Basic Word Count Validation

Deploy an LLM provider that validates request messages contain between 10 and 500 words:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  --data-binary @- <<'EOF'
version: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: word-count-provider
spec:
  displayName: Word Count Provider
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
    - name: word-count-guardrail
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            request:
              min: 10
              max: 500
              jsonPath: "$.messages[0].content"
EOF
```

**Test the guardrail:**

**Note**: Ensure that "openai" is mapped to the appropriate IP address (e.g., 127.0.0.1) in your `/etc/hosts` file. or remove the vhost from the llm provider cionfiguration and use localhost to invoke.

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

# Invalid request - too few words (should fail with HTTP 422)
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

- **Request and Response Validation**: Configure both `request` and `response` parameters to validate word counts in both directions. Use `showAssessment: true` to include detailed assessment information in error responses.

- **Inverted Logic**: Set `invert: true` to allow only content *outside* the specified word range. This is useful for blocking content that falls within a prohibited range.

- **Full Payload Validation**: Omit the `jsonPath` parameter to validate the entire request body without JSONPath extraction.

- **Field-Specific Validation**: Use `jsonPath` to extract and validate specific fields within JSON payloads (e.g., `"$.messages[0].content"` for message content or `"$.choices[0].message.content"` for response content).


## Use Cases

1. **Input Length Control**: Prevent users from submitting extremely long prompts that could impact system performance or costs.

2. **Response Quality Assurance**: Ensure AI-generated responses meet minimum length requirements for completeness.

3. **Cost Management**: Limit response lengths to control token usage and associated costs.

4. **Content Filtering**: Use inverted logic to block content that falls outside acceptable word count ranges.

## Error Response

When validation fails, the guardrail returns an HTTP 422 status code with the following structure:

```json
{
  "type": "WORD_COUNT_GUARDRAIL",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "word-count-guardrail",
    "actionReason": "Violation of applied word count constraints detected.",
    "direction": "REQUEST"
  }
}
```

If `showAssessment` is enabled, additional details are included:

```json
{
  "type": "WORD_COUNT_GUARDRAIL",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "word-count-guardrail",
    "actionReason": "Violation of applied word count constraints detected.",
    "assessments": "Violation of word count detected. Expected between 2 and 10 words.",
    "direction": "REQUEST"
  }
}
```

## Notes

- Word counting is performed on the extracted or full content after trimming whitespace.
- The validation is case-sensitive and counts all words separated by whitespace.
- When using JSONPath, if the path does not exist or the extracted value is not a string, validation will fail.
- Inverted logic is useful for blocking content that falls outside acceptable ranges rather than within them.
