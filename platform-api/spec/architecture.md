# Platform API Architecture

## Overview

Layered Go service exposing REST endpoints over HTTPS with SQLite persistence and integration hooks for portals, CLI, and gateway orchestration.

## Components

### HTTPS Server (Port 8443)
- Gin router serving `/api/v1/**` and `/api/internal/v1/**` routes.
- TLS enabled with auto-generated self-signed certificate for development.
- WebSocket upgrade support at `/api/internal/v1/ws/gateways/connect`.

### WebSocket Manager
- Maintains persistent bidirectional connections with gateways.
- Heartbeat monitoring with ping/pong every 20 seconds.
- Connection registry using sync.Map for thread-safe concurrent access.
- Supports multiple connections per gateway for clustering.

### Service Layer
- Business logic modules for organizations, projects, gateways, and APIs.
- Validation, defaulting, and orchestration across repositories.
- Gateway token generation, rotation, and verification with SHA-256 hashing and salt.
- Event broadcasting service for real-time gateway notifications.

### Repository Layer
- SQL repositories encapsulating CRUD operations and transactions.
- Handles relational writes for MTLS, security, rate limiting, and operations.

### Database
- SQLite database file (`./data/api_platform.db`).
- Schema bootstrapped from `internal/database/schema.sql`.

## Container Structure

```
+-------------------------------------------------------------------------+
|                    Platform API (container)                             |
|  +-------------------+    +-------------------+   +------------------+  |
|  |  HTTPS Server     | -> |   Service Layer   | ->|  WebSocket Mgr   |  |
|  | (REST + WS)       |    | (Business Logic)  |   | (Connections)    |  |
|  +-------------------+    +-------------------+   +------------------+  |
|           |                      |                         |            |
|           v                      v                         v            |
|      +----------+        +---------------+         +--------------+     |
|      | Router   |        | Repositories  |         | Gateway      |     |
|      | (Gin)    |        | (SQLite)      |         | Connections  |     |
|      +----------+        +---------------+         | (sync.Map)   |     |
|                                 |                  +--------------+     |
|                                 v                                       |
|                         +-----------------+                             |
|                         | SQLite Database |                             |
|                         | (api_platform)  |                             |
|                         +-----------------+                             |
+-------------------------------------------------------------------------+
                                   ^
                                   |
                          WebSocket (wss://)
                                   |
                                   v
                         +-------------------+
                         |  Gateway Instance |
                         +-------------------+
```

## Integration Points

- **Portals (API, Management, Enterprise)** → Platform API: fetch and mutate organization/project/API resources.
- **CLI** → Platform API: automate gateway registration and API lifecycle actions.
- **Gateway Controller** ← Platform API: receives deployment orchestration data for pushing APIs to gateways.
- **Gateways** ↔ Platform API: bidirectional WebSocket connections for real-time event notifications and heartbeat monitoring.
- **Gateways** → Platform API: authenticate using secure tokens with SHA-256 hash verification for both REST and WebSocket.
