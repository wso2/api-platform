# UppercaseBody Policy

The **UppercaseBody** policy transforms request body text to uppercase letters before forwarding to the upstream service.

## Policy Type

- **Phase**: Request
- **Execution**: Continues to next policy (does not short-circuit)
- **Body Processing**: Requires request body

## Use Cases

- Text normalization for case-insensitive processing
- Testing and debugging body transformations
- Standardizing text input for backend systems
- Converting message payloads to uppercase format

## Configuration Parameters

This policy has **no configuration parameters**. It automatically transforms the request body to uppercase.

## Examples

### Basic Text Transformation

Transform plain text request body:

```yaml
- name: UppercaseBody
  version: v1.0.0
  enabled: true
```

**Input:**
```
hello world
```

**Output:**
```
HELLO WORLD
```

### JSON Payload Transformation

The policy transforms all text in the body, including JSON field names and values:

```yaml
- name: UppercaseBody
  version: v1.0.0
  enabled: true
```

**Input:**
```json
{"message": "hello world", "status": "active"}
```

**Output:**
```json
{"MESSAGE": "HELLO WORLD", "STATUS": "ACTIVE"}
```

**Note:** This transforms the entire JSON structure including keys, which may break JSON parsing. Use carefully with JSON payloads.

### XML Payload Transformation

```yaml
- name: UppercaseBody
  version: v1.0.0
  enabled: true
```

**Input:**
```xml
<message>hello world</message>
```

**Output:**
```xml
<MESSAGE>HELLO WORLD</MESSAGE>
```

## Behavior

1. **No Body Present**: If no request body is present, the policy passes through without modification
2. **Empty Body**: Empty bodies are preserved (not modified)
3. **Body Size**: Works with bodies of any size (limited by policy engine buffer settings)
4. **Character Encoding**: Assumes UTF-8 encoding; non-ASCII characters are converted based on Go's `strings.ToUpper()` behavior
5. **Chain Execution**: Continues to next policy in the chain after transformation

## Integration Example

Combine with other policies for complete request processing:

```yaml
- route_key: /api/v1/messages
  policies:
    - name: APIKeyValidation
      version: v1.0.0
      enabled: true
      parameters:
        headerName: x-api-key

    - name: UppercaseBody
      version: v1.0.0
      enabled: true

    - name: SetHeader
      version: v1.0.0
      enabled: true
      parameters:
        action: SET
        headerName: x-body-transformed
        headerValue: "true"
```

In this example:
1. API key is validated first
2. Request body is transformed to uppercase
3. A header is added to indicate transformation occurred
4. Request is forwarded to upstream with uppercase body

## Performance Considerations

- The policy reads the entire request body into memory for transformation
- Large request bodies will increase memory usage
- Transformation is performed using Go's standard `strings.ToUpper()` function
- For high-throughput APIs with large bodies, consider the memory impact

## Important Notes

- **JSON/XML Compatibility**: Transforming structured data formats (JSON, XML) to uppercase will affect field names and may break parsing. Use this policy primarily for plain text payloads or when the backend expects uppercase text.
- **Case-Sensitive Backends**: Only use this policy when the upstream service can handle uppercase text appropriately.
- **Content-Type**: The policy does not modify the `Content-Type` header; ensure it remains appropriate for the transformed content.
