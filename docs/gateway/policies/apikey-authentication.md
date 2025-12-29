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

## API Key Management

The gateway controller provides REST APIs to manage API keys for APIs that use the API Key Authentication policy. These endpoints allow you to generate, view, rotate, and revoke API keys programmatically.

### Base URL

The gateway controller REST API is available at:
- **Local development**: `http://localhost:9090`
- **Docker/Kubernetes**: `http://gateway-controller:9090`

### Authentication

All API key management operations require authentication. The gateway controller REST API endpoints are secured using either:

- **Basic Authentication**: Username and password credentials
- **JWT Authentication**: JSON Web Token in the Authorization header

The gateway controller uses the authentication context of the requesting user to ensure that:
- Users can only manage API keys they created
- API keys are properly associated with the authenticated user
- Proper authorization is enforced for all operations

#### Basic Authentication Example

```bash
curl -X POST "http://localhost:9090/apis/weather-api-v1.0/generate-api-key" \
  -H "Content-Type: application/json" \
  -u "username:password" \
  -d '{"name": "production-key"}'
```

#### JWT Authentication Example

```bash
curl -X POST "http://localhost:9090/apis/weather-api-v1.0/generate-api-key" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  -d '{"name": "production-key"}'
```

### Generate API Key

Generate a new API key for a specific API.

**Endpoint**: `POST /apis/{id}/generate-api-key`

#### Request Parameters

| Parameter | Type | Location | Required | Description |
|-----------|------|----------|----------|-------------|
| `id` | string | path | Yes | Unique public identifier of the API (e.g., `weather-api-v1.0`) |

#### Request Body

```json
{
  "name": "weather-api-key",
  "expires_in": {
    "duration": 30,
    "unit": "days"
  }
}
```

**Request Body Schema:**

| Field                | Type | Required | Description |
|----------------------|------|----------|-------------|
| `name`               | string | No | Custom name for the API key. If not provided, a default name will be generated |
| `expires_at`         | string (ISO 8601) | No | Specific expiration timestamp for the API key. If both `expires_in` and `expires_at` are provided, `expires_at` takes precedence |
| `expires_in`         | object | No | Relative expiration time from creation |
| `expires_in.duration` | integer | Yes (if expiresIn used) | Duration value |
| `expires_in.unit`     | string | Yes (if expiresIn used) | Time unit: `seconds`, `minutes`, `hours`, `days`, `weeks`, `months` |

#### Example Request

**Using Basic Authentication:**
```bash
curl -X POST "http://localhost:9090/apis/weather-api-v1.0/generate-api-key" \
  -H "Content-Type: application/json" \
  -u "username:password" \
  -d '{
    "name": "production-key",
    "expires_in": {
      "duration": 90,
      "unit": "days"
    }
  }'
```

**Using JWT Authentication:**
```bash
curl -X POST "http://localhost:9090/apis/weather-api-v1.0/generate-api-key" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  -d '{
    "name": "production-key",
    "expires_in": {
      "duration": 90,
      "unit": "days"
    }
  }'
```

#### Successful Response (201 Created)

```json
{
  "api_key": {
    "apiId": "weather-api-v1.0",
    "api_key": "apip_3521f31335d98868f1526ef20b5c051a7aa42cdd0dd46747b1456e1264a7e6ad",
    "created_at": "2025-12-22T13:02:24.504957558Z",
    "created_by": "john",
    "expires_at": "2025-12-23T13:02:24.504957558Z",
    "name": "production-key",
    "operations": "[\"*\"]",
    "status": "active"
  },
  "message": "API key generated successfully",
  "status": "success"
}
```

#### Response Schema

| Field | Type | Description                                    |
|-------|------|------------------------------------------------|
| `status` | string | Operation status (`success`)                   |
| `message` | string | Detailed message of the status                 |
| `apiKey.name` | string | Name of the generated API key                  |
| `apiKey.apiId` | string | API identifier                                 |
| `apiKey.api_key` | string | The actual API key value (starts with `apip_`) |
| `apiKey.status` | string | Key status (`active`)                          |
| `apiKey.created_at` | string | ISO 8601 timestamp of creation                 |
| `apiKey.created_by` | string | User who created the key                       |
| `apiKey.expires_at` | string | ISO 8601 expiration timestamp (if set)         |
| `apiKey.operations` | array | Allowed operations (currently `["*"]` for all) |

### List API Keys

Retrieve all active API keys for the specified API created by the user.
If the user is an admin, all API keys for the API are returned.

**Endpoint**: `GET /apis/{id}/api-keys`

#### Request Parameters

| Parameter | Type | Location | Required | Description |
|-----------|------|----------|----------|-------------|
| `id` | string | path | Yes | Unique public identifier of the API |

#### Example Request

**Using Basic Authentication:**
```bash
curl -X GET "http://localhost:9090/apis/weather-api-v1.0/api-keys" \
  -u "username:password"
```

**Using JWT Authentication:**
```bash
curl -X GET "http://localhost:9090/apis/weather-api-v1.0/api-keys" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

#### Successful Response (200 OK)

```json
{
  "apiKeys": [
    {
      "apiId": "weather-api-v1.0",
      "created_at": "2025-12-22T13:02:24.504957558Z",
      "created_by": "john",
      "expires_at": "2025-12-23T13:02:24.504957558Z",
      "name": "test-key",
      "operations": "[\"*\"]",
      "status": "active"
    },
    {
      "apiId": "weather-api-v1.0",
      "created_at": "2025-12-22T13:02:24.504957558Z",
      "created_by": "admin",
      "expires_at": "2026-03-22T13:02:24.504957558Z",
      "name": "production-key",
      "operations": "[\"*\"]",
      "status": "active"
    }
  ],
  "status": "success",
  "totalCount": 2
}
```

#### Response Schema

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Operation status (`success`) |
| `totalCount` | integer | Total number of active API keys |
| `apiKeys` | array | List of API key objects (without the actual key value for security) |

**Note**: The actual API key values are not returned in the list response for security reasons.

### Rotate API Key

Rotate an existing API key, generating a new key value while maintaining the same name and metadata.
Only the user who created the key can perform this operation.

**Endpoint**: `POST /apis/{id}/api-keys/{apiKeyName}/regenerate`

#### Request Parameters

| Parameter | Type | Location | Required | Description |
|-----------|------|----------|----------|-------------|
| `id` | string | path | Yes | Unique public identifier of the API |
| `apiKeyName` | string | path | Yes | Name of the API key to rotate |

#### Request Body

```json
{
  "expires_in": {
    "duration": 60,
    "unit": "days"
  }
}
```

**Request Body Schema:** Same as the generate API key request body, but only expiration settings are typically updated during rotation.

#### Example Request

**Using Basic Authentication:**
```bash
curl -X POST "http://localhost:9090/apis/weather-api-v1.0/api-keys/production-key/regenerate" \
  -H "Content-Type: application/json" \
  -u "username:password" \
  -d '{
    "expires_in": {
      "duration": 60,
      "unit": "days"
    }
  }'
```

**Using JWT Authentication:**
```bash
curl -X POST "http://localhost:9090/apis/weather-api-v1.0/api-keys/production-key/regenerate" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  -d '{
    "expires_in": {
      "duration": 60,
      "unit": "days"
    }
  }'
```

#### Successful Response (200 OK)

```json
{
  "api_key": {
    "apiId": "weather-api-v1.0",
    "api_key": "apip_18dfd4da48f276043b32d3755c8ba3b0b244569f8c0f485ad50652cb95afae70",
    "created_at": "2025-12-22T12:26:47.626109914Z",
    "created_by": "thivindu",
    "expires_at": "2026-11-17T12:26:47.626109914Z",
    "name": "production-key",
    "operations": "[\"*\"]",
    "status": "active"
  },
  "message": "API key generated successfully",
  "status": "success"
}
```

**Note**: The old API key value becomes invalid immediately after rotation. Update your applications with the new key value.

### Revoke API Key

Revoke an existing API key, making it permanently invalid for authentication.
The user who created the key or an admin can perform this operation.

**Endpoint**: `POST /apis/{id}/revoke-api-key`

#### Request Parameters

| Parameter | Type | Location | Required | Description |
|-----------|------|----------|----------|-------------|
| `id` | string | path | Yes | Unique public identifier of the API |

#### Request Body

```json
{
  "api_key": "apip_4f3c2e1d5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d"
}
```

**Request Body Schema:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `api_key` | string | Yes | The actual API key value to revoke |

#### Example Request

**Using Basic Authentication:**
```bash
curl -X POST "http://localhost:9090/apis/weather-api-v1.0/revoke-api-key" \
  -H "Content-Type: application/json" \
  -u "username:password" \
  -d '{
    "api_key": "apip_4f3c2e1d5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d"
  }'
```

**Using JWT Authentication:**
```bash
curl -X POST "http://localhost:9090/apis/weather-api-v1.0/revoke-api-key" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..." \
  -d '{
    "api_key": "apip_4f3c2e1d5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d"
  }'
```

#### Successful Response (200 OK)

```json
{
  "status": "success",
  "message": "API key revoked successfully"
}
```

**Note**: Once revoked, an API key cannot be restored. Generate a new API key if needed.

### Error Responses

All API key management endpoints may return the following error responses:

#### 400 Bad Request
```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "Invalid configuration (validation failed)",
    "details": "API key name cannot be empty"
  }
}
```

#### 404 Not Found
```json
{
  "error": {
    "code": "NOT_FOUND",
    "message": "API configuration not found",
    "details": "API configuration handle 'weather-api-v1.0' not found"
  }
}
```

#### 500 Internal Server Error
```json
{
  "error": {
    "code": "INTERNAL_ERROR",
    "message": "Internal server error",
    "details": "Failed to generate API key due to system error"
  }
}
```

### API Key Format

All generated API keys follow a consistent format:
- **Prefix**: `apip_` (API Platform identifier)
- **Length**: 64 hexadecimal characters after the prefix
- **Total Length**: 69 characters
- **Example**: `apip_4f3c2e1d5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b1c2d`

### Best Practices

1. **Secure Storage**: Store API keys securely and never expose them in client-side code or version control
2. **Regular Rotation**: Rotate API keys periodically for enhanced security
3. **Proper Naming**: Use descriptive names for API keys to identify their purpose
4. **Expiration**: Set appropriate expiration times based on your security requirements
5. **Immediate Revocation**: Revoke API keys immediately if they are compromised
6. **Environment Separation**: Use different API keys for different environments (development, staging, production)

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
