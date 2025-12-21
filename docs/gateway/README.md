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
- **Ports**: 8080 (HTTP traffic), 8443 (HTTPS traffic)
- **Admin Port**: 9901 (Envoy admin interface)

### Policy Engine
- **Purpose**: External processor service that integrates with Envoy via ext_proc filter for flexible HTTP request/response processing through configurable policies
- **Technology**: Go, gRPC, ext_proc, xDS, CEL (Common Expression Language)
- **Port**: 9002 (Admin API)

### Policy Builder
- **Purpose**: Build-time tooling that discovers, validates, and compiles custom policy implementations into the Policy Engine binary
- **Technology**: Go, Docker
- **Distribution**: Docker image containing Policy Engine source code and build tooling

### CLI Tool (ap)
- **Purpose**: Command-line interface for managing gateways, APIs, and MCP proxies
- **Features**: 
  - Gateway management (add, list, remove, health check)
  - API lifecycle management (apply, list, get, delete)
  - MCP proxy management (generate, list, get, delete)
  - Configuration stored in: `~/.wso2ap/config.yaml`
- **Authentication**: Supports Basic Auth (via environment variables) and OAuth2 tokens

**Adding a Gateway:**
```bash
# Add gateway with OAuth2 token
ap gateway add --display-name dev --server http://localhost:9090 --token <TOKEN>

# Or use Basic Auth via environment variables
export WSO2AP_GW_USERNAME=admin
export WSO2AP_GW_PASSWORD=admin
ap gateway add --display-name dev --server http://localhost:9090

# Set as current gateway
ap gateway use --display-name dev
```

**Managing Gateways:**
```bash
# List all configured gateways
ap gateway list

# Show current gateway
ap gateway current

# Check gateway health
ap gateway health

# Remove a gateway
ap gateway remove --display-name dev
```

## Quick Start

### Using Docker Compose (Recommended)

```bash
## Prerequisites

A Docker-compatible container runtime such as:

- Docker Desktop (Windows / macOS)
- Rancher Desktop (Windows / macOS)
- Colima (macOS)
- Docker Engine + Compose plugin (Linux)

Ensure `docker` and `docker compose` commands are available.

    docker --version
    docker compose version
```

```bash
# Download distribution.
wget https://github.com/wso2/api-platform/releases/download/gateway-v0.1.0/gateway-v0.1.0.zip

# Unzip the downloaded distribution.
unzip gateway-v0.1.0.zip


# Start the complete stack
cd gateway-v0.1.0/
docker compose up -d

# Verify gateway controller is running
curl http://localhost:9090/health

# Or use the CLI (requires gateway to be added first)
ap gateway health

# Deploy an API configuration
curl -X POST http://localhost:9090/apis \
  -H "Content-Type: application/yaml" \
  --data-binary @- <<'EOF'
version: api-platform.wso2.com/v1
kind: http/rest
spec:
  name: Weather-API
  version: v1.0
  context: /weather/$version
  upstream:
    main:
      url: http://sample-backend:5000/api/v2
  operations:
    - method: GET
      path: /{country_code}/{city}
      policies:
        - name: ModifyHeaders
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


# Test routing through the gateway
curl http://localhost:8080/weather/v1.0/us/seattle
curl https://localhost:8443/weather/v1.0/us/seattle -k

# List all deployed APIs using CLI
ap gateway api list

# Get a specific API by name and version
ap gateway api get --display-name "Weather-API" --version v1.0 --format yaml

# Or get by ID
ap gateway api get --id <api-id> --format yaml

# Delete an API
ap gateway api delete --id <api-id> --confirm
```

### Stopping the Gateway

When stopping the gateway, you have two options:

**Option 1: Stop runtime, keep data (persisted APIs and configuration)**
```bash
docker compose down
```
This stops the containers but preserves the `controller-data` volume. When you restart with `docker compose up`, all your API configurations will be restored.

**Option 2: Complete shutdown with data cleanup (fresh start)**
```bash
docker compose down -v
```
This stops containers and removes the `controller-data` volume. Next startup will be a clean slate with no persisted APIs or configuration.

### Building from Source

#### Gateway-Controller

```bash
make build
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

The Gateway-Controller uses a structured configuration system with `GATEWAY_` prefix for environment variables:

- `GATEWAY_STORAGE_MODE`: Storage mode ("memory-only" or "persistent") - default: memory-only
- `GATEWAY_STORAGE_DATABASE_PATH`: Path to bbolt database file - default: /data/gateway-controller.db

For complete configuration options, see [Gateway-Controller Configuration](gateway-controller/README.md#configuration).

#### Router Environment Variables

- `XDS_SERVER_HOST`: Gateway-Controller xDS endpoint - default: gateway-controller:18000

## Documentation

- [Gateway Controller README](../../gateway/gateway-controller/README.md) - Detailed controller documentation
- [Router README](../../gateway/router/README.md) - Envoy configuration details
- [Gateway Controller API Specification](../../gateway/gateway-controller/api/openapi.yaml) - OpenAPI 3.0 spec
- [Quickstart Guide](quick-start-guide.md) - Step-by-step guide

## Examples

See [examples/](../../gateway/examples/) directory for sample API configurations.
