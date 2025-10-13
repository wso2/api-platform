# Organization Management Implementation

## Entry Points

- `platform-api/src/internal/handler/organization.go` – exposes `/api/v1/organizations` routes.
- `platform-api/src/internal/service/organization.go` – validates handles, coordinates repository calls, and provisions default projects.
- `platform-api/src/internal/repository/organization.go` – performs SQL CRUD operations.
- `platform-api/src/internal/database/schema.sql` – defines `organizations` table and related indexes.
- `platform-api/src/resources/openapi.yaml` – documents the organization endpoints for reference consumers.

## Behaviour

1. POST requests bind to `dto.Organization`, ensuring handle presence before calling the service.
2. Service enforces lowercase URL-friendly handles and uniqueness checks via repository lookups.
3. Upon creation, service inserts the organization and immediately creates a default project.
4. GET requests fetch by UUID, returning `404` when the organization is absent.

## Verification

- Create: `curl -k -X POST https://localhost:8443/api/v1/organizations -d '{"handle":"alpha","name":"Alpha"}' -H 'Content-Type: application/json'`.
- Fetch: `curl -k https://localhost:8443/api/v1/organizations/<uuid>`; expect JSON payload with organization metadata (handle, name, timestamps).
