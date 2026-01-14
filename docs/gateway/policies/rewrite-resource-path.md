# Rewrite Resource Path

## Overview

The Rewrite Resource Path policy allows you to dynamically replace the resource path of incoming requests before they are forwarded to upstream services. This policy replaces the entire resource path (the part after the hostname and before query parameters) with a specified path while preserving all query parameters from the original request.

## Features

- Replaces the resource path portion of incoming request URLs
- Preserves all existing query parameters from the original request
- Works with any HTTP method and request type
- Static path replacement with configurable target paths
- Proper URL handling and encoding
- No impact on request headers or body content

## Configuration

The Rewrite Resource Path policy uses a single-level configuration model where the target path is configured per-API/route in the API definition YAML. This policy does not require system-level configuration and operates entirely based on the configured path parameter.

### User Parameters (API Definition)

This parameter is configured per-API/route by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `path` | string | Yes | - | The new resource path that will replace the original path in the incoming request. This should be the complete path without the leading slash or hostname. Query parameters from the original request will be preserved. |

## API Definition Examples

### Example 1: Basic Path Rewrite

Replace all incoming paths with a fixed path:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: user-api-v1.0
spec:
  displayName: User-API
  version: v1.0
  context: /users/$version
  upstream:
    main:
      url: http://internal-backend:8080
  policies:
    - name: rewrite-resource-path
      version: v0.1.0
      params:
        path: "api/v2/customers"
  operations:
    - method: GET
      path: /profile
    - method: POST
      path: /profile
    - method: PUT
      path: /settings
```

### Example 2: Version Migration

Redirect requests from old API version to new version:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: legacy-api-v1.0
spec:
  displayName: Legacy-API
  version: v1.0
  context: /legacy/$version
  upstream:
    main:
      url: http://new-service:9000
  policies:
    - name: rewrite-resource-path
      version: v0.1.0
      params:
        path: "v3/modernized/endpoint"
  operations:
    - method: GET
      path: /old-endpoint
    - method: POST
      path: /deprecated-action
```

### Example 3: Route-Specific Path Rewrites

Apply different path rewrites to different routes:

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: multi-service-api-v1.0
spec:
  displayName: Multi-Service-API
  version: v1.0
  context: /api/$version
  upstream:
    main:
      url: http://backend-cluster:8080
  operations:
    - method: GET
      path: /users
      policies:
        - name: rewrite-resource-path
          version: v0.1.0
          params:
            path: "user-service/v2/accounts"
    - method: GET
      path: /orders
      policies:
        - name: rewrite-resource-path
          version: v0.1.0
          params:
            path: "order-service/v1/purchases"
    - method: GET
      path: /products
      policies:
        - name: rewrite-resource-path
          version: v0.1.0
          params:
            path: "catalog-service/v3/items"
```

## Request Transformation Examples

### Basic Path Replacement

**Original client request:**
```
GET /users/v1.0/profile?userId=123&format=json HTTP/1.1
Host: api-gateway.company.com
Accept: application/json
```

**Resulting upstream request (Example 1):**
```
GET /api/v2/customers?userId=123&format=json HTTP/1.1
Host: internal-backend:8080
Accept: application/json
```

### Preserving Query Parameters

**Original client request:**
```
POST /legacy/v1.0/old-endpoint?action=create&timestamp=1642781234 HTTP/1.1
Host: api-gateway.company.com
Content-Type: application/json
```

**Resulting upstream request (Example 2):**
```
POST /v3/modernized/endpoint?action=create&timestamp=1642781234 HTTP/1.1
Host: new-service:9000
Content-Type: application/json
```

## Policy Behavior

### Path Processing

The policy processes paths as follows:

- **Complete Replacement**: The entire resource path is replaced with the configured path
- **Query Preservation**: All query parameters from the original request are preserved
- **URL Encoding**: Proper URL encoding is maintained throughout the transformation
- **Method Independence**: Works with all HTTP methods (GET, POST, PUT, DELETE, etc.)

### Error Handling

The policy includes robust error handling:

1. **Missing Path Parameter**: If the `path` parameter is not provided or empty, the request passes through unchanged
2. **Invalid Configuration**: Malformed path configurations are handled gracefully
3. **URL Parse Errors**: If URL parsing fails, the policy falls back to direct path replacement
4. **Upstream Errors**: Policy execution errors do not affect request processing

### Performance Considerations

- **Minimal Overhead**: Lightweight string processing with minimal memory allocation
- **URL Parsing**: Efficient URL parsing and reconstruction using Go's standard library
- **No Network Calls**: All processing is done locally without external dependencies
- **Path Only**: Only modifies the request path, preserving headers and body content

## Common Use Cases

1. **API Versioning**: Redirect requests from old API versions to new backend service versions.

2. **Service Abstraction**: Hide complex internal service paths behind simple public API paths.

3. **Legacy Migration**: Gradually migrate from legacy endpoints to new service architectures.

4. **Microservice Routing**: Route public API calls to appropriate internal microservices.

5. **Backend Integration**: Integrate with third-party services that require specific path formats.

6. **Path Normalization**: Standardize different client path formats to a consistent backend format.

7. **Service Consolidation**: Route multiple external endpoints to a single backend service path.

8. **URL Rewriting**: Transform user-friendly URLs to backend-specific technical paths.

## Best Practices

1. **Path Design**: Design target paths to be clear, consistent, and follow RESTful conventions

2. **Documentation**: Document path transformations so both client and backend developers understand the mapping

3. **Testing**: Thoroughly test path rewrites with various query parameter combinations

4. **Versioning**: Consider version compatibility when rewriting paths between different API versions

5. **Monitoring**: Monitor upstream services to ensure rewritten paths are correctly handled

6. **Security**: Ensure rewritten paths don't expose internal service structures or sensitive information

7. **Performance**: Keep target paths reasonably short to minimize URL length

## Security Considerations

1. **Path Validation**: Ensure target paths are validated and don't contain malicious content

2. **Access Control**: Verify that rewritten paths maintain appropriate access controls at the upstream service

3. **Information Disclosure**: Avoid exposing internal service structures or sensitive paths

4. **Input Validation**: While the policy doesn't process user input directly, ensure upstream services validate the rewritten paths

5. **Logging**: Configure logging to track path transformations for security auditing

6. **Backend Security**: Ensure upstream services are configured to handle the expected rewritten paths securely
