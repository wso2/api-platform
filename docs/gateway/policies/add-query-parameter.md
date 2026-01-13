# Add Query Parameter

## Overview

The Add Query Parameter policy dynamically adds query parameters to outgoing requests before they are forwarded to upstream services.

## Features

- Adds query parameters to the request URL before forwarding to upstream
- Properly handles existing query parameters without conflicts
- URL-safe encoding of parameter names and values
- Static value assignment with support for special characters
- Works with any HTTP method and request type
- Preserves existing query parameters in the URL

## Configuration

The Add Query Parameter policy uses a single-level configuration model where parameters are configured per-API/route in the API definition YAML.
This policy does not require system-level configuration and operates entirely based on the configured name-value pairs.

### User Parameters (API Definition)

These parameters are configured per-API/route by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `name` | string | Yes | - | The name of the query parameter to add to the request URL. This parameter name will be URL-encoded to ensure proper handling of special characters. Example: "api_key", "version", "source". |
| `value` | string | Yes | - | The value of the query parameter to add to the request URL. This parameter value will be URL-encoded to ensure proper handling of special characters. Can be static text, empty string, or contain special characters. |

## API Definition Examples

### Example 1: Adding API Key Parameter

Add an API key parameter to all operations:

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
    - name: add-query-parameter
      version: v0.1.0
      params:
        name: api_key
        value: "12345-abcde-67890-fghij"
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 2: Adding Version Parameter

Add a version parameter to specify API version:

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
    - name: add-query-parameter
      version: v0.1.0
      params:
        name: version
        value: "2.0"
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 3: Adding Source Tracking Parameter

Add a source parameter to track request origins:

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
    - name: add-query-parameter
      version: v0.1.0
      params:
        name: source
        value: "api-gateway"
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 4: Adding Complex Value with Special Characters

Add a parameter with a complex value containing special characters:

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
    - name: add-query-parameter
      version: v0.1.0
      params:
        name: client_id
        value: "app@company.com:v1.2.3"
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 5: Route-Specific Parameters

Apply different query parameters to different routes:

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
        - name: add-query-parameter
          version: v0.1.0
          params:
            name: cache_enabled
            value: "true"
    - method: GET
      path: /alerts/active
      policies:
        - name: add-query-parameter
          version: v0.1.0
          params:
            name: real_time
            value: "true"
    - method: POST
      path: /alerts/active
      policies:
        - name: add-query-parameter
          version: v0.1.0
          params:
            name: async_mode
            value: "false"
```

### Example 6: Multiple Query Parameters

Add multiple query parameters by using multiple policy instances:

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
    - name: add-query-parameter
      version: v0.1.0
      params:
        name: api_key
        value: "12345-abcde-67890-fghij"
    - name: add-query-parameter
      version: v0.1.0
      params:
        name: format
        value: "json"
    - name: add-query-parameter
      version: v0.1.0
      params:
        name: source
        value: "gateway"
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

## Request Transformation Examples

### Before Policy Application

**Original client request:**
```
GET /weather/v1.0/US/NewYork HTTP/1.1
Host: api-gateway.company.com
Accept: application/json
```

**Resulting upstream request (Example 1):**
```
GET /api/v2/US/NewYork?api_key=12345-abcde-67890-fghij HTTP/1.1
Host: sample-backend:5000
Accept: application/json
```

### Handling Existing Query Parameters

**Original client request with existing parameters:**
```
GET /weather/v1.0/US/NewYork?units=metric&lang=en HTTP/1.1
Host: api-gateway.company.com
Accept: application/json
```

**Resulting upstream request (Example 1):**
```
GET /api/v2/US/NewYork?units=metric&lang=en&api_key=12345-abcde-67890-fghij HTTP/1.1
Host: sample-backend:5000
Accept: application/json
```

### Multiple Parameters (Example 6)

**Original client request:**
```
GET /weather/v1.0/US/NewYork HTTP/1.1
Host: api-gateway.company.com
Accept: application/json
```

**Resulting upstream request:**
```
GET /api/v2/US/NewYork?api_key=12345-abcde-67890-fghij&format=json&source=gateway HTTP/1.1
Host: sample-backend:5000
Accept: application/json
```

## Policy Behavior

### URL Encoding

The policy automatically handles URL encoding for both parameter names and values:

- **Special Characters**: Characters like `@`, `:`, `/`, `?`, `#` are properly encoded
- **Spaces**: Converted to `%20` or `+` depending on context
- **Unicode**: Properly encoded using percent-encoding
- **Existing Parameters**: Preserved and properly separated with `&`

### Error Handling

The policy includes robust error handling:

1. **Missing Parameters**: If `name` is not provided or empty, the policy passes the request through unchanged
2. **Invalid Values**: If `value` parameter is missing, the policy passes the request through unchanged
3. **URL Parse Errors**: If URL parsing fails, falls back to simple string concatenation
4. **Upstream Errors**: Policy execution errors do not affect request processing

### Performance Considerations

- **Minimal Overhead**: Lightweight string processing with minimal memory allocation
- **URL Parsing**: Efficient URL parsing and reconstruction using Go's standard library
- **No Network Calls**: All processing is done locally without external dependencies
- **Request Path Only**: Only modifies the request path, does not affect headers or body

## Common Use Cases

1. **API Key Injection**: Automatically add API keys for upstream services that require them in query parameters.

2. **Version Control**: Add version parameters to ensure requests are routed to the correct API version.

3. **Source Tracking**: Add source identifiers to track where requests originate from for analytics.

4. **Feature Flags**: Add feature flag parameters to enable/disable specific functionality in upstream services.

5. **Authentication Tokens**: Add authentication tokens as query parameters for legacy systems.

6. **Cache Control**: Add cache-related parameters to control upstream caching behavior.

7. **Debug Information**: Add debug flags or trace IDs for troubleshooting and monitoring.

8. **Client Identification**: Add client type, version, or platform information for upstream processing.

## Best Practices

1. **Parameter Naming**: Use clear, descriptive parameter names that don't conflict with client-provided parameters

2. **Value Security**: Be cautious about adding sensitive values like API keys - ensure they're properly managed and rotated

3. **URL Length Limits**: Be aware that adding query parameters increases URL length; consider server URL length limits

4. **Parameter Conflicts**: Avoid parameter names that clients might use to prevent conflicts

5. **Encoding Awareness**: The policy handles URL encoding automatically, but be aware of the final encoded values

6. **Multiple Parameters**: When adding multiple parameters, consider using multiple policy instances rather than complex value encoding

7. **Documentation**: Document added parameters so upstream service developers are aware of them

## Security Considerations

1. **Sensitive Data**: Avoid adding sensitive information in query parameters as they may be logged or cached

2. **Parameter Validation**: Ensure upstream services validate all query parameters, including those added by the gateway

3. **Log Sanitization**: Configure logging to sanitize or exclude sensitive query parameters

4. **URL Encoding**: Rely on the policy's built-in URL encoding rather than pre-encoding values

5. **Access Control**: Ensure only authorized users can configure policies that add query parameters

6. **Upstream Trust**: Only add parameters that upstream services are configured to handle securely
