# Gateway-Controller

The Gateway-Controller is the xDS control plane that manages API configurations and dynamically configures the Router (Envoy Proxy).

## Features

- **REST API**: Submit, update, delete, and query API configurations via HTTP
- **Validation**: Comprehensive validation with field-level error messages
- **Persistence**: Embedded SQLite database for configuration storage
- **In-Memory Cache**: Fast access with thread-safe operations
- **xDS Server**: gRPC server implementing Envoy's State-of-the-World protocol
- **Zero-Downtime Updates**: Configuration changes applied without dropping connections

## Architecture

```
REST API (Port 9090)
      ↓
  Validation
      ↓
Persistence (SQLite) + In-Memory Cache
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
2. **Environment variables** (prefix: `GATEWAY_`)
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
  type: sqlite            # "sqlite", "postgres" (future), or "memory"
  sqlite:
    path: ./data/gateway.db  # SQLite database file path

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

Override any configuration value using the `GATEWAY_` prefix:

```bash
# Override server API port
export GATEWAY_SERVER_API_PORT=9091

# Set storage type to memory
export GATEWAY_STORAGE_TYPE=memory

# Override SQLite database path
export GATEWAY_STORAGE_SQLITE_PATH=/custom/path/gateway.db

# Disable access logs
export GATEWAY_ROUTER_ACCESS_LOGS_ENABLED=false

# Set debug logging
export GATEWAY_LOGGING_LEVEL=debug

./bin/controller
```

Environment variable naming: `GATEWAY_<SECTION>_<KEY>` (uppercase, underscore-separated)

### Configuration Modes

#### Persistent Mode with SQLite (Default)
Use SQLite database for persistence across restarts:

```yaml
storage:
  type: sqlite
  sqlite:
    path: ./data/gateway.db
```

#### Memory-Only Mode
No persistent storage (useful for testing):

```yaml
storage:
  type: memory
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

The Gateway-Controller uses SQLite (embedded relational database) for persistent storage of API configurations.

### Database Schema

The SQLite database contains the following table:

- **`deployments`** - Stores API configurations with full lifecycle metadata

#### Table Structure

| Column | Type | Description |
|--------|------|-------------|
| `id` | TEXT (PRIMARY KEY) | Unique UUID identifier |
| `name` | TEXT | API name (indexed for fast lookups) |
| `version` | TEXT | API version (indexed for fast lookups) |
| `context` | TEXT | Base path (e.g., "/weather") |
| `kind` | TEXT | API type ("http/rest", "graphql", etc.) |
| `configuration` | TEXT | Full JSON-serialized API configuration |
| `status` | TEXT | Deployment status ("pending", "deployed", "failed") |
| `created_at` | TIMESTAMP | Record creation timestamp |
| `updated_at` | TIMESTAMP | Last modification timestamp |
| `deployed_at` | TIMESTAMP | Timestamp of successful deployment (NULL if never deployed) |
| `deployed_version` | INTEGER | xDS snapshot version number |

**Unique Constraint**: `(name, version)` - prevents duplicate API versions

### Database Configuration

SQLite is configured with the following settings for optimal performance:

- **Journal Mode**: WAL (Write-Ahead Logging) - enables concurrent reads during writes
- **Busy Timeout**: 5000ms - retries locked database for 5 seconds before failing
- **Synchronous**: NORMAL - balanced durability (faster than FULL, safer than OFF)
- **Cache Size**: 2000 pages (~2MB in-memory cache)
- **Foreign Keys**: ON - enables referential integrity

### Database Files

When using SQLite storage, the following files are created in the data directory:

- `./data/gateway.db` - Main database file
- `./data/gateway.db-wal` - Write-Ahead Log (transactions)
- `./data/gateway.db-shm` - Shared memory for WAL

### Inspecting the Database

You can inspect the SQLite database using the `sqlite3` command-line tool:

```bash
# Connect to the database
sqlite3 ./data/gateway.db

# List all API configurations
SELECT name, version, status FROM deployments;

# View specific API configuration (pretty-print JSON)
SELECT json(configuration) FROM deployments WHERE name = 'Weather API';

# Count configurations by status
SELECT status, COUNT(*) FROM deployments GROUP BY status;

# Check database size and schema
.dbinfo
.schema deployments

# Exit
.quit
```

### Database Backup

To backup the SQLite database:

```bash
# 1. Checkpoint WAL to main database file
sqlite3 ./data/gateway.db "PRAGMA wal_checkpoint(TRUNCATE);"

# 2. Copy database file
cp ./data/gateway.db ./backups/gateway-$(date +%Y%m%d-%H%M%S).db

# 3. Verify backup integrity
sqlite3 ./backups/gateway-*.db "PRAGMA integrity_check;"
```

### Troubleshooting

#### Database is Locked Error

If you encounter "database is locked" errors:

1. **Check for active connections**:
   ```bash
   lsof ./data/gateway.db
   ```

2. **Ensure only one gateway-controller instance is running**:
   SQLite with a single writer connection is designed for single-instance deployments.

3. **Restart the gateway-controller**:
   The controller will fail-fast on startup if the database is locked, with a clear error message.

4. **Force unlock** (DANGER: only if no processes are running):
   ```bash
   rm ./data/gateway.db-wal ./data/gateway.db-shm
   sqlite3 ./data/gateway.db "PRAGMA integrity_check;"
   ```

#### Empty Database on Startup

The gateway-controller automatically creates the database schema if it doesn't exist. If you see "schema initialization" in the logs, this is normal behavior on first startup.

#### Performance Issues

SQLite is optimized for up to 100+ API configurations. If you experience performance issues with larger datasets, consider:

1. **Check index usage**:
   ```bash
   sqlite3 ./data/gateway.db
   EXPLAIN QUERY PLAN SELECT * FROM deployments WHERE name = 'MyAPI' AND version = 'v1';
   ```

2. **Verify WAL mode is enabled**:
   ```bash
   sqlite3 ./data/gateway.db "PRAGMA journal_mode;"
   # Should return: wal
   ```

3. **For very large deployments** (1000+ configurations), consider migrating to PostgreSQL (future support planned)

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
│   │   └── stored_config.go     # Data structures
│   ├── storage/
│   │   ├── interface.go      # Storage abstraction
│   │   ├── memory.go         # In-memory cache
│   │   └── sqlite.go         # SQLite implementation
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
GATEWAY_LOGGING_LEVEL=debug ./bin/controller

# Or using config file
export GATEWAY_LOGGING_LEVEL=debug
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
