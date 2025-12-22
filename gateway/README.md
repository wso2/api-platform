# API Platform Gateway

A complete API gateway system consisting of Gateway-Controller (xDS control plane), Router (Envoy Proxy data plane), Policy Engine (request/response processing), and Policy Builder (policy compilation tooling).

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

### Policy Engine
- **Purpose**: External processor service that integrates with Envoy via ext_proc filter for flexible HTTP request/response processing through configurable policies
- **Technology**: Go, gRPC, ext_proc, xDS, CEL (Common Expression Language)
- **Port**: 9002 (Admin API)

### Policy Builder
- **Purpose**: Build-time tooling that discovers, validates, and compiles custom policy implementations into the Policy Engine binary
- **Technology**: Go, Docker
- **Distribution**: Docker image containing Policy Engine source code and build tooling

## Prerequisites

A Docker-compatible container runtime such as:

- Docker Desktop (Windows / macOS)
- Rancher Desktop (Windows / macOS)
- Colima (macOS)
- Docker Engine + Compose plugin (Linux)

Ensure `docker` and `docker compose` commands are available:

```bash
docker --version
docker compose version
```

For development:
- Go 1.25.1+
- Make

## Quick Start

### Building from Source

Build all gateway component Docker images locally:

```bash
# Build all components (recommended for local development)
make build-local

# Or build individual components
make build-local-controller
make build-local-policy-engine
make build-local-router
make build-local-gateway-builder
```

For multi-architecture builds using buildx:

```bash
make build
```

### Running with Docker Compose

After building the images locally, start the complete stack:

```bash
# Start all services
docker compose up -d

# Verify gateway controller is running
curl http://localhost:9090/health

# View logs
docker compose logs -f
```

### Deploy an API Configuration

```bash
curl -X POST http://localhost:9090/apis \
  -H "Content-Type: application/yaml" \
  --data-binary @- <<'EOF'
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: weather-api-v1.0
spec:
  displayName: Weather-API
  version: v1.0
  context: /weather/$version
  upstream:
    main:
      url: http://sample-backend:5000/api/v2
  operations:
    - method: GET
      path: /{country_code}/{city}
      policies:
        - name: modify-headers
          version: v1.0.0
          params:
            requestHeaders:
              - action: SET
                name: operation-level-req-header
                value: hello
            responseHeaders:
              - action: SET
                name: operation-level-res-header
                value: world
    - method: GET
      path: /alerts/active
EOF
```

### Test Routing

```bash
# HTTP
curl http://localhost:8080/weather/v1.0/us/seattle

# HTTPS
curl https://localhost:8443/weather/v1.0/us/seattle -k
```

### Stopping the Gateway

**Stop runtime, keep data (persisted APIs and configuration):**
```bash
docker compose down
```
This stops the containers but preserves the `controller-data` volume. When you restart with `docker compose up`, all your API configurations will be restored.

**Complete shutdown with data cleanup (fresh start):**
```bash
docker compose down -v
```
This stops containers and removes the `controller-data` volume. Next startup will be a clean slate with no persisted APIs or configuration.

## Build Targets

| Target | Description |
|--------|-------------|
| `make build-local` | Build all components locally (faster) |
| `make build` | Build all components using buildx |
| `make build-local-controller` | Build gateway-controller image |
| `make build-local-policy-engine` | Build policy-engine image |
| `make build-local-router` | Build router image |
| `make build-local-gateway-builder` | Build gateway-builder image |
| `make test` | Run unit tests for all components |
| `make clean` | Clean all build artifacts |

## Integration Tests

The gateway includes end-to-end integration tests using Godog (BDD framework) that validate API deployment, routing, policy enforcement, and service health.

### Running Integration Tests

```bash
# Build coverage-instrumented images and run tests
make test-integration-all

# Or run separately:
make build-coverage    # Build coverage-instrumented images
make test-integration  # Run integration tests
```

### Integration Test Commands

| Command | Description |
|---------|-------------|
| `make test-integration-all` | Build coverage images + run tests |
| `make test-integration` | Run integration tests only |
| `make build-coverage` | Build coverage-instrumented images |

For more details, see [it/README.md](it/README.md).

## Architecture

```
User -> Gateway-Controller (REST API)
         | (validates & persists config)
         | (translates to xDS)
         v
       Router (Envoy Proxy) -> Backend Services
```

### Data Flow

1. User submits API configuration (YAML/JSON) to Gateway-Controller
2. Gateway-Controller validates, persists to database, and updates in-memory maps
3. Gateway-Controller translates configuration to Envoy xDS resources
4. Gateway-Controller pushes xDS snapshot to Router via gRPC
5. Router applies configuration and starts routing traffic

## API Configuration Format

```yaml
apiVersion: gateway.api-platform.wso2.com/v1alpha1
kind: RestApi
metadata:
  name: weather-api-v1.0
spec:
  displayName: Weather API
  version: v1.0
  context: /weather/$version
  upstream:
    main:
      url: https://api.weather.com/api/v2
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
- **Persistence**: Configurations stored in embedded database
- **Observability**: Structured logging with configurable levels (debug, info, warn, error)
- **Resilience**: Router waits for Controller with exponential backoff at startup

## Configuration

### Gateway-Controller Environment Variables

The Gateway-Controller uses a structured configuration system with `GATEWAY_` prefix:

- `GATEWAY_GATEWAY__CONTROLLER_STORAGE_TYPE`: Storage type ("sqlite" or "memory")
- `GATEWAY_GATEWAY__CONTROLLER_STORAGE_SQLITE_PATH`: Path to SQLite database file
- `GATEWAY_GATEWAY__CONTROLLER_LOGGING_LEVEL`: Log level (debug, info, warn, error)

For complete configuration options, see [Gateway-Controller README](gateway-controller/README.md#configuration).

### Router Environment Variables

- `XDS_SERVER_HOST`: Gateway-Controller hostname
- `XDS_SERVER_PORT`: Gateway-Controller xDS port (default: 18000)

## Documentation

- [Gateway-Controller README](gateway-controller/README.md) - Detailed controller documentation
- [Router README](router/README.md) - Envoy configuration details
- [Integration Tests](it/README.md) - Integration test guide
- [API Specification](gateway-controller/api/openapi.yaml) - OpenAPI 3.0 spec

## Examples

See [examples/](examples/) directory for sample API configurations.
