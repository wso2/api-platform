# API Lifecycle Management Implementation

## Entry Points

- `platform-api/src/internal/handler/api.go` – implements `/api/v1/apis` CRUD, `/api/v1/projects/:projectId/apis` listing routes and `/api/v1/apis/:apiId/deploy-revision` for API deployment
- `platform-api/src/internal/service/api.go` – validates names, contexts, versions, orchestrates default values and generates deployment YAML and deploys APIs in the Gateway plus repository calls.
- `platform-api/src/internal/repository/api.go` – persists APIs, security, CORS, backend services, rate limiting, and operations using transactions.
- `platform-api/src/internal/database/schema.sql` – contains tables for APIs, security configs, backend services, rate limits, and operations.
- `platform-api/src/resources/openapi.yaml` – provides the published API lifecycle contract for client integrations.

## Behaviour

1. Create requests require name, context, version, and project ID, then enforce uniqueness within the project and seed defaults (provider, transport, lifecycle, operations).
2. Repository layer writes the main API record and related configuration tables within a single transaction.
3. GET routes return fully hydrated API structures, including nested security and backend definitions.
4. Update requests replace mutable fields and rebuild related configuration sets; deletes cascade via foreign keys.
5. Generates comprehensive deployment API YAML including:
    - API metadata and configuration
    - Security policies (mTLS, OAuth2, API Key)
- Create: `curl -k -X POST https://localhost:8443/api/v1/apis -H 'Content-Type: application/json' -d '{"name":"inventory","context":"/inventory","version":"v1","projectId":"<projectId>"}'`.
- Fetch: `curl -k https://localhost:8443/api/v1/apis/<apiId>`; confirm nested structures.
- List: `curl -k https://localhost:8443/api/v1/projects/<projectId>/apis` to verify pagination metadata and entries.
- Deploy API: `curl -k -X POST https://localhost:8443/api/v1/apis/<apiId>/deploy-revision -H 'Content-Type: application/json' -d '[{"name": "production-deployment","vhost": "api.production.com", "displayOnDevportal": true}]'` to trigger API deployment.
