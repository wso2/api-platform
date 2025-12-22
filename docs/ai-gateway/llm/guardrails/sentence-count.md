# Sentence Count Guardrail

## Overview

The Sentence Count Guardrail validates the sentence count of request or response body content against configurable minimum and maximum thresholds. This guardrail is useful for ensuring content completeness, controlling response verbosity, and maintaining consistent communication standards.

## Features

- Validates sentence count against minimum and maximum thresholds
- Supports JSONPath extraction to validate specific fields within JSON payloads
- Configurable inverted logic to pass when sentence count is outside the range
- Separate configuration for request and response phases
- Optional detailed assessment information in error responses

## Configuration

### Parameters

#### Request Phase

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `min` | integer | Yes | - | Minimum allowed sentence count (inclusive). Must be >= 0. |
| `max` | integer | Yes | - | Maximum allowed sentence count (inclusive). Must be >= 1. |
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, validates the entire payload as a string. |
| `invert` | boolean | No | `false` | If `true`, validation passes when sentence count is NOT within the min-max range. If `false`, validation passes when sentence count is within the range. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information in error responses. |

#### Response Phase

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `min` | integer | Yes | - | Minimum allowed sentence count (inclusive). Must be >= 0. |
| `max` | integer | Yes | - | Maximum allowed sentence count (inclusive). Must be >= 1. |
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, validates the entire payload as a string. |
| `invert` | boolean | No | `false` | If `true`, validation passes when sentence count is NOT within the min-max range. If `false`, validation passes when sentence count is within the range. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information in error responses. |

## JSONPath Support

The guardrail supports JSONPath expressions to extract and validate specific fields within JSON payloads. Common examples:

- `$.message` - Extracts the `message` field from the root object
- `$.data.content` - Extracts nested content from `data.content`
- `$.items[0].text` - Extracts text from the first item in an array
- `$.messages[0].content` - Extracts content from the first message in a messages array

If `jsonPath` is empty or not specified, the entire payload is treated as a string and validated.

## Sentence Detection

Sentences are detected based on standard sentence-ending punctuation marks:
- Period (.)
- Exclamation mark (!)
- Question mark (?)

The guardrail counts sequences of characters ending with these punctuation marks as sentences.

## Examples

### Example 1: Basic Sentence Count Validation

Deploy an LLM provider that ensures requests contain between 1 and 10 sentences:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  --data-binary @- <<'EOF'
version: gateway.api-platform.wso2.com/v1alpha1
kind: llm/provider
spec:
  name: sentence-count-provider
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
    - name: SentenceCountGuardrail
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            request:
              min: 1
              max: 10
              jsonPath: "$.messages[0].content"
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
        "content": "What is machine learning? How does it work? Can you explain it simply?"
      }
    ]
  }'

# Invalid request - too few sentences (should fail with HTTP 422)
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

- **Request and Response Validation**: Configure both `request` and `response` parameters to validate sentence counts in both directions. Use `showAssessment: true` to include detailed assessment information in error responses.

- **Inverted Logic**: Set `invert: true` to allow only content *outside* the specified sentence range. This is useful for blocking content that falls within a prohibited sentence count range.

- **Full Payload Validation**: Omit the `jsonPath` parameter to validate the entire request body without JSONPath extraction.

- **Field-Specific Validation**: Use `jsonPath` to extract and validate specific fields within JSON payloads (e.g., `"$.messages[0].content"` for message content or `"$.choices[0].message.content"` for response content).

## Use Cases

1. **Content Quality Assurance**: Ensure responses meet minimum sentence requirements for completeness and clarity.

2. **Response Length Control**: Limit verbosity to maintain concise communication standards.

3. **Input Validation**: Ensure user prompts contain sufficient context (minimum sentences) without being excessive.

4. **Consistency Enforcement**: Maintain consistent response formats across different AI interactions.

5. **Cost Management**: Control response length to manage token usage and associated costs.

## Error Response

When validation fails, the guardrail returns an HTTP 422 status code with the following structure:

```json
{
  "type": "SENTENCE_COUNT_GUARDRAIL",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "SentenceCountGuardrail",
    "actionReason": "Violation of applied sentence count constraints detected.",
    "direction": "REQUEST"
  }
}
```

If `showAssessment` is enabled, additional details are included:

```json
{
  "type": "SENTENCE_COUNT_GUARDRAIL",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "SentenceCountGuardrail",
    "actionReason": "Violation of applied sentence count constraints detected.",
    "assessments": "Violation of sentence count detected. Expected between 1 and 3 sentences.",
    "direction": "REQUEST"
  }
}
```

## Notes

- Sentence counting is performed on the extracted or full content after trimming whitespace.
- Sentences are identified by standard punctuation marks (., !, ?).
- When using JSONPath, if the path does not exist or the extracted value is not a string, validation will fail.
- Inverted logic is useful for blocking content that falls outside acceptable sentence count ranges.
- Consider the nature of your content when setting thresholds, as some content types may naturally have different sentence counts.
