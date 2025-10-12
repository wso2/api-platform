# Gateway-Controller

The Gateway-Controller is the xDS control plane that manages API configurations and dynamically configures the Router (Envoy Proxy).

## Features

- **REST API**: Submit, update, delete, and query API configurations via HTTP
- **Validation**: Comprehensive validation with field-level error messages
- **Persistence**: Embedded bbolt database for configuration storage
- **In-Memory Cache**: Fast access with thread-safe operations
- **xDS Server**: gRPC server implementing Envoy's State-of-the-World protocol
- **Zero-Downtime Updates**: Configuration changes applied without dropping connections
- **Audit Logging**: Complete audit trail of all configuration changes

## Architecture

```
REST API (Port 9090)
      ↓
  Validation
      ↓
Persistence (bbolt) + In-Memory Cache
      ↓
  xDS Translator
      ↓
xDS gRPC Server (Port 18000)
      ↓
  Router (Envoy)
```

## Building

### Prerequisites

- Go 1.25.1+
- Make

### Build from Source

```bash
# Generate API code from OpenAPI spec
make generate

# Build binary
make build

# Run tests
make test

# Build Docker image
make docker
```

## Running

### Local Development

```bash
# Run with default settings
make run

# Or run the binary directly
./bin/controller
```

### Docker

```bash
docker run -p 9090:9090 -p 18000:18000 \
  -v $(pwd)/data:/data \
  wso2/gateway-controller:latest
```

## Configuration

### Environment Variables

- `LOG_LEVEL`: Log level (debug, info, warn, error) - default: `info`
- `DB_PATH`: Path to bbolt database file - default: `/data/gateway-controller.db`
- `API_PORT`: REST API port - default: `9090`
- `XDS_PORT`: xDS gRPC server port - default: `18000`

### Example

```bash
export LOG_LEVEL=debug
export DB_PATH=./gateway-controller.db
./bin/controller
```

## API Reference

The Gateway-Controller exposes a REST API for managing API configurations.

### Base URL

```
http://localhost:9090
```

### Endpoints

#### Health Check

```bash
GET /health
```

Response:
```json
{
  "status": "healthy",
  "timestamp": "2025-10-12T15:45:00Z"
}
```

#### Create API Configuration

```bash
POST /apis
Content-Type: application/yaml

version: api-platform.wso2.com/v1
kind: http/rest
data:
  name: Weather API
  version: v1.0
  context: /weather
  upstream:
    - url: http://api.weather.com/api/v2
  operations:
    - method: GET
      path: /{country}/{city}
```

Response:
```json
{
  "status": "success",
  "message": "API configuration created successfully",
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "created_at": "2025-10-12T15:45:00Z"
}
```

#### List All APIs

```bash
GET /apis
```

#### Get API by ID

```bash
GET /apis/{id}
```

#### Update API

```bash
PUT /apis/{id}
Content-Type: application/yaml

<updated configuration>
```

#### Delete API

```bash
DELETE /apis/{id}
```

## Data Storage

The Gateway-Controller uses bbolt (embedded key-value store) with the following buckets:

- `apis/` - API configurations
- `audit/` - Audit event logs
- `metadata/` - System metadata

## xDS Protocol

The Gateway-Controller implements Envoy's State-of-the-World (SotW) xDS protocol:

1. Router connects to Gateway-Controller on port 18000
2. Gateway-Controller generates complete xDS snapshot from in-memory configurations
3. On configuration change, new snapshot is created and pushed to Router
4. Router applies configuration gracefully (in-flight requests complete)

## Development

### Project Structure

```
gateway-controller/
├── cmd/
│   └── controller/
│       └── main.go           # Entry point
├── pkg/
│   ├── api/
│   │   ├── generated/        # Generated from OpenAPI spec
│   │   ├── handlers/         # REST API handlers
│   │   └── middleware/       # Logging, error handling
│   ├── config/
│   │   ├── parser.go         # YAML/JSON parsing
│   │   └── validator.go      # Configuration validation
│   ├── models/
│   │   └── api_config.go     # Data structures
│   ├── storage/
│   │   ├── interface.go      # Storage abstraction
│   │   ├── memory.go         # In-memory cache
│   │   └── bbolt.go          # bbolt implementation
│   ├── xds/
│   │   ├── server.go         # xDS gRPC server
│   │   ├── snapshot.go       # Snapshot manager
│   │   └── translator.go     # Config → Envoy translation
│   └── logger/
│       └── logger.go         # Zap logger setup
├── api/
│   └── openapi.yaml          # OpenAPI 3.0 specification
├── oapi-codegen.yaml         # Code generation config
├── Dockerfile
└── Makefile
```

### Code Generation

The REST API is generated from the OpenAPI specification using oapi-codegen:

```bash
make generate
```

This creates `pkg/api/generated/generated.go` with:
- `ServerInterface` - Handler interface
- Request/response types
- Route registration

## Logging

The Gateway-Controller uses structured logging (Zap) with configurable levels.

### Debug Mode

```bash
LOG_LEVEL=debug ./bin/controller
```

Debug logs include:
- Complete API configuration payloads
- xDS snapshot details
- Configuration diffs for updates

## Performance

- Configuration validation: < 1 second
- xDS update push: < 5 seconds
- Supports 100+ API configurations
- Thread-safe concurrent operations

## License

Copyright WSO2. All rights reserved.
