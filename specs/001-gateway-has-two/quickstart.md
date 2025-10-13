# Quickstart Guide: Gateway with Controller and Router

**Date**: 2025-10-11
**Phase**: 1 - Design & Contracts
**Purpose**: Provide a quick start guide for developers to get the Gateway system running locally

## Overview

This guide walks you through:
1. Setting up the Gateway system using Docker Compose
2. Deploying your first API configuration
3. Testing the API through the Router
4. Managing API configurations (update, query, delete)

**Time to Complete**: ~10 minutes

---

## Prerequisites

- Docker and Docker Compose installed
- Basic familiarity with REST APIs and HTTP
- curl or similar HTTP client (or Postman)

---

## Step 1: Start the Gateway System

The Gateway system includes three components:
- **Gateway-Controller**: xDS server and configuration API (port 9090)
- **Router**: Envoy Proxy for routing traffic (port 8080)
- **Sample Backend**: Mock weather service for testing (port 3000)

### Start with Docker Compose

```bash
cd gateway/
docker compose up -d
```

This starts all three components. Wait ~10 seconds for services to initialize.

### Verify Services are Running

```bash
# Check Gateway-Controller health
curl http://localhost:9090/health

# Expected response:
# {"status":"healthy","timestamp":"2025-10-11T10:30:00Z"}

# Check Router is running (should return 404 - no routes configured yet)
curl http://localhost:8080/

# Expected response:
# {"error":"no route configured"}
```

---

## Step 2: Deploy Your First API Configuration

Create a file `weather-api.yaml` with the following content:

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
data:
  name: Weather API
  version: v1.0
  context: /weather
  upstream:
    - url: http://sample-backend:3000/api/v2
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: POST
      path: /{country_code}/{city}
```

### Submit the Configuration

```bash
curl -X POST http://localhost:9090/apis \
  -H "Content-Type: application/yaml" \
  --data-binary @weather-api.yaml
```

**Expected Response:**
```json
{
  "status": "success",
  "message": "API configuration created successfully",
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "created_at": "2025-10-11T10:35:00Z"
}
```

**Save the `id` value** - you'll need it for updates and deletes.

### What Just Happened?

1. Gateway-Controller validated your configuration
2. Stored it in the internal database (bbolt)
3. Translated it to Envoy xDS resources (Listener, Route, Cluster)
4. Pushed the configuration to the Router via xDS protocol
5. Router is now ready to route traffic for `/weather/*` endpoints

---

## Step 3: Test the API Through the Router

Wait ~5 seconds for the Router to apply the configuration, then test:

### GET Request

```bash
curl http://localhost:8080/weather/US/Seattle
```

**Expected Response** (from sample backend):
```json
{
  "country": "US",
  "city": "Seattle",
  "temperature": 15,
  "conditions": "Cloudy",
  "timestamp": "2025-10-11T10:36:00Z"
}
```

### POST Request

```bash
curl -X POST http://localhost:8080/weather/US/Seattle \
  -H "Content-Type: application/json" \
  -d '{"alert":"Rain expected"}'
```

**Expected Response:**
```json
{
  "status": "alert received",
  "country": "US",
  "city": "Seattle",
  "message": "Rain expected"
}
```

### Path Rewriting Verification

Notice that you called `/weather/US/Seattle`, but the sample backend received the request at `/api/v2/US/Seattle`. The Router automatically prepended the `/api/v2` path from the upstream URL configuration.

---

## Step 4: Query Deployed API Configurations

### List All APIs

```bash
curl http://localhost:9090/apis
```

**Expected Response:**
```json
{
  "status": "success",
  "count": 1,
  "apis": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "Weather API",
      "version": "v1.0",
      "context": "/weather",
      "status": "deployed",
      "created_at": "2025-10-11T10:35:00Z",
      "updated_at": "2025-10-11T10:35:00Z"
    }
  ]
}
```

### Get Specific API Details

```bash
curl http://localhost:9090/apis/550e8400-e29b-41d4-a716-446655440000
```

**Expected Response:**
```json
{
  "status": "success",
  "api": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "configuration": {
      "version": "api-platform.wso2.com/v1",
      "kind": "http/rest",
      "data": {
        "name": "Weather API",
        "version": "v1.0",
        "context": "/weather",
        "upstream": [
          {"url": "http://sample-backend:3000/api/v2"}
        ],
        "operations": [
          {"method": "GET", "path": "/{country_code}/{city}"},
          {"method": "POST", "path": "/{country_code}/{city}"}
        ]
      }
    },
    "metadata": {
      "status": "deployed",
      "created_at": "2025-10-11T10:35:00Z",
      "updated_at": "2025-10-11T10:35:00Z",
      "deployed_at": "2025-10-11T10:35:05Z"
    }
  }
}
```

---

## Step 5: Update an Existing API Configuration

Let's add a new operation (PUT endpoint) to the Weather API.

Create `weather-api-v2.yaml`:

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
data:
  name: Weather API
  version: v1.0
  context: /weather
  upstream:
    - url: http://sample-backend:3000/api/v2
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: POST
      path: /{country_code}/{city}
    - method: PUT
      path: /{country_code}/{city}     # NEW OPERATION
```

### Submit the Update

```bash
curl -X PUT http://localhost:9090/apis/550e8400-e29b-41d4-a716-446655440000 \
  -H "Content-Type: application/yaml" \
  --data-binary @weather-api-v2.yaml
```

**Expected Response:**
```json
{
  "status": "success",
  "message": "API configuration updated successfully",
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "updated_at": "2025-10-11T10:40:00Z"
}
```

### Test the New Operation

```bash
curl -X PUT http://localhost:8080/weather/US/Seattle \
  -H "Content-Type: application/json" \
  -d '{"temperature":20,"conditions":"Sunny"}'
```

**Expected Response:**
```json
{
  "status": "updated",
  "country": "US",
  "city": "Seattle",
  "temperature": 20,
  "conditions": "Sunny"
}
```

**Important**: Notice that existing GET/POST requests continued to work during the update with no downtime.

---

## Step 6: Delete an API Configuration

When you're done testing, you can remove the API configuration:

```bash
curl -X DELETE http://localhost:9090/apis/550e8400-e29b-41d4-a716-446655440000
```

**Expected Response:**
```json
{
  "status": "success",
  "message": "API configuration deleted successfully",
  "id": "550e8400-e29b-41d4-a716-446655440000"
}
```

### Verify Deletion

Try accessing the API through the Router:

```bash
curl http://localhost:8080/weather/US/Seattle
```

**Expected Response:**
```json
{
  "error": "no route configured"
}
```

The Router immediately stopped routing traffic for this API.

---

## Common Operations

### Deploy Multiple APIs

You can deploy multiple APIs with different contexts:

```bash
# Deploy a second API
cat << EOF | curl -X POST http://localhost:9090/apis \
  -H "Content-Type: application/yaml" \
  --data-binary @-
version: api-platform.wso2.com/v1
kind: http/rest
data:
  name: Traffic API
  version: v1.0
  context: /traffic
  upstream:
    - url: http://sample-backend:3000/traffic
  operations:
    - method: GET
      path: /{city}/status
EOF
```

Now you have two APIs running:
- `http://localhost:8080/weather/*` → Weather API
- `http://localhost:8080/traffic/*` → Traffic API

### Handle Validation Errors

If you submit an invalid configuration, you'll get clear error messages:

```bash
# Invalid: context doesn't start with /
cat << EOF | curl -X POST http://localhost:9090/apis \
  -H "Content-Type: application/yaml" \
  --data-binary @-
version: api-platform.wso2.com/v1
kind: http/rest
data:
  name: Invalid API
  version: v1.0
  context: weather
  upstream:
    - url: http://example.com
  operations:
    - method: GET
      path: /test
EOF
```

**Expected Response:**
```json
{
  "status": "error",
  "message": "Configuration validation failed",
  "errors": [
    {
      "field": "data.context",
      "message": "Context must start with / and cannot end with /"
    }
  ]
}
```

---

## Troubleshooting

### Gateway-Controller Not Responding

```bash
# Check if container is running
docker ps | grep gateway-controller

# Check logs
docker logs gateway-controller

# Restart if needed
docker compose restart gateway-controller
```

### Router Not Routing Traffic

```bash
# Check Router logs
docker logs router

# Verify xDS connection
# Look for "connection established" messages in Router logs
docker logs router | grep xDS
```

### Configuration Not Applying

- **Wait 5 seconds**: xDS updates are not instantaneous
- **Check validation**: Review the response from POST/PUT requests for errors
- **Check logs**: Look at Gateway-Controller logs for translation errors

```bash
docker logs gateway-controller | tail -20
```

---

## Next Steps

1. **Explore the OpenAPI Spec**: See `contracts/gateway-controller-api.yaml` for the complete API reference
2. **Read the Data Model**: See `data-model.md` for detailed information about configuration structure
3. **Review the Implementation Plan**: See `plan.md` for architecture and design decisions
4. **Deploy to Production**: Follow Docker best practices for production deployment (health checks, resource limits, monitoring)

---

## Cleanup

When you're finished testing:

```bash
# Stop all containers
cd gateway/
docker compose down

# Remove volumes (deletes stored configurations)
docker compose down -v
```

---

## Summary

You've learned how to:
- ✅ Start the Gateway system with Docker Compose
- ✅ Deploy API configurations via REST API
- ✅ Route traffic through the Router (Envoy Proxy)
- ✅ Update configurations with zero downtime
- ✅ Query and delete API configurations
- ✅ Handle validation errors

**Key Takeaways:**
- Configuration changes apply in ~5 seconds
- Updates are zero-downtime (in-flight requests complete)
- Validation errors are clear and actionable
- All operations are RESTful and easy to automate

For more details, refer to:
- **API Contract**: `contracts/gateway-controller-api.yaml`
- **Data Model**: `data-model.md`
- **Research Decisions**: `research.md`
- **Full Specification**: `spec.md`
