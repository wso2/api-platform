# Research: Gateway with Controller and Router

**Date**: 2025-10-11
**Phase**: 0 - Outline & Research
**Purpose**: Resolve technical clarifications and establish best practices for implementation

## Overview

This document consolidates research findings for key technical decisions required to implement the Gateway system. All "NEEDS CLARIFICATION" items from the Technical Context have been resolved through industry research and best practices analysis.

---

## Decision 1: go-control-plane Version

**Question**: Which version of go-control-plane should we use for Envoy xDS v3 API?

**Decision**: Use latest stable version from `github.com/envoyproxy/go-control-plane` (February 2025 release)

**Rationale**:
- The library is actively maintained with updates as recent as February 2025
- V2 control-plane code has been removed; the library now focuses exclusively on xDS v3 APIs
- Provides comprehensive v3 packages:
  - `github.com/envoyproxy/go-control-plane/pkg/cache/v3` (snapshot cache)
  - `github.com/envoyproxy/go-control-plane/pkg/server/v3` (xDS server implementation)
  - `github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3` (discovery service APIs)
- Apache-2.0 license with redistributable terms
- Well-documented with examples in the repository
- Matches Envoy 1.35.3 compatibility requirements

**Alternatives Considered**:
- **Writing custom xDS implementation**: Rejected because go-control-plane is the official, battle-tested implementation used by major service mesh projects
- **Using older v2 APIs**: Rejected because v2 is deprecated and removed from the library

**Implementation Notes**:
- Import path: `github.com/envoyproxy/go-control-plane`
- Use v3 packages exclusively
- Leverage the snapshot cache (`cache/v3`) for managing configuration versions
- Use the server implementation (`server/v3`) for gRPC xDS server

**References**:
- https://pkg.go.dev/github.com/envoyproxy/go-control-plane
- https://github.com/envoyproxy/go-control-plane

---

## Decision 2: Storage Library Choice

**Question**: Which embedded database should we use for persisting API configurations (bbolt, Badger, or SQLite)?

**Decision**: Use **bbolt** (etcd-io/bbolt)

**Rationale**:
- **Simplicity**: Single file B+tree database, no complex LSM-tree tuning required
- **ACID Compliance**: Full transactional support with strong consistency guarantees (critical for configuration integrity)
- **Proven Stability**: Originally BoltDB, now maintained by etcd team; development is intentionally "locked" (stable, minimal changes)
- **Bucket Model**: Provides logical separation of data via buckets (comparable to tables), perfect for organizing API configs
- **Low Maintenance**: No compaction, no background goroutines, simple operational model
- **Sufficient Performance**: 339 ops/sec writes and 874K ops/sec reads - more than adequate for configuration management workload (not high-frequency trading)
- **Small Footprint**: Single binary, minimal dependencies
- **Use Case Fit**: Configuration management prioritizes consistency and simplicity over raw write throughput

**Alternatives Considered**:
- **Badger**: Rejected despite superior performance (faster than RocksDB) because:
  - LSM-tree complexity is overkill for configuration management
  - Lacks bucket concept for logical data organization
  - More operational overhead (compaction, background processes)
  - Configuration workload is read-heavy with infrequent writes, doesn't need LSM optimization

- **SQLite**: Rejected because:
  - Adds CGo dependency (complicates cross-compilation and Docker builds)
  - Relational model is overkill for key-value configuration storage
  - Larger binary size and dependency footprint
  - bbolt's simpler model better matches our use case

**Implementation Notes**:
- Import: `go.etcd.io/bbolt`
- Use buckets to separate: API configurations, audit logs, metadata
- Single read-write transaction per configuration operation (atomic updates)
- File location: `/data/gateway-controller.db` (mount as Docker volume)
- Consider implementing storage interface for future flexibility (but start with bbolt)

**References**:
- https://github.com/etcd-io/bbolt
- https://github.com/xeoncross/go-embeddable-stores (benchmark comparison)

---

## Decision 3: Build Tool Selection

**Question**: Is Make the appropriate build tool, or should we consider alternatives?

**Decision**: Use **Make** (Makefile)

**Rationale**:
- **Industry Standard**: Make remains the #1 choice for Go projects in 2025, widely adopted across major projects (Kubernetes, Docker, etc.)
- **Ubiquitous Availability**: Pre-installed on virtually all Unix-like systems (Linux, macOS), including CI/CD environments
- **Simplicity for Use Case**: Our needs are straightforward:
  - `make build` - compile Go binary
  - `make test` - run tests
  - `make docker` - build Docker images
  - `make clean` - cleanup artifacts
- **No Additional Dependencies**: Team members don't need to install extra tools
- **Good Enough**: While Make syntax is archaic, our Makefile will be simple task orchestration, not complex dependency management
- **Consistency**: Using Make for both Gateway-Controller and Router provides uniform build interface

**Alternatives Considered**:
- **Taskfile**: Rejected because:
  - Requires team to install separate binary (`task`)
  - YAML syntax is cleaner but provides minimal benefit for our simple use case
  - Adds unnecessary dependency to development environment

- **Mage**: Rejected because:
  - Requires Go-based build scripts (magefile.go)
  - Overkill for simple task automation
  - Team needs to learn Mage API instead of simple Make targets

- **Just**: Rejected because:
  - Another tool to install
  - Explicitly not a build tool (command runner only)
  - Less familiar to most developers than Make

**Implementation Notes**:
- Create Makefiles for both `gateway-controller/` and `router/` directories
- Standard targets: `build`, `test`, `docker`, `clean`, `run`, `help`
- Use `.PHONY` declarations for non-file targets
- Include helpful comments and `make help` target
- Docker multi-stage builds to keep images minimal

**References**:
- https://www.alexedwards.net/blog/a-time-saving-makefile-for-your-go-projects
- https://vincent.bernat.ch/en/blog/2019-makefile-build-golang

---

## Decision 4: Go Project Structure

**Question**: What project structure should we follow for Gateway-Controller?

**Decision**: Use **standard Go project layout** with `cmd/`, `pkg/`, and `tests/` directories

**Rationale**:
- **Industry Convention**: The golang-standards/project-layout pattern is widely adopted, especially in infrastructure projects like Kubernetes
- **Clear Separation**:
  - `cmd/` - application entry points (executables)
  - `pkg/` - reusable library code organized by domain
  - `tests/` - test code separate from implementation
- **Modularity**: Aligns with Kubernetes design goals (modularity, decoupling, explicit package organization)
- **Future Extensibility**: If we later add CLI tools or additional services, the structure accommodates them easily
- **Not Over-Engineered**: Despite being a "standard layout," we're only using core directories relevant to our needs
- **Avoid `/internal` Initially**: We'll use `/pkg` for now since we may want to reuse xDS translation logic in other platform components

**Alternatives Considered**:
- **Flat Structure (all code in root)**: Rejected because:
  - Works for tiny projects but Gateway-Controller has multiple concerns (API, xDS, storage, config parsing)
  - Harder to navigate and test as codebase grows

- **Heavy DDD Structure**: Rejected because:
  - Domain-driven design with extensive layering is overkill
  - Our domain is straightforward (configuration management and xDS translation)
  - Adds unnecessary directory depth and complexity

**Implementation Notes**:
- Package organization within `pkg/`:
  - `api/` - Gin HTTP handlers and middleware
  - `config/` - YAML/JSON parsing and validation
  - `models/` - Data structures
  - `storage/` - Database abstraction and implementation
  - `xds/` - xDS server and Envoy configuration translation
  - `logger/` - Zap logging setup
- Single executable in `cmd/controller/main.go`
- Tests organized to mirror source structure

**References**:
- https://github.com/golang-standards/project-layout
- https://leapcell.medium.com/learning-large-scale-go-project-architecture-from-k8s-6c8f2c3862d8
- https://www.alexedwards.net/blog/11-tips-for-structuring-your-go-projects

---

## Additional Research: Envoy xDS Protocol

**Context**: Understanding how to implement the xDS server and translate API configs to Envoy configuration

**Key Findings**:
- **xDS Resources**: Need to implement:
  - Listener Discovery Service (LDS) - defines listeners for HTTP traffic
  - Route Discovery Service (RDS) - configures routes for listeners
  - Cluster Discovery Service (CDS) - defines upstream backend clusters
  - Endpoint Discovery Service (EDS) - provides endpoints for clusters (optional for our use case)
- **SotW (State-of-the-World) Protocol**:
  - xDS protocol variant where the control plane sends the complete configuration state in each response
  - Envoy connects via gRPC stream and requests resource types (LDS, RDS, CDS, etc.)
  - Control plane responds with ALL resources of that type (not incremental deltas)
  - Simpler than incremental xDS; suitable for configuration management use cases
  - go-control-plane's snapshot cache implements SotW by default
- **Snapshot Cache**: go-control-plane provides snapshot-based cache that simplifies configuration management
  - Create new snapshot when API config changes
  - Each snapshot contains the complete state of all resources
  - Cache handles versioning and distribution to connected Envoys
  - Envoy polls or streams updates based on resource versions
  - Snapshot version is monotonically increasing integer (converted to string)
- **Translation Pattern**:
  1. Load all API configurations from database to in-memory maps on startup
  2. Generate initial xDS snapshot from in-memory maps
  3. On configuration change:
     - Update in-memory maps + database atomically
     - Translate ALL API configs from in-memory maps to Envoy resources
     - Create new snapshot with incremented version
     - Update cache with new snapshot (SotW approach)
  4. Envoy receives complete configuration state and applies it
- **Graceful Updates**: Envoy's connection draining ensures in-flight requests complete before configuration changes take effect

**Implementation Impact**:
- In-memory maps structure in `pkg/storage/memory.go`
- Database loader on startup in `pkg/storage/[implementation].go`
- xDS translation logic reads from in-memory maps in `pkg/xds/translator.go`
- SotW snapshot management in `pkg/xds/snapshot.go`
- xDS SotW server setup in `pkg/xds/server.go`

**References**:
- https://www.envoyproxy.io/docs/envoy/latest/api-docs/xds_protocol
- https://blog.christianposta.com/envoy/guidance-for-building-a-control-plane-to-manage-envoy-proxy-based-infrastructure/

---

## Additional Research: Docker Multi-Stage Builds

**Context**: Both components need minimal Docker images

**Key Findings**:
- **Gateway-Controller**:
  - Stage 1: Use `golang:1.25.1` to build binary
  - Stage 2: Use `alpine:latest` or `scratch` for runtime
  - Result: Final image <20MB (compared to 1GB+ with full Go image)

- **Router**:
  - Use `envoyproxy/envoy:v1.35.3` as base
  - Copy bootstrap YAML into container
  - No custom build needed (Envoy binary already present)

**Implementation Impact**:
- Gateway-Controller Dockerfile uses multi-stage build
- Router Dockerfile is simple (FROM + COPY + ENTRYPOINT)
- Both images support linux/amd64 and linux/arm64 architectures

---

---

## Decision 5: API Configuration Identity Strategy

**Question**: How should API configurations be uniquely identified when multiple versions of the same API exist?

**Decision**: Use **composite key** format `{name}/{version}` (e.g., "PetStore/v1", "WeatherAPI/v2.1")

**Rationale**:
- **URL-Friendly**: Slash separator works naturally in REST API paths (`/apis/PetStore/v1`)
- **Human-Readable**: Clear semantic meaning in logs, error messages, and API responses
- **Consistent with Kubernetes**: Similar to Kubernetes resource naming (`namespace/name`)
- **Filesystem Compatible**: Can be used for file-based storage or exports if needed
- **Query Simplicity**: Easy to parse and validate with simple string split operation

**Alternatives Considered**:
- **`{name}@{version}`**: Rejected because `@` symbol requires URL encoding in REST paths
- **Auto-generated UUID**: Rejected because it hides semantic meaning, makes debugging harder
- **Name-only identifier**: Rejected because it doesn't support multiple API versions

**Implementation Notes**:
- Storage key in bbolt: Use composite key as primary bucket key
- In-memory indexing: Maintain `map[string]string` with key `"name:version"` → config ID
- REST API paths: `/apis/{name}/{version}` for CRUD operations
- Validation: Ensure both name and version are non-empty; version follows semantic versioning

---

## Decision 6: Router Startup Resilience Strategy

**Question**: What should the Router (Envoy) do when it cannot connect to the Gateway-Controller xDS server at startup?

**Decision**: **Wait indefinitely with exponential backoff retry** until xDS connection succeeds

**Rationale**:
- **Orchestration Compatibility**: In Docker Compose or Kubernetes, services may start in any order; waiting prevents startup race conditions
- **Operational Simplicity**: No need to manually restart Router after Controller comes online
- **Fail-Safe Default**: Ensures Router never serves stale or missing configuration; waits for authoritative source
- **Standard Envoy Behavior**: Aligns with Envoy's built-in retry mechanisms for xDS connections
- **No Stale Data**: Router will not route traffic until it receives valid configuration from Controller

**Alternatives Considered**:
- **Fail fast after timeout**: Rejected because it forces manual intervention and complicates container orchestration
- **Start with empty routing rules**: Rejected because it would route all requests to 404, creating confusing behavior
- **Load cached configuration from disk**: Rejected because Router is designed to be stateless per spec constraint

**Implementation Notes**:
- Configure Envoy bootstrap with `initial_fetch_timeout: 0` (wait indefinitely)
- Set exponential backoff: `base_interval: 1s`, `max_interval: 30s`
- Log retry attempts at INFO level for observability
- Router admin endpoint (`:9901`) remains available during connection attempts for health checks

**Envoy Bootstrap Configuration**:
```yaml
dynamic_resources:
  cds_config:
    initial_fetch_timeout: 0s  # Wait indefinitely
    resource_api_version: V3
    api_config_source:
      api_type: GRPC
      transport_api_version: V3
      grpc_services:
        - envoy_grpc:
            cluster_name: xds_cluster
      set_node_on_first_message_only: true
      retry_policy:
        retry_back_off:
          base_interval: 1s
          max_interval: 30s
```

---

## Decision 7: Logging and Observability Strategy

**Question**: What logging approach should the Gateway-Controller use for troubleshooting configuration issues?

**Decision**: **Structured logging with configurable levels** (debug, info, warn, error) using Zap logger

**Rationale**:
- **Production Default (INFO)**: Minimal noise in production; logs only significant events (config changes, errors)
- **Debug Mode**: When enabled, logs include:
  - Full API configuration payloads (YAML/JSON input)
  - Configuration diffs (before/after for updates)
  - Complete xDS snapshot payloads sent to Envoy
  - Detailed validation error traces
- **Structured Format**: JSON output for easy parsing by log aggregation tools (ELK, Splunk, CloudWatch)
- **Performance**: Zap is zero-allocation logger; negligible overhead even at debug level
- **Runtime Configuration**: Log level set via environment variable (`LOG_LEVEL=debug`) or CLI flag (`--log-level=debug`)

**Alternatives Considered**:
- **Fixed log level**: Rejected because production systems need different verbosity than development/troubleshooting
- **Plain text logs only**: Rejected because structured logs are essential for modern observability stacks
- **Always-on debug logging**: Rejected because it creates excessive log volume and potential PII exposure

**Implementation Notes**:
- Use `go.uber.org/zap` for structured logging
- Log fields to include:
  - `timestamp` (ISO 8601)
  - `level` (debug/info/warn/error)
  - `operation` (create/update/delete/query)
  - `api_id` (composite key `name/version`)
  - `status` (success/failed)
  - `error` (if applicable)
  - `config_payload` (debug level only)
  - `xds_snapshot` (debug level only)
- Default configuration:
  ```go
  logger, _ := zap.NewProduction() // INFO level, JSON output
  if os.Getenv("LOG_LEVEL") == "debug" {
      logger, _ = zap.NewDevelopment() // DEBUG level, console output
  }
  ```

---

## Decision 8: Validation Error Response Format

**Question**: When configuration validation fails, what level of detail should error responses contain?

**Decision**: **Structured JSON error response with field-level details**

**Rationale**:
- **Developer Experience**: Precise field paths (e.g., `spec.operations[0].path`) allow developers to quickly locate and fix errors
- **Automation-Friendly**: Structured format enables automated error handling in CI/CD pipelines
- **Multiple Errors**: Single validation pass can return all errors, not just the first failure
- **Consistency**: Matches modern API best practices (RFC 7807 Problem Details, JSON:API error format)
- **Testability**: Explicit error structure makes integration tests easier to write and maintain

**Error Response Schema**:
```json
{
  "status": "error",
  "message": "Configuration validation failed",
  "errors": [
    {
      "field": "spec.context",
      "message": "Context must start with / and cannot end with /"
    },
    {
      "field": "spec.operations[0].method",
      "message": "Invalid HTTP method: INVALID (must be GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS)"
    },
    {
      "field": "spec.upstreams[0].url",
      "message": "Invalid URL format: missing protocol scheme"
    }
  ]
}
```

**Alternatives Considered**:
- **Generic error message only**: Rejected because it forces trial-and-error debugging
- **Line number references**: Rejected because field paths are more semantic and survive reformatting
- **Configurable detail level**: Rejected because detailed errors should always be returned (no security risk in config validation)

**Implementation Notes**:
- Use Go struct for error response:
  ```go
  type ErrorResponse struct {
      Status  string            `json:"status"`
      Message string            `json:"message"`
      Errors  []ValidationError `json:"errors"`
  }

  type ValidationError struct {
      Field   string `json:"field"`
      Message string `json:"message"`
  }
  ```
- Validation library: Use `go-playground/validator/v10` with custom error translation
- HTTP status codes:
  - `400 Bad Request`: Validation errors
  - `409 Conflict`: Uniqueness violations (duplicate name/version)
  - `500 Internal Server Error`: Unexpected failures (database errors, xDS generation failures)

---

---

## Decision 9: REST API Code Generation Strategy

**Question**: Should we manually implement the REST API handlers or use code generation from the OpenAPI specification?

**Decision**: Use **oapi-codegen** for generating server boilerplate code from OpenAPI specification

**Rationale**:
- **Reduced Boilerplate**: Automatically generates request/response types, parameter parsing, and handler interfaces from OpenAPI spec
- **Contract-First Development**: Ensures implementation matches the OpenAPI contract in `gateway-controller-api.yaml`
- **Type Safety**: Generated Go types match the OpenAPI schema exactly, preventing drift between spec and code
- **Active Maintenance**: oapi-codegen is actively maintained (moved to its own organization in May 2024) and recommended by Go community
- **Framework Integration**: Native support for Gin framework via `gin-server` generation mode
- **Zero Dependencies**: Generated code has no runtime dependencies beyond the chosen framework (Gin)
- **Single File Output**: All generated code in one file for simplicity
- **Implementation Focus**: Developers write business logic in handler implementations; boilerplate is generated

**Alternatives Considered**:
- **Manual Implementation**: Rejected because:
  - High maintenance burden keeping code and OpenAPI spec in sync
  - Error-prone manual request validation and type conversion
  - Repetitive boilerplate for each endpoint

- **OpenAPI Generator (fork of Swagger Codegen)**: Rejected because:
  - Community feedback indicates oapi-codegen produces more idiomatic Go code
  - oapi-codegen is specifically designed for Go, not a multi-language tool adapted for Go
  - Less active Go-specific development compared to oapi-codegen

- **Swagger Codegen**: Rejected because:
  - Older tool with less active development for Go servers
  - oapi-codegen is the recommended modern alternative for Go

**Implementation Notes**:

1. **Configuration File** (`gateway-controller/oapi-codegen.yaml`):
```yaml
package: api
output: pkg/api/generated.go
generate:
  gin-server: true        # Generate Gin framework handlers
  models: true            # Generate request/response types
  embedded-spec: true     # Embed OpenAPI spec for documentation
  strict-server: false    # Use standard server interface (not strict mode)
```

2. **Code Generation Command**:
```bash
# Install oapi-codegen
go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@latest

# Generate code (from gateway-controller directory)
oapi-codegen --config=oapi-codegen.yaml api/openapi.yaml
```

3. **Makefile Target**:
```makefile
.PHONY: generate
generate:
	@echo "Generating API server code from OpenAPI spec..."
	oapi-codegen --config=oapi-codegen.yaml api/openapi.yaml
```

4. **Generated Artifacts**:
   - `ServerInterface`: Go interface with methods for each OpenAPI operation
   - Request/Response types: Go structs matching OpenAPI schemas
   - `RegisterHandlers`: Function to register routes with Gin router
   - Embedded OpenAPI spec for runtime documentation

5. **Handler Implementation Pattern**:
```go
// pkg/api/handlers/handlers.go
type APIServer struct {
    storage storage.Storage
    xdsServer *xds.Server
    logger *zap.Logger
}

// Implement ServerInterface methods
func (s *APIServer) CreateAPI(c *gin.Context) {
    var req api.APIConfiguration
    if err := c.ShouldBindJSON(&req); err != nil {
        // Error handling
        return
    }

    // Business logic: validate, store, update xDS
    // ...

    c.JSON(http.StatusCreated, api.APICreateResponse{
        Status: "success",
        Message: "API configuration created successfully",
        ID: newID,
        CreatedAt: time.Now(),
    })
}

// Similar implementations for GetAPIByID, UpdateAPI, DeleteAPI, ListAPIs
```

6. **Server Setup** (`cmd/controller/main.go`):
```go
func main() {
    // Initialize dependencies
    store := storage.NewBBoltStorage("data/gateway-controller.db")
    xdsServer := xds.NewServer()
    logger := logger.NewLogger()

    // Create handler implementation
    apiServer := handlers.NewAPIServer(store, xdsServer, logger)

    // Setup Gin router
    router := gin.Default()

    // Register generated handlers
    api.RegisterHandlers(router, apiServer)

    // Start server
    router.Run(":9090")
}
```

**Benefits**:
- **Contract Enforcement**: Code generation ensures API implementation matches OpenAPI spec
- **Rapid Development**: Focus on business logic; skip repetitive request/response handling
- **Refactoring Safety**: Changing OpenAPI spec and regenerating code reveals breaking changes at compile time
- **Documentation Alignment**: Generated code is always in sync with API documentation
- **Testing Support**: Generated types make it easy to write type-safe tests

**Trade-offs**:
- **Build Step**: Requires running code generation before building (added to Makefile)
- **Generated Code Review**: Generated file should be committed to version control for transparency
- **Customization Limits**: Generated code cannot be manually edited (changes must go through OpenAPI spec)

**Version Control**:
- Commit both the OpenAPI spec (`api/openapi.yaml`) and generated code (`pkg/api/generated.go`)
- Use `go generate` directive or Makefile to document generation process
- CI/CD pipeline should verify generated code is up-to-date with spec

---

## Decision 10: Router Access Logging Strategy

**Question**: What access logging should the Router (Envoy) implement for observability of HTTP traffic routing through the gateway?

**Decision**: Configure **structured JSON access logs to stdout** at the HTTP Connection Manager level in the bootstrap configuration

**Rationale**:
- **Container-Native**: Logging to stdout follows Docker/Kubernetes best practices; logs captured by container runtime
- **Structured Format**: JSON format enables easy parsing by log aggregation tools (ELK, Splunk, CloudWatch, etc.)
- **Production Ready**: Provides essential observability without requiring external log sinks or file mounts
- **Minimal Configuration**: Access logs configured in bootstrap (static config), no xDS complexity
- **Performance**: File-based access logs to stdout have minimal overhead compared to gRPC access log service
- **Standard Fields**: Include request method, path, response code, duration, upstream cluster for troubleshooting

**Alternatives Considered**:
- **gRPC Access Log Service**: Rejected because:
  - Adds complexity (requires separate log collection service)
  - Increases latency for each request (synchronous gRPC call)
  - Overkill for basic gateway use case

- **Plain Text Format**: Rejected because:
  - Harder to parse programmatically
  - JSON is industry standard for structured logs
  - Modern log aggregation tools prefer JSON

- **No Access Logging**: Rejected because:
  - Observability is critical for troubleshooting routing issues
  - Production systems need request-level visibility
  - Minimal performance impact with file-based logging

**Implementation Notes**:

1. **Bootstrap Configuration** (`gateway/router/config/envoy-bootstrap.yaml`):
   - Access logs configured under static resources (not dynamic xDS)
   - Applied to all dynamically configured listeners via HTTP Connection Manager defaults

2. **Log Format Fields**:
   ```yaml
   json_format:
     start_time: "%START_TIME%"
     method: "%REQ(:METHOD)%"
     path: "%REQ(X-ENVOY-ORIGINAL-PATH?:PATH)%"
     protocol: "%PROTOCOL%"
     response_code: "%RESPONSE_CODE%"
     response_flags: "%RESPONSE_FLAGS%"
     bytes_received: "%BYTES_RECEIVED%"
     bytes_sent: "%BYTES_SENT%"
     duration: "%DURATION%"
     upstream_service_time: "%RESP(X-ENVOY-UPSTREAM-SERVICE-TIME)%"
     x_forwarded_for: "%REQ(X-FORWARDED-FOR)%"
     user_agent: "%REQ(USER-AGENT)%"
     request_id: "%REQ(X-REQUEST-ID)%"
     authority: "%REQ(:AUTHORITY)%"
     upstream_host: "%UPSTREAM_HOST%"
     upstream_cluster: "%UPSTREAM_CLUSTER%"
   ```

3. **Standard Envoy Command Operators Used**:
   - `%START_TIME%`: Request start time (ISO 8601 format)
   - `%PROTOCOL%`: HTTP protocol version (HTTP/1.1, HTTP/2, HTTP/3)
   - `%RESPONSE_CODE%`: HTTP response status code
   - `%RESPONSE_FLAGS%`: Envoy response flags (UH, UF, etc. for troubleshooting)
   - `%DURATION%`: Total request duration in milliseconds
   - `%UPSTREAM_CLUSTER%`: Which backend cluster handled the request (maps to API config)
   - `%UPSTREAM_HOST%`: Actual backend host that processed the request

4. **Example Log Output**:
   ```json
   {
     "start_time": "2025-10-12T10:30:45.123Z",
     "method": "GET",
     "path": "/weather/US/Seattle",
     "protocol": "HTTP/1.1",
     "response_code": 200,
     "response_flags": "-",
     "bytes_received": 0,
     "bytes_sent": 1024,
     "duration": 45,
     "upstream_service_time": "42",
     "x_forwarded_for": "192.168.1.10",
     "user_agent": "curl/7.68.0",
     "request_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
     "authority": "gateway.example.com",
     "upstream_host": "api.weather.com:443",
     "upstream_cluster": "cluster_api_weather_com"
   }
   ```

5. **Configuration Location**:
   - Access logs ARE configured in the xDS-generated Listener resources by the Gateway-Controller
   - Each Listener resource includes an `access_log` field (part of Listener protobuf definition)
   - The Gateway-Controller's xDS translator will include access log configuration in all generated Listeners
   - This approach allows consistent logging across all APIs without requiring per-API customization

**Decision**: Configure access logs **dynamically via xDS** in generated Listener resources:
- Gateway-Controller includes access log configuration in all generated Listeners
- Consistent logging format across all APIs
- Simpler than bootstrap static configuration (no Router bootstrap changes needed)
- All configuration comes from Gateway-Controller (single source of truth)
- Future enhancement: Allow per-API access log customization via API configuration

6. **Volume Considerations**:
   - High-traffic gateways may generate large log volumes
   - Container log rotation handled by Docker/Kubernetes runtime
   - Consider log sampling or filtering for extremely high-throughput scenarios (future enhancement)

**References**:
- https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/usage
- https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/access_loggers/file/v3/file.proto
- https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/usage#command-operators

---

## Summary of Resolved Clarifications

| Item | Decision | Confidence |
|------|----------|-----------|
| go-control-plane version | Latest stable (Feb 2025), v3 packages | ✅ High |
| Storage library | bbolt (go.etcd.io/bbolt) | ✅ High |
| Build tool | Make (Makefile) | ✅ High |
| Project structure | Standard layout (cmd/pkg/tests) | ✅ High |
| API identity format | Composite key `{name}/{version}` | ✅ High |
| Router startup behavior | Wait indefinitely with exponential backoff | ✅ High |
| Logging strategy | Structured logging (Zap) with configurable levels | ✅ High |
| Validation errors | Structured JSON with field paths | ✅ High |
| REST API code generation | oapi-codegen with Gin framework | ✅ High |
| Router access logs | JSON format to stdout in bootstrap config | ✅ High |

**Status**: All technical clarifications resolved including spec clarifications from 2025-10-12, code generation strategy from 2025-10-12, and access logging strategy from 2025-10-12. Ready to proceed to Phase 1 (Data Model & Contracts).
