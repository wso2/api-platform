# Platform API Design

## Overview

Service follows a clean layering pattern that separates HTTP handling, business rules, and persistence while relying on environment-driven configuration.

## Components

- **Configuration**: `config.GetConfig` loads environment variables once and seeds database settings.
- **Server Wiring**: `internal/server.StartPlatformAPIServer` constructs repositories, services, and Gin routes.
- **Domain Services**: Organization, project, and API services encapsulate validation and cross-entity coordination.
- **Persistence**: Repositories execute SQL statements and manage cascading writes in transactions.

## Key Decisions

1. **Singleton Configuration** – Use `sync.Once` to prevent repeated env parsing and guarantee consistent runtime configuration.
2. **SQLite Default** – Provide a single-file database for local development; abstraction allows future drivers.
3. **Gin Framework** – Leverages performant routing and JSON handling while keeping middleware extensible.
4. **Transactional API Writes** – Complex API structures (security, operations) persist within a single transaction to maintain integrity.
5. **TLS by Default** – Service always starts with HTTPS, generating self-signed certificates when none are supplied to encourage secure defaults.
