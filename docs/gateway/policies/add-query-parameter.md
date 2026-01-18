# Add Query Parameter

## Overview

The Add Query Parameter policy dynamically adds multiple query parameters to incoming requests before they are forwarded to upstream services.

## Features

- Adds multiple query parameters to the request URL before forwarding to upstream
- Supports multiple values for the same parameter name (values are appended)
- Properly handles existing query parameters without conflicts
- URL-safe encoding of parameter names and values
- Static value assignment with support for special characters
- Works with any HTTP method and request type
- Preserves existing query parameters in the URL

## Configuration

The Add Query Parameter policy uses a single-level configuration model where parameters are configured per-API/route in the API definition YAML.
This policy does not require system-level configuration and operates entirely based on the configured query parameter array.

### User Parameters (API Definition)

These parameters are configured per-API/route by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `queryParameters` | array | Yes | - | An array of query parameters to add to the request URL. Each query parameter consists of a name and value object. Multiple parameters with the same name will result in multiple values for that parameter. |

#### Query Parameter Object

Each item in the `queryParameters` array has the following structure:

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | Yes | - | The name of the query parameter to add to the request URL. This parameter name will be URL-encoded to ensure proper handling of special characters. Example: "api_key", "version", "source", "filter". |
| `value` | string | Yes | - | The value of the query parameter to add to the request URL. This parameter value will be URL-encoded to ensure proper handling of special characters. Can be static text, empty string, or contain special characters. Multiple parameters with the same name will have all values appended. |

## API Definition Examples

### Example 1: Adding Multiple Query Parameters

Add an API key and version parameter to all operations:

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
        queryParameters:
          - name: api_key
            value: "12345-abcde-67890-fghij"
          - name: version
            value: "2.0"
          - name: source
            value: "gateway"
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 2: Multiple Values for Same Parameter Name

Add multiple filter parameters that will result in multiple values for the `filter` query parameter:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: product-api-v1.0
spec:
  displayName: Product-API
  version: v1.0
  context: /products/$version
  upstream:
    main:
      url: http://product-service:8080/api
  policies:
    - name: add-query-parameter
      version: v0.1.0
      params:
        queryParameters:
          - name: filter
            value: "category:electronics"
          - name: filter
            value: "price:<100"
          - name: filter
            value: "availability:in_stock"
          - name: api_key
            value: "prod-12345"
  operations:
    - method: GET
      path: /search
    - method: GET
      path: /categories/{category}
```

### Example 3: Operation-Specific Query Parameters

Add different query parameters to specific operations:

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
            queryParameters:
              - name: include_forecast
                value: "true"
              - name: units
                value: "metric"
    - method: GET
      path: /alerts/active
      policies:
        - name: add-query-parameter
          version: v0.1.0
          params:
            queryParameters:
              - name: severity
                value: "high"
              - name: format
                value: "json"
    - method: POST
      path: /alerts/active
      policies:
        - name: add-query-parameter
          version: v0.1.0
          params:
            queryParameters:
              - name: validate
                value: "true"
```

### Example 4: Empty Values and Special Characters

Handle empty values and special characters in query parameters:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: search-api-v1.0
spec:
  displayName: Search-API
  version: v1.0
  context: /search/$version
  upstream:
    main:
      url: http://search-service:9200
  policies:
    - name: add-query-parameter
      version: v0.1.0
      params:
        queryParameters:
          - name: debug
            value: ""  # Empty value
          - name: query
            value: "type:article AND status:published"
          - name: highlight
            value: "title,content"
          - name: source
            value: "api-gateway"
  operations:
    - method: GET
      path: /documents
    - method: POST
      path: /documents/search
```

## Configuration Examples

### Single Query Parameter

To add a single query parameter, provide one item in the queryParameters array:

```yaml
policies:
  - name: add-query-parameter
    version: v0.1.0
    params:
      queryParameters:
        - name: api_key
          value: "12345-abcde"
```

**Request Transformation:**
- Input URL: `/weather/US/NewYork`
- Output URL: `/weather/US/NewYork?api_key=12345-abcde`

### Multiple Different Parameters

To add multiple different query parameters:

```yaml
policies:
  - name: add-query-parameter
    version: v0.1.0
    params:
      queryParameters:
        - name: api_key
          value: "12345"
        - name: version
          value: "2.0"
        - name: format
          value: "json"
```

**Request Transformation:**
- Input URL: `/api/data`
- Output URL: `/api/data?api_key=12345&version=2.0&format=json`

### Multiple Values for Same Parameter

To add multiple values for the same query parameter name:

```yaml
policies:
  - name: add-query-parameter
    version: v0.1.0
    params:
      queryParameters:
        - name: filter
          value: "category:books"
        - name: filter  
          value: "price:<50"
        - name: filter
          value: "rating:>4"
```

**Request Transformation:**
- Input URL: `/products/search`
- Output URL: `/products/search?filter=category%3Abooks&filter=price%3A%3C50&filter=rating%3A%3E4`

### Handling Existing Query Parameters

The policy properly handles existing query parameters:

```yaml
policies:
  - name: add-query-parameter
    version: v0.1.0
    params:
      queryParameters:
        - name: source
          value: "gateway"
        - name: timestamp
          value: "2024-01-01"
```

**Request Transformation:**
- Input URL: `/api/search?q=weather&limit=10`
- Output URL: `/api/search?q=weather&limit=10&source=gateway&timestamp=2024-01-01`

## Request Transformation Examples

### Before Policy Application

**Original client request:**
```
GET /weather/v1.0/US/NewYork HTTP/1.1
Host: api-gateway.company.com
Accept: application/json
```

**Resulting upstream request (using single parameter):**
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

**Resulting upstream request (adding multiple parameters):**
```
GET /api/v2/US/NewYork?units=metric&lang=en&api_key=12345&version=2.0&source=gateway HTTP/1.1
Host: sample-backend:5000
Accept: application/json
```

### Multiple Values for Same Parameter

**Original client request:**
```
GET /products/search HTTP/1.1
Host: api-gateway.company.com
Accept: application/json
```

**With multiple filter values, resulting upstream request:**
```
GET /api/products/search?filter=category%3Aelectronics&filter=price%3A%3C100&filter=availability%3Ain_stock HTTP/1.1
Host: product-service:8080
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

1. **Missing queryParameters**: If `queryParameters` array is not provided, the policy passes the request through unchanged
2. **Invalid Array Format**: If `queryParameters` is not an array, the policy passes the request through unchanged  
3. **Missing Parameter Fields**: If a parameter object is missing `name` or `value` fields, that parameter is skipped
4. **Empty Parameter Names**: Parameters with empty or missing names are skipped
5. **Invalid Parameter Objects**: Non-object entries in the array are skipped
6. **URL Parse Errors**: If URL parsing fails, falls back to simple string concatenation
7. **Upstream Errors**: Policy execution errors do not affect request processing

The policy is designed to be forgiving - invalid parameters are skipped rather than causing the entire policy to fail.

### Multiple Values Behavior

- **Same Parameter Name**: When multiple parameter objects have the same `name`, all values are added as separate query parameter entries
- **URL Encoding**: Each value is individually URL-encoded  
- **Order Preservation**: Parameters are processed and added in the order they appear in the array
- **Existing Parameters**: New parameters are appended to any existing query parameters in the URL

## Best Practices

1. **Parameter Naming**: Use clear, descriptive parameter names that don't conflict with client-provided parameters

2. **Array Organization**: Group related parameters together in the array for better readability

3. **Value Security**: Be cautious about adding sensitive values like API keys - ensure they're properly managed and rotated

4. **URL Length Limits**: Be aware that adding query parameters increases URL length; consider server URL length limits

5. **Parameter Conflicts**: Avoid parameter names that clients might use to prevent conflicts

6. **Multiple Values**: When you need multiple values for the same parameter, add multiple objects with the same `name` rather than trying to encode multiple values in a single string

7. **Documentation**: Document added parameters so upstream service developers are aware of them

8. **Validation**: Validate your `queryParameters` array configuration during development to catch syntax errors early

### Performance Considerations

- **Minimal Overhead**: Lightweight array iteration and string processing with minimal memory allocation
- **URL Parsing**: Efficient URL parsing and reconstruction using Go's standard library
- **No Network Calls**: All processing is done locally without external dependencies
- **Request Path Only**: Only modifies the request path, does not affect headers or body
- **Array Processing**: Processing time scales linearly with the number of parameters in the array

## Common Use Cases

1. **API Key Injection**: Automatically add API keys for upstream services that require them in query parameters

2. **Version Control**: Add version parameters to ensure requests are routed to the correct API version

3. **Source Tracking**: Add source identifiers to track where requests originate from for analytics

4. **Feature Flags**: Add multiple feature flag parameters to enable/disable specific functionality in upstream services

5. **Search Filters**: Add multiple filter parameters for search APIs (e.g., multiple `filter` parameters with different criteria)

6. **Authentication Tokens**: Add authentication tokens as query parameters for legacy systems

7. **Cache Control**: Add cache-related parameters to control upstream caching behavior

8. **Debug Information**: Add debug flags or trace IDs for troubleshooting and monitoring

9. **Client Identification**: Add client type, version, or platform information for upstream processing

10. **Multi-Value Parameters**: Add parameters that naturally have multiple values (like tags, categories, or filters)

## Security Considerations

1. **Sensitive Data**: Avoid adding sensitive information in query parameters as they may be logged or cached

2. **Parameter Validation**: Ensure upstream services validate all query parameters, including those added by the gateway

3. **Log Sanitization**: Configure logging to sanitize or exclude sensitive query parameters

4. **URL Encoding**: Rely on the policy's built-in URL encoding rather than pre-encoding values

5. **Access Control**: Ensure only authorized users can configure policies that add query parameters

6. **Upstream Trust**: Only add parameters that upstream services are configured to handle securely
