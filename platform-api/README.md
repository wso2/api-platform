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
```

## Documentation

See [spec/](spec/) for product, architecture, design, and implementation documentation.
