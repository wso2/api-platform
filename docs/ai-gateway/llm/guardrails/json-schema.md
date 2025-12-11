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

Validate that request contains a user object with required fields:

```yaml
policies:
  - name: JSONSchemaGuardrail
    version: v0.1.0
    enabled: true
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
```

### Example 2: Field-Specific Validation

Validate a specific field within the JSON payload:

```yaml
policies:
  - name: JSONSchemaGuardrail
    version: v0.1.0
    enabled: true
    params:
      request:
        schema: |
          {
            "type": "object",
            "properties": {
              "content": {"type": "string", "minLength": 10, "maxLength": 1000}
            },
            "required": ["content"]
          }
        jsonPath: "$.messages[0]"
```

### Example 3: Array Validation

Validate that response contains an array of items with specific structure:

```yaml
policies:
  - name: JSONSchemaGuardrail
    version: v0.1.0
    enabled: true
    params:
      response:
        schema: |
          {
            "type": "array",
            "items": {
              "type": "object",
              "properties": {
                "id": {"type": "string"},
                "title": {"type": "string", "minLength": 1},
                "score": {"type": "number", "minimum": 0, "maximum": 100}
              },
              "required": ["id", "title"]
            },
            "minItems": 1,
            "maxItems": 100
          }
        jsonPath: "$.results"
        showAssessment: true
```

### Example 4: Complex Nested Validation

Validate nested structures with multiple levels:

```yaml
policies:
  - name: JSONSchemaGuardrail
    version: v0.1.0
    enabled: true
    params:
      request:
        schema: |
          {
            "type": "object",
            "properties": {
              "user": {
                "type": "object",
                "properties": {
                  "profile": {
                    "type": "object",
                    "properties": {
                      "firstName": {"type": "string"},
                      "lastName": {"type": "string"},
                      "preferences": {
                        "type": "array",
                        "items": {"type": "string"}
                      }
                    },
                    "required": ["firstName", "lastName"]
                  }
                },
                "required": ["profile"]
              }
            },
            "required": ["user"]
          }
```

### Example 5: Inverted Logic

Block requests that match a specific schema pattern:

```yaml
policies:
  - name: JSONSchemaGuardrail
    version: v0.1.0
    enabled: true
    params:
      request:
        schema: |
          {
            "type": "object",
            "properties": {
              "sensitive": {"type": "boolean"}
            },
            "required": ["sensitive"]
          }
        invert: true
```

## Use Cases

1. **API Contract Enforcement**: Ensure requests and responses conform to API specifications.

2. **Data Quality**: Validate data structure and types before processing.

3. **Security**: Enforce required fields and prevent injection of unexpected data structures.

4. **Integration**: Ensure compatibility with downstream systems that expect specific formats.

5. **Compliance**: Enforce data formats required by regulatory standards.

## Error Response

When validation fails, the guardrail returns an HTTP 446 status code with the following structure:

```json
{
  "code": 900514,
  "type": "JSON_SCHEMA_GUARDRAIL",
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "actionReason": "Violation of JSON schema detected.",
    "direction": "REQUEST",
    "interveningGuardrail": "JSONSchemaGuardrail"
  }
}
```

If `showAssessment` is enabled, detailed validation errors are included:

```json
{
  "code": 900514,
  "message": {
    "action": "GUARDRAIL_INTERVENED",
    "actionReason": "Violation of JSON schema detected.",
    "assessments": [
      {
        "description": "String length must be greater than or equal to 5",
        "field": "messages.0.content",
        "value": "Hi"
      }
    ],
    "direction": "REQUEST",
    "interveningGuardrail": "JSONSchemaGuardrail"
  },
  "type": "JSON_SCHEMA_GUARDRAIL"
}
```

## Notes

- The schema must be valid JSON. Use proper escaping when embedding in YAML.
- JSON Schema Draft 7 is supported with all standard features.
- When using JSONPath, if the path does not exist or the extracted value is not valid JSON, validation will fail.
- Inverted logic is useful for blocking content that matches specific schema patterns.
- Complex schemas may impact performance; test thoroughly with expected content volumes.
- The guardrail validates the structure and types but does not validate business logic or semantic meaning.
