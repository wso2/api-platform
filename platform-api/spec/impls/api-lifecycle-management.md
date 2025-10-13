# API Lifecycle Management Implementation

## Entry Points

- `platform-api/src/internal/handler/api.go` – implements `/api/v1/apis` CRUD and `/api/v1/projects/:project_uuid/apis` listing routes.
- `platform-api/src/internal/service/api.go` – validates names, contexts, versions, and orchestrates default values plus repository calls.
- `platform-api/src/internal/repository/api.go` – persists APIs, security, CORS, backend services, rate limiting, and operations using transactions.
- `platform-api/src/internal/database/schema.sql` – contains tables for APIs, security configs, backend services, rate limits, and operations.

## Behaviour

1. Create requests require name, context, version, and project ID, then enforce uniqueness within the project and seed defaults (provider, transport, lifecycle, operations).
2. Repository layer writes the main API record and related configuration tables within a single transaction.
3. GET routes return fully hydrated API structures, including nested security and backend definitions.
4. Update requests replace mutable fields and rebuild related configuration sets; deletes cascade via foreign keys.

## Verification

- Create: `curl -k -X POST https://localhost:8443/api/v1/apis -H 'Content-Type: application/json' -d '{"name":"inventory","context":"/inventory","version":"v1","project_id":"<project_uuid>"}'`.
- Fetch: `curl -k https://localhost:8443/api/v1/apis/<api_uuid>`; confirm nested structures.
- List: `curl -k https://localhost:8443/api/v1/projects/<project_uuid>/apis` to verify pagination metadata and entries.
