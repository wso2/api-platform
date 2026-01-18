# Remove Query Parameter

## Overview

The Remove Query Parameter policy dynamically removes a query parameter from incoming requests before they are forwarded to upstream services.

## Features

- Removes specified query parameter from the request URL before forwarding to upstream
- Properly handles existing query parameters without affecting others
- Works with any HTTP method and request type
- Preserves all other existing query parameters in the URL
- Safe handling when specified parameter doesn't exist in the request

## Configuration

The Remove Query Parameter policy uses a single-level configuration model where parameters are configured per-API/route in the API definition YAML.
This policy does not require system-level configuration and operates entirely based on the configured parameter names array.

### User Parameters (API Definition)

These parameters are configured per-API/route by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `queryParameters` | array | Yes | - | An array of query parameter names to remove from the request URL. Each item in the array specifies the name of a query parameter to be removed. |

#### Query Parameter Object

Each item in the `queryParameters` array has the following structure:

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `name` | string | Yes | - | The name of the query parameter to remove from the request URL. If a query parameter with this name exists in the request URL, it will be removed. Otherwise the request URL will remain unchanged. Example: "debug", "internal_token", "temp_param". |

## API Definition Examples

### Example 1: Removing Debug Parameter

Remove a debug parameter from all operations:

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
    - name: remove-query-parameter
      version: v0.1.0
      params:
        queryParameters:
          - name: debug
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 2: Route-Specific Parameter Removal

Remove different query parameters from different routes:

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
        - name: remove-query-parameter
          version: v0.1.0
          params:
            queryParameters:
              - name: cache_debug
              - name: internal_trace
    - method: GET
      path: /alerts/active
      policies:
        - name: remove-query-parameter
          version: v0.1.0
          params:
            queryParameters:
              - name: internal_flag
              - name: test_alert
    - method: POST
      path: /alerts/active
      policies:
        - name: remove-query-parameter
          version: v0.1.0
          params:
            queryParameters:
              - name: test_mode
              - name: dry_run
```

### Example 3: Multiple Query Parameter Removal

Remove multiple query parameters in a single policy configuration:

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
    - name: remove-query-parameter
      version: v0.1.0
      params:
        queryParameters:
          - name: debug
          - name: internal_token
          - name: temp_session
          - name: csrf_token
          - name: admin_flag
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
GET /weather/v1.0/US/NewYork?units=metric&debug=true&lang=en HTTP/1.1
Host: api-gateway.company.com
Accept: application/json
```

**Resulting upstream request (Example 1 - removing debug parameter):**
```
GET /api/v2/US/NewYork?units=metric&lang=en HTTP/1.1
Host: sample-backend:5000
Accept: application/json
```

### Handling Non-Existent Parameters

**Original client request without the target parameter:**
```
GET /weather/v1.0/US/NewYork?units=metric&lang=en HTTP/1.1
Host: api-gateway.company.com
Accept: application/json
```

**Resulting upstream request (Example 1 - debug parameter doesn't exist):**
```
GET /api/v2/US/NewYork?units=metric&lang=en HTTP/1.1
Host: sample-backend:5000
Accept: application/json
```

### Multiple Parameters Removal (Example 3)

**Original client request:**
```
GET /weather/v1.0/US/NewYork?units=metric&debug=true&internal_token=abc123&temp_session=xyz789&lang=en HTTP/1.1
Host: api-gateway.company.com
Accept: application/json
```

**Resulting upstream request:**
```
GET /api/v2/US/NewYork?units=metric&lang=en HTTP/1.1
Host: sample-backend:5000
Accept: application/json
```

### Single Parameter Among Many

**Original client request:**
```
GET /weather/v1.0/US/NewYork?api_key=12345&units=metric&debug=verbose&format=json&lang=en HTTP/1.1
Host: api-gateway.company.com
Accept: application/json
```

**Resulting upstream request (Example 1 - removing debug parameter):**
```
GET /api/v2/US/NewYork?api_key=12345&units=metric&format=json&lang=en HTTP/1.1
Host: sample-backend:5000
Accept: application/json
```

## Policy Behavior

### Parameter Removal Logic

The policy safely handles query parameter removal:

- **Exact Match**: Only removes parameters that exactly match the specified names
- **Case Sensitive**: Parameter name matching is case-sensitive  
- **Non-Existent Parameters**: If specified parameters don't exist, no changes are made
- **URL Structure**: Properly maintains URL structure and encoding after removal
- **Multiple Parameters**: Processes all parameters in the array sequentially
- **Duplicate Names**: Handles duplicate parameter names in the configuration (removes all instances)

### Array Processing

- **Order Independence**: Parameters are processed in array order, but removal order doesn't affect the final result
- **Error Tolerance**: Invalid entries in the array are skipped rather than causing policy failure
- **Performance**: Linear processing time based on array size

### Error Handling

The policy includes robust error handling:

1. **Missing queryParameters**: If `queryParameters` array is not provided, the policy passes the request through unchanged
2. **Invalid Array Format**: If `queryParameters` is not an array, the policy passes the request through unchanged  
3. **Missing Parameter Names**: If a parameter object is missing the `name` field, that parameter is skipped
4. **Empty Parameter Names**: Parameters with empty names are skipped
5. **Invalid Parameter Objects**: Non-object entries in the array are skipped
6. **URL Parse Errors**: Parameter removal handles URL parsing safely
7. **Upstream Errors**: Policy execution errors do not affect request processing

The policy is designed to be forgiving - invalid parameters are skipped rather than causing the entire policy to fail.

## Performance Considerations

- **Minimal Overhead**: Lightweight array iteration with minimal memory allocation
- **Linear Processing**: Processing time scales linearly with the number of parameters in the array (~40-115 ns/op)
- **URL Handling**: Efficient parameter removal using Go's standard library
- **No Network Calls**: All processing is done locally without external dependencies
- **Request Path Only**: Only modifies the request path, does not affect headers or body

## Common Use Cases

1. **Security Parameter Removal**: Remove sensitive parameters like API keys, tokens, or session IDs that shouldn't reach upstream services

2. **Debug Parameter Cleanup**: Remove debug, trace, or development-specific parameters in production environments  

3. **Tracking Parameter Removal**: Strip marketing and analytics tracking parameters (UTM parameters, Facebook/Google click IDs)

4. **Cache Parameter Cleanup**: Remove cache-busting parameters or timestamps that aren't needed by upstream services

5. **Internal Parameter Filtering**: Remove gateway-specific or internal parameters that were added for routing/processing

6. **Privacy Compliance**: Remove parameters that might contain personal information or tracking data

7. **Legacy Parameter Cleanup**: Remove deprecated or obsolete parameters that are no longer supported by upstream services

8. **A/B Testing Parameter Removal**: Remove test flags or experiment parameters after routing decisions are made

## Best Practices

1. **Parameter Selection**: Only remove parameters that you're certain shouldn't reach the upstream service

2. **Array Organization**: Group related parameters together in the array for better readability

3. **Case Sensitivity**: Be aware that parameter matching is case-sensitive (e.g., 'Debug' vs 'debug')

4. **Documentation**: Document which parameters are being removed and why for future maintenance

5. **Testing**: Test with various parameter combinations to ensure only intended parameters are removed

6. **Performance**: For large parameter lists, consider if all parameters really need to be removed or if some can be grouped


## Security Considerations

1. **Sensitive Data**: Removing sensitive parameters prevents them from reaching upstream services and being logged

2. **Parameter Validation**: Ensure the policy configuration doesn't accidentally remove legitimate API parameters

3. **Audit Trail**: Consider maintaining logs of removed parameters for security auditing

4. **Access Control**: Ensure only authorized users can configure policies that remove query parameters

5. **Upstream Impact**: Verify that removing parameters doesn't break upstream service functionality

6. **Information Disclosure**: Use this policy to prevent internal information from being exposed to upstream services

7. **Array Validation**: Validate the `queryParameters` array configuration during development to catch syntax errors

## Troubleshooting

### Common Issues

1. **Parameter Not Removed**: Verify the parameter name matches exactly (case-sensitive)
2. **Wrong Parameter Removed**: Double-check the configuration for typos in the parameter name  
3. **Policy Not Executing**: Ensure the policy is correctly placed in the policy chain and enabled
4. **URL Malformation**: Check that the original request URL is valid before policy execution
5. **Array Configuration**: Verify the `queryParameters` array format is correct with proper `name` fields

### Debugging Tips

1. Enable request logging to see before/after URLs
2. Test with both existing and non-existing parameters to verify behavior
3. Check policy configuration syntax and parameter naming in the array
4. Use integration tests to validate parameter removal behavior
5. Verify that only intended parameters are being removed by checking the array configuration
