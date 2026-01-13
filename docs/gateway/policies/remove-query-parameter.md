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
This policy does not require system-level configuration and operates entirely based on the configured parameter name.

### User Parameters (API Definition)

These parameters are configured per-API/route by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
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
        name: debug
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
            name: cache_debug
    - method: GET
      path: /alerts/active
      policies:
        - name: remove-query-parameter
          version: v0.1.0
          params:
            name: internal_flag
    - method: POST
      path: /alerts/active
      policies:
        - name: remove-query-parameter
          version: v0.1.0
          params:
            name: test_mode
```

### Example 3: Multiple Query Parameter Removal

Remove multiple query parameters by using multiple policy instances:

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
        name: debug
    - name: remove-query-parameter
      version: v0.1.0
      params:
        name: internal_token
    - name: remove-query-parameter
      version: v0.1.0
      params:
        name: temp_session
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

- **Exact Match**: Only removes parameters that exactly match the specified name
- **Case Sensitive**: Parameter name matching is case-sensitive
- **Non-Existent Parameters**: If the specified parameter doesn't exist, no changes are made
- **URL Structure**: Properly maintains URL structure and encoding after removal
- **Other Parameters**: All other query parameters are preserved unchanged

### Error Handling

The policy includes robust error handling:

1. **Missing Parameter Name**: If `name` is not provided or empty, the policy passes the request through unchanged
2. **Invalid Configuration**: If the parameter configuration is malformed, the policy passes the request through unchanged
3. **URL Parse Errors**: If URL parsing fails, the original request is forwarded unchanged
4. **Upstream Errors**: Policy execution errors do not affect request processing

### Performance Considerations

- **Minimal Overhead**: Lightweight string processing with minimal memory allocation
- **URL Parsing**: Efficient URL parsing and reconstruction using Go's standard library
- **No Network Calls**: All processing is done locally without external dependencies
- **Request Path Only**: Only modifies the request path, does not affect headers or body

## Common Use Cases

1. **Security Sanitization**: Remove internal tokens, debug flags, or sensitive parameters before forwarding to upstream services.

2. **API Cleanup**: Remove temporary or client-side specific parameters that shouldn't reach the backend.

3. **Version Migration**: Remove deprecated parameters during API version transitions.

4. **Debug Parameter Filtering**: Remove debug or development parameters in production environments.

5. **Internal Parameter Stripping**: Remove gateway-specific parameters that are only used for internal processing.

6. **Cache Parameter Cleanup**: Remove cache-related parameters that shouldn't affect upstream processing.

7. **Session Management**: Remove temporary session identifiers that are only relevant at the gateway level.

8. **Testing Parameter Removal**: Remove test flags or parameters used during development/testing phases.

## Best Practices

1. **Parameter Naming**: Use clear naming conventions for parameters that should be removed to avoid accidental removal of legitimate client parameters

2. **Documentation**: Document which parameters are removed so API consumers are aware of the filtering

3. **Testing**: Test the policy with various parameter combinations to ensure only intended parameters are removed

4. **Case Sensitivity**: Be aware that parameter name matching is case-sensitive

5. **Multiple Removals**: When removing multiple parameters, use separate policy instances for clarity and maintainability

6. **Upstream Coordination**: Ensure upstream services don't depend on parameters that are being removed

7. **Logging**: Consider logging removed parameters for debugging and audit purposes

## Security Considerations

1. **Sensitive Data**: Removing sensitive parameters prevents them from reaching upstream services and being logged

2. **Parameter Validation**: Ensure the policy configuration doesn't accidentally remove legitimate API parameters

3. **Audit Trail**: Consider maintaining logs of removed parameters for security auditing

4. **Access Control**: Ensure only authorized users can configure policies that remove query parameters

5. **Upstream Impact**: Verify that removing parameters doesn't break upstream service functionality

6. **Information Disclosure**: Use this policy to prevent internal information from being exposed to upstream services

## Troubleshooting

### Common Issues

1. **Parameter Not Removed**: Verify the parameter name matches exactly (case-sensitive)
2. **Wrong Parameter Removed**: Double-check the configuration for typos in the parameter name
3. **Policy Not Executing**: Ensure the policy is correctly placed in the policy chain and enabled
4. **URL Malformation**: Check that the original request URL is valid before policy execution

### Debugging Tips

1. Enable request logging to see before/after URLs
2. Use multiple policy instances to remove parameters incrementally for testing
3. Test with both existing and non-existing parameters to verify behavior
4. Check policy configuration syntax and parameter naming
