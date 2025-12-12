# Regex Guardrail

## Overview

The Regex Guardrail validates request or response body content against regular expression patterns. This guardrail enables pattern-based content validation, allowing you to enforce specific formats, detect prohibited patterns, or ensure content matches expected structures.

## Features

- Pattern matching using regular expressions
- Supports JSONPath extraction to validate specific fields within JSON payloads
- Configurable inverted logic to pass when pattern does not match
- Separate configuration for request and response phases
- Optional detailed assessment information in error responses

## Configuration

### Parameters

#### Request Phase

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `regex` | string | Yes | - | Regular expression pattern to match against the content. Must be at least 1 character. |
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, validates the entire payload as a string. |
| `invert` | boolean | No | `false` | If `true`, validation passes when regex does NOT match. If `false`, validation passes when regex matches. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information in error responses. |

#### Response Phase

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `regex` | string | Yes | - | Regular expression pattern to match against the content. Must be at least 1 character. |
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, validates the entire payload as a string. |
| `invert` | boolean | No | `false` | If `true`, validation passes when regex does NOT match. If `false`, validation passes when regex matches. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed assessment information in error responses. |

## JSONPath Support

The guardrail supports JSONPath expressions to extract and validate specific fields within JSON payloads. Common examples:

- `$.message` - Extracts the `message` field from the root object
- `$.data.content` - Extracts nested content from `data.content`
- `$.items[0].text` - Extracts text from the first item in an array
- `$.messages[0].content` - Extracts content from the first message in a messages array

If `jsonPath` is empty or not specified, the entire payload is treated as a string and validated.

## Regular Expression Syntax

The guardrail uses Go's standard regexp package, which supports RE2 syntax. Key features:

- Case-sensitive matching by default
- Use `(?i)` flag for case-insensitive matching
- Anchors: `^` (start), `$` (end)
- Character classes: `[a-z]`, `[0-9]`, `\d`, `\w`, `\s`
- Quantifiers: `*`, `+`, `?`, `{n}`, `{n,m}`
- Groups and alternation: `(abc|def)`, `(?:non-capturing)`

## Examples

### Example 1: Email Validation

Ensure user input contains a valid email address:

```yaml
policies:
  - name: RegexGuardrail
    version: v0.1.0
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          request:
            regex: "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$"
            jsonPath: "$.messages[0].content"
```

### Example 2: Block Prohibited Patterns

Block requests containing password-related content using inverted logic:

```yaml
policies:
  - name: RegexGuardrail
    version: v0.1.0
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          request:
            regex: "(?i).*password.*"
            invert: true
            jsonPath: "$.messages[0].content"
```

### Example 3: Response Format Validation

Ensure AI responses follow a specific format (e.g., must start with a capital letter):

```yaml
policies:
  - name: RegexGuardrail
    version: v0.1.0
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          response:
            regex: "^[A-Z].*"
            jsonPath: "$.choices[0].message.content"
            showAssessment: true
```

### Example 4: Phone Number Validation

Validate phone numbers in a specific format:

```yaml
policies:
  - name: RegexGuardrail
    version: v0.1.0
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          request:
            regex: "^\\+?[1-9]\\d{1,14}$"
            jsonPath: "$.phone"
```

### Example 5: Block Admin-Related Content

Prevent admin-related requests using case-insensitive matching and inverted logic:

```yaml
policies:
  - name: RegexGuardrail
    version: v0.1.0
    paths:
      - path: /chat/completions
        methods: [POST]
        params:
          request:
            regex: "(?i).*admin.*"
            invert: true
            jsonPath: "$.messages[0].content"
```

## Use Cases

1. **Format Validation**: Ensure user inputs match expected formats (emails, phone numbers, IDs).

2. **Content Filtering**: Block or allow content based on pattern matching (prohibited words, sensitive patterns).

3. **Security Enforcement**: Detect and block potentially malicious patterns or injection attempts.

4. **Data Quality**: Ensure responses follow specific formatting requirements or contain required elements.

5. **Compliance**: Enforce patterns required by regulatory standards or business rules.

## Error Response

When validation fails, the guardrail returns an HTTP 446 status code with the following structure:

```json
{
    "code": 900514,
    "message": {
        "action": "GUARDRAIL_INTERVENED",
        "actionReason": "Violation of regular expression detected.",
        "assessments": "Violated regular expression: (?i)ignore\\s+all\\s+previous\\s+instructions",
        "direction": "REQUEST",
        "interveningGuardrail": "RegexGuardrail"
    },
    "type": "REGEX_GUARDRAIL"
}
```

If `showAssessment` is enabled, additional details are included:

```json
{
    "code": 900514,
    "message": {
        "action": "GUARDRAIL_INTERVENED",
        "actionReason": "Violation of regular expression detected.",
        "assessments": "Violated regular expression: (?i)ignore\\s+all\\s+previous\\s+instructions",
        "direction": "REQUEST",
        "interveningGuardrail": "Regex Guardrail"
    },
    "type": "REGEX_GUARDRAIL"
}
```

## Notes

- Regular expressions are evaluated using Go's regexp package (RE2 syntax).
- Pattern matching is case-sensitive by default. Use `(?i)` flag for case-insensitive matching.
- When using JSONPath, if the path does not exist or the extracted value is not a string, validation will fail.
- Inverted logic is useful for blocking content that matches prohibited patterns.
- Complex regex patterns may impact performance; test thoroughly with expected content volumes.
