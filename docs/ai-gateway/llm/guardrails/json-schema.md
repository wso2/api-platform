# JSON Schema Guardrail

## Overview

The JSON Schema Guardrail validates request or response body content against a JSON Schema definition. This guardrail enables structured data validation, ensuring that JSON payloads conform to expected formats, data types, and constraints.

## Features

- Validates content against JSON Schema Draft 7
- Supports JSONPath extraction to validate specific fields within JSON payloads
- Configurable inverted logic to pass when schema validation fails
- Separate configuration for request and response phases
- Detailed validation error information in error responses

## Configuration

### Parameters

#### Request Phase

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `schema` | string | Yes | - | JSON Schema as a string (must be valid JSON). Supports all JSON Schema draft 7 features. |
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload for validation. If empty, validates the entire payload against the schema. |
| `invert` | boolean | No | `false` | If `true`, validation passes when schema validation FAILS. If `false`, validation passes when schema validation succeeds. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed validation error information in error responses. |

#### Response Phase

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `schema` | string | Yes | - | JSON Schema as a string (must be valid JSON). Supports all JSON Schema draft 7 features. |
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload for validation. If empty, validates the entire payload against the schema. |
| `invert` | boolean | No | `false` | If `true`, validation passes when schema validation FAILS. If `false`, validation passes when schema validation succeeds. |
| `showAssessment` | boolean | No | `false` | If `true`, includes detailed validation error information in error responses. |

## JSONPath Support

The guardrail supports JSONPath expressions to extract and validate specific fields within JSON payloads. Common examples:

- `$.data` - Extracts the `data` object for validation
- `$.userInfo` - Extracts user information object
- `$.items[0]` - Extracts the first item in an array
- `$.messages[0]` - Extracts the first message object

If `jsonPath` is empty or not specified, the entire payload is validated against the schema.

## JSON Schema Features

The guardrail supports JSON Schema Draft 7, including:

- **Types**: `string`, `number`, `integer`, `boolean`, `object`, `array`, `null`
- **Properties**: Define object properties and their schemas
- **Required Fields**: Specify which properties are mandatory
- **Constraints**: `minLength`, `maxLength`, `minimum`, `maximum`, `pattern`, `enum`
- **Nested Structures**: Complex nested objects and arrays
- **Conditional Logic**: `if`, `then`, `else`, `allOf`, `anyOf`, `oneOf`, `not`

## Examples

### Example 1: Basic Object Validation

Deploy an LLM provider that validates that request contains a user object with required fields:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  --data-binary @- <<'EOF'
version: gateway.api-platform.wso2.com/v1alpha1
kind: llm/provider
spec:
  name: json-schema-provider
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
    - name: JSONSchemaGuardrail
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
          params:
            request:
              schema: |
                {
                  "type": "object",
                  "properties": {
                    "name": {"type": "string", "minLength": 1},
                    "email": {"type": "string", "format": "email"},
                    "age": {"type": "integer", "minimum": 18}
                  },
                  "required": ["name", "email"]
                }
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
        "content": "Hello"
      }
    ],
    "name": "John Doe",
    "email": "john@example.com",
    "age": 25
  }'

# Invalid request - missing required fields (should fail with HTTP 422)
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Hello"
      }
    ]
  }'
```

### Additional Configuration Options

You can customize the guardrail behavior by modifying the `policies` section:

- **Request and Response Validation**: Configure both `request` and `response` parameters to validate JSON schemas in both directions. Use `showAssessment: true` to include detailed validation error information in error responses.

- **Inverted Logic**: Set `invert: true` to allow only content that does *not* match the schema. This is useful for blocking requests that match specific schema patterns.

- **Full Payload Validation**: Omit the `jsonPath` parameter to validate the entire request body against the schema.

- **Field-Specific Validation**: Use `jsonPath` to extract and validate specific fields within JSON payloads (e.g., `"$.messages[0]"` for message objects or `"$.results"` for response arrays).

## Use Cases

1. **API Contract Enforcement**: Ensure requests and responses conform to API specifications.

2. **Data Quality**: Validate data structure and types before processing.

3. **Security**: Enforce required fields and prevent injection of unexpected data structures.

4. **Integration**: Ensure compatibility with downstream systems that expect specific formats.

5. **Compliance**: Enforce data formats required by regulatory standards.

## Error Response

When validation fails, the guardrail returns an HTTP 422 status code with the following structure:

```json
{
  "type": "JSON_SCHEMA_GUARDRAIL",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "JSONSchemaGuardrail",
    "actionReason": "Violation of JSON schema detected.",
    "direction": "REQUEST"
  }
}
```

If `showAssessment` is enabled, detailed validation errors are included:

```json
{
  "type": "JSON_SCHEMA_GUARDRAIL",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "interveningGuardrail": "JSONSchemaGuardrail",
    "actionReason": "Violation of JSON schema detected.",
    "assessments": [
      {
        "description": "String length must be greater than or equal to 5",
        "field": "messages.0.content",
        "value": "Hi"
      }
    ],
    "direction": "REQUEST"
  }
}
```

## Notes

- The schema must be valid JSON. Use proper escaping when embedding in YAML.
- JSON Schema Draft 7 is supported with all standard features.
- When using JSONPath, if the path does not exist or the extracted value is not valid JSON, validation will fail.
- Inverted logic is useful for blocking content that matches specific schema patterns.
- Complex schemas may impact performance; test thoroughly with expected content volumes.
- The guardrail validates the structure and types but does not validate business logic or semantic meaning.
