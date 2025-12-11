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

Ensure requests contain between 1 and 10 sentences:

```yaml
policies:
  - name: SentenceCountGuardrail
    version: v1.0.0
    enabled: true
    params:
      request:
        min: 1
        max: 10
        jsonPath: "$.messages[0].content"
```

### Example 2: Response Quality Control

Ensure AI responses are comprehensive (at least 3 sentences) but concise (maximum 50 sentences):

```yaml
policies:
  - name: SentenceCountGuardrail
    version: v1.0.0
    enabled: true
    params:
      response:
        min: 3
        max: 50
        jsonPath: "$.choices[0].message.content"
        showAssessment: true
```

### Example 3: Inverted Logic

Block requests that are too brief (less than 2 sentences) or too verbose (more than 20 sentences):

```yaml
policies:
  - name: SentenceCountGuardrail
    version: v1.0.0
    enabled: true
    params:
      request:
        min: 2
        max: 20
        invert: true
        jsonPath: "$.messages[0].content"
```

### Example 4: Full Payload Validation

Validate the entire request body without JSONPath extraction:

```yaml
policies:
  - name: SentenceCountGuardrail
    version: v1.0.0
    enabled: true
    params:
      request:
        min: 1
        max: 100
```

## Use Cases

1. **Content Quality Assurance**: Ensure responses meet minimum sentence requirements for completeness and clarity.

2. **Response Length Control**: Limit verbosity to maintain concise communication standards.

3. **Input Validation**: Ensure user prompts contain sufficient context (minimum sentences) without being excessive.

4. **Consistency Enforcement**: Maintain consistent response formats across different AI interactions.

5. **Cost Management**: Control response length to manage token usage and associated costs.

## Error Response

When validation fails, the guardrail returns an HTTP 446 status code with the following structure:

```json
{
  "code": 900514,
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
    "code": 900514,
    "type": "SENTENCE_COUNT_GUARDRAIL",
    "message": {
        "action": "GUARDRAIL_INTERVENED",
        "interveningGuardrail": "Sentence Count Guardrail",
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
