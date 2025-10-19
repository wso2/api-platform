# Platform API

Backend service that powers the API Platform portals, gateways, and automation flows.

## Quick Start

```bash
# Build
cd platform-api/src
go build ./cmd/main.go

# Run (TLS with self-signed certificates)
cd platform-api/src
go run ./cmd/main.go

# Verify (create and fetch an organization)
curl -k -X POST https://localhost:8443/api/v1/organizations \
  -H 'Content-Type: application/json' \
  -d '{"handle":"alpha","name":"Alpha"}'
curl -k https://localhost:8443/api/v1/organizations/<uuid>

# Create a project
curl -k -X POST https://localhost:8443/api/v1/projects \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Production APIs",
    "organizationId": "<organization_uuid>"
  }'

# Create an API
curl -k -X POST https://localhost:8443/api/v1/apis \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "Weather API",
    "displayName": "Weather Information API",
    "description": "API for retrieving weather information",
    "context": "/weather",
    "version": "v1.0",
    "projectId": "<project_uuid>",
    "type": "HTTP",
    "transport": ["http", "https"],
    "lifeCycleStatus": "CREATED"
  }'

# Deploy an API revision
curl -k -X POST 'https://localhost:8443/api/v1/apis/<api_uuid>/deploy-revision?revisionId=<revision_uuid>' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -d '[
    {
      "revisionId": "<revision_uuid>",
      "gatewayId": "<gateway_uuid>",
      "status": "CREATED",
      "vhost": "mg.wso2.com",
      "displayOnDevportal": true
    }
  ]'
```

## Documentation

See [spec/](spec/) for product, architecture, design, and implementation documentation.
