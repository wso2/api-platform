# Gateway Design

## Overview

Gateway follows a clean separation between control plane (Gateway-Controller) and data plane (Router), with policy-first architecture and environment-driven configuration.

## Components

- **Configuration**: Environment variables with `GATEWAY_` prefix override YAML config; default config file at `/etc/gateway-controller/config.yaml`.
- **Server Wiring**: `cmd/controller/main.go` initializes storage, xDS server, and Gin REST API handlers.
- **Validation Layer**: `pkg/config/validator.go` provides field-level validation with JSON path error messages.
- **xDS Translator**: `pkg/xds/translator.go` converts API configurations to Envoy Listener/Route/Cluster resources.
- **Storage Abstraction**: `pkg/storage/interface.go` defines database-agnostic operations; implementations: SQLite (persistent), Memory (transient).

## Key Decisions

1. **OpenAPI-First REST API** – Use oapi-codegen v2 to generate server interfaces and types from `api/openapi.yaml`, ensuring API contract consistency.
2. **SQLite Default Persistence** – Single-file database (`./data/gateway.db`) with WAL mode for concurrent reads during writes, suitable for single-instance deployments.
3. **Composite Key Identity** – APIs uniquely identified by `(name, version)` composite key with SQLite unique constraint preventing duplicates.
4. **State-of-the-World xDS** – Complete configuration snapshot pushed to Envoy on each change; simpler than incremental/ADS but requires full state regeneration.
5. **Policy-First Architecture** – Everything beyond basic routing (auth, rate limiting, transformations) implemented as policies in separate Policy Engine component.
6. **In-Memory Cache + Persistent Store** – Dual-layer storage: SQLite for durability, in-memory maps for fast lookup during xDS snapshot generation.
7. **Environment-Driven Config** – Support config file, environment variables (`GATEWAY_*`), and CLI flags with precedence: env vars > config file > defaults.
8. **Zero-Downtime Updates** – xDS protocol enables graceful configuration changes; Envoy drains old listeners while activating new routes without dropping connections.
9. **Database Abstraction for Future Migration** – Storage interface isolates database-specific code, enabling future PostgreSQL/MySQL support for cloud deployments without handler/xDS changes.
10. **Configuration JSON Storage** – API configurations stored as TEXT (JSON-serialized) in SQLite, facilitating migration to other SQL databases and enabling JSON querying capabilities.
11. **Graceful Shutdown with WAL Checkpoint** – On SIGINT/SIGTERM, perform SQLite WAL checkpoint to flush pending transactions before closing database.
