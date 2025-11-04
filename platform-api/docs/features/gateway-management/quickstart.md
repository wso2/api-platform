# Gateway Management - Quick Start Guide

## Prerequisites

- Docker (for running platform-api)
- curl or similar HTTP client
- A valid JWT token with `organization` claim (see Authentication Setup)

## Environment Setup

### 1. Start Platform API

```bash
# From api-platform/platform-api directory
docker build -t platform-api:local .
docker run -p 8443:8443 platform-api:local
```

The API will be available at `https://localhost:8443`

### 2. Authentication Setup

All gateway management endpoints require a JWT token with an `organization` claim. The organization ID is automatically extracted from this claim.

**JWT Token Format:**
```json
{
  "organization": "123e4567-e89b-12d3-a456-426614174000",
  "sub": "admin@example.com",
  "iat": 1635724800,
  "exp": 1635811200
}
```

Export your JWT token as an environment variable:

```bash
export JWT_TOKEN="eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9..."
```

### 3. Verify Platform API is Running

```bash
curl -k https://localhost:8443/health
```

Expected response: `{"status":"healthy"}`

## Core Workflows

### Workflow 1: Register a New Gateway

**Step 1:** Create a gateway with basic configuration

```bash
curl -k -X POST https://localhost:8443/api/v1/gateways \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "prod-gateway-01",
    "displayName": "Production Gateway 01",
    "description": "Primary production gateway for API traffic",
    "vhost": "api.example.com",
    "isCritical": true,
    "functionalityType": "regular"
  }'
```

**Expected Response (201 Created):**
```json
{
  "id": "987e6543-e21b-45d3-a789-426614174999",
  "organizationId": "123e4567-e89b-12d3-a456-426614174000",
  "name": "prod-gateway-01",
  "displayName": "Production Gateway 01",
  "description": "Primary production gateway for API traffic",
  "vhost": "api.example.com",
  "isCritical": true,
  "functionalityType": "regular",
  "isActive": false,
  "createdAt": "2025-10-26T10:30:00Z",
  "updatedAt": "2025-10-26T10:30:00Z",
  "token": {
    "tokenId": "abc12345-f678-90de-f123-456789abcdef",
    "token": "kR3mF9pL2vX8qN5wY7jK4sT1hU6gB0cD9aE8fI2mN5oP7qR3sT6uV9xY2zA5bC8e",
    "createdAt": "2025-10-26T10:30:00Z"
  }
}
```

**Important:** Save the `token` value - this is the only time it will be shown in plain text!

**Step 2:** Verify the gateway was created

```bash
# Export the gateway ID from the previous response
export GATEWAY_ID="987e6543-e21b-45d3-a789-426614174999"

curl -k https://localhost:8443/api/v1/gateways/$GATEWAY_ID \
  -H "Authorization: Bearer $JWT_TOKEN"
```

### Workflow 2: Gateway Type Classification

Gateway types enable specialized routing and processing:

**Regular Gateway (default):**
```bash
curl -k -X POST https://localhost:8443/api/v1/gateways \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "standard-gateway",
    "displayName": "Standard API Gateway",
    "vhost": "api.example.com",
    "isCritical": false,
    "functionalityType": "regular"
  }'
```

**AI Gateway (for AI workloads):**
```bash
curl -k -X POST https://localhost:8443/api/v1/gateways \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "ai-gateway-01",
    "displayName": "AI Gateway 01",
    "vhost": "ai-api.example.com",
    "isCritical": true,
    "functionalityType": "ai"
  }'
```

**Event Gateway (for event processing):**
```bash
curl -k -X POST https://localhost:8443/api/v1/gateways \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "event-gateway-01",
    "displayName": "Event Gateway 01",
    "vhost": "events.example.com",
    "isCritical": false,
    "functionalityType": "event"
  }'
```

### Workflow 3: List All Gateways

```bash
curl -k https://localhost:8443/api/v1/gateways \
  -H "Authorization: Bearer $JWT_TOKEN"
```

**Expected Response (200 OK):**
```json
{
  "count": 2,
  "list": [
    {
      "id": "987e6543-e21b-45d3-a789-426614174999",
      "organizationId": "123e4567-e89b-12d3-a456-426614174000",
      "name": "prod-gateway-01",
      "displayName": "Production Gateway 01",
      "vhost": "api.example.com",
      "isCritical": true,
      "functionalityType": "regular",
      "isActive": true,
      "createdAt": "2025-10-26T10:30:00Z",
      "updatedAt": "2025-10-26T10:30:00Z"
    },
    {
      "id": "abc12345-f678-90de-f123-456789abcdef",
      "organizationId": "123e4567-e89b-12d3-a456-426614174000",
      "name": "ai-gateway-01",
      "displayName": "AI Gateway 01",
      "vhost": "ai-api.example.com",
      "isCritical": true,
      "functionalityType": "ai",
      "isActive": false,
      "createdAt": "2025-10-26T11:00:00Z",
      "updatedAt": "2025-10-26T11:00:00Z"
    }
  ],
  "pagination": {
    "total": 2,
    "offset": 0,
    "limit": 2
  }
}
```

### Workflow 4: Monitor Gateway Status (Lightweight Polling)

For frequent status checks from management portals:

**Get all gateway statuses:**
```bash
curl -k https://localhost:8443/api/v1/status/gateways \
  -H "Authorization: Bearer $JWT_TOKEN"
```

**Get specific gateway status:**
```bash
curl -k "https://localhost:8443/api/v1/status/gateways?gatewayId=$GATEWAY_ID" \
  -H "Authorization: Bearer $JWT_TOKEN"
```

**Expected Response (200 OK):**
```json
{
  "count": 1,
  "list": [
    {
      "id": "987e6543-e21b-45d3-a789-426614174999",
      "name": "prod-gateway-01",
      "isActive": true,
      "isCritical": true,
      "functionalityType": "regular"
    }
  ],
  "pagination": {
    "total": 1,
    "offset": 0,
    "limit": 1
  }
}
```

### Workflow 5: Update Gateway Metadata

```bash
curl -k -X PUT https://localhost:8443/api/v1/gateways/$GATEWAY_ID \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "displayName": "Production Gateway 01 - Updated",
    "description": "Updated description for production gateway",
    "isCritical": false
  }'
```

**Note:** Immutable properties (id, name, organizationId, vhost, functionalityType) cannot be changed.

### Workflow 6: Token Rotation (Zero-Downtime)

Token rotation allows you to generate a new token while keeping the old one active.

**Step 1:** Rotate the token (max 2 active tokens allowed)

```bash
curl -k -X POST https://localhost:8443/api/v1/gateways/$GATEWAY_ID/tokens \
  -H "Authorization: Bearer $JWT_TOKEN"
```

**Expected Response (201 Created):**
```json
{
  "tokenId": "def45678-g901-23hi-j456-789012klmnop",
  "token": "nT8qW2kL5xR9pY3sM7jF1vC6hU4gB0dE9aI8fN2mP5oQ7rT3sV6uX9yZ2aB5cD8f",
  "createdAt": "2025-10-26T14:20:00Z",
  "message": "New token generated successfully. Old token remains active until revoked."
}
```

**Step 2:** Configure the new token in your gateway

Update your gateway configuration file with the new token.

**Step 3:** Verify both tokens work (during rotation period)

Test authentication with both old and new tokens to ensure zero downtime.

**Step 4:** Revoke the old token (after gateway reconfiguration)

```bash
# Export the old token ID
export OLD_TOKEN_ID="abc12345-f678-90de-f123-456789abcdef"

curl -k -X DELETE https://localhost:8443/api/v1/gateways/$GATEWAY_ID/tokens/$OLD_TOKEN_ID \
  -H "Authorization: Bearer $JWT_TOKEN"
```

**Expected Response (204 No Content)**

### Workflow 7: Delete a Gateway

**Delete gateway successfully:**

```bash
curl -k -X DELETE https://localhost:8443/api/v1/gateways/$GATEWAY_ID \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -v
```

**Expected Response (204 No Content)**

**Note:** All associated tokens are automatically deleted (CASCADE).

**Verify gateway deletion (should return 404):**

```bash
curl -k https://localhost:8443/api/v1/gateways/$GATEWAY_ID \
  -H "Authorization: Bearer $JWT_TOKEN"
```

**Expected Response (404 Not Found):**
```json
{
  "code": 404,
  "message": "Not Found",
  "description": "The specified resource does not exist"
}
```

## Testing Scenarios

### Scenario 1: Duplicate Prevention

```bash
# Register first gateway
curl -k -X POST https://localhost:8443/api/v1/gateways \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-gateway",
    "displayName": "Test Gateway",
    "vhost": "test.example.com",
    "isCritical": false,
    "functionalityType": "regular"
  }'

# Attempt duplicate (should fail)
curl -k -X POST https://localhost:8443/api/v1/gateways \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-gateway",
    "displayName": "Duplicate Gateway",
    "vhost": "test2.example.com",
    "isCritical": false,
    "functionalityType": "regular"
  }'
```

**Expected Error (409 Conflict):**
```json
{
  "code": 409,
  "message": "Conflict",
  "description": "gateway with name 'test-gateway' already exists in this organization"
}
```

### Scenario 2: Maximum Tokens Enforcement

```bash
# Rotate once (2 active tokens total)
curl -k -X POST https://localhost:8443/api/v1/gateways/$GATEWAY_ID/tokens \
  -H "Authorization: Bearer $JWT_TOKEN"

# Rotate twice (still 2 active tokens, but different ones)
curl -k -X POST https://localhost:8443/api/v1/gateways/$GATEWAY_ID/tokens \
  -H "Authorization: Bearer $JWT_TOKEN"

# Attempt third rotation without revoking (should fail)
curl -k -X POST https://localhost:8443/api/v1/gateways/$GATEWAY_ID/tokens \
  -H "Authorization: Bearer $JWT_TOKEN"
```

**Expected Error (400 Bad Request):**
```json
{
  "code": 400,
  "message": "Bad Request",
  "description": "maximum 2 active tokens allowed. Revoke old tokens before rotating"
}
```

### Scenario 3: Multi-Organization Isolation

Different organizations can use the same gateway names:

```bash
# Organization A registers "prod-gateway"
export JWT_TOKEN_ORG_A="<token-with-org-a-claim>"
curl -k -X POST https://localhost:8443/api/v1/gateways \
  -H "Authorization: Bearer $JWT_TOKEN_ORG_A" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "prod-gateway",
    "displayName": "Org A Production",
    "vhost": "org-a.example.com",
    "isCritical": true,
    "functionalityType": "regular"
  }'

# Organization B registers "prod-gateway" (succeeds)
export JWT_TOKEN_ORG_B="<token-with-org-b-claim>"
curl -k -X POST https://localhost:8443/api/v1/gateways \
  -H "Authorization: Bearer $JWT_TOKEN_ORG_B" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "prod-gateway",
    "displayName": "Org B Production",
    "vhost": "org-b.example.com",
    "isCritical": true,
    "functionalityType": "regular"
  }'
```

Both registrations succeed because gateway names are unique per organization.

### Scenario 4: Verify Token Cascade Deletion

```bash
# Create gateway
curl -k -X POST https://localhost:8443/api/v1/gateways \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "cascade-test",
    "displayName": "Cascade Test",
    "vhost": "cascade.example.com",
    "isCritical": false,
    "functionalityType": "regular"
  }'

export CASCADE_GW_ID="<gateway-id-from-response>"

# Rotate token to create a second token
curl -k -X POST https://localhost:8443/api/v1/gateways/$CASCADE_GW_ID/tokens \
  -H "Authorization: Bearer $JWT_TOKEN"

# Verify 2 tokens exist
curl -k https://localhost:8443/api/v1/gateways/$CASCADE_GW_ID \
  -H "Authorization: Bearer $JWT_TOKEN"

# Delete gateway
curl -k -X DELETE https://localhost:8443/api/v1/gateways/$CASCADE_GW_ID \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -v

# Verify tokens are also deleted (gateway no longer exists)
curl -k https://localhost:8443/api/v1/gateways/$CASCADE_GW_ID \
  -H "Authorization: Bearer $JWT_TOKEN"

# Expected: HTTP 404 Not Found (gateway and all tokens deleted)
```

### Scenario 5: Invalid Gateway ID Format

```bash
# Use invalid UUID format
curl -k -X DELETE https://localhost:8443/api/v1/gateways/invalid-id \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -v
```

**Expected Response (400 Bad Request)**

### Scenario 6: Unauthorized Access

```bash
# Attempt deletion without JWT token
curl -k -X DELETE https://localhost:8443/api/v1/gateways/$GATEWAY_ID \
  -v
```

**Expected Response (401 Unauthorized)**

## Error Reference

| Status Code | Message | Common Causes |
|-------------|---------|---------------|
| 400 | Bad Request | Missing required fields, invalid UUID format, max tokens reached, invalid vhost |
| 401 | Unauthorized | Missing/invalid JWT token, missing organization claim |
| 404 | Not Found | Gateway not found, wrong organization, token not found |
| 409 | Conflict | Duplicate gateway name within organization, active deployments/connections (future) |
| 500 | Internal Server Error | Database errors, token generation failures |

## Common Error Examples

### Missing Required Fields

```bash
curl -k -X POST https://localhost:8443/api/v1/gateways \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "incomplete-gateway"
  }'
```

**Response (400 Bad Request):**
```json
{
  "code": 400,
  "message": "Bad Request",
  "description": "missing required field: displayName"
}
```

### Invalid Gateway Type

```bash
curl -k -X POST https://localhost:8443/api/v1/gateways \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "invalid-type-gateway",
    "displayName": "Invalid Type",
    "vhost": "test.example.com",
    "isCritical": false,
    "functionalityType": "invalid"
  }'
```

**Response (400 Bad Request):**
```json
{
  "code": 400,
  "message": "Bad Request",
  "description": "functionalityType must be one of: regular, ai, event"
}
```

### Missing Authorization

```bash
curl -k -X POST https://localhost:8443/api/v1/gateways \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test-gateway",
    "displayName": "Test",
    "vhost": "test.example.com",
    "isCritical": false,
    "functionalityType": "regular"
  }'
```

**Response (401 Unauthorized):**
```json
{
  "code": 401,
  "message": "Unauthorized",
  "description": "Authorization header is required"
}
```

## Advanced Topics

### Gateway Connection Status

The `isActive` field indicates real-time WebSocket connection status:

- `false` (default): Gateway registered but not connected
- `true`: Gateway has active WebSocket connection

**Connection lifecycle:**
1. Gateway registered → `isActive: false`
2. Gateway establishes WebSocket connection → `isActive: true`
3. Gateway disconnects → `isActive: false`

**Note:** `isActive` is read-only and managed automatically by WebSocket events.

### Gateway Criticality

The `isCritical` flag enables operational monitoring:

- Critical gateways may have different SLA requirements
- Monitoring systems can prioritize alerts for critical gateways
- Useful for capacity planning and incident response

### Virtual Host Configuration

The `vhost` field supports domain-based routing:

```bash
# Different gateways for different domains
curl -k -X POST https://localhost:8443/api/v1/gateways \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "api-gateway",
    "displayName": "API Gateway",
    "vhost": "api.example.com",
    "isCritical": true,
    "functionalityType": "regular"
  }'

curl -k -X POST https://localhost:8443/api/v1/gateways \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "internal-gateway",
    "displayName": "Internal Gateway",
    "vhost": "internal-api.example.com",
    "isCritical": false,
    "functionalityType": "regular"
  }'
```

## Troubleshooting

### Problem: "organization not found" error

**Solution:** Ensure the JWT token contains a valid `organization` claim that matches an existing organization in the database.

### Problem: Token rotation fails with "max tokens" error

**Solution:** Revoke at least one existing token before rotating:

```bash
# List active tokens (implement via API if needed)
# Then revoke old token
curl -k -X DELETE https://localhost:8443/api/v1/gateways/$GATEWAY_ID/tokens/$OLD_TOKEN_ID \
  -H "Authorization: Bearer $JWT_TOKEN"

# Now rotate
curl -k -X POST https://localhost:8443/api/v1/gateways/$GATEWAY_ID/tokens \
  -H "Authorization: Bearer $JWT_TOKEN"
```

### Problem: Gateway deletion fails with 409 Conflict

**Solution:** Check if gateway has active deployments or connections:
1. Undeploy all APIs from the gateway first
2. Close all gateway WebSocket connections
3. Retry deletion after resolving conflicts

**Note:** Safety checks for active deployments and connections are planned for future implementation.

### Problem: Cannot access gateway from another organization

**Solution:** This is expected behavior. Gateways are scoped to organizations. Use a JWT token with the correct organization claim.

## Next Steps

- Review [README.md](./README.md) for complete feature overview
- Check OpenAPI specification at `platform-api/src/resources/openapi.yaml`
- Explore WebSocket connection management in [gateway-websocket-connections](../gateway-websocket-connections)
