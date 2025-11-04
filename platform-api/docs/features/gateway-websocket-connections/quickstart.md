# Quickstart Guide: Gateway Event Notification System

## Overview

This guide helps you quickly set up and test the Gateway Event Notification System, which enables real-time communication between the platform API and gateway instances via WebSocket connections.

## Prerequisites

- Platform API server running with TLS/HTTPS enabled
- Go 1.21+ installed (for running test client)
- `wscat` installed (optional, for WebSocket testing)
- `curl` installed (for REST API calls)

## Quick Start Steps

### Step 1: Start the Platform API Server

```bash
cd platform-api/src
go build -o ../bin/platform-api ./cmd/main.go
cd ../bin
./platform-api
```

The WebSocket endpoint will be available at:
```
wss://localhost:8443/api/internal/v1/ws/gateways/connect
```

### Step 2: Register a Gateway

Create a new gateway instance:

```bash
curl -k -X POST https://localhost:8443/api/v1/gateways \
  -H 'Content-Type: application/json' \
  -d '{
    "organizationId": "<your-org-uuid>",
    "name": "test-gateway",
    "displayName": "Test Gateway"
  }'
```

**Response:**
```json
{
  "id": "d1aa71bc-8cb5-4294-8a26-fe1273c28632",
  "name": "test-gateway",
  "displayName": "Test Gateway",
  "organizationId": "<your-org-uuid>",
  "createdAt": "2025-10-19T..."
}
```

Save the gateway `id` for the next steps.

### Step 3: Generate Gateway API Token

Generate an authentication token for the gateway:

```bash
curl -k -X POST https://localhost:8443/api/v1/gateways/<gateway-id>/tokens \
  -H 'Accept: application/json'
```

**Response:**
```json
{
  "id": "1db8b0e4-f237-4aa3-a6f2-e466c878de0f",
  "token": "guDgqzePBJTMD8iElVH-q4_hc3IWZE87PgBqfzS_qPA",
  "createdAt": "2025-10-19T14:00:43Z",
  "message": "New token generated successfully."
}
```

Save the `token` value for connecting via WebSocket.

### Step 4: Connect Gateway via WebSocket

#### Option A: Using the Test Client (Recommended)

The platform includes a test client for easy WebSocket testing:

```bash
cd platform-api
go run test-websocket-client.go -api-key <your-token>
```

**Expected Output:**
```
Connecting to wss://localhost:8443/api/internal/v1/ws/gateways/connect...
✓ Connected successfully!
✓ Received connection ACK:
  Gateway ID: d1aa71bc-8cb5-4294-8a26-fe1273c28632
  Connection ID: 85d759e5-f152-4a39-8cd1-e0923657268a
  Timestamp: 2025-10-19T07:11:04+05:30

Connection established. Waiting for messages...
Press Ctrl+C to disconnect
---
```

#### Option B: Using wscat

If you have `wscat` installed:

```bash
wscat -n -c wss://localhost:8443/api/internal/v1/ws/gateways/connect \
  -H "api-key: <your-token>"
```

**Expected Output:**
```
Connected (press CTRL+C to quit)
< {"type":"connection.ack","gatewayId":"d1aa71bc...","connectionId":"85d759e5...","timestamp":"2025-10-19T07:11:04+05:30"}
```

### Step 5: Trigger an API Deployment Event

With the gateway connected, deploy an API to trigger a real-time event:

```bash
curl -k -X POST 'https://localhost:8443/api/v1/apis/<api-uuid>/deploy-revision?revisionId=<revision-uuid>' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d '[{
    "revisionId": "<revision-uuid>",
    "gatewayId": "<your-gateway-id>",
    "status": "CREATED",
    "vhost": "mg.wso2.com",
    "displayOnDevportal": true
  }]'
```

**Expected Gateway Output (via WebSocket):**
```json
{
  "type": "api.deployed",
  "payload": {
    "apiUuid": "23826f9e-8daf-4638-b295-14898312759f",
    "revisionId": "90d10e1c-8560-5c36-9d5a-124ecaa17485",
    "vhost": "mg.wso2.com",
    "environment": "production"
  },
  "timestamp": "2025-10-19T07:11:07+05:30",
  "correlationId": "16408ddb-0cc9-48bc-a35d-6debb8d90c28"
}
```

## Configuration Options

You can customize WebSocket behavior using environment variables:

```bash
# Maximum concurrent connections (default: 1000)
export WS_MAX_CONNECTIONS=5000

# Heartbeat timeout in seconds (default: 30)
export WS_CONNECTION_TIMEOUT=60

# Connection rate limit per IP per minute (default: 10)
export WS_RATE_LIMIT_PER_MINUTE=20

# Start server with custom configuration
./platform-api
```

## Verification Checklist

Use these tests to verify your setup:

### ✓ Connection Authentication

**Test invalid API key (should fail):**
```bash
go run test-websocket-client.go -api-key "invalid-key"
```

**Expected:** Connection rejected with authentication error

### ✓ Heartbeat Mechanism

**Test connection stability:**
1. Connect gateway using test client
2. Wait 60+ seconds without activity
3. Verify connection remains alive (ping/pong working)

**Expected Output (in test client):**
```
← Received PING from server
→ Sent PONG to server
```

### ✓ Multiple Connections

**Test gateway clustering:**
1. Open 2 terminal windows
2. Connect same gateway ID from both terminals
3. Deploy API
4. Verify both connections receive the event

**Expected:** Both gateways receive identical deployment events

### ✓ Graceful Disconnection

**Test clean shutdown:**
1. Connect gateway
2. Press Ctrl+C in test client
3. Verify clean disconnection message

**Expected:**
```
Interrupt received, closing connection...
Connection closed by server
```

## Troubleshooting

### Connection Refused

**Problem:** Cannot connect to WebSocket endpoint

**Solution:**
- Verify platform API is running: `curl -k https://localhost:8443/health`
- Check TLS certificates are properly configured
- Ensure port 8443 is not blocked by firewall

### Authentication Failures

**Problem:** Connection rejected with 401 Unauthorized

**Solution:**
- Verify API token is valid and not expired
- Regenerate token: `curl -k -X POST https://localhost:8443/api/v1/gateways/<gateway-id>/tokens`
- Check gateway ID exists in the system

### No Events Received

**Problem:** Gateway connected but doesn't receive deployment events

**Solution:**
- Verify gateway ID in deployment request matches connected gateway
- Check platform API logs for event delivery errors
- Ensure API and revision UUIDs are valid

### Connection Drops

**Problem:** Connection closes unexpectedly

**Solution:**
- Check network stability
- Verify heartbeat timeout configuration is appropriate
- Review platform API logs for disconnection reasons
- Ensure gateway sends pong responses to ping frames

### Rate Limiting

**Problem:** Connection rejected with "rate limit exceeded"

**Solution:**
- Wait 1 minute before retrying
- Adjust `WS_RATE_LIMIT_PER_MINUTE` if legitimate high connection rate
- Check for connection flood from your IP address

## Test Client Usage

The included test client (`test-websocket-client.go`) supports these options:

```bash
# Basic connection
go run test-websocket-client.go -api-key <token>

# Custom WebSocket URL
go run test-websocket-client.go -api-key <token> -url wss://example.com/ws/connect

# Help
go run test-websocket-client.go -h
```

**Client Features:**
- Automatic TLS certificate handling (accepts self-signed certs)
- Heartbeat ping/pong logging
- Connection acknowledgment display
- Event payload pretty-printing
- Graceful shutdown with Ctrl+C

## Event Message Format

All events received by gateways follow this structure:

```json
{
  "type": "<event-type>",
  "payload": { ... },
  "timestamp": "<RFC3339-timestamp>",
  "correlationId": "<uuid>"
}
```

**Current Event Types:**
- `connection.ack` - Connection successfully established
- `api.deployed` - API deployed to gateway

**Future Event Types (planned):**
- `api.undeployed` - API removed from gateway
- `gateway.config.updated` - Configuration changes
- `api.lifecycle.changed` - API state transitions

## Next Steps

After completing this quickstart:

1. **Implement Gateway Logic** - Build your gateway's event handler to process deployment events
2. **Handle API YAML Retrieval** - Fetch full API configuration from `/api/internal/v1/api-runtime-artifacts/{apiId}`
3. **Add Reconnection Logic** - Implement exponential backoff reconnection in your gateway
4. **Monitor Connection Health** - Track heartbeat timestamps and connection status
5. **Test Failure Scenarios** - Simulate network failures, server restarts, and authentication issues

## Additional Resources

- **Implementation Notes:** `platform-api/spec/impls/gateway-websocket-events/gateway-websocket-events.md`
- **Feature Overview:** `README.md` (in this directory)
- **Feature Specification:** `spec.md` (in this directory)
- **API Documentation:** `platform-api/src/resources/openapi.yaml`

## Support

For issues or questions:
- Review platform API logs: Check for connection and event delivery errors
- Verify configuration: Double-check environment variables and TLS setup
- Test isolation: Use the included test client to rule out gateway implementation issues