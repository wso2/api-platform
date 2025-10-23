# Gateway Product Requirements

## Product Overview

Production-ready Envoy-based gateway with Go xDS control plane, providing dynamic API configuration management through declarative YAML/JSON interface with zero-downtime updates and policy-based traffic management.

## Functional Requirements

- [FR1: API Configuration Management](prds/api-configuration-management.md) – Accept, validate, and persist API configurations via REST API with composite key `(name, version)` identity and structured error reporting.
- [FR2: xDS Server](prds/xds-server.md) – Implement Envoy xDS v3 State-of-the-World protocol for dynamic Router configuration with sub-5-second update propagation.
- [FR3: SQLite Persistence](prds/sqlite-persistence.md) – Persist configurations to SQLite database with WAL mode, composite unique constraints, and migration path to PostgreSQL/MySQL.
- [FR4: Zero-Downtime Updates](prds/zero-downtime-updates.md) – Apply configuration changes without dropping in-flight requests using graceful xDS updates.
- [FR5: Policy Engine Integration](prds/policy-engine.md) – Policy-first architecture with authentication, authorization, rate limiting, and custom policy support.

## Non-Functional Requirements

### NFR1: Performance
Configuration validation completes in <1 second; xDS updates propagate within 5 seconds; supports 100+ API configurations per instance.

### NFR2: Reliability
SQLite ACID transactions ensure data consistency; Router waits indefinitely with exponential backoff if xDS unavailable; graceful shutdown with WAL checkpoint.

### NFR3: Operability
Docker container deployment with volume mounts for data directory; environment variable configuration with `GATEWAY_` prefix; structured JSON logging to stdout; health check endpoints.
