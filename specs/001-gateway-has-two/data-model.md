# Data Model: Gateway with Controller and Router

**Date**: 2025-10-11
**Phase**: 1 - Design & Contracts
**Purpose**: Define data structures for API configurations, storage, and xDS translation

## Overview

This document defines the data model for the Gateway system. The model includes:
1. **User-Facing Entities**: API Configuration format (YAML/JSON input)
2. **Internal Entities**: Storage representations and xDS snapshots
3. **Validation Rules**: Constraints and requirements for each entity
4. **State Transitions**: Configuration lifecycle states

---

## Entity 1: API Configuration

### Description
The API Configuration represents a complete REST API definition provided by users in YAML or JSON format. It defines how the gateway should route HTTP requests to backend services.

### Format Specification

```yaml
version: api-platform.wso2.com/v1
kind: http/rest
data:
  name: string              # Human-readable API name
  version: string           # Semantic version (e.g., v1.0, v2.1)
  context: string           # Base path for all routes (e.g., /weather)
  upstream:
    - url: string           # Backend service URL (may include path prefix)
  operations:
    - method: string        # HTTP method (GET, POST, PUT, DELETE, PATCH, etc.)
      path: string          # Route path with optional {params}
```

### Example

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
    - method: PUT
      path: /{country_code}/{city}
```

### Go Data Structure

```go
// APIConfiguration represents the complete user-provided API configuration
type APIConfiguration struct {
    Version string           `yaml:"version" json:"version"`
    Kind    string           `yaml:"kind" json:"kind"`
    Data    APIConfigData    `yaml:"data" json:"data"`
}

// APIConfigData contains the API-specific configuration
type APIConfigData struct {
    Name       string          `yaml:"name" json:"name"`
    Version    string          `yaml:"version" json:"version"`
    Context    string          `yaml:"context" json:"context"`
    Upstream   []Upstream      `yaml:"upstream" json:"upstream"`
    Operations []Operation     `yaml:"operations" json:"operations"`
}

// Upstream represents a backend service
type Upstream struct {
    URL string `yaml:"url" json:"url"`
}

// Operation represents a single HTTP operation/route
type Operation struct {
    Method string `yaml:"method" json:"method"`
    Path   string `yaml:"path" json:"path"`
}
```

### Validation Rules

| Field | Constraint | Error if Violated |
|-------|-----------|-------------------|
| `version` | Must equal "api-platform.wso2.com/v1" | "Unsupported API version" |
| `kind` | Must equal "http/rest" | "Unsupported API kind (only http/rest supported)" |
| `data.name` | Non-empty string, 1-100 characters | "API name is required and must be 1-100 characters" |
| `data.version` | Non-empty string, matches semantic version pattern (e.g., v1.0) | "API version is required and must follow format vX.Y" |
| `data.context` | Must start with `/`, no trailing slash, 1-200 characters | "Context must start with / and cannot end with /" |
| `data.upstream` | At least one upstream URL | "At least one upstream URL is required" |
| `data.upstream[].url` | Valid HTTP/HTTPS URL | "Invalid upstream URL format" |
| `data.operations` | At least one operation | "At least one operation is required" |
| `data.operations[].method` | One of: GET, POST, PUT, DELETE, PATCH, HEAD, OPTIONS | "Invalid HTTP method" |
| `data.operations[].path` | Must start with `/`, valid path with optional `{param}` placeholders | "Invalid operation path format" |

### Additional Constraints

1. **Uniqueness**: The combination of `(name, version)` must be unique across all deployed APIs
2. **Context Consistency**: For the same API `name` but different `version`, the `context` must be identical
3. **Path Parameters**: Placeholders in `path` (e.g., `{city}`) are allowed; validation ensures balanced braces
4. **Upstream Path Handling**: If upstream URL contains a path component (e.g., `/api/v2`), the gateway must preserve it when forwarding requests

---

## Entity 2: Stored Configuration

### Description
Internal representation of API Configuration as stored in bbolt database. Includes metadata for lifecycle management.

### Go Data Structure

```go
// StoredAPIConfig represents the configuration stored in the database
type StoredAPIConfig struct {
    ID             string               `json:"id"`              // Unique identifier (generated)
    Configuration  APIConfiguration     `json:"configuration"`   // User-provided config
    Status         ConfigStatus         `json:"status"`          // Current state
    CreatedAt      time.Time            `json:"created_at"`
    UpdatedAt      time.Time            `json:"updated_at"`
    DeployedAt     *time.Time           `json:"deployed_at"`     // Nil if not yet deployed
    DeployedVersion int64               `json:"deployed_version"` // xDS snapshot version
}

// ConfigStatus represents the lifecycle state
type ConfigStatus string

const (
    StatusPending   ConfigStatus = "pending"    // Submitted but not yet deployed
    StatusDeployed  ConfigStatus = "deployed"   // Active in Router
    StatusFailed    ConfigStatus = "failed"     // Deployment failed
)
```

### Storage Schema (bbolt Buckets)

```
gateway-controller.db
├── apis/                        # Bucket: API configurations
│   └── {id} → StoredAPIConfig   # Key: config ID, Value: JSON-encoded config
│
├── audit/                       # Bucket: Audit logs
│   └── {timestamp}_{id} → AuditEvent  # Key: timestamp+ID, Value: event details
│
└── metadata/                    # Bucket: System metadata
    └── last_snapshot_version → int64  # Last xDS snapshot version number
```

### State Transitions

```
[User submits config] → Pending
                         ↓
                  [Validation pass] → Deployed
                         ↓
                  [Validation fail] → Failed

[User updates config] → Deployed (if valid) or Failed (if invalid)
[User deletes config] → [Removed from storage]
```

---

## Entity 3: Audit Event

### Description
Record of configuration changes for audit trail and debugging.

### Go Data Structure

```go
// AuditEvent represents a configuration change event
type AuditEvent struct {
    ID            string                 `json:"id"`             // Event unique ID
    Timestamp     time.Time              `json:"timestamp"`
    Operation     AuditOperation         `json:"operation"`      // CREATE, UPDATE, DELETE
    ConfigID      string                 `json:"config_id"`      // Affected configuration ID
    ConfigName    string                 `json:"config_name"`    // API name for readability
    ConfigVersion string                 `json:"config_version"` // API version
    Status        string                 `json:"status"`         // SUCCESS, FAILED
    ErrorMessage  string                 `json:"error_message"`  // If status=FAILED
    Details       map[string]interface{} `json:"details"`        // Additional context
}

// AuditOperation represents the type of change
type AuditOperation string

const (
    AuditCreate AuditOperation = "CREATE"
    AuditUpdate AuditOperation = "UPDATE"
    AuditDelete AuditOperation = "DELETE"
    AuditQuery  AuditOperation = "QUERY"  // Optional: log read operations
)
```

### Validation Rules

- `ID`: Auto-generated UUID
- `Timestamp`: Auto-set to current time
- `Operation`: Must be one of the defined constants
- `ConfigID`: Must reference a valid configuration ID
- `Status`: "SUCCESS" or "FAILED"

---

## Entity 4: xDS Snapshot

### Description
Internal representation of Envoy configuration snapshot. This is generated from API Configuration and pushed to Router via xDS protocol.

### Go Data Structure (using go-control-plane types)

```go
import (
    cluster "github.com/envoyproxy/go-control-plane/envoy/config/cluster/v3"
    listener "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
    route "github.com/envoyproxy/go-control-plane/envoy/config/route/v3"
    "github.com/envoyproxy/go-control-plane/pkg/cache/v3"
)

// xDSSnapshot is an alias for go-control-plane's snapshot
type xDSSnapshot = cache.Snapshot

// SnapshotResources contains the Envoy resources for a complete configuration
type SnapshotResources struct {
    Version   string                // Snapshot version (incrementing integer as string)
    Listeners []*listener.Listener  // HTTP listeners
    Routes    []*route.RouteConfiguration  // Route configurations
    Clusters  []*cluster.Cluster    // Upstream clusters
    Endpoints []*endpoint.ClusterLoadAssignment  // Optional: endpoint assignments
}
```

### Translation Logic: API Config → Envoy Resources

#### Listener Creation
For each unique context path, create an Envoy Listener:
- **Name**: `listener_http_{port}` (e.g., `listener_http_8080`)
- **Address**: `0.0.0.0:{port}` (default: 8080)
- **Filter Chain**: HTTP Connection Manager with reference to RouteConfiguration

#### Route Configuration Creation
For each API Configuration, create a RouteConfiguration:
- **Name**: `route_{api_name}_{api_version}` (e.g., `route_weather_api_v1_0`)
- **Virtual Host**:
  - **Domains**: `["*"]` (accept all domains)
  - **Routes**: One route per operation mapping `{method, path}` to cluster

#### Cluster Creation
For each unique upstream URL, create a Cluster:
- **Name**: `cluster_{sanitized_upstream_url}` (e.g., `cluster_api_weather_com`)
- **Type**: `STRICT_DNS` or `LOGICAL_DNS`
- **Load Assignment**: Endpoint with upstream host and port

#### Path Rewriting
If upstream URL contains a path prefix (e.g., `https://api.weather.com/api/v2`):
- Extract base URL: `https://api.weather.com`
- Extract path prefix: `/api/v2`
- Configure route to prepend `/api/v2` to all forwarded requests

### Example Translation

**Input (API Configuration)**:
```yaml
data:
  name: Weather API
  version: v1.0
  context: /weather
  upstream:
    - url: https://api.weather.com/api/v2
  operations:
    - method: GET
      path: /{country_code}/{city}
```

**Output (Envoy Resources - Conceptual)**:
- **Listener**: `listener_http_8080` listening on `0.0.0.0:8080`
- **Route**: `route_weather_api_v1_0`
  - Match: `GET /weather/{country_code}/{city}`
  - Action: Forward to `cluster_api_weather_com`
  - Prefix Rewrite: Prepend `/api/v2` → final path: `/api/v2/{country_code}/{city}`
- **Cluster**: `cluster_api_weather_com`
  - Host: `api.weather.com:443`
  - TLS: Enabled (HTTPS upstream)

---

## Data Flow Summary

```
1. User submits API Configuration (YAML/JSON)
           ↓
2. Gateway-Controller parses → APIConfiguration struct
           ↓
3. Validation checks (rules from Entity 1)
           ↓
4. Store as StoredAPIConfig in bbolt
           ↓
5. Log AuditEvent (CREATE operation)
           ↓
6. Translate to Envoy resources (Listener, Route, Cluster)
           ↓
7. Create xDS Snapshot with new version
           ↓
8. Update go-control-plane snapshot cache
           ↓
9. Envoy (Router) polls/streams xDS and receives new config
           ↓
10. Router applies configuration and starts routing traffic
```

---

## Validation & Error Handling

### Validation Stages

1. **Syntax Validation**: YAML/JSON structure is well-formed
2. **Schema Validation**: All required fields present, types correct
3. **Business Rules Validation**: Uniqueness constraints, context consistency
4. **Envoy Translation Validation**: Ensure configuration translates to valid Envoy resources

### Error Response Format

```go
// ValidationError represents a configuration validation failure
type ValidationError struct {
    Field   string `json:"field"`    // Field that failed validation
    Message string `json:"message"`  // Human-readable error message
}

// ErrorResponse is returned for failed API requests
type ErrorResponse struct {
    Status  string            `json:"status"`   // "error"
    Message string            `json:"message"`  // High-level error description
    Errors  []ValidationError `json:"errors"`   // Detailed validation errors
}
```

**Example Error Response**:
```json
{
  "status": "error",
  "message": "Configuration validation failed",
  "errors": [
    {
      "field": "data.context",
      "message": "Context must start with / and cannot end with /"
    },
    {
      "field": "data.operations[0].method",
      "message": "Invalid HTTP method: INVALID"
    }
  ]
}
```

---

## Concurrency & Consistency

### Thread Safety
- **bbolt Transactions**: All read/write operations wrapped in bbolt transactions (ACID guarantees)
- **xDS Snapshot Updates**: go-control-plane cache is thread-safe; use mutex for snapshot version generation

### Atomicity
- Configuration create/update/delete operations are atomic:
  - Database write succeeds → xDS snapshot updated
  - Database write fails → no xDS update, error returned to user
  - xDS update should not block API response (async push to Envoy)

### Consistency Rules
1. API name + version uniqueness enforced via database check before insert
2. Context consistency for same API name validated before insert/update
3. Snapshot version monotonically increases (never decreases)

---

## Summary

This data model provides:
- **Clear contract** for user-facing API Configuration format
- **Storage schema** using bbolt buckets for configurations, audit logs, and metadata
- **Translation strategy** from user config to Envoy xDS resources
- **Validation rules** at multiple stages to ensure correctness
- **State management** for configuration lifecycle

**Status**: Data model complete. Ready for API contract generation.
