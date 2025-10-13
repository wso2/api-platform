# API Gateway System

A two-component API gateway system consisting of Gateway-Controller (xDS control plane) and Router (Envoy Proxy data plane).

## Components

### Gateway-Controller
- **Purpose**: xDS control plane that manages API configurations and dynamically configures the Router
- **Technology**: Go, Gin, oapi-codegen, bbolt, go-control-plane, Zap
- **Port**: 9090 (REST API)
- **xDS Port**: 18000 (gRPC)

### Router
- **Purpose**: Envoy Proxy-based data plane that routes HTTP traffic to backend services
- **Technology**: Envoy Proxy 1.35.3
- **Port**: 8080 (HTTP traffic)
- **Admin Port**: 9901 (Envoy admin interface)

## Quick Start

### Using Docker Compose (Recommended)

```bash
# Start the complete stack
docker compose up -d

# Verify services are running
curl http://localhost:9090/health

# Deploy an API configuration
curl -X POST http://localhost:9090/apis \
  -H "Content-Type: application/yaml" \
  --data-binary @examples/weather-api.yaml

# Test routing through the gateway
curl http://localhost:8080/weather/US/Seattle
```

### Building from Source

#### Gateway-Controller

```bash
cd gateway-controller/
make generate  # Generate API server code from OpenAPI spec
make build     # Build binary
make run       # Run locally
```

#### Router

```bash
cd router/
make docker    # Build Docker image
```

## Architecture

```
User → Gateway-Controller (REST API)
         ↓ (validates & persists config)
         ↓ (translates to xDS)
         ↓
       Router (Envoy Proxy) → Backend Services
```

### Data Flow

1. User submits API configuration (YAML/JSON) to Gateway-Controller
2. Gateway-Controller validates, persists to bbolt database, and updates in-memory maps
3. Gateway-Controller translates configuration to Envoy xDS resources
4. Gateway-Controller pushes xDS snapshot to Router via gRPC
5. Router applies configuration and starts routing traffic

## API Configuration Format

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
data:
  name: Weather API
  version: v1.0
  context: /weather
  upstream:
    - url: https://api.weather.com/api/v2
  operations:
    - method: GET
      path: /{country_code}/{city}
    - method: POST
      path: /{country_code}/{city}
```

## Features

- **Full CRUD Lifecycle**: Create, update, delete, and query API configurations
- **Zero-Downtime Updates**: Configuration changes apply without dropping connections
- **Validation**: Comprehensive validation with structured error messages
- **Persistence**: Configurations stored in embedded bbolt database
- **Observability**: Structured logging with configurable levels (debug, info, warn, error)
- **Resilience**: Router waits for Controller with exponential backoff at startup

## Development

### Prerequisites

- Go 1.25.1+
- Docker and Docker Compose
- Make

### Testing

```bash
# Run unit tests
cd gateway-controller/
make test

# Run integration tests
make test
```

### Configuration

#### Gateway-Controller Environment Variables

The Gateway-Controller uses a structured configuration system with `GC_` prefix for environment variables:

- `GC_STORAGE_MODE`: Storage mode ("memory-only" or "persistent") - default: memory-only
- `GC_STORAGE_DATABASE_PATH`: Path to bbolt database file - default: /data/gateway-controller.db

For complete configuration options, see [Gateway-Controller Configuration](gateway-controller/README.md#configuration).

#### Router Environment Variables

- `XDS_SERVER_HOST`: Gateway-Controller xDS endpoint - default: gateway-controller:18000

## Documentation

- [Gateway-Controller README](gateway-controller/README.md) - Detailed controller documentation
- [Router README](router/README.md) - Envoy configuration details
- [API Specification](gateway-controller/api/openapi.yaml) - OpenAPI 3.0 spec
- [Quickstart Guide](../specs/001-gateway-has-two/quickstart.md) - Step-by-step guide
- [Data Model](../specs/001-gateway-has-two/data-model.md) - Configuration structure
- [Implementation Plan](../specs/001-gateway-has-two/plan.md) - Architecture and design decisions

## Examples

See [examples/](examples/) directory for sample API configurations.

## Contributing

Please read the specification documents in `specs/001-gateway-has-two/` before contributing.

## License

Copyright WSO2. All rights reserved.
