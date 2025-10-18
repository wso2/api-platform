# Project Management Implementation

## Entry Points

- `platform-api/src/internal/handler/project.go` – registers `/api/v1/projects` and `/api/v1/organizations/:orgId/projects` routes.
- `platform-api/src/internal/service/project.go` – handles validation, duplicate checks, and deletion constraints (last project, projects with APIs).
- `platform-api/src/internal/repository/project.go` – executes SQL CRUD operations scoped to organizations.
- `platform-api/src/internal/database/schema.sql` – defines the `projects` table with foreign key and index support.
- `platform-api/src/resources/openapi.yaml` – captures the project management operations surfaced to clients.

## Behaviour

1. Create requests validate presence of `name` and `organizationId`, then confirm organization existence and uniqueness within that org.
2. Service blocks duplicate project names per organization and prevents deleting the last remaining project or one that still owns APIs.
3. Listing routes return all projects for an organization; update routes enforce uniqueness before persisting.
4. Delete operations guard the constraints above and return informative errors when a project cannot be removed.

## Verification

- Create: `curl -k -X POST https://localhost:8443/api/v1/projects -H 'Content-Type: application/json' -d '{"name":"Beta","organizationId":"<orgId>"}'`.
- List: `curl -k https://localhost:8443/api/v1/organizations/<orgId>/projects`.
- Delete guards:
  - Attempt to delete the only project in an organization and expect a `400` response.
  - Attempt to delete a project that still has APIs attached and expect a `400` response.
