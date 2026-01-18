# Remove Headers

## Overview

The Remove Headers policy dynamically removes HTTP headers from incoming requests before they are forwarded to upstream services, and/or removes headers from outgoing responses before they are returned to clients. This policy provides comprehensive header removal functionality for both request and response flows.

## Features

- Removes specified headers from requests before forwarding to upstream services
- Removes specified headers from responses before returning to clients
- Supports both request and response phases independently or simultaneously
- Case-insensitive header name matching for reliable removal
- Header name normalization (lowercase conversion for consistency)
- Works with any HTTP method and request type
- Graceful handling of non-existent headers (no error if header doesn't exist)
- Comprehensive validation of header configurations

## Configuration

The Remove Headers policy can be configured for removal in request phase, response phase, or both.
This policy does not require system-level configuration and operates entirely based on the configured header name arrays.

### User Parameters (API Definition)

These parameters are configured per-API/route by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `requestHeaders` | array | No | - | Array of header objects to remove from requests before forwarding to upstream. Each object must contain a `name` field specifying the header name. At least one of `requestHeaders` or `responseHeaders` must be specified. |
| `responseHeaders` | array | No | - | Array of header objects to remove from responses before returning to clients. Each object must contain a `name` field specifying the header name. At least one of `requestHeaders` or `responseHeaders` must be specified. |

### Header Object Structure

Each header object in the `requestHeaders` and `responseHeaders` arrays must contain:

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `name` | string | Yes | The name of the HTTP header to remove. Header names are matched case-insensitively. Cannot be empty or whitespace-only. |

## API Definition Examples

### Example 1: Removing Sensitive Request Headers

Remove authentication headers before forwarding to upstream:

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
    - name: remove-headers
      version: v0.1.0
      params:
        requestHeaders:
          - name: Authorization
          - name: X-API-Key
          - name: Cookie
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 2: Removing Server Information from Responses

Remove server identification headers from responses:

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
    - name: remove-headers
      version: v0.1.0
      params:
        responseHeaders:
          - name: Server
          - name: X-Powered-By
          - name: X-AspNet-Version
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 3: Removing Headers from Both Request and Response

Remove sensitive headers from both directions:

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
    - name: remove-headers
      version: v0.1.0
      params:
        requestHeaders:
          - name: X-Internal-Token
          - name: X-Debug-Mode
        responseHeaders:
          - name: X-Internal-Server-ID
          - name: X-Debug-Info
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 4: Route-Specific Header Removal

Apply different header removal rules to different routes:

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
        - name: remove-headers
          version: v0.1.0
          params:
            requestHeaders:
              - name: X-Cache-Control  # Remove caching hints for weather data
            responseHeaders:
              - name: Last-Modified    # Remove caching headers
    - method: GET
      path: /alerts/active
      policies:
        - name: remove-headers
          version: v0.1.0
          params:
            requestHeaders:
              - name: If-Modified-Since  # Remove conditional headers for alerts
            responseHeaders:
              - name: ETag               # Remove caching headers for real-time alerts
    - method: POST
      path: /alerts/active
      policies:
        - name: remove-headers
          version: v0.1.0
          params:
            requestHeaders:
              - name: X-Requested-With  # Remove AJAX headers
            responseHeaders:
              - name: Location          # Remove redirect headers for API responses
```

### Example 5: Multiple Remove Headers Policies

Use multiple remove-headers policies for different purposes:

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
    # Remove authentication headers
    - name: remove-headers
      version: v0.1.0
      params:
        requestHeaders:
          - name: Authorization
          - name: X-API-Key
          - name: Cookie
    # Remove server identification
    - name: remove-headers
      version: v0.1.0
      params:
        responseHeaders:
          - name: Server
          - name: X-Powered-By
    # Remove debugging headers
    - name: remove-headers
      version: v0.1.0
      params:
        requestHeaders:
          - name: X-Debug-Mode
        responseHeaders:
          - name: X-Debug-Info
          - name: X-Trace-ID
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

## Request/Response Transformation Examples

### Request Headers Removal (Example 1)

**Original client request:**
```
GET /weather/v1.0/US/NewYork HTTP/1.1
Host: api-gateway.company.com
Accept: application/json
Authorization: Bearer secret-token
X-API-Key: client-secret-key
User-Agent: WeatherApp/1.0
```

**Resulting upstream request:**
```
GET /api/v2/US/NewYork HTTP/1.1
Host: sample-backend:5000
Accept: application/json
User-Agent: WeatherApp/1.0
```

### Response Headers Removal (Example 2)

**Original upstream response:**
```
HTTP/1.1 200 OK
Content-Type: application/json
Server: Apache/2.4.41
X-Powered-By: PHP/7.4.0
X-AspNet-Version: 4.0.30319
Content-Length: 256

{"temperature": 22, "humidity": 65}
```

**Resulting client response:**
```
HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 256

{"temperature": 22, "humidity": 65}
```

### Both Request and Response Headers (Example 3)

**Original client request:**
```
GET /weather/v1.0/US/NewYork HTTP/1.1
Host: api-gateway.company.com
Accept: application/json
X-Internal-Token: internal-secret
X-Debug-Mode: enabled
```

**Resulting upstream request:**
```
GET /api/v2/US/NewYork HTTP/1.1
Host: sample-backend:5000
Accept: application/json
```

**Original upstream response:**
```
HTTP/1.1 200 OK
Content-Type: application/json
X-Internal-Server-ID: server-123
X-Debug-Info: processed-in-45ms
Content-Length: 256

{"temperature": 22, "humidity": 65}
```

**Resulting client response:**
```
HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 256

{"temperature": 22, "humidity": 65}
```

## Policy Behavior

### Header Removal Behavior

The policy uses **safe removal semantics**:

- **Existing Headers Only**: Only removes headers that actually exist; no error if header is not present
- **Case-Insensitive Matching**: Header names are matched case-insensitively (e.g., "authorization" matches "Authorization")
- **Complete Removal**: All values for multi-value headers are removed
- **Order Preservation**: Removal operations don't affect the order of remaining headers
- **No Side Effects**: Removing non-existent headers doesn't cause errors or warnings

### Header Name Normalization

The policy automatically normalizes header names for consistent matching:

- **Case Conversion**: All header names are converted to lowercase for processing
- **Whitespace Trimming**: Leading and trailing whitespace is removed from header names
- **Pattern Validation**: Only valid header name characters are accepted (letters, numbers, hyphens, underscores)
- **Matching**: Normalized names are used for header matching during removal

### Error Handling

The policy includes robust error handling and validation:

1. **Missing Configuration**: If neither `requestHeaders` nor `responseHeaders` is specified, validation fails at configuration time
2. **Invalid Arrays**: If header arrays are not properly formatted, validation fails at configuration time
3. **Invalid Names**: If header names are not strings or are empty, validation fails at configuration time
4. **Runtime Safety**: Missing headers during execution don't cause errors (graceful handling)
5. **Graceful Degradation**: Policy execution errors don't affect request processing

### Performance Considerations

- **Minimal Overhead**: Lightweight header removal with minimal memory allocation
- **Header Processing**: Efficient header removal using Go's standard HTTP header handling
- **No Network Calls**: All processing is done locally without external dependencies
- **Dual Phase**: Separate request and response processing for optimal performance

## Common Use Cases

1. **Security Enhancement**: Remove server identification headers like `Server`, `X-Powered-By` to reduce information disclosure.

2. **Authentication Cleanup**: Remove authentication headers like `Authorization`, `X-API-Key` before forwarding to internal services.

3. **Debug Information Removal**: Remove debug headers like `X-Debug-Mode`, `X-Trace-ID` from production responses.

4. **Cache Control**: Remove caching headers like `ETag`, `Last-Modified` to disable caching for specific endpoints.

5. **Privacy Protection**: Remove tracking headers or cookies before forwarding requests to upstream services.

6. **API Standardization**: Remove vendor-specific headers to present a consistent API interface.

7. **Compliance**: Remove headers containing sensitive information to meet regulatory requirements.

8. **Performance Optimization**: Remove unnecessary headers to reduce message size and improve performance.

## Best Practices

1. **Security Focus**: Always remove server identification headers in production environments to reduce attack surface.

2. **Sensitive Data**: Remove headers containing sensitive information like internal tokens, debug data, or system identifiers.

3. **Documentation**: Document removed headers so client developers and upstream services are aware of the changes.

4. **Testing**: Test header removal in development environments to ensure applications work correctly without removed headers.

5. **Monitoring**: Monitor for applications that break when expected headers are removed.

6. **Minimal Impact**: Only remove headers that are truly unnecessary or pose security risks.

7. **Upstream Coordination**: Coordinate with upstream services to ensure they don't depend on headers you plan to remove.

8. **Client Communication**: Inform client developers about headers that will be removed from responses.

## Security Considerations

1. **Information Disclosure**: Remove headers that reveal internal system information, versions, or architecture details.

2. **Authentication Tokens**: Remove authentication headers when forwarding to internal services that don't need them.

3. **Debug Information**: Never expose debug headers, trace IDs, or internal processing information in production.

4. **Compliance**: Use header removal to meet data protection and privacy compliance requirements.

5. **Attack Surface Reduction**: Remove headers that could be used by attackers for reconnaissance or exploitation.

6. **Session Management**: Consider removing session-related headers when they're not needed by upstream services.

7. **Logging**: Be aware that removed headers won't appear in upstream service logs, which may affect debugging.

8. **Validation**: Ensure that removing headers doesn't break authentication, authorization, or other security mechanisms.
