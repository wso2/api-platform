# API Key Validation Policy v1.0.0

Sample/reference policy implementation demonstrating API key authentication for the Envoy Policy Engine.

## Overview

The API Key Validation policy validates API keys from request headers against a configured list of valid keys. This policy demonstrates:
- Request phase authentication
- Short-circuit execution on invalid credentials
- Metadata storage for downstream policies
- Configurable error messages
- Case-sensitive/insensitive key comparison

## Category

**Authentication** - Validates client identity using API keys

## Parameters

### headerName (string, required)
Name of the header containing the API key.
- Default: `X-API-Key`
- Validation: 1-100 characters, alphanumeric and hyphens only
- Example: `X-API-Key`, `Authorization`

### validKeys (array, required)
List of valid API keys to accept.
- Validation:
  - Array must contain 1-1000 keys
  - Each key must be 16-256 characters
- Example:
  ```yaml
  validKeys:
    - "test-key-abc123def456"
    - "prod-key-xyz789uvw012"
  ```

### caseSensitive (boolean, optional)
Whether API key comparison is case-sensitive.
- Default: `true`
- Example: `true`

### storeMetadata (boolean, optional)
Store API key validation result in metadata for downstream policies.
- Default: `true`
- When enabled, sets:
  - `apikey.validated` = "true"
  - `apikey.header` = header name
  - Adds `X-API-Key-Validated: true` header for upstream

### unauthorizedMessage (string, optional)
Custom error message returned for invalid API key.
- Default: `"Unauthorized: Invalid or missing API key"`
- Validation: Maximum 500 characters
- Example: `"Access denied: Valid API key required"`

## Execution Behavior

### Request Phase
1. Extracts API key from configured header (case-insensitive header lookup)
2. If header missing → Returns 401 Unauthorized (short-circuits)
3. Validates key against `validKeys` list
4. If invalid → Returns 401 Unauthorized (short-circuits)
5. If valid → Stores metadata (if enabled) and continues chain

### Response Phase
Not applicable - this policy only operates on requests

### Short-Circuit Behavior
- **Triggers on**: Missing or invalid API key
- **Action**: Returns 401 Unauthorized immediately
- **Effect**: Remaining request policies are skipped, no upstream request made

## Body Requirements

- **Request body**: Not required (headers-only policy)
- **Response body**: Not required

## Execution Condition Support

This policy supports conditional execution. Example conditions:
```yaml
condition: request.Path.startsWith("/api/")
```

## Configuration Examples

### Example 1: Basic API Key Validation
```yaml
policies:
  - name: APIKeyValidation
    version: v1.0.0
    config:
      headerName: X-API-Key
      validKeys:
        - "dev-key-12345"
        - "test-key-67890"
      caseSensitive: true
      storeMetadata: true
```

### Example 2: Case-Insensitive with Custom Message
```yaml
policies:
  - name: APIKeyValidation
    version: v1.0.0
    config:
      headerName: Authorization
      validKeys:
        - "Bearer secret-token-abc"
      caseSensitive: false
      unauthorizedMessage: "Please provide a valid authorization token"
      storeMetadata: false
```

### Example 3: Conditional Execution (Protected Routes Only)
```yaml
policies:
  - name: APIKeyValidation
    version: v1.0.0
    condition: request.Path.startsWith("/api/protected/")
    config:
      headerName: X-API-Key
      validKeys:
        - "protected-route-key"
      caseSensitive: true
```

## Response Headers

### On Success
- `X-API-Key-Validated: true` (if storeMetadata is enabled)

### On Failure (401 Unauthorized)
- `Content-Type: text/plain`
- `WWW-Authenticate: API-Key realm="<headerName>"`
- `X-Policy-Rejection: APIKeyValidation`
- `X-Policy-Rejection-Reason: Missing API key` or `Invalid API key`

## Metadata Storage

When `storeMetadata` is enabled, the following metadata is stored for downstream policies:
- `apikey.validated`: "true"
- `apikey.header`: Name of the header that was validated

Downstream policies can read this metadata to implement per-API-key logic (e.g., rate limiting).

## Security Considerations

1. **Key Storage**: Valid keys are configured in policy configuration (not stored in policy code)
2. **Key Length**: Minimum 16 characters enforced to prevent weak keys
3. **Case Sensitivity**: Default is case-sensitive for security
4. **Header Removal**: Consider adding a policy to remove API key header before upstream to prevent leakage
5. **Logging**: Invalid attempts are logged (but key values are not logged)

## Performance

- **Latency**: < 1ms (headers-only, in-memory validation)
- **Body Buffering**: None required
- **Scaling**: Supports up to 1000 valid keys per configuration

## Testing

### Valid Request
```bash
curl -H "X-API-Key: test-key-abc123def456" http://localhost:8080/api/resource
# Expected: 200 OK (if upstream returns 200)
```

### Invalid Request (Wrong Key)
```bash
curl -H "X-API-Key: wrong-key" http://localhost:8080/api/resource
# Expected: 401 Unauthorized
# Body: "Unauthorized: Invalid or missing API key"
```

### Invalid Request (Missing Key)
```bash
curl http://localhost:8080/api/resource
# Expected: 401 Unauthorized
# Body: "Unauthorized: Invalid or missing API key"
```

## Compilation

This policy is a **sample/reference implementation** and is NOT bundled with the Policy Engine runtime. To include it in your custom binary:

```bash
docker run --rm \
  -v $(pwd)/policies:/policies \
  -v $(pwd)/output:/output \
  policy-engine-builder:latest
```

## Use Cases

1. **API Gateway**: Protect backend services with API key authentication
2. **Microservices**: Validate inter-service communication keys
3. **Partner Integration**: Authenticate external partner requests
4. **Development Environments**: Simple authentication for dev/test APIs
5. **Rate Limiting**: Combine with metadata storage to enable per-API-key rate limiting

## Limitations

1. Keys stored in configuration (not ideal for production - consider using a secrets manager)
2. No key rotation support (requires configuration update)
3. No key expiration or validity period
4. No audit logging of key usage patterns
5. Linear search for validation (O(n) complexity)

## Future Enhancements (v2.0.0 ideas)

- External key store integration (Redis, database)
- Key rotation and expiration support
- Enhanced audit logging
- Key metadata (permissions, rate limits)
- Hash-based key storage for security
- Wildcard/prefix matching for keys

## Related Policies

- **JWT Validation**: Token-based authentication alternative
- **Rate Limiting**: Combine with API key metadata for per-key limits
- **Security Headers**: Add security headers to responses

## License

This sample policy is provided for reference and demonstration purposes.
