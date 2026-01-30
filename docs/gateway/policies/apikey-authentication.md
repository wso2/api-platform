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

The gateway controller provides REST APIs to manage API keys for APIs that use the API Key Authentication policy. These endpoints allow you to generate, inject, update, view, regenerate, and revoke API keys programmatically.

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
curl -X POST "http://localhost:9090/apis/weather-api-v1.0/api-keys" \
  -H "Content-Type: application/json" \
  -u "username:password" \
  -d '{"name": "production-key"}'
```

#### JWT Authentication Example

```bash
curl -X POST "http://localhost:9090/apis/weather-api-v1.0/api-keys" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <JWT_TOKEN>" \
  -d '{"name": "production-key"}'
```

### Generate API Key

Generate a new API key for a specific API.

**Endpoint**: `POST /apis/{id}/api-keys`

#### Request Parameters

| Parameter | Type | Location | Required | Description |
|-----------|------|----------|----------|-------------|
| `id` | string | path | Yes | Unique public identifier of the API (e.g., `weather-api-v1.0`) |

#### Request Body

```json
{
  "displayName": "weather-api-key",
  "expires_in": {
    "duration": 30,
    "unit": "days"
  }
}
```

**Request Body Schema:**

| Field                | Type | Required | Description |
|----------------------|------|----------|-------------|
| `displayName`               | string | No | Custom name for the API key. If not provided, a default name will be generated |
| `expires_at`         | string (ISO 8601) | No | Specific expiration timestamp for the API key. If both `expires_in` and `expires_at` are provided, `expires_at` takes precedence |
| `expires_in`         | object | No | Relative expiration time from creation |
| `expires_in.duration` | integer | Yes (if expiresIn used) | Duration value |
| `expires_in.unit`     | string | Yes (if expiresIn used) | Time unit: `seconds`, `minutes`, `hours`, `days`, `weeks`, `months` |

#### Example Request

**Using Basic Authentication:**
```bash
curl -X POST "http://localhost:9090/apis/weather-api-v1.0/api-keys" \
  -H "Content-Type: application/json" \
  -u "username:password" \
  -d '{
    "displayName": "production-key",
    "expires_in": {
      "duration": 90,
      "unit": "days"
    }
  }'
```

**Using JWT Authentication:**
```bash
curl -X POST "http://localhost:9090/apis/weather-api-v1.0/api-keys" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <JWT_TOKEN>" \
  -d '{
    "displayName": "production-key",
    "expires_in": {
      "duration": 90,
      "unit": "days"
    }
  }'
```

#### Successful Response (201 Created)

```json
{
  "status": "success",
  "message": "API key generated successfully",
  "remaining_api_key_quota": 9,
  "api_key": {
    "name": "production-key",
    "displayName": "production-key",
    "api_key": "apip_<64_hex>_<22_chars>",
    "apiId": "weather-api-v1.0",
    "operations": "[\"*\"]",
    "status": "active",
    "created_at": "2025-12-22T13:02:24.504957558Z",
    "created_by": "john",
    "expires_at": "2025-12-23T13:02:24.504957558Z"
  }
}
```

#### Response Schema

| Field | Type | Description                                    |
|-------|------|------------------------------------------------|
| `status` | string | Operation status (`success`)                   |
| `message` | string | Detailed message of the status                 |
| `remaining_api_key_quota` | integer | Remaining API key quota for the user |
| `api_key.displayName` | string | Name of the generated API key                  |
| `api_key.name` | string | Identifier of the generated API key                  |
| `api_key.apiId` | string | API identifier                                 |
| `api_key.api_key` | string | The actual API key value (starts with `apip_`) |
| `api_key.status` | string | Key status (`active`)                          |
| `api_key.created_at` | string | ISO 8601 timestamp of creation                 |
| `api_key.created_by` | string | User who created the key                       |
| `api_key.expires_at` | string | ISO 8601 expiration timestamp (if set)         |
| `api_key.operations` | string | Allowed operations (currently `["*"]` for all) |

### Inject API Key

This operation uses the same endpoint as [Generate API Key](#generate-api-key) (`POST /apis/{id}/api-keys`). The behavior is determined by the presence of the `api_key` field in the request body: omit `api_key` to generate a system key, or include `api_key` to inject an external key. See the request body examples in each section for the differing payloads.

Inject an externally generated API key for a specific API.

**Endpoint**: `POST /apis/{id}/api-keys`

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
  },
  "api_key": "apip_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
}
```

**Request Body Schema:**

| Field                | Type | Required | Description |
|----------------------|------|----------|-------------|
| `displayName`               | string | No | Custom name for the API key. If not provided, a default name will be generated |
| `name`               | string | No | Identifier of the API key. If not provided, a default identifier will be generated |
| `api_key`               | string | No | The API key value to inject. Injected keys can be externally generated and are not required to use the platform `apip_` prefix; platform-generated keys do use the `apip_` prefix. See [Update API Key](#update-api-key) for the same `api_key` semantics when updating. |
| `expires_at`         | string (ISO 8601) | No | Specific expiration timestamp for the API key. If both `expires_in` and `expires_at` are provided, `expires_at` takes precedence |
| `expires_in`         | object | No | Relative expiration time from creation |
| `expires_in.duration` | integer | Yes (if expiresIn used) | Duration value |
| `expires_in.unit`     | string | Yes (if expiresIn used) | Time unit: `seconds`, `minutes`, `hours`, `days`, `weeks`, `months` |

#### Example Request

**Using Basic Authentication:**
```bash
curl -X POST "http://localhost:9090/apis/weather-api-v1.0/api-keys" \
  -H "Content-Type: application/json" \
  -u "username:password" \
  -d '{
    "displayName": "production-key",
    "api_key": "apip_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    "expires_in": {
      "duration": 90,
      "unit": "days"
    }
  }'
```

**Using JWT Authentication:**
```bash
curl -X POST "http://localhost:9090/apis/weather-api-v1.0/api-keys" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <JWT_TOKEN>" \
  -d '{
    "displayName": "production-key",
    "api_key": "apip_1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef",
    "expires_in": {
      "duration": 90,
      "unit": "days"
    }
  }'
```

#### Successful Response (201 Created)

```json
{
  "status": "success",
  "message": "API key generated successfully",
  "remaining_api_key_quota": 9,
  "api_key": {
    "displayName": "production-key",
    "name": "production-key",
    "apiId": "weather-api-v1.0",
    "operations": "[\"*\"]",
    "status": "active",
    "created_at": "2025-12-22T13:02:24.504957558Z",
    "created_by": "john",
    "expires_at": "2025-12-23T13:02:24.504957558Z"
  }
}
```

#### Response Schema


| Field | Type | Description                                    |
|-------|------|------------------------------------------------|
| `status` | string | Operation status (`success`)                   |
| `message` | string | Detailed message of the status                 |
| `remaining_api_key_quota` | integer | Remaining API key quota for the user |
| `api_key.name` | string | Identifier of the generated API key                  |
| `api_key.displayName` | string | Display name of the generated API key                  |
| `api_key.apiId` | string | API identifier                                 |
| `api_key.api_key` | string | The actual API key value (starts with `apip_`) |
| `api_key.status` | string | Key status (`active`)                          |
| `api_key.created_at` | string | ISO 8601 timestamp of creation                 |
| `api_key.created_by` | string | User who created the key                       |
| `api_key.expires_at` | string | ISO 8601 expiration timestamp (if set)         |
| `api_key.operations` | string | Allowed operations (currently `["*"]` for all) |

### Update API Key

Update an existing API key with a new externally provided API key value and optionally update the display name and expiration settings. This endpoint is useful when you need to migrate an existing external API key into the platform or update a key's value while maintaining its identity and metadata.

**Key Differences from Regenerate:**
- **Update**: Replace the API key with a specific value you provide (for external key migration or synchronization)
- **Regenerate**: Generate a new random API key value automatically (for security rotation)

**Endpoint**: `PUT /apis/{id}/api-keys/{apiKeyName}`

#### Request Parameters

| Parameter | Type | Location | Required | Description |
|-----------|------|----------|----------|-------------|
| `id` | string | path | Yes | Unique public identifier of the API (e.g., `weather-api-v1.0`) |
| `apiKeyName` | string | path | Yes | Name of the API key to update |

#### Request Body

```json
{
  "displayName": "updated-weather-key",
  "api_key": "apip_newvalue1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
  "expires_in": {
    "duration": 60,
    "unit": "days"
  }
}
```

**Request Body Schema:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `displayName` | string | No | Display name for the API key |
| `api_key` | string | Yes | The new API key value to set. Must meet minimum length requirements and can be any format (not restricted to platform-generated format) |
| `expires_at` | string (ISO 8601) | No | Specific expiration timestamp. If both `expires_at` and `expires_in` are provided, `expires_at` takes precedence. Omitting both `expires_at` and `expires_in` clears the key's expiration (no expiry). |
| `expires_in` | object | No | Relative expiration time from now. Omitting both `expires_at` and `expires_in` removes the API key's expiration (UpdateAPIKey clears expiry when `request.ExpiresAt` and `request.ExpiresIn` are both nil). |
| `expires_in.duration` | integer | Yes (if expiresIn used) | Duration value |
| `expires_in.unit` | string | Yes (if expiresIn used) | Time unit: `seconds`, `minutes`, `hours`, `days`, `weeks`, `months` |

#### Example Request

**Using Basic Authentication:**
```bash
curl -X PUT "http://localhost:9090/apis/weather-api-v1.0/api-keys/production-key" \
  -H "Content-Type: application/json" \
  -u "username:password" \
  -d '{
    "displayName": "updated-production-key",
    "api_key": "apip_abc123def456789abc123def456789abc123def456789abc123def456789abc12",
    "expires_in": {
      "duration": 60,
      "unit": "days"
    }
  }'
```

**Using JWT Authentication:**
```bash
curl -X PUT "http://localhost:9090/apis/weather-api-v1.0/api-keys/production-key" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <JWT_TOKEN>" \
  -d '{
    "displayName": "updated-production-key",
    "api_key": "apip_abc123def456789abc123def456789abc123def456789abc123def456789abc12",
    "expires_in": {
      "duration": 60,
      "unit": "days"
    }
  }'
```

#### Successful Response (200 OK)

```json
{
  "status": "success",
  "message": "API key updated successfully",
  "remaining_api_key_quota": 9,
  "api_key": {
    "name": "production-key",
    "displayName": "updated-production-key",
    "api_key": "apip_abc123def456789abc123def456789abc123def456789abc123def456789abc12",
    "apiId": "weather-api-v1.0",
    "operations": "[\"*\"]",
    "status": "active",
    "created_at": "2025-12-22T12:26:47.626109914Z",
    "created_by": "john",
    "updated_at": "2025-12-22T14:30:15.123456789Z",
    "updated_by": "john",
    "expires_at": "2026-02-20T14:30:15.123456789Z"
  }
}
```

#### Response Schema

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Operation status (`success`) |
| `message` | string | Detailed message of the status |
| `remaining_api_key_quota` | integer | Remaining API key quota for the user (unchanged by update) |
| `api_key.name` | string | Identifier of the API key (unchanged) |
| `api_key.displayName` | string | Updated display name of the API key |
| `api_key.api_key` | string | The new API key value |
| `api_key.apiId` | string | API identifier (unchanged) |
| `api_key.status` | string | Key status (`active`) |
| `api_key.created_at` | string | Original ISO 8601 timestamp of creation (unchanged) |
| `api_key.created_by` | string | Original user who created the key (unchanged) |
| `api_key.updated_at` | string | ISO 8601 timestamp of the update |
| `api_key.updated_by` | string | User who updated the key |
| `api_key.expires_at` | string | Updated ISO 8601 expiration timestamp (if provided) |
| `api_key.operations` | string | Allowed operations (currently `["*"]` for all) |

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
  -H "Authorization: Bearer <JWT_TOKEN>"
```

#### Successful Response (200 OK)

```json
{
  "status": "success",
  "totalCount": 2,
  "apiKeys": [
    {
      "name": "test-key",
      "api_key": "apip_3521f3*********",
      "apiId": "weather-api-v1.0",
      "operations": "[\"*\"]",
      "status": "active",
      "created_at": "2025-12-22T13:02:24.504957558Z",
      "created_by": "john",
      "expires_at": "2025-12-23T13:02:24.504957558Z"
    },
    {
      "name": "production-key",
      "api_key": "apip_18dfd4*********",
      "apiId": "weather-api-v1.0",
      "operations": "[\"*\"]",
      "status": "active",
      "created_at": "2025-12-22T13:02:24.504957558Z",
      "created_by": "admin",
      "expires_at": "2026-03-22T13:02:24.504957558Z"
    }
  ]
}
```

#### Response Schema

| Field | Type | Description |
|-------|------|-------------|
| `status` | string | Operation status (`success`) |
| `totalCount` | integer | Total number of active API keys |
| `apiKeys` | array | List of API key objects |

**Note**: For security reasons, the actual API key values are masked in the list response, showing only the first 10 characters followed by asterisks. The full API key value is only returned when generating or regenerating a key.

### Regenerate API Key

Regenerate an existing API key, generating a new key value while maintaining the same name and metadata.
Only the user who created the key can perform this operation.

**Endpoint**: `POST /apis/{id}/api-keys/{apiKeyName}/regenerate`

#### Request Parameters

| Parameter | Type | Location | Required | Description                         |
|-----------|------|----------|----------|-------------------------------------|
| `id` | string | path | Yes | Unique public identifier of the API |
| `apiKeyName` | string | path | Yes | Name of the API key to regenerate   |

#### Request Body

```json
{
  "expires_in": {
    "duration": 60,
    "unit": "days"
  }
}
```

**Request Body Schema:** Same as the generate API key request body, but only expiration settings are typically updated during regeneration.

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
  -H "Authorization: Bearer <JWT_TOKEN>" \
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
  "status": "success",
  "message": "API key generated successfully",
  "remaining_api_key_quota": 9,
  "api_key": {
    "name": "production-key",
    "api_key": "apip_18dfd4da48f276043b32d37_bhuced7y3gfd8r4w8bcf4wg",
    "apiId": "weather-api-v1.0",
    "operations": "[\"*\"]",
    "status": "active",
    "created_at": "2025-12-22T12:26:47.626109914Z",
    "created_by": "thivindu",
    "expires_at": "2026-11-17T12:26:47.626109914Z"
  }
}
```

**Note**: The old API key value becomes invalid immediately after regeneration. Update your applications with the new key value.

### Revoke API Key

Revoke an existing API key, making it permanently invalid for authentication.
The user who created the key or an admin can perform this operation.

**Endpoint**: `DELETE /apis/{id}/api-keys/{apiKeyName}`

#### Request Parameters

| Parameter | Type | Location | Required | Description |
|-----------|------|----------|----------|-------------|
| `id` | string | path | Yes | Unique public identifier of the API |
| `apiKeyName` | string | path | Yes | Name of the API key to revoke |

#### Example Request

**Using Basic Authentication:**
```bash
curl -X DELETE "http://localhost:9090/apis/weather-api-v1.0/api-keys/production-key" \
  -u "username:password"
```

**Using JWT Authentication:**
```bash
curl -X DELETE "http://localhost:9090/apis/weather-api-v1.0/api-keys/production-key" \
  -H "Authorization: Bearer <JWT_TOKEN>"
```

#### Successful Response (200 OK)

```json
{
  "status": "success",
  "message": "API key revoked successfully",
  "remaining_api_key_quota": 9
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

### API Key Quotas

The API key management system includes quota controls to limit the number of API keys a user can create per API. This helps prevent abuse and ensures fair usage of the platform.

#### Key Features:
- **Per-User, Per-API Limits**: Each user has a separate quota for each API
- **Configurable Limits**: Administrators can configure the maximum number of API keys allowed per user per API
- **Quota Tracking**: The system tracks remaining quota and returns this information in API responses
- **Generation vs Regeneration**: Generating a new API key decreases the quota, while regenerating an existing key does not affect the quota
- **Revocation Impact**: Revoking an API key increases the available quota for that user

#### Response Fields:
API key generation and regeneration responses include a `remaining_api_key_quota` field that shows how many additional API keys the user can create for the specific API.

### API Key Format

All generated API keys follow a consistent format:
- **Prefix**: `apip_` (API Platform identifier)
- **Length**: - 64 hexadecimal characters after the prefix + "_" + 22 URL-safe characters after the prefix
- **Total Length**: 92 characters
- **Example**: `apip_b9abae64a955aded2eb700aff88235ce3f7e6a8ca0f2f52ba31f73bcbb960360_jh~cPInvccQ09goMO5-4mQ`

### API Key Security

The platform implements comprehensive security measures for API key management:

#### Secure Hashing
API keys are securely hashed before being stored in the database using configurable cryptographic algorithms:

- **SHA-256**: Fast and secure hashing with salt
- **bcrypt**: Adaptive hashing with configurable cost factor  
- **Argon2id**: Memory-hard hashing algorithm resistant to GPU attacks

The hashing algorithm can be configured by administrators. If no algorithm is specified, SHA-256 is used by default.

#### Masked Display
For security reasons, API keys are masked when displayed in list operations:
- Only the first 10 characters are shown (e.g., `apip_3521f3*********`)
- Full API key values are only returned during generation and regeneration
- This prevents accidental exposure in logs, screenshots, or shared screens

#### Secure Storage
- API keys are never stored in plain text
- Only hashed values are persisted to the database
- The system supports migration between different hashing algorithms
- Keys are validated using constant-time comparison to prevent timing attacks

#### Access Control
- Users can only manage API keys they created
- Administrators have access to all API keys for management purposes
- API key operations require proper authentication (Basic Auth or JWT)
- All operations are logged for audit purposes

### Best Practices

1. **Secure Storage**: Store API keys securely and never expose them in client-side code or version control
2. **Regular Regeneration**: Regenerate API keys periodically for enhanced security using the regenerate endpoint
   - Use **Regenerate** for routine security rotation with automatically generated random values
   - Use **Update** when you need to set a specific key value (e.g., migrating from external systems)
3. **Descriptive Naming**: Use descriptive names for API keys to identify their purpose (e.g., "production-app-key", "staging-webhook")
4. **Appropriate Expiration**: Set appropriate expiration times based on your security requirements and usage patterns
5. **Immediate Revocation**: Revoke API keys immediately if they are compromised or no longer needed
6. **Environment Separation**: Use different API keys for different environments (development, staging, production)
7. **Monitor Usage**: Monitor API key usage patterns and set up alerts for unusual activity
8. **Quota Management**: Be aware of your API key quotas and plan key generation accordingly
9. **HTTPS Only**: Always use API keys over HTTPS to prevent interception
10. **Logging Security**: Be cautious with logging - API keys are automatically masked in list responses but should be kept secure in application logs

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
- **Lifecycle**: Keys can be created, injected, updated, regenerated, revoked, and expired through platform APIs
- **Security**: Keys are securely stored and managed by the platform infrastructure in the gateway environment

## Security Considerations

1. **HTTPS Only**: Always use API key authentication over HTTPS to prevent key interception during transmission
2. **Cryptographic Hashing**: API keys are automatically hashed using secure algorithms (SHA-256, bcrypt, or Argon2id) before storage
3. **Key Masking**: API keys are masked in list operations showing only the first 10 characters to prevent accidental exposure
4. **Secure Storage**: Keys are never stored in plain text - only cryptographic hashes are persisted
5. **Regular Regeneration**: Use the regenerate endpoint to regenerate API keys regularly without affecting your quota
6. **Access Control**: Users can only manage their own API keys; administrators have broader access for management purposes
7. **Audit Logging**: All API key operations are logged with correlation IDs for security auditing
8. **Quota Limits**: API key quotas prevent abuse and ensure fair resource usage across users
9. **Timing Attack Protection**: Key validation uses constant-time comparison to prevent timing-based attacks
10. **Query Parameter Caution**: Be careful when using API keys in query parameters as they may appear in access logs or browser history
