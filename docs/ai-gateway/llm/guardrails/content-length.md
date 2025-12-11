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

Limit request payloads to between 100 bytes and 1MB:

```yaml
policies:
  - name: ContentLengthGuardrail
    version: v0.1.0
    enabled: true
    params:
      request:
        min: 100
        max: 1048576
```

### Example 2: Response Size Control

Ensure AI responses are substantial (at least 500 bytes) but not excessive (maximum 100KB):

```yaml
policies:
  - name: ContentLengthGuardrail
    version: v0.1.0
    enabled: true
    params:
      response:
        min: 500
        max: 102400
        jsonPath: "$.choices[0].message.content"
        showAssessment: true
```

### Example 3: Field-Specific Validation

Validate a specific field within the JSON payload:

```yaml
policies:
  - name: ContentLengthGuardrail
    version: v0.1.0
    enabled: true
    params:
      request:
        min: 10
        max: 1000
        jsonPath: "$.messages[0].content"
```

### Example 4: Inverted Logic

Block requests that are too small (less than 50 bytes) or too large (more than 10MB) using inverted logic:

```yaml
policies:
  - name: ContentLengthGuardrail
    version: v0.1.0
    enabled: true
    params:
      request:
        min: 50
        max: 10485760
        invert: true
```

## Use Cases

1. **Resource Protection**: Prevent excessively large payloads that could exhaust system resources or cause performance degradation.

2. **Network Optimization**: Control payload sizes to optimize network transfer times and reduce bandwidth costs.

3. **Storage Management**: Limit content sizes to manage storage requirements effectively.

4. **API Rate Limiting**: Enforce size constraints as part of rate limiting strategies.

5. **Quality Assurance**: Ensure responses meet minimum size requirements for completeness.

## Error Response

When validation fails, the guardrail returns an HTTP 446 status code with the following structure:

```json
{
    "code": 900514,
    "message": {
        "action": "GUARDRAIL_INTERVENED",
        "actionReason": "Violation of applied content length constraints detected.",
        "direction": "REQUEST",
        "interveningGuardrail": "ContentLengthGuardrail"
    },
    "type": "CONTENT_LENGTH_GUARDRAIL"
}
```

If `showAssessment` is enabled, additional details are included:

```json
{
    "code": 900514,
    "message": {
        "action": "GUARDRAIL_INTERVENED",
        "actionReason": "Violation of applied content length constraints detected.",
        "assessments": "Violation of content length detected. Expected between 10 and 100 bytes.",
        "direction": "REQUEST",
        "interveningGuardrail": "Content Length Guardrail"
    },
    "type": "CONTENT_LENGTH_GUARDRAIL"
}
```

## Notes

- Byte length is calculated on the UTF-8 encoded representation of the content.
- When using JSONPath, if the path does not exist or the extracted value is not a string, validation will fail.
- Inverted logic is useful for blocking content that falls outside acceptable size ranges.
- Consider network and storage constraints when setting maximum values.
- Minimum values help ensure content quality and completeness.
