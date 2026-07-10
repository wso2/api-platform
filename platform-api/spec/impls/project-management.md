# Project Management Implementation

## Entry Points

- `platform-api/src/internal/handler/project.go` – registers `/api/v0.9/projects` and `/api/v0.9/projects/{projectId}` routes.
- `platform-api/src/internal/service/project.go` – handles validation, duplicate checks, and deletion constraints (last project, projects with APIs).
- `platform-api/src/internal/repository/project.go` – executes SQL CRUD operations scoped to organizations.
- `platform-api/src/internal/database/schema.sql` – defines the `projects` table with foreign key and index support.
- `platform-api/src/resources/openapi.yaml` – captures the project management operations surfaced to clients.

## Behaviour

1. Create requests validate presence of `displayName`; `organizationId` is not part of the request body — it is extracted from the JWT. The `id` (handle) is optional and auto-generated from `displayName` if omitted.
2. Service blocks duplicate `displayName` values and duplicate handles per organization, and prevents deleting the last remaining project or one that still owns APIs.
3. `GET /projects` returns all projects for the organization in the caller's JWT; `{projectId}` routes (GET/PUT/DELETE) address a project by its handle and enforce uniqueness before persisting on update.
4. Delete operations guard the constraints above and return informative errors when a project cannot be removed.

## Verification

- Create: `curl -k -X POST https://localhost:9243/api/v0.9/projects -H 'Content-Type: application/json' -H 'Authorization: Bearer <your-token>' -d '{"displayName":"Beta"}'`.
- List: `curl -k https://localhost:9243/api/v0.9/projects -H 'Authorization: Bearer <your-token>'`.
- Delete guards:
  - Attempt to delete the only project in an organization and expect a `400` response.
  - Attempt to delete a project that still has APIs attached and expect a `400` response.
