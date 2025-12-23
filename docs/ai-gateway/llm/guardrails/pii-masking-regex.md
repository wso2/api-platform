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

Deploy an LLM provider that masks email addresses and phone numbers in requests and restores them in responses:

```bash
curl -X POST http://localhost:9090/llm-providers \
  -H "Content-Type: application/yaml" \
  -H "Authorization: Basic YWRtaW46YWRtaW4=" \
  --data-binary @- <<'EOF'
version: gateway.api-platform.wso2.com/v1alpha1
kind: LlmProvider
metadata:
  name: pii-masking-provider
spec:
  displayName: PII Masking Provider
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
    - name: pii-masking-regex
      version: v0.1.0
      paths:
        - path: /chat/completions
          methods: [POST]
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
EOF
```

**Test the guardrail:**

**Note**: Ensure that "openai" is mapped to the appropriate IP address (e.g., 127.0.0.1) in your `/etc/hosts` file. or remove the vhost from the llm provider configuration and use localhost to invoke.

```bash
# Request with PII (should be masked)
curl -X POST http://openai:8080/chat/completions \
  -H "Content-Type: application/json" \
  -H "Host: openai" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {
        "role": "user",
        "content": "Contact me at john.doe@example.com or call +1234567890"
      }
    ]
  }'
```

### Additional Configuration Options

You can customize the guardrail behavior by modifying the `policies` section:

- **PII Redaction**: Set `redactPII: true` to permanently replace PII with "*****" (cannot be restored). Set `redactPII: false` to use masking mode with placeholders that can be restored in responses.

- **Multiple PII Types**: Configure multiple `piiEntities` in the array to detect and mask/redact various PII types (e.g., EMAIL, PHONE, CREDIT_CARD, SSN, IP_ADDRESS, DATE_OF_BIRTH).

- **Full Payload Processing**: Omit the `jsonPath` parameter to process the entire request body without JSONPath extraction.

- **Field-Specific Processing**: Use `jsonPath` to extract and process PII from specific fields within JSON payloads (e.g., `"$.messages[0].content"` for message content).

- **Response Restoration**: When using masking mode (`redactPII: false`), set `response.redactPII: false` to restore masked PII values in responses. Set `response.redactPII: true` if PII was permanently redacted in the request phase.

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
