# API Key Authentication

## Overview

The API Key Authentication policy validates API keys to secure APIs by verifying pre-generated keys before allowing access to protected resources. This policy is essential for API security, supporting both header-based and query parameter-based key validation.

## Features

- Validates API keys from request headers or query parameters
- Configurable key extraction with optional prefix stripping
- Flexible authentication source configuration (header/query)
- Pre-generated key validation against gateway-managed key lists
- Request context enrichment with authentication metadata
- Case-insensitive header matching

## Configuration

The API Key Authentication policy uses a single-level configuration model where all parameters are configured per-API/route in the API definition YAML. This policy does not require system-level configuration as API keys are managed by the platform's key management system.

### User Parameters (API Definition)

These parameters are configured per-API/route by the API developer:

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `key` | string | Yes | - | The name of the header or query parameter that contains the API key. For headers: case-insensitive matching is used (e.g., "X-API-Key", "Authorization"). For query parameters: exact name matching is used (e.g., "api_key", "token"). |
| `in` | string | Yes | - | Specifies where to look for the API key. Must be either "header" or "query". |
| `value-prefix` | string | No | - | Optional prefix that should be stripped from the API key value before validation. Case-insensitive matching and removal. Common use case is "Bearer " for Authorization headers. |

## API Definition Examples

### Example 1: Basic API Key Authentication (Header)

Apply API key authentication using a custom header:

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
    - name: api-key-auth
      version: v0.1.0
      params:
        key: X-API-Key
        in: header
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 2: Authorization Header with Bearer Prefix

Use API key in Authorization header with Bearer prefix:

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
    - name: api-key-auth
      version: v0.1.0
      params:
        key: Authorization
        in: header
        value-prefix: "Bearer "
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 3: Query Parameter Authentication

Extract API key from query parameter:

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
    - name: api-key-auth
      version: v0.1.0
      params:
        key: api_key
        in: query
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 4: Custom Header with Custom Prefix

Use a custom header with a custom prefix:

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
    - name: api-key-auth
      version: v0.1.0
      params:
        key: X-Custom-Auth
        in: header
        value-prefix: "ApiKey "
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: GET
      path: /alerts/active
    - method: POST
      path: /alerts/active
```

### Example 5: Route-Specific Authentication

Apply different API key configurations to different routes:

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
    - name: api-key-auth
      version: v0.1.0
      params:
        key: X-Custom-Auth
        in: header
        value-prefix: "ApiKey "
  operations:
    - method: GET
      path: /{country_code}/{city}
      policies:
        - name: api-key-auth
          version: v0.1.0
          params:
            key: X-API-Key
            in: header
    - method: GET
      path: /alerts/active
      policies:
        - name: api-key-auth
          version: v0.1.0
          params:
            key: Authorization
            in: header
            value-prefix: "Bearer "
    - method: POST
      path: /alerts/active
```

## Use Cases

1. **Simple API Security**: Protect APIs with straightforward pre-shared key authentication for internal services or partner integrations.

2. **Partner API Access**: Provide API keys to trusted partners for accessing specific API resources without complex OAuth flows.

3. **Legacy System Integration**: Integrate with legacy systems that support simple API key authentication mechanisms.

4. **Development and Testing**: Use API keys for development and testing environments where full OAuth implementations might be overkill.

5. **Service-to-Service Communication**: Enable simple authentication between internal microservices using API keys.

6. **Third-Party Integrations**: Provide API access to third-party services using API keys for webhook callbacks or data synchronization.

## Key Management

API keys used with this policy are managed by the platform's key management system:

- **Generation**: Keys are generated through the gateway, management portal, or developer portal
- **Validation**: The policy validates incoming keys against the policy engine's key store
- **Lifecycle**: Keys can be created, rotated, revoked, and expired through platform APIs
- **Security**: Keys are securely stored and managed by the platform infrastructure in the gateway environment

## Security Considerations

1. **HTTPS Only**: Always use API key authentication over HTTPS to prevent key interception
2. **Key Rotation**: Regularly rotate API keys to maintain security
3. **Key Storage**: Store keys securely on the client side and avoid hardcoding in source code
4. **Monitoring**: Monitor API key usage for suspicious activity
5. **Prefix Usage**: Use prefixes like "Bearer " to clearly identify the authentication scheme
6. **Query Parameter Caution**: Be cautious when using query parameters as they may be logged in access logs
