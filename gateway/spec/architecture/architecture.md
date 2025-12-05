# Gateway Architecture

## Overview

Envoy-based gateway system with Go xDS control plane for dynamic API configuration, policy enforcement, and traffic management. Supports both single-instance deployments with SQLite and scalable cloud deployments.

## Components

### Gateway-Controller (Port 9090 REST, 18000 xDS)
- REST API server accepting YAML/JSON API configurations using Gin router.
- Validation layer providing field-level error messages with structured reporting.
- xDS v3 server implementing State-of-the-World protocol for Envoy configuration.
- SQLite database for persistent storage (`./data/gateway.db`) with WAL mode.
- In-memory cache for fast configuration access with thread-safe operations.

### Router (Envoy Proxy, Port 8080)
- Envoy Proxy 1.35.3 routing HTTP traffic to backend services.
- Bootstrap configuration connecting to Gateway-Controller xDS server.
- JSON-formatted access logs to stdout for observability.
- Zero-downtime configuration updates via xDS protocol.

### Policy Engine (Standard Tier)
- Authentication policies: API Key, OAuth, JWT validation.
- Authorization policies: RBAC, scope validation.
- Traffic management policies: Header modification, request/response transformation.

### Rate Limiter (Standard Tier)
- Distributed rate limiting with Redis backend.
- Quota management and throttling.
- Spike arrest and burst protection.

### Database
- SQLite database file (`./data/gateway.db`).
- Schema with `deployments` table storing configurations as JSON TEXT.
- Composite unique constraint on `(name, version)`.
- Indexes on frequently queried fields: `name+version`, `status`, `context`, `kind`.
- Migration path to PostgreSQL/MySQL for cloud deployments.

## Container Structure

```
+-------------------------------------------------------------+
|                Gateway-Controller (container)               |
|  +-------------------+    +-------------------+             |
|  |  REST API Server  | -> | Validation Layer  |             |
|  |   (Port 9090)     |    +-------------------+             |
|  +-------------------+            |                         |
|           |                       v                         |
|           |            +----------------------+             |
|           |            | SQLite + In-Memory   |             |
|           |            |       Cache          |             |
|           |            +----------------------+             |
|           |                       |                         |
|           v                       v                         |
|  +-------------------+    +-------------------+             |
|  |   xDS Translator  | -> |   xDS v3 Server   |             |
|  +-------------------+    |   (Port 18000)    |             |
|                           +-------------------+             |
+-------------------------------------------------------------+
                                   |
                                   | xDS gRPC
                                   v
+-------------------------------------------------------------+
|                    Router (Envoy container)                 |
|  +-------------------+    +-------------------+             |
|  |  Envoy Proxy      | -> |  Backend Services |             |
|  |  (Port 8080)      |    +-------------------+             |
|  +-------------------+                                      |
+-------------------------------------------------------------+
```

## Integration Points

- **API Developers** → Gateway-Controller: Submit API configurations via REST API.
- **Router** ← Gateway-Controller: Receives xDS configuration updates via gRPC.
- **Backend Services** ← Router: Forwards HTTP requests based on API configurations.
- **Platform API** → Gateway: Orchestrates API deployments to gateways.
- **Portals/CLI** → Platform API → Gateway: Indirect configuration management.

## Deployment Tiers

### Basic Gateway
- Components: Gateway-Controller (memory-only), Router, Policy Engine.
- No persistence (configurations lost on restart).
- Basic rate limiting built into Router.
- Use case: Development, testing, 14-day trial.

### Standard Gateway
- Components: All Basic + Rate Limiter + Redis + SQLite.
- Persistent storage with SQLite (configurable to PostgreSQL/MySQL).
- Advanced distributed rate limiting.
- Use case: Production, enterprise deployments.

## Data Flow

### API Configuration Lifecycle
1. User submits API config (YAML/JSON) to REST API (port 9090).
2. Gateway-Controller validates configuration structure and fields.
3. Configuration persisted to SQLite and cached in memory.
4. xDS translator generates Envoy configuration from API config.
5. xDS server pushes new snapshot to Router via gRPC (port 18000).
6. Router applies configuration gracefully (zero downtime).

### Runtime Request Flow
1. HTTP request arrives at Router (port 8080).
2. Router matches request to API configuration (method, path, context).
3. Policy Engine evaluates policies (auth, rate limit, etc.).
4. Request forwarded to backend service upstream URL.
5. Response returned to client.
6. Access log written to stdout in JSON format.
