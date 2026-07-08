# Organization Management Implementation

## Entry Points

- `platform-api/internal/handler/organization.go` – exposes `/api/v0.9/organizations` routes.
- `platform-api/internal/service/organization.go` – validates handles, coordinates repository calls, and provisions default projects.
- `platform-api/internal/repository/organization.go` – performs SQL CRUD operations.
- `platform-api/internal/database/schema.sql` – defines `organizations` table and related indexes.
- `platform-api/resources/openapi.yaml` – documents the organization endpoints for reference consumers.

## Behaviour

1. POST requests bind to the `Organization` schema, requiring `id` (handle), `displayName`, and `region` before calling the service.
2. Service enforces lowercase URL-friendly handles and uniqueness checks via repository lookups for both handle and UUID.
3. Upon registration, service inserts the organization with region information and immediately creates a default project.
4. GET/HEAD requests fetch by handle (`{organizationId}` path param), returning `404` when the organization is absent.

## Verification

- Register: `curl -k -X POST https://localhost:9243/api/v0.9/organizations -d '{"id":"alpha","displayName":"Alpha","region":"us"}' -H 'Content-Type: application/json'`.
- Fetch: `curl -k https://localhost:9243/api/v0.9/organizations/alpha`; expect JSON payload with organization metadata (`id`, `displayName`, `region`, timestamps).
