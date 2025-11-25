# Policy xDS Server

This document describes the Policy xDS Server component of the Gateway Controller.

## Overview

The Policy xDS Server is a separate gRPC server that runs alongside the main xDS server and distributes policy configurations to clients (e.g., Envoy proxies or custom policy engines). It runs in its own goroutine and operates independently of the main API routing xDS server.

## Architecture

```
┌─────────────────────────────────────────────────┐
│         Gateway Controller                      │
│                                                 │
│  ┌──────────────┐        ┌──────────────┐      │
│  │   Main xDS   │        │  Policy xDS  │      │
│  │   Server     │        │   Server     │      │
│  │  (Port 18000)│        │ (Port 18001) │      │
│  └──────────────┘        └──────────────┘      │
│         │                       │               │
│         │                       │               │
│  ┌──────▼──────┐        ┌──────▼──────┐        │
│  │ Snapshot    │        │  Policy     │        │
│  │ Manager     │        │  Snapshot   │        │
│  │             │        │  Manager    │        │
│  └──────┬──────┘        └──────┬──────┘        │
│         │                      │                │
│  ┌──────▼──────┐        ┌─────▼───────┐        │
│  │ Config      │        │   Policy    │        │
│  │ Store       │        │   Store     │        │
│  └─────────────┘        └─────────────┘        │
└─────────────────────────────────────────────────┘
```

## Components

### 1. Policy Models (`pkg/models/policy.go`)

Defines the data structures for policy configurations:

- **PolicyConfiguration**: Root structure containing routes and metadata
- **RoutePolicy**: Policy configuration for a specific route
- **Policy**: Individual policy with name, version, execution condition, and config
- **Metadata**: Metadata about the policy configuration (timestamps, versions, API info)
- **StoredPolicyConfig**: Stored representation with versioning

### 2. Policy Store (`pkg/storage/policy_store.go`)

Thread-safe in-memory storage for policy configurations:

- Stores policies by ID and composite key (api_name:version:context)
- Provides CRUD operations
- Manages resource versioning
- Thread-safe with RWMutex

### 3. Policy xDS Server (`pkg/policyxds/`)

#### `server.go`
- gRPC server implementation
- Implements xDS protocol (ADS, RTDS)
- Runs on separate port (default: 18001)
- Provides xDS callbacks for logging and monitoring

#### `snapshot.go`
- Snapshot manager for policy configurations
- Translates policies to xDS resources
- Manages cache and versioning
- Updates snapshots when policies change

#### `manager.go`
- High-level policy management API
- Handles adding/removing policies
- Triggers snapshot updates
- Provides JSON parsing utilities

## Configuration

### config.yaml

```yaml
# Policy xDS Server configuration
policyserver:
  # Enable or disable the policy xDS server
  enabled: true

  # Policy xDS gRPC port for policy distribution
  port: 18001
```

### Environment Variables

You can override configuration using environment variables:

```bash
GATEWAY_POLICYSERVER_ENABLED=true
GATEWAY_POLICYSERVER_PORT=18001
```

## Usage

### Starting the Server

The Policy xDS Server starts automatically when the Gateway Controller starts if `policyserver.enabled` is set to `true` in the configuration.

```bash
./gateway-controller --config config/config.yaml
```

You should see log messages like:

```
INFO    Initializing Policy xDS server    {"port": 18001}
INFO    Generating initial policy xDS snapshot
INFO    Starting Policy xDS server    {"port": 18001}
```

### Policy Data Structure

Policies follow this JSON structure:

```json
{
    "routes": [
        {
            "route_key": "api-v1-users",
            "request_policies": [
                {
                    "name": "apiKeyValidation",
                    "version": "v1.0.0",
                    "executionCondition": "request.metadata[authenticated] != true",
                    "config": {
                        "header": "X-API-Key",
                        "validKeys": ["key-123", "key-456"]
                    }
                }
            ],
            "response_policies": [
                {
                    "name": "setHeader",
                    "version": "v1.0.0",
                    "executionCondition": null,
                    "config": {
                        "header": "X-Processed-By",
                        "value": "Policy-Engine"
                    }
                }
            ]
        }
    ],
    "metadata": {
        "created_at": 1700000000,
        "updated_at": 1700000100,
        "resource_version": 5,
        "api_name": "api1",
        "version": "v1",
        "context": "samplecontext"
    }
}
```

### Programmatic Usage

```go
import (
    "github.com/wso2/api-platform/gateway/gateway-controller/pkg/models"
    "github.com/wso2/api-platform/gateway/gateway-controller/pkg/policyxds"
    "github.com/wso2/api-platform/gateway/gateway-controller/pkg/storage"
)

// Create policy store and manager
policyStore := storage.NewPolicyStore()
snapshotManager := policyxds.NewSnapshotManager(policyStore, logger)
policyManager := policyxds.NewPolicyManager(policyStore, snapshotManager, logger)

// Parse policy from JSON
policyConfig, err := policyxds.ParsePolicyJSON(jsonString)
if err != nil {
    log.Fatal(err)
}

// Create stored policy
storedPolicy := policyxds.CreateStoredPolicy("policy-id-123", *policyConfig)

// Add policy (automatically updates xDS snapshot)
err = policyManager.AddPolicy(storedPolicy)
if err != nil {
    log.Fatal(err)
}

// List all policies
policies := policyManager.ListPolicies()

// Remove policy
err = policyManager.RemovePolicy("policy-id-123")
```

## Scalability Features

1. **Concurrent Operation**: Runs in separate goroutine, doesn't block main server
2. **Thread-Safe Storage**: All policy store operations are thread-safe with RWMutex
3. **Resource Versioning**: Automatic versioning for cache invalidation
4. **Composite Indexing**: Fast lookup by ID or api_name:version:context
5. **gRPC Streaming**: Efficient xDS streaming protocol
6. **Independent Lifecycle**: Can be enabled/disabled independently
7. **TLS Support**: Optional TLS encryption for secure communication

## Security

### TLS Configuration

The Policy xDS server supports TLS for secure communication with clients. TLS can be enabled via configuration:

```yaml
policyserver:
  enabled: true
  port: 18001
  tls:
    enabled: true
    cert_file: "/path/to/server.crt"
    key_file: "/path/to/server.key"
```

**Usage in Code:**

```go
// Without TLS (default)
server := policyxds.NewServer(snapshotManager, port, logger)

// With TLS
server := policyxds.NewServer(
    snapshotManager, 
    port, 
    logger,
    policyxds.WithTLS("/path/to/server.crt", "/path/to/server.key"),
)
```

**Generating Self-Signed Certificates for Testing:**

```bash
# Generate private key
openssl genrsa -out server.key 2048

# Generate self-signed certificate
openssl req -new -x509 -sha256 -key server.key -out server.crt -days 365 \
  -subj "/CN=localhost"
```

**Important Notes:**
- TLS is optional and disabled by default for backward compatibility
- When TLS is enabled, clients must connect using TLS credentials
- For production, use certificates from a trusted CA
- The server validates certificates at startup and fails fast if invalid

## Extension Points

The current implementation provides a foundation for policy distribution. You can extend it by:

1. **Custom xDS Resources**: Implement Envoy extension configurations
2. **Policy Validation**: Add validation logic in the manager
3. **Persistence**: Add database backing for policies
4. **REST API**: Create REST endpoints for policy management
5. **Policy Templates**: Add policy template support
6. **Policy Composition**: Support for policy inheritance and composition

## Monitoring

The server includes detailed logging via callbacks:

- Stream open/close events
- Request/response tracking
- Resource distribution monitoring

Enable debug logging to see detailed xDS protocol messages:

```yaml
logging:
  level: debug
```

## Sample Data

A sample policy configuration is provided in `data/sample-policy.json` for testing and reference.

## Thread Safety

All components are designed to be thread-safe:

- PolicyStore uses RWMutex for concurrent access
- Snapshot updates are atomic
- gRPC server handles concurrent connections

## Next Steps

1. Integrate with control plane for centralized policy management
2. Add REST API endpoints for policy CRUD operations
3. Implement policy validation rules
4. Add persistence layer for policies
5. Create policy templates and versioning
6. Add policy testing framework
