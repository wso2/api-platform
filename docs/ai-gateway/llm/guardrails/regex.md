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

Deploy an LLM provider that protects against sensitive data leaks by blocking any payloads that mention the word "password" (case-insensitive) in either the user’s message or the LLM’s response. This is achieved by using the regex policy to validate both request and response payloads:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: regex-provider
spec:
  displayName: Regex Provider
  version: v1.0
  template: openai
  vhost: openai
  upstream:
    url: "https://api.openai.com/v1"
    auth:
      type: api-key
      header: Authorization
      value: Bearer <openai-apikey>
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
    - name: regex-guardrail
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            request:
              regex: "(?i).*password.*"
              invert: true
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
        "content": "This is a safe message without sensitive data"
      }
    ]
  }'

# Invalid request - no email (should fail with HTTP 422)
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "My password is 1234567"
      }
    ]
  }'
```

### Additional Configuration Options

You can customize the guardrail behavior by modifying the `policies` section:

- **Request and Response Validation**: Configure both `request` and `response` parameters to validate patterns in both directions. Use `showAssessment: true` to include detailed assessment information in error responses.

- **Inverted Logic**: Set `invert: true` to allow only content that does *not* match the regex pattern. This is useful for blocking prohibited patterns (e.g., password-related content, admin keywords).

- **Full Payload Validation**: Omit the `jsonPath` parameter to validate the entire request body without JSONPath extraction.

- **Field-Specific Validation**: Use `jsonPath` to extract and validate specific fields within JSON payloads (e.g., `"$.messages[0].content"` for message content or `"$.choices[0].message.content"` for response content).

## Use Cases

1. **Format Validation**: Ensure user inputs match expected formats (emails, phone numbers, IDs).

2. **Content Filtering**: Block or allow content based on pattern matching (prohibited words, sensitive patterns).

3. **Security Enforcement**: Detect and block potentially malicious patterns or injection attempts.

4. **Data Quality**: Ensure responses follow specific formatting requirements or contain required elements.

5. **Compliance**: Enforce patterns required by regulatory standards or business rules.

## Error Response

When validation fails, the guardrail returns an HTTP 422 status code with the following structure:

```json
{
  "type": "REGEX_GUARDRAIL",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "regex-guardrail",
    "actionReason": "Violation of regular expression detected.",
    "direction": "REQUEST"
  }
}
```

If `showAssessment` is enabled, additional details are included:

```json
{
  "type": "REGEX_GUARDRAIL",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "regex-guardrail",
    "actionReason": "Violation of regular expression detected.",
    "assessments": "Violation of regular expression detected. (?i)ignore\\s+all\\s+previous\\s+instructions",
    "direction": "REQUEST"
  }
}
```

## Notes

- Regular expressions are evaluated using Go's regexp package (RE2 syntax).
- Pattern matching is case-sensitive by default. Use `(?i)` flag for case-insensitive matching.
- When using JSONPath, if the path does not exist or the extracted value is not a string, validation will fail.
- Inverted logic is useful for blocking content that matches prohibited patterns.
- Complex regex patterns may impact performance; test thoroughly with expected content volumes.
