# Project Workspace Management Implementation

## Entry Points

- `platform-api/src/internal/handler/project.go` – registers `/api/v1/projects` and `/api/v1/organizations/:org_uuid/projects` routes.
- `platform-api/src/internal/service/project.go` – handles validation, duplicate checks, and default project protections.
- `platform-api/src/internal/repository/project.go` – executes SQL CRUD operations scoped to organizations.
- `platform-api/src/internal/database/schema.sql` – defines the `projects` table with foreign key and index support.

## Behaviour

1. Create requests validate presence of `name` and `organization_id`, then confirm organization existence and uniqueness within that org.
2. Service prevents multiple default projects and protects the default project from deletion.
3. Listing routes return all projects for an organization; update routes enforce uniqueness before persisting.
4. Delete operations return `400` when attempting to remove the default project.

## Verification

- Create: `curl -k -X POST https://localhost:8443/api/v1/projects -H 'Content-Type: application/json' -d '{"name":"Beta","organization_id":"<org_uuid>"}'`.
- List: `curl -k https://localhost:8443/api/v1/organizations/<org_uuid>/projects`.
- Delete default check: attempt to delete the default project's UUID and expect `400`.
