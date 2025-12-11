# PII Masking Regex Guardrail

## Overview

The PII Masking Regex Guardrail masks or redacts Personally Identifiable Information (PII) from request and response bodies using configurable regular expression patterns. This guardrail helps protect sensitive user data by replacing PII with placeholders or redaction markers before content is processed or returned.

## Features

- Configurable PII entity detection using regular expressions
- Two modes: masking (reversible) and redaction (permanent)
- Automatic PII restoration in responses when using masking mode
- Supports JSONPath extraction to process specific fields within JSON payloads
- Separate configuration for request and response phases

## Configuration

### Request Phase Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `piiEntities` | array | Yes | - | Array of PII entity configurations. Each entity contains `piiEntity` (name/type) and `piiRegex` (regular expression pattern). |
| `jsonPath` | string | No | `""` | JSONPath expression to extract a specific value from JSON payload. If empty, processes the entire payload as a string. |
| `redactPII` | boolean | No | `false` | If `true`, redacts PII by replacing with "*****" (permanent, cannot be restored). If `false`, masks PII with placeholders that can be restored in responses. |

### Response Phase Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `redactPII` | boolean | No | `false` | If `true`, no restoration is performed (PII was permanently redacted). If `false`, restores masked PII from request phase. |

### PII Entity Configuration

Each PII entity in the `piiEntities` array must contain:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `piiEntity` | string | Yes | Name/type of the PII entity (e.g., "EMAIL", "PHONE", "SSN", "CREDIT_CARD"). |
| `piiRegex` | string | Yes | Regular expression pattern to match the PII entity. |

## JSONPath Support

The guardrail supports JSONPath expressions to extract and process specific fields within JSON payloads. Common examples:

- `$.message` - Extracts the `message` field from the root object
- `$.data.content` - Extracts nested content from `data.content`
- `$.items[0].text` - Extracts text from the first item in an array
- `$.messages[0].content` - Extracts content from the first message in a messages array

If `jsonPath` is empty or not specified, the entire payload is processed as a string.

## PII Masking Modes

### Masking Mode (`redactPII: false`)

- PII is replaced with placeholders in the format `[ENTITY_TYPE_XXXX]` (e.g., `[EMAIL_0001]`, `[PHONE_0002]`)
- Placeholders can be restored in responses to their original values
- Original PII values are stored temporarily for restoration
- Recommended when you need to preserve data for downstream processing or response generation

### Redaction Mode (`redactPII: true`)

- PII is permanently replaced with "*****"
- Cannot be restored in responses
- More secure but loses original data
- Recommended for maximum privacy protection when original values are not needed

## Examples

### Example 1: Basic PII Masking

Mask email addresses and phone numbers in requests, restore in responses:

```yaml
policies:
  - name: PIIMaskingRegex
    version: v0.1.0
    enabled: true
    params:
      request:
        piiEntities:
          - piiEntity: "EMAIL"
            piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
          - piiEntity: "PHONE"
            piiRegex: "\\+?[1-9]\\d{1,14}"
        jsonPath: "$.messages[0].content"
        redactPII: false
      response:
        redactPII: false
```

### Example 2: PII Redaction

Permanently redact credit card numbers and SSNs:

```yaml
policies:
  - name: PIIMaskingRegex
    version: v0.1.0
    enabled: true
    params:
      request:
        piiEntities:
          - piiEntity: "CREDIT_CARD"
            piiRegex: "\\b\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}[\\s-]?\\d{4}\\b"
          - piiEntity: "SSN"
            piiRegex: "\\b\\d{3}-\\d{2}-\\d{4}\\b"
        jsonPath: "$.messages[0].content"
        redactPII: true
      response:
        redactPII: true
```

### Example 3: Multiple PII Types

Mask various PII types with different patterns:

```yaml
policies:
  - name: PIIMaskingRegex
    version: v0.1.0
    enabled: true
    params:
      request:
        piiEntities:
          - piiEntity: "EMAIL"
            piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
          - piiEntity: "PHONE"
            piiRegex: "\\+?[1-9]\\d{1,14}"
          - piiEntity: "IP_ADDRESS"
            piiRegex: "\\b(?:\\d{1,3}\\.){3}\\d{1,3}\\b"
          - piiEntity: "DATE_OF_BIRTH"
            piiRegex: "\\b\\d{4}-\\d{2}-\\d{2}\\b"
        jsonPath: "$.messages[0].content"
        redactPII: false
      response:
        redactPII: false
```

### Example 4: Full Payload Processing

Process the entire request body without JSONPath extraction:

```yaml
policies:
  - name: PIIMaskingRegex
    version: v0.1.0
    enabled: true
    params:
      request:
        piiEntities:
          - piiEntity: "EMAIL"
            piiRegex: "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
        redactPII: false
      response:
        redactPII: false
```

## Use Cases

1. **Privacy Protection**: Mask or redact PII before sending data to AI services or external systems.

2. **Compliance**: Meet regulatory requirements (GDPR, CCPA, HIPAA) for PII handling.

3. **Data Minimization**: Reduce exposure of sensitive data in logs, analytics, or third-party integrations.

4. **Secure Processing**: Allow AI processing while protecting user privacy through masking.

5. **Audit Trail**: Maintain masked versions of data for auditing while protecting original values.

## How It Works

### Request Phase (Masking)

1. Extract content using JSONPath (if specified) or use entire payload
2. Apply each PII regex pattern to find matches
3. Replace matches with placeholders (`[ENTITY_TYPE_XXXX]`) or redaction markers (`*****`)
4. Store mapping of placeholders to original values (for masking mode)
5. Forward masked content to upstream service

### Response Phase (Restoration)

1. Extract content from response
2. If `redactPII: false`, replace placeholders with original PII values from request phase
3. If `redactPII: true`, no restoration is performed
4. Return restored or redacted content


#### Sample Payload after intervention from Regex PII Masking with redact=true

```
{
  "messages": [
    {
      "role": "user",
      "content": "Prepare an email with my contact information, email: *****, and website: https://example.com."
    }
  ]
}
```

## Notes

- Regular expressions use Go's regexp package (RE2 syntax).
- PII detection is case-sensitive by default. Use `(?i)` flag for case-insensitive matching.
- When using masking mode, the placeholder-to-original mapping is stored in request metadata and used for response restoration.
- Multiple PII entities can match the same content; each match is processed according to its entity type.
- Placeholder format is `[ENTITY_TYPE_XXXX]` where XXXX is a hexadecimal counter.
- When using JSONPath, if the path does not exist or the extracted value is not a string, processing will skip that field.
- Redaction mode is irreversible; use masking mode if you need to restore PII in responses.
- Complex regex patterns may impact performance; test thoroughly with expected content volumes.
