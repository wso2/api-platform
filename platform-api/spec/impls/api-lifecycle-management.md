# API Lifecycle Management Implementation

## Entry Points

- `platform-api/src/internal/handler/api.go` – implements `/api/v1/apis` CRUD, `/api/v1/projects/:projectId/apis` listing routes, `/api/v1/apis/:apiId/deploy-revision` for API deployment, and `/api/v1/apis/:apiId/gateways` for retrieving deployment status
- `platform-api/src/internal/service/api.go` – validates names, contexts, versions, orchestrates default values and generates deployment YAML and deploys APIs in the Gateway plus repository calls. Also handles gateway deployment queries.
- `platform-api/src/internal/repository/api.go` – persists APIs, security, CORS, backend services, rate limiting, and operations using transactions.
- `platform-api/src/internal/repository/gateway.go` – handles gateway operations including querying which gateways have specific APIs deployed.
- `platform-api/src/internal/database/schema.sql` – contains tables for APIs, security configs, backend services, rate limits, operations, and API deployments tracking.
- `platform-api/src/resources/openapi.yaml` – provides the published API lifecycle contract for client integrations.

## Behaviour

1. Create requests require name, context, version, and project ID, then enforce uniqueness within the project and seed defaults (provider, transport, lifecycle, operations).
2. Repository layer writes the main API record and related configuration tables within a single transaction.
3. GET routes return fully hydrated API structures, including nested security and backend definitions.
4. Update requests replace mutable fields and rebuild related configuration sets; deletes cascade via foreign keys.
5. Gateway deployment tracking uses the `api_deployments` table to maintain relationships between APIs and gateways.
6. The gateways endpoint queries deployed APIs by joining API deployments with gateway records, filtered by organization for security.
7. Generates comprehensive deployment API YAML including:
    - API metadata and configuration
    - Security policies (mTLS, OAuth2, API Key)
    - API-level and operation-level policies

## Verification
- Create: `curl -k -X POST https://localhost:9243/api/v1/apis -H 'Content-Type: application/json' -d '{"name":"inventory","context":"/inventory","version":"v1","projectId":"<projectId>"}'`.
- Fetch: `curl -k https://localhost:9243/api/v1/apis/<apiId>`; confirm nested structures.
- List: `curl -k https://localhost:9243/api/v1/projects/<projectId>/apis` to verify pagination metadata and entries.
- Deploy API: `curl -k -X POST https://localhost:9243/api/v1/apis/<apiId>/deploy-revision -H 'Content-Type: application/json' -d '[{"name": "production-deployment","gatewayId": "987e6543-e21b-45d3-a789-426614174999", "displayOnDevportal": true}]'` to trigger API deployment.
- Get API Gateways: `curl -k https://localhost:9243/api/v1/apis/<apiId>/gateways` to retrieve all gateways where the API is deployed; expect JSON array with gateway details (id, name, displayName, vhost, isActive, etc.).
