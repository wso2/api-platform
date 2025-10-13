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

The Gateway-Controller supports configuration via:
1. **Configuration file** (YAML)
2. **Environment variables** (prefix: `GC_`)
3. **Command-line flags**

Priority: Environment variables > Config file > Defaults

### Configuration File

Create a `config.yaml` file (default location: `/etc/gateway-controller/config.yaml`):

```yaml
# Server configuration
server:
  api_port: 9090          # REST API port
  xds_port: 18000         # xDS gRPC server port
  shutdown_timeout: 15s   # Graceful shutdown timeout

# Storage configuration
storage:
  mode: persistent        # "persistent" or "memory-only"
  database_path: /data/gateway-controller.db

# Router (Envoy) configuration
router:
  access_logs:
    enabled: true         # Enable/disable access logs
    format: json          # "json" or "text"
  listener_port: 8080     # Envoy proxy port

# Logging configuration
logging:
  level: info             # "debug", "info", "warn", "error"
  format: json            # "json" or "console"
```

### Command-Line Flags

```bash
# Specify custom config file location
./bin/controller --config /path/to/config.yaml
```

### Environment Variables

Override any configuration value using the `GC_` prefix:

```bash
# Override server API port
export GC_SERVER_API_PORT=9091

# Set storage mode to memory-only
export GC_STORAGE_MODE=memory-only

# Disable access logs
export GC_ROUTER_ACCESS_LOGS_ENABLED=false

# Set debug logging
export GC_LOGGING_LEVEL=debug

./bin/controller
```

Environment variable naming: `GC_<SECTION>_<KEY>` (uppercase, underscore-separated)

### Configuration Modes

#### Persistent Mode (Default)
Use bbolt database for persistence across restarts:

```yaml
storage:
  mode: persistent
  database_path: /data/gateway-controller.db
```

#### Memory-Only Mode
No persistent storage (useful for testing):

```yaml
storage:
  mode: memory-only
```

### Access Logs

#### JSON Format (Default)
Structured logs for aggregation:

```yaml
router:
  access_logs:
    enabled: true
    format: json
```

Example output:
```json
{"start_time":"2025-10-12T15:45:00Z","method":"GET","path":"/weather/US/Seattle","response_code":200,"duration":125}
```

#### Text Format
Human-readable format:

```yaml
router:
  access_logs:
    enabled: true
    format: text
```

Example output:
```
[2025-10-12T15:45:00Z] "GET /weather/US/Seattle HTTP/1.1" 200 - 512 1024 125 "10.0.0.1" "curl/7.68.0"
```

#### Disable Access Logs
For performance or privacy:

```yaml
router:
  access_logs:
    enabled: false
```

### Example Configurations

See `config/` directory for examples:
- `config.yaml` - Default production configuration
- `config-memory-only.yaml` - Testing/development configuration

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

#### Get API by Name and Version

```bash
GET /apis/{name}/{version}
```

Example:
```bash
GET /apis/Weather%20API/v1.0
```

#### Update API

```bash
PUT /apis/{name}/{version}
Content-Type: application/yaml

<updated configuration>
```

Example:
```bash
PUT /apis/Weather%20API/v1.0
Content-Type: application/yaml

version: api-platform.wso2.com/v1
kind: http/rest
data:
  name: Weather API
  version: v1.0
  context: /weather
  upstream:
    - url: http://api.weather.com/api/v3
  operations:
    - method: GET
      path: /{country}/{city}
```

#### Delete API

```bash
DELETE /apis/{name}/{version}
```

Example:
```bash
DELETE /apis/Weather%20API/v1.0
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
│   │   ├── config.go         # Configuration loader (Koanf)
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
├── config/
│   ├── config.yaml           # Default configuration
│   └── config-memory-only.yaml  # Memory-only example
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
# Using environment variable
GC_LOGGING_LEVEL=debug ./bin/controller

# Or using config file
export GC_LOGGING_LEVEL=debug
./bin/controller
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
