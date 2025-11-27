# Respond Policy

The **Respond** policy terminates request processing and returns an immediate response to the client without forwarding the request to the upstream backend.

## Policy Type

- **Phase**: Request
- **Execution**: Short-circuit (stops policy chain execution)

## Use Cases

- Mock API responses for testing
- Return error responses based on validation rules
- Implement API rate limit exceeded responses
- Return cached responses
- Block requests with custom error messages

## Configuration Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `statusCode` | integer | No | 200 | HTTP status code (100-599) |
| `body` | string or bytes | No | empty | Response body content |
| `headers` | array | No | empty | Array of response headers |

### Headers Array Format

Each header in the `headers` array must be an object with:
- `name` (string, required): Header name
- `value` (string, required): Header value

## Examples

### Basic Response

Return a simple JSON response:

```yaml
- name: Respond
  version: v1.0.0
  enabled: true
  parameters:
    statusCode: 200
    body: '{"message": "Hello from policy engine"}'
    headers:
      - name: content-type
        value: application/json
```

### Error Response

Return a 403 Forbidden error:

```yaml
- name: Respond
  version: v1.0.0
  enabled: true
  parameters:
    statusCode: 403
    body: '{"error": "Access denied"}'
    headers:
      - name: content-type
        value: application/json
      - name: x-error-code
        value: ACCESS_DENIED
```

### Rate Limit Response

Return a 429 Too Many Requests response:

```yaml
- name: Respond
  version: v1.0.0
  enabled: true
  parameters:
    statusCode: 429
    body: 'Rate limit exceeded. Please try again later.'
    headers:
      - name: content-type
        value: text/plain
      - name: retry-after
        value: "60"
```

### Mock API Response

Mock an API endpoint response:

```yaml
- name: Respond
  version: v1.0.0
  enabled: true
  parameters:
    statusCode: 200
    body: |
      {
        "id": 123,
        "name": "Sample Product",
        "price": 29.99,
        "inStock": true
      }
    headers:
      - name: content-type
        value: application/json
      - name: x-mock-response
        value: "true"
```

## Behavior

1. **Request Body**: Not required - the policy terminates processing before upstream communication
2. **Chain Execution**: Stops all subsequent policies in the chain
3. **Upstream Request**: Never sent - response returned immediately
4. **Response Phase**: Not executed - request phase terminates early

## Integration Example

Combine with conditional execution to selectively return immediate responses:

```yaml
- route_key: /api/v1/products
  policies:
    - name: APIKeyValidation
      version: v1.0.0
      enabled: true
      parameters:
        headerName: x-api-key

    - name: Respond
      version: v1.0.0
      enabled: true
      condition: 'request.headers["x-mock"] == "true"'
      parameters:
        statusCode: 200
        body: '{"mock": true, "data": []}'
        headers:
          - name: content-type
            value: application/json
```

In this example, the Respond policy only executes when the `x-mock` header is set to `true`, allowing the same route to serve both real and mocked responses.

## Notes

- Response headers are normalized to lowercase
- Empty body is allowed (useful for 204 No Content responses)
- Status code defaults to 200 if not specified
- This policy should typically be placed later in the policy chain, after authentication/authorization policies
