# ModifyHeaders Policy

The **ModifyHeaders** policy provides comprehensive header manipulation capabilities for both request and response flows. It supports three operations: SET (replace), APPEND (add), and DELETE (remove) headers.

## Policy Type

- **Phase**: Request and Response
- **Execution**: Continues to next policy (does not short-circuit)
- **Body Processing**: Not required (header-only policy)

## Use Cases

- Add custom headers to requests and responses
- Remove sensitive headers before forwarding requests
- Append tracking or correlation headers
- Standardize headers across multiple backends
- Add security headers to responses
- Transform legacy headers to modern equivalents
- Remove server identification headers

## Configuration Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `requestHeaders` | array | Conditional* | Header modifications to apply during request phase |
| `responseHeaders` | array | Conditional* | Header modifications to apply during response phase |

**At least one of `requestHeaders` or `responseHeaders` must be specified.*

### Header Modification Object

Each header modification object in the arrays contains:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `action` | string | Yes | Operation to perform: `SET`, `APPEND`, or `DELETE` |
| `name` | string | Yes | Header name (case-insensitive) |
| `value` | string | Conditional** | Header value (required for SET and APPEND) |

**Value is required for SET and APPEND actions, ignored for DELETE.*

## Actions

### SET
Replaces the header value. If the header exists, it's overwritten. If it doesn't exist, it's created.

### APPEND
Adds a value to the header. If the header exists, the value is appended. If it doesn't exist, it's created.

### DELETE
Removes the header completely.

## Examples

### Basic Request Header Modification

Add a custom header to all requests:

```yaml
- name: ModifyHeaders
  version: v1.0.0
  enabled: true
  parameters:
    requestHeaders:
      - action: SET
        name: x-api-version
        value: v2.0
      - action: SET
        name: x-client-id
        value: policy-engine
```

### Basic Response Header Modification

Add security headers to responses:

```yaml
- name: ModifyHeaders
  version: v1.0.0
  enabled: true
  parameters:
    responseHeaders:
      - action: SET
        name: x-frame-options
        value: DENY
      - action: SET
        name: x-content-type-options
        value: nosniff
      - action: SET
        name: strict-transport-security
        value: max-age=31536000; includeSubDomains
```

### Remove Sensitive Headers

Remove sensitive headers from requests before forwarding to upstream:

```yaml
- name: ModifyHeaders
  version: v1.0.0
  enabled: true
  parameters:
    requestHeaders:
      - action: DELETE
        name: x-internal-token
      - action: DELETE
        name: x-debug-mode
```

### Hide Server Information

Remove server identification from responses:

```yaml
- name: ModifyHeaders
  version: v1.0.0
  enabled: true
  parameters:
    responseHeaders:
      - action: DELETE
        name: server
      - action: DELETE
        name: x-powered-by
      - action: SET
        name: server
        value: API Gateway
```

### Append Forwarding Headers

Track request path through multiple proxies:

```yaml
- name: ModifyHeaders
  version: v1.0.0
  enabled: true
  parameters:
    requestHeaders:
      - action: APPEND
        name: x-forwarded-for
        value: 192.168.1.1
      - action: APPEND
        name: x-forwarded-proto
        value: https
```

### Both Request and Response Modifications

Comprehensive header manipulation in a single policy:

```yaml
- name: ModifyHeaders
  version: v1.0.0
  enabled: true
  parameters:
    requestHeaders:
      - action: SET
        name: x-request-id
        value: req-12345
      - action: DELETE
        name: x-legacy-auth
      - action: SET
        name: authorization
        value: Bearer new-token
    responseHeaders:
      - action: SET
        name: x-response-time
        value: 42ms
      - action: SET
        name: cache-control
        value: no-store, must-revalidate
      - action: DELETE
        name: x-internal-info
```

### CORS Headers

Add CORS headers to responses:

```yaml
- name: ModifyHeaders
  version: v1.0.0
  enabled: true
  parameters:
    responseHeaders:
      - action: SET
        name: access-control-allow-origin
        value: "*"
      - action: SET
        name: access-control-allow-methods
        value: GET, POST, PUT, DELETE, OPTIONS
      - action: SET
        name: access-control-allow-headers
        value: Content-Type, Authorization
      - action: SET
        name: access-control-max-age
        value: "3600"
```

### Content Transformation Headers

Update content-related headers when transforming the body:

```yaml
- name: ModifyHeaders
  version: v1.0.0
  enabled: true
  parameters:
    requestHeaders:
      - action: SET
        name: content-type
        value: application/json
      - action: SET
        name: accept
        value: application/json
    responseHeaders:
      - action: SET
        name: content-type
        value: application/json; charset=utf-8
```

## Behavior

1. **Header Names**: All header names are normalized to lowercase internally
2. **Multiple Values**: APPEND action adds to existing header values
3. **Order of Operations**: Within the same array, operations are applied in order
4. **Request Phase**: `requestHeaders` modifications are applied before forwarding to upstream
5. **Response Phase**: `responseHeaders` modifications are applied before returning to client
6. **Pass-through**: If only `requestHeaders` is configured, the policy still runs in response phase but makes no modifications (and vice versa)

## Integration Example

Combine with other policies for complete request/response processing:

```yaml
- route_key: /api/v1/secure
  policies:
    - name: APIKeyValidation
      version: v1.0.0
      enabled: true
      parameters:
        headerName: x-api-key

    - name: ModifyHeaders
      version: v1.0.0
      enabled: true
      parameters:
        requestHeaders:
          - action: SET
            name: x-validated
            value: "true"
          - action: DELETE
            name: x-api-key  # Remove after validation
        responseHeaders:
          - action: SET
            name: x-frame-options
            value: DENY
          - action: SET
            name: x-content-type-options
            value: nosniff

    - name: UppercaseBody
      version: v1.0.0
      enabled: true
```

In this example:
1. API key is validated
2. Request headers are modified (add validation marker, remove API key)
3. Response security headers are added
4. Body is transformed to uppercase

## Performance Considerations

- Header operations are in-memory and very fast
- No performance impact for large request/response bodies (header-only policy)
- Minimal overhead for header lookups and modifications
- Safe for high-throughput APIs

## Best Practices

1. **Security Headers**: Use response headers to add security headers consistently
2. **Header Cleanup**: Remove internal/sensitive headers before forwarding
3. **Standardization**: Use SET to ensure consistent header values across routes
4. **Correlation**: Add correlation IDs for request tracking
5. **Avoid Duplication**: Use this policy instead of multiple separate header policies for better performance

## Comparison with SetHeader Policy

| Feature | SetHeader | ModifyHeaders |
|---------|-----------|---------------|
| Request Phase | Yes | Yes |
| Response Phase | Yes | Yes |
| SET Action | Yes | Yes |
| APPEND Action | Yes | Yes |
| DELETE Action | Yes | Yes |
| Multiple Headers | One at a time | Multiple in one config |
| Separate Request/Response Config | No | Yes |

**When to use ModifyHeaders:**
- Need to modify multiple headers in one policy instance
- Want separate configurations for request vs response
- Prefer clearer separation of request and response operations

**When to use SetHeader:**
- Simple single-header modifications
- Legacy configurations already using SetHeader
