# Gateway Implementation Overview

## Summary

Gateway-Controller entry point in `cmd/controller/main.go` initializes storage backend (SQLite or memory), starts xDS gRPC server, and launches Gin REST API. Validation, persistence, and xDS translation layers work together to provide zero-downtime API configuration management.

## Feature Implementations

- [Basic Gateway with Controller](impls/1-basic-gateway-with-controller/spec.md) – Initial gateway implementation with controller, xDS server, and Envoy router integration.
- [Use SQLite](impls/2-use-sqlite/spec.md) – SQLite database integration for persistent API configuration storage.
- [Gateway to Control Plane Registration](impls/gateway-control-plane-registration.md) – Gateway registration with control plane via WebSocket connection with heartbeat monitoring and reconnection.

Each implementation note captures entrypoints, supporting modules, and verification tips for manual or automated checks.

## Quick Reference

## Development Environment Setup

### Prerequisites
- Go 1.25.1 or later
- Docker and Docker Compose
- Make
- oapi-codegen v2 (for API code generation)

### Initial Setup

```bash
# Clone repository
git clone <repo-url>
cd api-platform/gateway

# Install Gateway-Controller dependencies
cd gateway-controller
go mod download

# Generate API server code from OpenAPI spec
make generate

# Build the controller
make build

# Run tests
make test
```

## Project Structure

### Gateway-Controller (Go Service)

```
gateway/gateway-controller/
├── cmd/
│   └── controller/
│       └── main.go              # Application entry point
├── pkg/
│   ├── api/
│   │   ├── generated.go         # Generated from OpenAPI spec (oapi-codegen)
│   │   ├── handlers/            # REST API handler implementations
│   │   └── middleware/          # Logging, correlation, error handling
│   ├── config/
│   │   ├── parser.go            # YAML/JSON parsing
│   │   └── validator.go         # API configuration validation
│   ├── models/
│   │   └── api_config.go        # Data structures
│   ├── storage/
│   │   ├── interface.go         # Storage abstraction
│   │   ├── memory.go            # In-memory maps
│   │   └── bbolt.go             # bbolt persistence implementation
│   ├── xds/
│   │   ├── server.go            # xDS v3 server (SotW)
│   │   ├── snapshot.go          # Snapshot cache management
│   │   └── translator.go        # API config → Envoy config translation
│   └── logger/
│       └── logger.go            # Zap logger setup
├── tests/
│   ├── unit/                    # Unit tests for individual packages
│   └── integration/             # End-to-end API lifecycle tests
├── api/
│   └── openapi.yaml             # OpenAPI 3.0 specification
├── oapi-codegen.yaml            # Code generation configuration
├── Makefile                     # Build, test, docker targets
├── Dockerfile                   # Multi-stage Go build
└── go.mod
```

### Router (Envoy Proxy)

```
gateway/router/
├── config/
│   └── envoy-bootstrap.yaml     # Bootstrap config with xds_cluster
├── Dockerfile                   # Based on envoyproxy/envoy:v1.35.3
├── Makefile                     # Build docker image
└── README.md
```

## Build Commands

### Gateway-Controller

```bash
cd gateway/gateway-controller

# Generate API server code from OpenAPI spec
make generate

# Build binary
make build

# Run unit tests
make test

# Run with race detection
make test-race

# Build Docker image
make docker

# Run locally (development mode)
make run

# Clean build artifacts
make clean

# Show all available targets
make help
```

### Router

```bash
cd gateway/router

# Build Docker image
make docker

# Clean build artifacts
make clean
```

### Full Stack

```bash
cd gateway

# Start all services (Controller + Router)
docker compose up -d

# View logs
docker compose logs -f

# Stop services
docker compose down

# Rebuild and restart
docker compose up -d --build
```

## Development Workflow

### 1. API Contract Changes

When modifying the Gateway-Controller REST API:

```bash
# 1. Edit OpenAPI specification
vim gateway-controller/api/openapi.yaml

# 2. Regenerate Go code
cd gateway-controller
make generate

# 3. Update handler implementations to match new ServerInterface
vim pkg/api/handlers/handlers.go

# 4. Run tests to verify
make test

# 5. Commit both spec and generated code
git add api/openapi.yaml pkg/api/generated.go
git commit -m "Update API contract: <description>"
```

### 2. Adding New API Configuration Fields

```bash
# 1. Update data model
vim pkg/models/api_config.go

# 2. Update validation logic
vim pkg/config/validator.go

# 3. Update xDS translator to handle new fields
vim pkg/xds/translator.go

# 4. Add unit tests
vim tests/unit/validator_test.go
vim tests/unit/translator_test.go

# 5. Run tests
make test
```

### 3. Testing Configuration Changes

```bash
# 1. Start gateway stack
cd gateway
docker compose up -d

# 2. Deploy test API configuration
curl -X POST http://localhost:9090/apis \
  -H "Content-Type: application/json" \
  -d @examples/test-api.yaml

# 3. Verify Router received configuration
curl http://localhost:9901/config_dump | jq '.configs'

# 4. Test API through Router
curl http://localhost:8081/<context-path>

# 5. Check logs
docker compose logs gateway-controller
docker compose logs router
```

## Testing Strategy

### Unit Tests

Located in `gateway-controller/tests/unit/`:

- **Parser Tests**: Validate YAML/JSON parsing
- **Validator Tests**: Test configuration validation rules (table-driven tests)
- **Translator Tests**: Verify xDS resource generation from API configs
- **Storage Tests**: Test in-memory and bbolt operations

```bash
# Run all unit tests
make test

# Run specific package tests
go test -v ./pkg/config/... -cover

# Run with coverage report
go test -v ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Integration Tests

Located in `gateway-controller/tests/integration/`:

- **API Tests**: Full REST API lifecycle (create, read, update, delete)
- **xDS Tests**: Verify xDS server behavior and snapshot updates
- **End-to-End Tests**: Deploy API → verify Router routes traffic correctly

```bash
# Start test environment
docker compose -f docker compose.test.yaml up -d

# Run integration tests
make test-integration

# Cleanup test environment
docker compose -f docker compose.test.yaml down
```

### Manual Testing Checklist

See [Testing Checklist](impls/testing-checklist.md) for comprehensive manual test scenarios.

## Data Flow Architecture

### Startup Sequence

```
1. Gateway-Controller starts
   ├── Load configuration from bbolt database
   ├── Populate in-memory maps
   ├── Generate initial xDS snapshot
   └── Start REST API server (port 9090)
   └── Start xDS gRPC server (port 18000)

2. Router starts
   ├── Read bootstrap config (envoy-bootstrap.yaml)
   ├── Connect to xDS server (Gateway-Controller:18000)
   ├── Wait for initial configuration (indefinite retry with backoff)
   └── Start serving traffic (port 8081)
```

### Runtime API Configuration Update

```
1. User submits API configuration
   ├── POST /apis (REST API)
   └── Gin handler receives request

2. Gateway-Controller processes request
   ├── Parse YAML/JSON
   ├── Validate configuration structure
   ├── Generate composite key: {name}/{version}
   ├── Update in-memory maps
   ├── Persist to bbolt database (atomic transaction)
   └── Log audit event

3. xDS snapshot update
   ├── Read ALL configs from in-memory maps
   ├── Translate to Envoy resources (LDS, RDS, CDS)
   ├── Increment snapshot version
   ├── Update xDS cache (SotW approach)
   └── Push to Router

4. Router applies configuration
   ├── Receive xDS update via gRPC stream
   ├── Validate new configuration
   ├── Gracefully drain in-flight connections
   ├── Apply new routing rules
   └── Emit access logs for new requests
```

## Configuration Management

### Environment Variables

Gateway-Controller supports the following environment variables:

```bash
# Log level (debug, info, warn, error)
LOG_LEVEL=info

# REST API server port
API_PORT=9090

# xDS gRPC server port
XDS_PORT=18000

# Database file path
DB_PATH=/data/gateway-controller.db

# Configuration file (optional)
CONFIG_FILE=/etc/gateway-controller/config.yaml
```

### Configuration File

Example `config.yaml`:

```yaml
server:
  api_port: 9090
  xds_port: 18000

storage:
  type: bbolt
  path: /data/gateway-controller.db

logging:
  level: info
  format: json

xds:
  node_id: gateway-router
  snapshot_cache_size: 100
```

## Deployment

### Docker Compose (Local/Testing)

```bash
cd gateway
docker compose up -d
```

### Docker (Manual)

```bash
# Build images
cd gateway-controller && docker build -t gateway-controller:latest .
cd ../router && docker build -t gateway-router:latest .

# Run Gateway-Controller
docker run -d \
  --name gateway-controller \
  -p 9090:9090 \
  -p 18000:18000 \
  -v $(pwd)/data:/data \
  -e LOG_LEVEL=info \
  gateway-controller:latest

# Run Router
docker run -d \
  --name gateway-router \
  -p 8081:8081 \
  -p 9901:9901 \
  --link gateway-controller:xds-server \
  gateway-router:latest
```

### Kubernetes (Future)

See `impls/kubernetes-deployment.md` for Kubernetes manifests and deployment guide (coming soon).

## Troubleshooting

### Common Issues

**Issue: Router not receiving configuration from Controller**
```bash
# Check xDS gRPC server is running
curl http://localhost:9090/health

# Check Router admin interface
curl http://localhost:9901/config_dump

# View Controller logs (debug mode)
docker compose exec gateway-controller sh
export LOG_LEVEL=debug
```

**Issue: Configuration validation errors**
```bash
# Enable debug logging to see full validation details
export LOG_LEVEL=debug

# Check error response from API
curl -X POST http://localhost:9090/apis \
  -H "Content-Type: application/json" \
  -d @api-config.yaml -v
```

**Issue: Router returning 404 for configured routes**
```bash
# Verify configuration was applied
curl http://localhost:9901/config_dump | jq '.configs[2].dynamic_active_clusters'

# Check access logs
docker compose logs router | grep "404"

# Verify API configuration matches request
curl http://localhost:9090/apis/<name>/<version>
```

### Debug Mode

Enable comprehensive logging:

```bash
# Gateway-Controller
export LOG_LEVEL=debug

# View xDS payloads in logs
docker compose logs gateway-controller | grep "xds_snapshot"

# View configuration diffs
docker compose logs gateway-controller | grep "config_diff"
```

### Health Checks

```bash
# Gateway-Controller health
curl http://localhost:9090/health

# Router admin interface
curl http://localhost:9901/ready
curl http://localhost:9901/stats

# xDS connection status
curl http://localhost:9901/config_dump | jq '.configs[0].dynamic_active_clusters'
```

## Performance Optimization

### In-Memory Maps

Gateway-Controller uses in-memory maps for fast configuration access:
- Startup: Load all configurations from database to memory
- Runtime: All xDS translations read from in-memory maps
- Updates: Atomic update of both in-memory maps and database

### bbolt Best Practices

```go
// Use read-only transactions for queries
db.View(func(tx *bolt.Tx) error {
    bucket := tx.Bucket([]byte("apis"))
    data := bucket.Get([]byte("PetStore/v1"))
    return nil
})

// Use read-write transactions for updates
db.Update(func(tx *bolt.Tx) error {
    bucket := tx.Bucket([]byte("apis"))
    return bucket.Put([]byte("PetStore/v1"), data)
})
```

### xDS Snapshot Cache

- SotW approach: Complete state in each update
- Snapshot versioning: Monotonically increasing integers
- Cache size: Limit to last 100 snapshots (configurable)

## Security Considerations

### Validation
- Validate all user input before processing
- Sanitize API configuration fields
- Prevent injection attacks in backend URLs

### Logging
- Redact sensitive data (API keys, tokens) from logs
- Use structured logging with appropriate log levels
- Avoid logging full request/response bodies in production

### Access Control
- Optional API Key authentication for control plane
- Secure gateway registration in hybrid mode
- mTLS support between components (future)

---

**Document Version**: 1.0
**Last Updated**: 2025-10-13
**Status**: Active Development
