# Organization Management Implementation

## Entry Points

- `platform-api/src/internal/handler/organization.go` – exposes `/api/v1/organizations` routes.
- `platform-api/src/internal/service/organization.go` – validates handles, coordinates repository calls, and provisions default projects.
- `platform-api/src/internal/repository/organization.go` – performs SQL CRUD operations.
- `platform-api/src/internal/database/schema.sql` – defines `organizations` table and related indexes.
- `platform-api/src/resources/openapi.yaml` – documents the organization endpoints for reference consumers.

## Behaviour

1. POST requests bind to `dto.Organization`, ensuring UUID, handle, name and region presence before calling the service.
2. Service enforces lowercase URL-friendly handles and uniqueness checks via repository lookups for both ID and handle.
3. Upon registration, service inserts the organization with region information and immediately creates a default project.
4. GET requests fetch by UUID, returning `404` when the organization is absent.

## Verification

- Register: `curl -k -X POST https://localhost:8443/api/v1/organizations -d '{"id":"123e4567-e89b-12d3-a456-426614174000","handle":"alpha","name":"Alpha","region":"us-east-1"}' -H 'Content-Type: application/json'`.
- Fetch: `curl -k https://localhost:8443/api/v1/organizations/<orgId>`; expect JSON payload with organization metadata (handle, name, region, timestamps).
