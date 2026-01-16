# Log Message

## Overview

The Log Message policy provides the capability to log the payload and headers of request/response messages.
This policy operates on both the request flow (logging client requests) and the response flow (logging responses from upstream services before returning to clients).
It is designed for observability and debugging purposes without modifying the actual request/response data.

## Features

- **Configurable Logging**: Control logging of payloads and headers independently
- **Header Filtering**: Exclude sensitive headers from logging using a comma-separated list
- **Security**: Authorization headers are automatically masked with "***"
- **Request ID Tracking**: Tracks request IDs for correlation across request/response flows
- **Structured Logging**: JSON-formatted log output using Go's `slog` package at INFO level for easy parsing and analysis
- **Flow Identification**: Logs are tagged with mediation flow (REQUEST/RESPONSE)
- **Non-intrusive**: Does not modify request/response data, only logs for observability
- **Case-insensitive Header Handling**: Header exclusion works regardless of header name casing

## Configuration

The Log Message policy supports configuration of logging behavior through separate parameters for request and response flows. This allows you to control what information is logged for requests and responses independently, and which headers should be excluded for security reasons.

### User Parameters (API Definition)

These parameters are configured per-API/route by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `logRequestPayload` | boolean | No | `false` | Enables logging of request payloads. When set to `true`, the request bodies will be logged. When set to `false`, request payloads will not be logged. |
| `logRequestHeaders` | boolean | No | `false` | Enables logging of request headers. When set to `true`, the request headers will be logged. When set to `false`, request headers will not be logged. |
| `excludedRequestHeaders` | string | No | `""` | A comma-separated list of header names to exclude from request logging when `logRequestHeaders` is enabled. Example: `"Authorization,X-API-Key"` will exclude these headers from being logged. This parameter is optional and only applies when `logRequestHeaders` is true. Header names are case-insensitive. |
| `logResponsePayload` | boolean | No | `false` | Enables logging of response payloads. When set to `true`, the response bodies will be logged. When set to `false`, response payloads will not be logged. |
| `logResponseHeaders` | boolean | No | `false` | Enables logging of response headers. When set to `true`, the response headers will be logged. When set to `false`, response headers will not be logged. |
| `excludedResponseHeaders` | string | No | `""` | A comma-separated list of header names to exclude from response logging when `logResponseHeaders` is enabled. Example: `"Authorization,X-API-Key"` will exclude these headers from being logged. This parameter is optional and only applies when `logResponseHeaders` is true. Header names are case-insensitive. |

### System Parameters

This policy does not require any system-level configuration parameters.

## API Definition Examples

### Example 1: Default Behavior (No Logging)

When no parameters are specified, no logging is performed (all parameters default to false):

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: no-logging-api-v1.0
spec:
  displayName: No Logging API
  version: v1.0
  context: /no-logging/$version
  upstream:
    main:
      url: http://backend-service:8080
  policies:
    - name: log-message
      version: v0.1.0
      # No params specified - defaults to all false (no logging)
  operations:
    - method: GET
      path: /data
    - method: POST
      path: /submit
```

### Example 2: Basic Log Message Configuration

Log both payloads and headers for all requests and responses:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: user-api-v1.0
spec:
  displayName: User API with Logging
  version: v1.0
  context: /users/$version
  upstream:
    main:
      url: http://user-service:8080
  policies:
    - name: log-message
      version: v0.1.0
      params:
        logRequestPayload: true
        logRequestHeaders: true
        logResponsePayload: true
        logResponseHeaders: true
  operations:
    - method: GET
      path: /profile
    - method: POST
      path: /profile
    - method: PUT
      path: /settings
```

### Example 2: Request-Only Logging

Log only request payloads and headers, skip response logging:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: request-only-api-v1.0
spec:
  displayName: Request Only API
  version: v1.0
  context: /request-only/$version
  upstream:
    main:
      url: http://backend-service:8080
  policies:
    - name: log-message
      version: v0.1.0
      params:
        logRequestPayload: true
        logRequestHeaders: true
        logResponsePayload: false
        logResponseHeaders: false
  operations:
    - method: POST
      path: /sensitive-data
```

### Example 3: Response-Only Logging

Log only response payloads and headers, skip request logging:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: response-only-api-v1.0
spec:
  displayName: Response Only API
  version: v1.0
  context: /response-only/$version
  upstream:
    main:
      url: http://backend-service:8080
  policies:
    - name: log-message
      version: v0.1.0
      params:
        # Request parameters default to false (omitted)
        logResponsePayload: true
        logResponseHeaders: true
  operations:
    - method: GET
      path: /public-data
```

### Example 4: Headers with Different Exclusions

Log headers but exclude different sensitive headers for requests vs responses:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: payment-api-v1.0
spec:
  displayName: Payment API
  version: v1.0
  context: /payments/$version
  upstream:
    main:
      url: http://payment-service:8080
  policies:
    - name: log-message
      version: v0.1.0
      params:
        logRequestPayload: true
        logRequestHeaders: true
        excludedRequestHeaders: "Authorization,X-API-Key,X-Payment-Token"
        logResponsePayload: true
        logResponseHeaders: true
        excludedResponseHeaders: "Set-Cookie,X-Internal-Token"
  operations:
    - method: GET
      path: /transactions
    - method: POST
      path: /charge
```

### Example 5: Selective Logging

Log only request payloads and response headers:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: selective-api-v1.0
spec:
  displayName: Selective API
  version: v1.0
  context: /selective/$version
  upstream:
    main:
      url: http://backend-service:8080
  policies:
    - name: log-message
      version: v0.1.0
      params:
        logRequestPayload: true
        logRequestHeaders: false
        logResponsePayload: false
        logResponseHeaders: true
  operations:
    - method: POST
      path: /analyze
```

### Example 6: Operation-Specific Logging

Apply different logging configurations to different operations:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: mixed-api-v1.0
spec:
  displayName: Mixed API
  version: v1.0
  context: /mixed/$version
  upstream:
    main:
      url: http://backend-service:8080
  operations:
    - method: GET
      path: /public-data
      policies:
        - name: log-message
          version: v0.1.0
          params:
            logRequestPayload: false
            logRequestHeaders: true
            excludedRequestHeaders: "Authorization"
            logResponsePayload: true
            logResponseHeaders: false
    - method: POST
      path: /sensitive-operation
      policies:
        - name: log-message
          version: v0.1.0
          params:
            logRequestPayload: true
            logRequestHeaders: false
            logResponsePayload: false
            logResponseHeaders: false
    - method: PUT
      path: /debug-endpoint
      policies:
        - name: log-message
          version: v0.1.0
          params:
            logRequestPayload: true
            logRequestHeaders: true
            excludedRequestHeaders: "Authorization,X-Debug-Token"
            logResponsePayload: true
            logResponseHeaders: true
            excludedResponseHeaders: "X-Internal-Key,Set-Cookie"
```

## Log Format and Examples

The policy outputs structured JSON logs with the following format:

```json
{
  "mediation-flow": "REQUEST|RESPONSE|FAULT",
  "request-id": "unique-request-identifier",
  "http-method": "GET|POST|PUT|DELETE|etc",
  "resource-path": "/api/path/to/resource",
  "payload": "request/response body content",
  "headers": {
    "header-name": "header-value",
    "multi-value-header": ["value1", "value2"]
  }
}
```

### Log Fields Description

- **mediation-flow**: Identifies the flow phase (`REQUEST` or `RESPONSE`)
- **request-id**: Value from `x-request-id` header for request correlation (shows `<request-id-unavailable>` if not present)
- **http-method**: HTTP method (GET, POST, PUT, DELETE, etc.)
- **resource-path**: The API resource path being accessed
- **payload**: Request/response body content (only included if `logPayload: true`)
- **headers**: HTTP headers map (only included if `logHeaders: true`)

### Request Log Example

```json
{
  "mediation-flow": "REQUEST",
  "request-id": "req-12345-abcde",
  "http-method": "POST",
  "resource-path": "/users/v1.0/profile",
  "payload": "{\"name\":\"John Doe\",\"email\":\"john@example.com\"}",
  "headers": {
    "content-type": "application/json",
    "user-agent": "MyApp/1.0",
    "authorization": "***",
    "x-custom-header": "custom-value"
  }
}
```

### Response Log Example

```json
{
  "mediation-flow": "RESPONSE",
  "request-id": "req-12345-abcde", 
  "http-method": "POST",
  "resource-path": "/users/v1.0/profile",
  "payload": "{\"status\":\"success\",\"userId\":123}",
  "headers": {
    "content-type": "application/json",
    "cache-control": "no-cache",
    "x-response-time": "45ms"
  }
}
```

### Payload-Only Log Example

```json
{
  "mediation-flow": "REQUEST",
  "request-id": "req-67890-fghij",
  "http-method": "GET",
  "resource-path": "/secure/v1.0/data",
  "payload": "{\"query\":\"user-data\",\"filters\":[\"active\"]}"
}
```

## Policy Behavior

### Logging Control

The policy behavior is controlled by separate boolean parameters for request and response flows:

- **Request Flow Control**:
  - `logRequestPayload: true`: Includes request body content in logs
  - `logRequestPayload: false`: Excludes request payload from logs
  - `logRequestHeaders: true`: Includes request headers in logs (with security filtering)
  - `logRequestHeaders: false`: Excludes request headers from logs

- **Response Flow Control**:
  - `logResponsePayload: true`: Includes response body content in logs
  - `logResponsePayload: false`: Excludes response payload from logs
  - `logResponseHeaders: true`: Includes response headers in logs (with security filtering)
  - `logResponseHeaders: false`: Excludes response headers from logs

- **Independent Control**: Request and response logging can be configured independently
- **Skipping Flows**: If both payload and headers are disabled for a flow, that entire flow is skipped

### Security Features

#### Automatic Authorization Masking

- **Authorization headers** are automatically masked with `"***"` regardless of exclusion settings
- This prevents accidental logging of bearer tokens, basic auth credentials, and API keys in Authorization headers
- Applies to headers with name `authorization` (case-insensitive)

#### Configurable Header Exclusion

- Use `excludedRequestHeaders` parameter to exclude sensitive headers from request logging
- Use `excludedResponseHeaders` parameter to exclude sensitive headers from response logging
- Header names are case-insensitive (`"authorization"`, `"Authorization"`, `"AUTHORIZATION"` all work)
- Multiple headers can be excluded using comma separation: `"Authorization,X-API-Key,Cookie"`
- Excluded headers are completely omitted from the log output
- Different exclusion lists can be configured for requests vs responses

#### Request ID Correlation

- Extracts `x-request-id` header value for request correlation
- Same request ID appears in both REQUEST and RESPONSE logs
- Shows `<request-id-unavailable>` if the header is not present
- Enables tracing requests across the entire request/response lifecycle

### Content Processing

- **Non-intrusive**: The policy does not modify request or response data
- **Memory buffering**: Request and response bodies are buffered for logging
- **Header processing**: All headers are processed for filtering and security
- **Flow identification**: Automatically identifies and tags REQUEST vs RESPONSE flows

### Empty or Missing Content

- **Empty Request Body**: Log entry created without payload field
- **Empty Response Body**: Log entry created without payload field
- **Missing Headers**: Log entry created without headers field
- **No Request ID**: Uses `<request-id-unavailable>` as fallback value

## Common Use Cases

1. **API Debugging**: Log full request/response details for troubleshooting API issues
2. **Security Monitoring**: Monitor API usage patterns while protecting sensitive headers
3. **Performance Analysis**: Track request/response sizes and patterns
4. **Compliance Logging**: Maintain audit trails for regulatory compliance
5. **Integration Testing**: Verify request/response formats during development
6. **Error Investigation**: Capture request details when errors occur
7. **Request Correlation**: Track requests across microservices using request IDs

## Best Practices

1. **Sensitive Data Protection**: Always exclude authentication and sensitive headers using `excludedHeaders`
2. **Performance Consideration**: Be mindful of logging large payloads in high-traffic scenarios
3. **Log Storage**: Ensure adequate log storage capacity when enabling payload logging
4. **Request ID Usage**: Include `x-request-id` headers in client requests for better traceability
5. **Selective Logging**: Use operation-specific policies to log different levels of detail for different endpoints
6. **Header Filtering**: Regularly review and update excluded headers list as new sensitive headers are introduced
7. **Log Retention**: Implement appropriate log retention policies for compliance and storage management

## Security Considerations

1. **Authorization Masking**: Authorization headers are automatically masked to prevent token exposure
2. **Sensitive Headers**: Use `excludedHeaders` to exclude headers containing API keys, tokens, or personal data
3. **Payload Content**: Be aware that payload logging may capture sensitive business data
4. **Log Access Control**: Restrict access to logs containing request/response data to authorized personnel only
5. **Log Transmission**: Ensure secure transmission and storage of logs containing sensitive information
6. **Compliance**: Consider data privacy regulations (GDPR, CCPA) when logging request/response data

## Performance Considerations

- **Memory Usage**: Request and response bodies are buffered in memory during processing
- **Processing Overhead**: JSON marshaling and logging add CPU overhead to each request
- **Log Volume**: Payload logging can generate significant log volume in high-traffic scenarios  
- **Storage Impact**: Large payloads increase log storage requirements
- **I/O Operations**: Frequent logging may impact I/O performance

## Limitations

1. **Memory Buffering**: Large payloads require significant memory for buffering during logging
2. **No Partial Logging**: Cannot log only specific parts of payloads (logs entire content)
3. **Binary Content**: Binary payloads may not log readably (will be logged as raw bytes)
4. **Real-time Constraints**: Logging overhead may not be suitable for ultra-low-latency requirements
5. **Log Format**: Output format is fixed JSON structure and cannot be customized

## Troubleshooting

### Common Issues

1. **No Logs Generated**: If no parameters are specified, all logging is disabled by default. Set the appropriate parameters to `true` for the flows you want to log (`logRequestPayload`, `logRequestHeaders`, `logResponsePayload`, `logResponseHeaders`)
2. **Missing Logs**: Verify the appropriate parameters are set to `true` for the flows you want to log
3. **Sensitive Data Exposure**: Ensure `excludedRequestHeaders` and `excludedResponseHeaders` include all sensitive header names
4. **Performance Degradation**: Consider disabling payload logging for large file uploads/downloads
5. **Log Volume**: Monitor disk space and log rotation when enabling comprehensive logging
6. **Request Correlation**: Include `x-request-id` header in client requests for proper correlation

### Configuration Validation

- **Optional Parameters**: All logging parameters are optional and default to `false` (no logging by default)
- **Parameter Types**: Ensure boolean values are used for all logging parameters when specified
- **Header Names**: Verify excluded header names are spelled correctly (case-insensitive matching)
- **Comma Separation**: Ensure proper comma separation in excluded headers parameters without extra spaces

## Related Policies

- **Request/Response Transformation**: Use alongside transformation policies for complete request/response visibility
- **Authentication Policies**: Combine with authentication policies while excluding auth headers from logging
- **Rate Limiting**: Log rate-limited requests for analysis and monitoring
- **Error Handling**: Capture request details when custom error responses are generated
