# Platform API Design

## Overview

Service follows a clean layering pattern that separates HTTP handling, business rules, and persistence while relying on environment-driven configuration.

## Components

- **Configuration**: `config.GetConfig` loads environment variables once and seeds database settings.
- **Server Wiring**: `internal/server.StartPlatformAPIServer` constructs repositories, services, and Gin routes.
- **Domain Services**: Organization, project, gateway, and API services encapsulate validation and cross-entity coordination.
- **Persistence**: Repositories execute SQL statements and manage cascading writes in transactions.

## Key Decisions

1. **Singleton Configuration** – Use `sync.Once` to prevent repeated env parsing and guarantee consistent runtime configuration.
2. **SQLite Default** – Provide a single-file database for local development; abstraction allows future drivers.
3. **Gin Framework** – Leverages performant routing and JSON handling while keeping middleware extensible.
4. **Transactional API Writes** – Complex API structures (security, operations) persist within a single transaction to maintain integrity.
5. **TLS by Default** – Service always starts with HTTPS, generating self-signed certificates when none are supplied to encourage secure defaults.
6. **Gateway Token Security** – SHA-256 hash with unique salt per token, constant-time verification using `crypto/subtle`, 32-byte tokens from `crypto/rand`, never store plain-text.
7. **Zero-Downtime Token Rotation** – Maximum 2 active tokens allows overlap period for gateway reconfiguration without service interruption.
8. **Organization-Scoped Uniqueness** – Composite unique constraint `(organization_id, name)` prevents duplicate gateway names within organizations while allowing cross-organization reuse.
9. **WebSocket Transport Abstraction** – Interface-based design decouples protocol from business logic enabling future protocol changes without modifying connection management.
10. **sync.Map for Connection Registry** – Read-optimized concurrent map reduces lock contention for frequent event delivery lookups while supporting per-gateway clustering.
11. **Per-Connection Heartbeat Goroutines** – Isolates connection monitoring preventing cascade failures and enabling graceful shutdown via context cancellation.
12. **Partial Delivery Success Model** – API deployments succeed if any gateway connection receives the event balancing availability over strict consistency.
13. **No Event Persistence** – Clean slate reconnection design where gateways sync full state after connection avoiding complex replay logic and storage overhead.
