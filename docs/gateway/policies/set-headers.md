# Set Headers

## Overview

The Set Headers policy dynamically sets HTTP headers on incoming requests before they are forwarded to upstream services, and/or sets headers on outgoing responses before they are returned to clients. **Headers are set/replaced instead of appended**, which means existing headers with the same name will be overwritten with the new value.

## Features

- Sets custom headers on requests before forwarding to upstream services
- Sets custom headers on responses before returning to clients
- Supports both request and response phases independently or simultaneously
- **Overwrites headers instead of appending**: Existing headers with the same name are replaced
- Proper header name normalization (lowercase conversion for HTTP/2 compatibility)
- Static value assignment with support for special characters and complex values
- Works with any HTTP method and request type
- Last-value-wins behavior for duplicate header names in configuration
- Comprehensive validation of header configurations

## Configuration

The Set Headers policy can be configured for request phase, response phase, or both.
This policy does not require system-level configuration and operates entirely based on the configured header arrays.
At least one of `requestHeaders` or `responseHeaders` must be specified in the policy configuration. The policy will fail validation if both arrays are empty or omitted.

### User Parameters (API Definition)

These parameters are configured per-API/route by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `requestHeaders` | array | No | - | Array of header objects to set on requests before forwarding to upstream. Each object must contain `name` and `value` fields. At least one of `requestHeaders` or `responseHeaders` must be specified. |
| `responseHeaders` | array | No | - | Array of header objects to set on responses before returning to clients. Each object must contain `name` and `value` fields. At least one of `requestHeaders` or `responseHeaders` must be specified. |

### Header Object Structure

Each header object in the `requestHeaders` and `responseHeaders` arrays must contain:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | The name of the HTTP header to set. Header names are automatically normalized to lowercase for consistency. Cannot be empty or whitespace-only. |
| `value` | string | Yes | The value of the HTTP header to set. Can be static text, empty string, or contain special characters and complex values. |

## API Definition Examples

### Example 1: Setting Request Headers for Authentication

Set authentication headers on all requests sent to upstream:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: weather-api-v1.0
spec:
  displayName: Weather-API
  version: v1.0
  context: /weather/$version
  upstream:
    main:
      url: http://sample-backend:5000/api/v2
  policies:
    - name: set-headers
      version: v0.1.0
      params:
        requestHeaders:
          - name: X-API-Key
            value: "12345-abcde-67890-fghij"
          - name: X-Client-Version
            value: "1.2.3"
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 2: Setting Response Headers for Security

Set security headers on all responses returned to clients:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: weather-api-v1.0
spec:
  displayName: Weather-API
  version: v1.0
  context: /weather/$version
  upstream:
    main:
      url: http://sample-backend:5000/api/v2
  policies:
    - name: set-headers
      version: v0.1.0
      params:
        responseHeaders:
          - name: X-Content-Type-Options
            value: "nosniff"
          - name: X-Frame-Options
            value: "DENY"
          - name: X-XSS-Protection
            value: "1; mode=block"
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 3: Setting Headers on Both Request and Response

Set headers on both requests (for upstream) and responses (for clients):

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: weather-api-v1.0
spec:
  displayName: Weather-API
  version: v1.0
  context: /weather/$version
  upstream:
    main:
      url: http://sample-backend:5000/api/v2
  policies:
    - name: set-headers
      version: v0.1.0
      params:
        requestHeaders:
          - name: X-Source
            value: "api-gateway"
          - name: X-Request-ID
            value: "req-12345"
        responseHeaders:
          - name: X-Cache-Status
            value: "HIT"
          - name: X-Server-Version
            value: "2.1.0"
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 4: Route-Specific Headers

Apply different headers to different routes:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: weather-api-v1.0
spec:
  displayName: Weather-API
  version: v1.0
  context: /weather/$version
  upstream:
    main:
      url: http://sample-backend:5000/api/v2
  operations:
    - method: GET
      path: /{country_code}/{city}
      policies:
        - name: set-headers
          version: v0.1.0
          params:
            requestHeaders:
              - name: X-Operation-Type
                value: "weather-query"
            responseHeaders:
              - name: X-Data-Source
                value: "weather-service"
    - method: GET
      path: /alerts/active
      policies:
        - name: set-headers
          version: v0.1.0
          params:
            requestHeaders:
              - name: X-Operation-Type
                value: "alert-query"
            responseHeaders:
              - name: X-Real-Time
                value: "true"
    - method: POST
      path: /alerts/active
      policies:
        - name: set-headers
          version: v0.1.0
          params:
            requestHeaders:
              - name: X-Operation-Type
                value: "alert-create"
            responseHeaders:
              - name: X-Processing-Mode
                value: "sync"
```

### Example 5: Overwriting Existing Headers (Set Behavior)

Demonstrate header overwriting behavior - existing headers with same name are replaced:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: weather-api-v1.0
spec:
  displayName: Weather-API
  version: v1.0
  context: /weather/$version
  upstream:
    main:
      url: http://sample-backend:5000/api/v2
  policies:
    - name: set-headers
      version: v0.1.0
      params:
        responseHeaders:
          - name: Cache-Control
            value: "public, max-age=3600"  # This will overwrite any existing Cache-Control header
          - name: Server
            value: "API-Gateway/2.1.0"    # This will overwrite the original Server header
          - name: Content-Type
            value: "application/json; charset=utf-8"  # This will overwrite existing Content-Type
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

## Request/Response Transformation Examples

### Request Headers Setting (Example 1)

**Original client request:**
```
GET /weather/v1.0/US/NewYork HTTP/1.1
Host: api-gateway.company.com
Accept: application/json
User-Agent: WeatherApp/1.0
```

**Resulting upstream request:**
```
GET /api/v2/US/NewYork HTTP/1.1
Host: sample-backend:5000
Accept: application/json
User-Agent: WeatherApp/1.0
x-api-key: 12345-abcde-67890-fghij
x-client-version: 1.2.3
```

### Response Headers Setting (Example 2)

**Original upstream response:**
```
HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 256

{"temperature": 22, "humidity": 65}
```

**Resulting client response:**
```
HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 256
x-content-type-options: nosniff
x-frame-options: DENY
x-xss-protection: 1; mode=block

{"temperature": 22, "humidity": 65}
```

### Header Overwriting Behavior (Example 5)

**Original upstream response:**
```
HTTP/1.1 200 OK
Content-Type: text/plain
Server: Apache/2.4.41
Cache-Control: no-cache
Content-Length: 256

{"temperature": 22, "humidity": 65}
```

**Resulting client response (headers overwritten):**
```
HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
Server: API-Gateway/2.1.0
Cache-Control: public, max-age=3600
Content-Length: 256

{"temperature": 22, "humidity": 65}
```

## Policy Behavior

### Header Set/Overwrite Behavior

The policy uses **set/overwrite semantics** instead of append semantics:

- **Existing Headers**: Headers set by the policy will overwrite any existing headers with the same name
- **Multiple Configuration**: If the same header name is configured multiple times in the policy, the last value wins
- **Replacement**: Existing headers from the original request/response are replaced, not preserved
- **Single Value**: Each header name will have only one value after the policy executes
- **Common Use Case**: Useful for enforcing specific header values or standardizing headers

### Header Name Normalization

The policy automatically normalizes header names for consistency:

- **Case Conversion**: All header names are converted to lowercase (e.g., "X-API-Key" becomes "x-api-key")
- **Whitespace Trimming**: Leading and trailing whitespace is removed from header names
- **HTTP/2 Compatibility**: Lowercase headers ensure compatibility with HTTP/2 protocol requirements
- **Existing Headers**: Normalization applies to headers being set by the policy

### Header Value Handling

Header values are preserved exactly as configured:

- **Special Characters**: All special characters in values are preserved without encoding
- **Whitespace**: Leading and trailing whitespace in values is preserved
- **Empty Values**: Empty string values are supported and valid
- **Unicode**: Unicode characters are supported and preserved
- **Case Sensitivity**: Header values are case-sensitive and preserved as-is

### Error Handling

The policy includes robust error handling and validation:

1. **Missing Configuration**: If neither `requestHeaders` nor `responseHeaders` is specified, validation fails at configuration time
2. **Invalid Arrays**: If header arrays are not properly formatted, validation fails at configuration time
3. **Missing Fields**: If header objects are missing `name` or `value` fields, validation fails at configuration time
4. **Empty Names**: If header names are empty or whitespace-only, validation fails at configuration time
5. **Runtime Errors**: Policy execution errors do not affect request processing (graceful degradation)

### Performance Considerations

- **Minimal Overhead**: Lightweight header manipulation with minimal memory allocation
- **Header Processing**: Efficient header setting using Go's standard HTTP header handling
- **No Network Calls**: All processing is done locally without external dependencies
- **Dual Phase**: Separate request and response processing for optimal performance

## Common Use Cases

1. **Authentication Headers**: Set API keys, tokens, or client identifiers for upstream services.

2. **Security Headers**: Set security-related headers like CORS, CSP, or XSS protection on responses.

3. **Header Standardization**: Enforce specific header values by overwriting existing ones.

4. **API Version Control**: Set API version or client version information for upstream processing.

5. **Content Type Override**: Override upstream Content-Type headers with standardized values.

6. **Server Identity**: Set or replace Server headers to hide upstream server information.

7. **Cache Control**: Set specific cache control headers, overriding upstream settings.

8. **Compliance Headers**: Set mandatory headers for regulatory or compliance requirements.

## Best Practices

1. **Header Naming**: Use clear, descriptive header names with appropriate prefixes (e.g., "X-" for custom headers).

2. **Value Security**: Be cautious about setting sensitive values like API keys - ensure they're properly managed and secured.

3. **Overwrite Awareness**: Remember that this policy will overwrite existing headers, not append to them.

4. **Normalization Consideration**: Remember that header names will be normalized to lowercase.

5. **Multiple Policies**: Consider the order when using multiple header manipulation policies together.

6. **Documentation**: Document set headers so client developers and upstream service developers are aware of them.

7. **Validation**: Always validate header configurations during API development and testing.

8. **Performance**: Avoid setting unnecessary headers that increase request/response size.

## Security Considerations

1. **Sensitive Data**: Be careful about setting sensitive information in headers as they may be logged or cached.

2. **Header Injection**: Ensure header values come from trusted sources to prevent header injection attacks.

3. **Log Sanitization**: Configure logging to sanitize or exclude sensitive headers from logs.

4. **Access Control**: Ensure only authorized users can configure policies that set headers.

5. **Upstream Trust**: Only set headers that upstream services are configured to handle securely.

6. **Client Exposure**: Be aware that response headers are visible to clients and may expose internal information.

7. **Header Size Limits**: Be mindful of HTTP header size limits in proxies, load balancers, and servers.

8. **Cross-Origin**: When setting CORS headers, ensure they align with your security policies and don't expose sensitive resources.
