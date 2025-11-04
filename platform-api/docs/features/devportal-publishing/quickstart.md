# Quickstart: API Publishing to Developer Portal

## Overview

This guide provides step-by-step instructions for testing the API publishing to developer portal feature, including configuration, verification commands, and troubleshooting.

## Prerequisites

1. **Platform API running**: `http://localhost:8080`
2. **Developer Portal running**: `http://172.17.0.1:3001` (default Docker host network address)
3. **JWT Token**: Valid JWT token with organization claim for authentication
4. **Existing API**: At least one API created in platform-api to publish

## Configuration

### Environment Variables

Set the following environment variables to enable developer portal integration:

```bash
# Enable developer portal integration
export DEVPORTAL_ENABLED=true

# Developer portal base URL (default: 172.17.0.1:3001)
export DEVPORTAL_BASE_URL=172.17.0.1:3001

# Developer portal API key
export DEVPORTAL_API_KEY=dev-api-key-12345

# HTTP timeout in seconds (default: 15)
export DEVPORTAL_TIMEOUT=15
```

### Docker Compose Configuration

If running platform-api via Docker Compose, add to `docker-compose.yaml`:

```yaml
services:
  platform-api:
    environment:
      - DEVPORTAL_ENABLED=true
      - DEVPORTAL_BASE_URL=172.17.0.1:3001
      - DEVPORTAL_API_KEY=dev-api-key-12345
      - DEVPORTAL_TIMEOUT=15
```

## Verification Steps

### Step 1: Verify Platform API is Running

```bash
curl -X GET http://localhost:8080/health
```

**Expected Response**:
```json
{
  "status": "healthy"
}
```

---

### Step 2: Create an Organization (if not exists)

```bash
curl -X POST http://localhost:8080/api/v1/organizations \
  -H "Content-Type: application/json" \
  -d '{
    "id": "org-12345",
    "name": "test-org",
    "displayName": "Test Organization",
    "description": "Test organization for devportal publishing"
  }'
```

**Expected Response**: `201 Created`
```json
{
  "id": "org-12345",
  "name": "test-org",
  "displayName": "Test Organization",
  "handle": "test-org",
  "createdAt": "2025-10-29T10:00:00Z"
}
```

**Note**: If developer portal is enabled, this will automatically:
1. Create the organization in developer portal
2. Create a default "unlimited" subscription policy for the organization

---

### Step 3: Create an API in Platform API

```bash
JWT_TOKEN="your-jwt-token-here"

curl -X POST http://localhost:8080/api/v1/apis \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -d '{
    "name": "Petstore API",
    "handle": "petstore-api",
    "version": "1.0.0",
    "description": "Sample Petstore API",
    "visibility": "PUBLIC"
  }'
```

**Expected Response**: `201 Created`
```json
{
  "id": "123e4567-e89b-12d3-a456-426614174000",
  "name": "Petstore API",
  "handle": "petstore-api",
  "version": "1.0.0",
  "organizationId": "org-12345",
  "createdAt": "2025-10-29T10:15:00Z"
}
```

---

### Step 4: Publish API to Developer Portal

```bash
API_ID="123e4567-e89b-12d3-a456-426614174000"
JWT_TOKEN="your-jwt-token-here"

curl -X POST "http://localhost:8080/api/v1/apis/$API_ID/publish-to-devportal?devportalId=default" \
  -H "Authorization: Bearer $JWT_TOKEN"
```

**Expected Response**: `200 OK`
```json
{
  "message": "API successfully published to developer portal",
  "apiId": "123e4567-e89b-12d3-a456-426614174000",
  "devPortalRefId": "dev-api-abc123",
  "publishedAt": "2025-10-29T10:30:00Z"
}
```

---

### Step 5: Verify API in Developer Portal

```bash
ORG_ID="org-12345"
DEVPORTAL_API_KEY="dev-api-key-12345"

curl -X GET "http://172.17.0.1:3001/devportal/organizations/$ORG_ID/apis" \
  -H "x-wso2-api-key: $DEVPORTAL_API_KEY"
```

**Expected Response**: List of APIs including the published one
```json
{
  "count": 1,
  "apis": [
    {
      "id": "dev-api-abc123",
      "name": "Petstore API",
      "handle": "petstore-api",
      "version": "1.0.0",
      "referenceId": "123e4567-e89b-12d3-a456-426614174000",
      "organizationId": "org-12345",
      "subscriptionTier": "unlimited",
      "createdAt": "2025-10-29T10:30:00Z"
    }
  ]
}
```

---

## Error Scenarios

### Scenario 1: Developer Portal Not Configured

```bash
# Unset DEVPORTAL_ENABLED or set to false
unset DEVPORTAL_ENABLED

curl -X POST "http://localhost:8080/api/v1/apis/$API_ID/publish-to-devportal?devportalId=default" \
  -H "Authorization: Bearer $JWT_TOKEN"
```

**Expected Response**: `400 Bad Request`
```json
{
  "error": "Developer portal not configured",
  "code": "DEVPORTAL_DISABLED",
  "message": "Developer portal integration is not enabled. Set DEVPORTAL_ENABLED=true"
}
```

---

### Scenario 2: API Not Found

```bash
INVALID_API_ID="99999999-e89b-12d3-a456-426614174000"

curl -X POST "http://localhost:8080/api/v1/apis/$INVALID_API_ID/publish-to-devportal?devportalId=default" \
  -H "Authorization: Bearer $JWT_TOKEN"
```

**Expected Response**: `404 Not Found`
```json
{
  "error": "API not found",
  "code": "NOT_FOUND",
  "message": "API with ID 99999999-e89b-12d3-a456-426614174000 does not exist"
}
```

---

### Scenario 3: Developer Portal Unavailable

```bash
# Stop developer portal or use invalid URL
export DEVPORTAL_BASE_URL=invalid-host:9999

curl -X POST "http://localhost:8080/api/v1/apis/$API_ID/publish-to-devportal?devportalId=default" \
  -H "Authorization: Bearer $JWT_TOKEN"
```

**Expected Response**: `503 Service Unavailable` (after 3 retries, ~60 seconds total)
```json
{
  "error": "Developer portal unavailable",
  "code": "SERVICE_UNAVAILABLE",
  "message": "Failed to reach developer portal after 3 retry attempts (total 60 seconds)"
}
```

---

### Scenario 4: Organization Not Synchronized

If the organization doesn't exist in developer portal:

```bash
# Create org with devportal disabled
export DEVPORTAL_ENABLED=false
curl -X POST http://localhost:8080/api/v1/organizations -d '{...}'

# Enable devportal and try to publish
export DEVPORTAL_ENABLED=true
curl -X POST "http://localhost:8080/api/v1/apis/$API_ID/publish-to-devportal?devportalId=default" \
  -H "Authorization: Bearer $JWT_TOKEN"
```

**Expected Response**: `400 Bad Request` or `404 Not Found` from developer portal
```json
{
  "error": "Organization not found in developer portal",
  "code": "ORG_NOT_FOUND",
  "message": "Organization org-12345 does not exist in developer portal. Create organization first."
}
```

---

## Troubleshooting

### Check Platform API Logs

```bash
# If running via Docker
docker logs platform-api -f

# If running via binary
tail -f /var/log/platform-api/app.log
```

**Look for**:
- `[DevPortal] Publishing API <id> to developer portal...`
- `[DevPortal] Retry attempt 1/3 for API publishing`
- `[DevPortal] API successfully published with devportal ref ID: <id>`
- `[DevPortal] Failed to publish API after 3 retries: <error>`

---

### Verify Developer Portal Connectivity

```bash
# Test direct connectivity
curl -X GET "http://172.17.0.1:3001/devportal/health" \
  -H "x-wso2-api-key: $DEVPORTAL_API_KEY"
```

**Expected**: `200 OK` with health status

---

### Check Configuration

```bash
# Inside platform-api container or process
env | grep DEVPORTAL
```

**Expected output**:
```
DEVPORTAL_ENABLED=true
DEVPORTAL_BASE_URL=172.17.0.1:3001
DEVPORTAL_API_KEY=dev-api-key-12345
DEVPORTAL_TIMEOUT=15
```

---

## Performance Testing

### Test Retry Logic

```bash
# Stop developer portal
docker stop developer-portal

# Measure time for publish request (should be ~60 seconds)
time curl -X POST "http://localhost:8080/api/v1/apis/$API_ID/publish-to-devportal?devportalId=default" \
  -H "Authorization: Bearer $JWT_TOKEN"
```

**Expected**:
- Total time: ~60 seconds (3 retries × 15 seconds + network overhead)
- Response: `503 Service Unavailable`

---

### Test Successful Publish Time

```bash
# With developer portal running
time curl -X POST "http://localhost:8080/api/v1/apis/$API_ID/publish-to-devportal?devportalId=default" \
  -H "Authorization: Bearer $JWT_TOKEN"
```

**Expected**:
- Total time: < 5 seconds
- Response: `200 OK` with publish details

---
