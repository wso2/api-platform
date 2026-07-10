# API Lifecycle Management Implementation

## Entry Points

- `platform-api/internal/handler/api.go` – implements `/api/v0.9/rest-apis` CRUD, `/api/v0.9/projects/:projectId/apis` listing routes, `/api/v0.9/rest-apis/:apiId/deployments` for API deployment, and `/api/v0.9/rest-apis/:apiId/gateways` for retrieving deployment status
- `platform-api/internal/service/api.go` – validates names, contexts, versions, orchestrates default values and generates deployment YAML and deploys APIs in the Gateway plus repository calls. Also handles gateway deployment queries.
- `platform-api/internal/repository/api.go` – persists APIs, security, CORS, backend services, rate limiting, and operations using transactions.
- `platform-api/internal/repository/gateway.go` – handles gateway operations including querying which gateways have specific APIs deployed.
- `platform-api/internal/database/schema.sql` – contains tables for APIs, security configs, backend services, rate limits, operations, and API deployments tracking.
- `platform-api/resources/openapi.yaml` – provides the published API lifecycle contract for client integrations.

## Behaviour

1. Create requests require `displayName`, `context`, `version`, `projectId` (project handle), and `upstream`, then enforce uniqueness within the project and seed defaults (`id` handle auto-generated if omitted, transport, lifecycle status, operations).
2. Repository layer writes the main API record and related configuration tables within a single transaction.
3. GET routes (`{restApiId}` = API handle) return fully hydrated API structures, including nested security and backend definitions.
4. Update requests replace mutable fields and rebuild related configuration sets; the handle (`id`) is immutable — a PUT body `id` that differs from the path handle returns `400`. Deletes cascade via foreign keys.
5. Gateway deployment tracking uses the `deployments` table to maintain immutable deployment artifacts per API/gateway pair, with lifecycle status (`DEPLOYED`, `UNDEPLOYED`, `DEPLOYING`, `UNDEPLOYING`, `FAILED`, `ARCHIVED`).
6. The `/gateways` sub-resource queries deployed APIs by joining deployments with gateway records, filtered by organization for security.
7. Deployment generates the gateway-facing deployment artifact including:
    - API metadata and configuration
    - Security policies (mTLS, OAuth2, API Key)
    - API-level and operation-level policies

## Verification
- Create: `curl -k -X POST https://localhost:9243/api/v0.9/rest-apis -H 'Content-Type: application/json' -H 'Authorization: Bearer <your-token>' -d '{"displayName":"inventory","context":"inventory","version":"1.0","projectId":"<projectHandle>","upstream":{"main":{"url":"http://sample-backend:5000"}}}'`.
- Fetch: `curl -k https://localhost:9243/api/v0.9/rest-apis/<apiHandle> -H 'Authorization: Bearer <your-token>'`; confirm nested structures.
- List: `curl -k 'https://localhost:9243/api/v0.9/rest-apis?projectId=<projectHandle>' -H 'Authorization: Bearer <your-token>'` to verify the `{list, pagination}` envelope.
- Deploy API: `curl -k -X POST https://localhost:9243/api/v0.9/rest-apis/<apiHandle>/deployments -H 'Content-Type: application/json' -H 'Authorization: Bearer <your-token>' -d '{"name":"production-deployment","base":"current","gatewayId":"<gatewayHandle>"}'` to trigger API deployment.
- Get API Gateways: `curl -k https://localhost:9243/api/v0.9/rest-apis/<apiHandle>/gateways -H 'Authorization: Bearer <your-token>'` to retrieve all gateways where the API is deployed; expect a `RESTAPIGatewayListResponse` with gateway details (id, displayName, endpoints, isActive, etc.) plus deployment status.
