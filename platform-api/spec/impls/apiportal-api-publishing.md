# API Portal API Publishing

## Overview

Enable platform-api to publish and unpublish APIs to an external API portal service, with automatic organization synchronization and support for updating published APIs.

## Features

- **Organization Sync**: Automatically creates organizations in API portal when created in platform-api
- **API Publishing**: Publish APIs to API portal with metadata and OpenAPI definitions
- **API Unpublishing**: Remove APIs from API portal
- **Update Support**: Republish existing APIs to update metadata (unpublish + publish approach)
- **Retry Logic**: 3 retries with 15-second timeout per request
- **Graceful Degradation**: Optional integration - platform-api works even if API portal is disabled

## Configuration

Environment variables in `config/config.go`:
- `APIPORTAL_ENABLED`: Enable/disable API portal integration
- `APIPORTAL_BASE_URL`: API portal base URL (default: `172.17.0.1:3001`)
- `APIPORTAL_API_KEY`: API key for authentication
- `APIPORTAL_TIMEOUT`: Request timeout in seconds (default: 15)

## API Endpoints

### 1. Publish API to API Portal

**Endpoint**: `POST /api/v1/apis/{apiId}/api-portals/publish`

**Description**: Publishes an API to the API portal. If the API already exists, it will be unpublished and republished to update metadata.

**Request**:
```bash
curl --location --request POST 'https://localhost:9243/api/v1/apis/1f01c9b2-440b-4cfe-bf48-f8f168062df4/api-portals/publish' \
--header 'Authorization: Bearer <token>'
```

**Response** (200 OK):
```json
{
    "message": "API published successfully to API portal",
    "apiId": "1f01c9b2-440b-4cfe-bf48-f8f168062df4",
    "apiPortalRefId": "1f01c9b2-440b-4cfe-bf48-f8f168062df4",
    "publishedAt": "2025-10-29T21:25:09.418859918+05:30"
}
```

**Error Responses**:
- `404 Not Found`: API does not exist
- `503 Service Unavailable`: API portal is disabled or unavailable
- `500 Internal Server Error`: Publishing failed after retries

---

### 2. Unpublish API from API Portal

**Endpoint**: `POST /api/v1/apis/{apiId}/api-portals/unpublish`

**Description**: Removes an API from the API portal.

**Request**:
```bash
curl --location --request POST 'https://localhost:9243/api/v1/apis/1f01c9b2-440b-4cfe-bf48-f8f168062df4/api-portals/unpublish' \
--header 'Authorization: Bearer <token>' 
```

**Response** (200 OK):
```json
{
    "message": "API unpublished successfully from API portal",
    "apiId": "1f01c9b2-440b-4cfe-bf48-f8f168062df4",
    "unpublishedAt": "2025-10-29T21:08:59.834784232+05:30"
}
```

**Error Responses**:
- `404 Not Found`: API does not exist
- `503 Service Unavailable`: API portal is disabled or unavailable
- `500 Internal Server Error`: Unpublishing failed after retries

## Implementation Details

### Files Modified

**Configuration**:
- `config/config.go`: Added APIPortal configuration struct

**Client Layer** (`internal/client/apiportal/`):
- `apiportal_client.go`: HTTP client with retry logic for API portal operations
- `dto/organization_request.go`: Organization sync DTOs
- `dto/subscription_policy_request.go`: Subscription policy DTOs
- `dto/api_publish_request.go`: API publish/unpublish DTOs

**Service Layer**:
- `internal/service/organization.go`: Added automatic organization sync on creation
- `internal/service/api.go`: Added PublishAPI and UnpublishAPI methods with update detection

**Handler Layer**:
- `internal/handler/api.go`: Added publish/unpublish endpoints
- `internal/dto/api.go`: Added handler-level request/response DTOs

**Server**:
- `internal/server/server.go`: Wired API portal client to API service

### Key Features

**Automatic Organization Sync**:
- Organizations created in platform-api are automatically synced to API portal
- Default "unlimited" subscription policy is created for each organization

**Update Detection**:
- Before publishing, checks if API already exists in API portal
- If exists: unpublishes first, then publishes (ensures metadata updates)
- If not exists: publishes directly

**Error Handling**:
- APIPortalError type with structured error information
- Automatic retries for transient failures (5xx errors, network issues)
- Graceful degradation when API portal is disabled

## API Portal API Contract

### Organization Creation
- `POST /devportal/organizations`: Create organization
- `POST /devportal/organizations/{orgId}/subscription-policies`: Create subscription policy

### API Publishing
- `GET /devportal/organizations/{orgId}/apis/{apiId}?view=default`: Check if API exists
- `POST /devportal/apis`: Publish API (multipart/form-data with metadata + OpenAPI definition)
- `DELETE /devportal/apis/{apiId}`: Unpublish API

## Testing

**Prerequisites**:
1. API portal service running at configured base URL
2. Valid API key configured
3. Organization already created in platform-api

**Test Sequence**:
1. Create an API in platform-api
2. Publish API using `/api-portals/publish` endpoint
3. Verify API appears in API portal
4. Republish API to test update flow
5. Unpublish API using `/api-portals/unpublish` endpoint
6. Verify API is removed from API portal

## Future Enhancements

- Replace placeholder OpenAPI definitions with actual retrieval from storage
- Support for custom subscription policies (currently hardcoded to "unlimited")
- Database persistence for publish/unpublish events
- Webhook notifications for publish/unpublish events
