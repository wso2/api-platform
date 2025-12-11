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

Validate that request messages contain between 10 and 500 words:

```yaml
policies:
  - name: WordCountGuardrail
    version: v0.1.0
    enabled: true
    params:
      request:
        min: 10
        max: 500
        jsonPath: "$.messages[0].content"
```

### Example 2: Response Quality Control

Ensure AI responses are comprehensive (at least 50 words) but not excessive (maximum 2000 words):

```yaml
policies:
  - name: WordCountGuardrail
    version: v0.1.0
    enabled: true
    params:
      response:
        min: 50
        max: 2000
        jsonPath: "$.choices[0].message.content"
        showAssessment: true
```

### Example 3: Inverted Logic

Block requests that are too short (less than 5 words) or too long (more than 1000 words) using inverted logic:

```yaml
policies:
  - name: WordCountGuardrail
    version: v0.1.0
    enabled: true
    params:
      request:
        min: 5
        max: 1000
        invert: true
        jsonPath: "$.messages[0].content"
```

### Example 4: Full Payload Validation

Validate the entire request body without JSONPath extraction:

```yaml
policies:
  - name: WordCountGuardrail
    version: v0.1.0
    enabled: true
    params:
      request:
        min: 1
        max: 10000
```

## Use Cases

1. **Input Length Control**: Prevent users from submitting extremely long prompts that could impact system performance or costs.

2. **Response Quality Assurance**: Ensure AI-generated responses meet minimum length requirements for completeness.

3. **Cost Management**: Limit response lengths to control token usage and associated costs.

4. **Content Filtering**: Use inverted logic to block content that falls outside acceptable word count ranges.

## Error Response

When validation fails, the guardrail returns an HTTP 446 status code with the following structure:

```json
{
  "code": 900514,
  "type": "WORD_COUNT_GUARDRAIL",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "WordCountGuardrail",
    "actionReason": "Violation of applied word count constraints detected.",
    "direction": "REQUEST"
  }
}
```

If `showAssessment` is enabled, additional details are included:

```json
{
  "code": 900514,
  "type": "WORD_COUNT_GUARDRAIL",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "WordCountGuardrail",
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
