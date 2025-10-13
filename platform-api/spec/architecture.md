# Platform API Architecture

## Overview

Layered Go service exposing REST endpoints over HTTPS with SQLite persistence and integration hooks for portals, CLI, and gateway orchestration.

## Components

### HTTPS Server (Port 8443)
- Gin router serving `/api/v1/**` routes.
- TLS enabled with auto-generated self-signed certificate for development.

### Service Layer
- Business logic modules for organizations, projects, and APIs.
- Validation, defaulting, and orchestration across repositories.

### Repository Layer
- SQL repositories encapsulating CRUD operations and transactions.
- Handles relational writes for MTLS, security, rate limiting, and operations.

### Database
- SQLite database file (`./data/api_platform.db`).
- Schema bootstrapped from `internal/database/schema.sql`.

## Container Structure

```
+-------------------------------------------------------------+
|                Platform API (container)                     |
|  +-------------------+    +-------------------+             |
|  |  HTTPS Server     | -> |   Service Layer   |             |
|  +-------------------+    +-------------------+             |
|           |                      |                          |
|           v                      v                          |
|      +----------+        +---------------+                  |
|      | Router   |        | Repositories  |                  |
|      +----------+        +---------------+                  |
|                                 |                           |
|                                 v                           |
|                         +-----------------+                 |
|                         | SQLite Database |                 |
|                         +-----------------+                 |
+-------------------------------------------------------------+
```

## Integration Points

- **Portals (API, Management, Enterprise)** → Platform API: fetch and mutate tenant/project/API resources.
- **CLI** → Platform API: automate gateway registration and API lifecycle actions.
- **Gateway Controller** ← Platform API: receives deployment orchestration data for pushing APIs to gateways.
