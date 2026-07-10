# Platform API

Backend service that powers the API Platform portals, gateways, and automation flows.

## Quick Start

### Prerequisites

Before using the Platform API, obtain a bearer token for authentication. In local JWT mode (default) you can generate a token using the configured `AUTH_JWT_SECRET_KEY`. In IDP mode, obtain a token from your identity provider.

### Build and Run

```bash
# Build
cd platform-api
go build ./cmd/main.go

# Run (TLS with self-signed certificates)
cd platform-api
go run ./cmd/main.go
```

### Database Configuration

Platform API supports `sqlite3` (default), `postgres`, and `sqlserver`.

```bash
# SQL Server example
export DATABASE_DRIVER=sqlserver
export DATABASE_HOST=sqlserver.example.internal
export DATABASE_PORT=1433
export DATABASE_NAME=platform_api
export DATABASE_USER=sa
export DATABASE_PASSWORD='<strong-password>'
export DATABASE_SSL_MODE=disable

cd platform-api
go run ./cmd/main.go
```

### Step-by-Step Workflow

Across the API, resources with a handle expose it as `id` (an immutable, URL-safe
slug), with a separate human-readable `displayName`. Path parameters are
handle-based, not UUIDs — e.g. `{projectId}`, `{gatewayId}`, `{restApiId}` are
all handles. See [`src/resources/openapi.yaml`](src/resources/openapi.yaml)
for the full contract.

**1. Register an Organization**

```bash
curl -k -X POST https://localhost:9243/api/v0.9/organizations \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <your-token>' \
  -d '{"id":"acme","displayName":"ACME Corporation","region":"us-east-1"}'
```

**2. Create a Project**

```bash
curl -k -X POST https://localhost:9243/api/v0.9/projects \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <your-token>' \
  -d '{
    "displayName": "Production APIs"
  }'
```

Response includes the project handle, auto-generated from `displayName` if `id` is omitted:
```json
{
  "id": "production-apis",
  "displayName": "Production APIs",
  "organizationId": "acme",
  "createdAt": "2026-06-21T15:12:44+05:30",
  "updatedAt": "2026-06-21T15:12:44+05:30"
}
```

**3. Create a Gateway**

```bash
curl -k -X POST https://localhost:9243/api/v0.9/gateways \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer <your-token>' \
  -d '{
    "id": "prod-gateway-01",
    "displayName": "Production Gateway 01",
    "endpoints": ["https://prod-gateway-01.example.com:8443/api/v1"],
    "functionalityType": "regular"
  }'
```

Response includes the gateway handle (used as `{gatewayId}` in all subsequent calls):
```json
{
  "id": "prod-gateway-01",
  "displayName": "Production Gateway 01",
  "organizationId": "acme",
  "endpoints": ["https://prod-gateway-01.example.com:8443/api/v1"],
  "functionalityType": "regular",
  "isCritical": false,
  "version": "1.0",
  "isActive": false,
  "createdAt": "2026-06-21T15:12:44+05:30",
  "updatedAt": "2026-06-21T15:12:44+05:30"
}
```

**4. Generate Gateway Token**

```bash
curl -k -X POST https://localhost:9243/api/v0.9/gateways/prod-gateway-01/tokens \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer <your-token>'
```

Response includes the gateway authentication token:
```json
{
  "id": "7ed55286-66a4-43ae-9271-bd1ead475a55",
  "token": "QY8Rnm9bJ-incsGU0xtFz2vx16I1IVhEf0Ma_4O5F9s",
  "createdAt": "2026-06-21T15:12:57+05:30",
  "message": "New token generated successfully. Old token remains active until revoked."
}
```

**List Gateway Tokens:**
```bash
curl -k https://localhost:9243/api/v0.9/gateways/prod-gateway-01/tokens \
  -H 'Authorization: Bearer <your-token>'
```

Returns a bare array of token summaries (`[{"id": "...", "status": "active", "createdAt": "...", "revokedAt": null}]`) — token hashes are never exposed.

**5. Connect Gateway to Platform (WebSocket)**

Install wscat if not already installed:
```bash
npm install -g wscat
```

Connect using the gateway token:
```bash
wscat -n -c wss://localhost:9243/api/internal/v1/ws/gateways/connect \
  -H "api-key: <gateway-token>"
```

Expected output:
```
Connected (press CTRL+C to quit)
< {"type":"connection.ack","gatewayId":"4dac93bd-07ba-417e-aef8-353cebe3ba73","connectionId":"3150a8b6-649d-4d12-8512-7d72e8ec7f13","timestamp":"2026-06-21T14:42:13+05:30"}
```

Note: `gatewayId` on WebSocket events is the gateway's internal UUID, not the
handle returned by the REST API — the gateway itself doesn't need to know its
handle.

Keep this connection open to receive real-time deployment events.

**6. Create an API**

```bash
curl -k -X POST 'https://localhost:9243/api/v0.9/rest-apis' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <your-token>' \
  -d '{
      "id": "weather-api",
      "displayName": "Weather API",
      "description": "Weather API with main and sandbox upstreams",
      "context": "weather",
      "version": "1.0.0",
      "projectId": "production-apis",
      "lifeCycleStatus": "CREATED",
      "transport": ["http","https"],
      "upstream": {
         "main": { "url": "http://sample-backend:5000" },
         "sandbox": { "url": "http://sample-backend:5000/sandbox" }
       }
    }'
```

`projectId` is the project's handle (from step 2), not its UUID.

**7. Deploy API to Gateway**

```bash
curl -k -X POST 'https://localhost:9243/api/v0.9/rest-apis/weather-api/deployments' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer <your-token>' \
  -d '{
    "name": "weather-v1-prod",
    "base": "current",
    "gatewayId": "prod-gateway-01",
    "metadata": {
      "vhostMain": "example.wso2.com",
      "vhostSandbox": "sand-example.wso2.com"
    }
}'
```

`gatewayId` is the gateway's handle (from step 3), not its UUID.

Expected response:
```json
{
  "deploymentId": "90d10e1c-8560-5c36-9d5a-124ecaa17485",
  "name": "weather-v1-prod",
  "gatewayId": "prod-gateway-01",
  "status": "DEPLOYED",
  "metadata": {
    "vhostMain": "example.wso2.com",
    "vhostSandbox": "sand-example.wso2.com"
  },
  "createdAt": "2026-06-21T16:15:18+05:30",
  "updatedAt": "2026-06-21T16:15:18+05:30",
  "baseDeploymentId": null
}
```

The connected gateway will receive a deployment event via WebSocket:
```
< {"type":"api.deployed","payload":{"apiId":"54588845-c860-4a56-8802-c06b03028543","deploymentId":"90d10e1c-8560-5c36-9d5a-124ecaa17485","performedAt":"2026-06-21T16:15:18+05:30"},"gatewayId":"4dac93bd-07ba-417e-aef8-353cebe3ba73","timestamp":"2026-06-21T16:15:18+05:30","correlationId":"ae7488ec-9559-4a81-bddd-b85e1391d2c0"}
```

`apiId` and `gatewayId` in the event payload are internal UUIDs, distinct from the handle-based `id` used in the REST API.

## Configuration

All configuration is supplied via environment variables.

### Authentication

Two authentication modes are supported. Exactly one should be active at a time.

```
AUTH_IDP_ENABLED=false (default)  →  Local JWT mode  (HMAC signature verification)
AUTH_IDP_ENABLED=true             →  IDP mode        (JWKS-based verification)
```

> **Demo mode (`APIP_DEMO_MODE`).** Defaults to `true`; an explicit `false`/`0` opts into
> production-grade startup checks. With demo mode off, the server will not fall back to an
> ephemeral encryption key — you must set `ENCRYPTION_KEY` or `ENCRYPTION_KEY_FILE` — and it
> warns loudly if `AUTH_JWT_SKIP_VALIDATION=true`.

---

#### Local JWT Mode (default)

The server signs and validates HMAC login tokens using `AUTH_JWT_SECRET_KEY` — a 32-byte key (64 hex chars or base64). Set `AUTH_JWT_SKIP_VALIDATION=true` only in local development environments where you do not have a token issuer available — all bearer values will be accepted without any signature check.

| Variable | Default | Description                                                         |
|---|---|---------------------------------------------------------------------|
| `AUTH_JWT_SECRET_KEY` | _(empty)_ | HMAC key for signing/verifying login JWTs — 32-byte value (64 hex or base64; `openssl rand -hex 32`). Required in production; demo generates an ephemeral one. |
| `AUTH_JWT_ISSUER` | `platform-api` | Expected `iss` claim value                                          |
| `AUTH_JWT_SKIP_VALIDATION` | `false` | Skip signature verification — **development only**                  |
| `DEV_MODE` | `false` | Suppresses the startup warning when `AUTH_JWT_SKIP_VALIDATION=true` |

Local development with no token issuer:
```bash
export AUTH_JWT_SKIP_VALIDATION=true
export DEV_MODE=true
go run ./cmd/main.go
```

Production with HMAC verification:
```bash
export AUTH_JWT_SECRET_KEY=<strong-random-key>
export AUTH_JWT_ISSUER=https://your-token-issuer
go run ./cmd/main.go
```

**Legacy variable names** (still accepted, deprecated):

| Old name | New name |
|---|---|
| `JWT_SECRET_KEY` | `AUTH_JWT_SECRET_KEY` |
| `JWT_ISSUER` | `AUTH_JWT_ISSUER` |
| `JWT_SKIP_VALIDATION` | `AUTH_JWT_SKIP_VALIDATION` |
| `JWT_SKIP_PATHS` | `AUTH_SKIP_PATHS` |

---

#### IDP Mode

Tokens are validated against any standards-compliant identity provider (Thunder, Asgardeo, Keycloak, Azure AD, Okta, etc.) using its JWKS endpoint. Set `AUTH_IDP_ENABLED=true` and supply at minimum `AUTH_IDP_JWKS_URL` and `AUTH_IDP_ISSUER`.

| Variable | Default | Description |
|---|---|---|
| `AUTH_IDP_ENABLED` | `false` | Set to `true` to activate IDP mode |
| `AUTH_IDP_NAME` | _(empty)_ | Optional label shown in startup logs (e.g. `thunder`, `asgardeo`) |
| `AUTH_IDP_JWKS_URL` | _(required)_ | IDP's JWKS endpoint for public key retrieval |
| `AUTH_IDP_ISSUER` | _(required)_ | Accepted JWT issuer |
| `AUTH_IDP_AUDIENCE` | _(empty)_ | Accepted JWT audience. When set, the token's `aud` claim must contain this value; empty skips the check |
| `AUTH_IDP_ORGANIZATION_CLAIM_NAME` | `organization` | JWT claim holding the org UUID for the active session |
| `AUTH_IDP_ORG_NAME_CLAIM_NAME` | `org_name` | JWT claim for the org display name |
| `AUTH_IDP_ORG_HANDLE_CLAIM_NAME` | `org_handle` | JWT claim for the org URL-safe handle |
| `AUTH_IDP_USER_ID_CLAIM_NAME` | `sub` | JWT claim used as the canonical user identifier |
| `AUTH_IDP_USERNAME_CLAIM_NAME` | `username` | JWT claim for the human-readable username |
| `AUTH_IDP_EMAIL_CLAIM_NAME` | `email` | JWT claim for the user's email address |
| `AUTH_IDP_SCOPE_CLAIM_NAME` | `scope` | JWT claim carrying granted OAuth2 scopes |
| `AUTH_IDP_VALIDATION_MODE` | `scope` | Authorization mode: `scope` (validate scope claim directly) or `role` (expand IDP roles to platform roles) |
| `AUTH_IDP_ROLES_CLAIM_PATH` | _(empty)_ | Dot-notation path to the roles claim (e.g. `realm_access.roles`). Required when `AUTH_IDP_VALIDATION_MODE=role` |
| `AUTH_IDP_ROLE_MAPPINGS` | _(empty)_ | Comma-separated `idp-role=platform-role` pairs (e.g. `PLATFORM_ADMIN=admin,PLATFORM_DEV=developer`). When empty, IDP role values are used as-is |

**Example — Asgardeo:**
```bash
export AUTH_IDP_ENABLED=true
export AUTH_IDP_NAME=asgardeo
export AUTH_IDP_JWKS_URL=https://api.asgardeo.io/t/<org>/oauth2/jwks
export AUTH_IDP_ISSUER=https://api.asgardeo.io/t/<org>/oauth2/token
export AUTH_IDP_AUDIENCE=<client-id>
export AUTH_IDP_ORGANIZATION_CLAIM_NAME=organizationId
export AUTH_IDP_VALIDATION_MODE=scope
export AUTH_IDP_ROLES_CLAIM_PATH=scope
```

---

#### Skip Paths

Path prefixes listed here bypass authentication entirely. Used for internal gateway traffic and health checks.

| Variable | Default |
|---|---|
| `AUTH_SKIP_PATHS` | `/health,/metrics,/api/internal/v1/ws/gateways/connect,...` |

To extend the default list:
```bash
export AUTH_SKIP_PATHS="/health,/metrics,/api/internal/v1/ws/gateways/connect,/my-custom-path"
```

---

### Role-Based Access Control (RBAC)

Per-route scope checks are enforced when `ENABLE_SCOPE_VALIDATION=true`. Five built-in platform roles exist:

| Role | Persona | Access level |
|---|---|---|
| `admin` | Platform administrator | Full access to all resources and operations |
| `developer` | API designer | Full API lifecycle; cannot manage gateways or subscription plans |
| `publisher` | DevPortal manager | Read APIs and publish/unpublish to DevPortals; cannot create or deploy |
| `operator` | CI/CD service account | Deploy and undeploy operations only; cannot create resources or manage credentials |
| `viewer` | Auditor | Read-only access to all resources |

| Variable | Default | Description |
|---|---|---|
| `ENABLE_SCOPE_VALIDATION` | `false` | Set to `true` to enforce per-route scope/role checks |

In **local JWT mode**, scopes are read directly from the `scope` claim in the token.  
In **IDP mode with `AUTH_IDP_VALIDATION_MODE=scope`**, scopes are read from the claim named by `AUTH_IDP_SCOPE_CLAIM_NAME`.  
In **IDP mode with `AUTH_IDP_VALIDATION_MODE=role`**, IDP roles are resolved from `AUTH_IDP_ROLES_CLAIM_PATH`, mapped via `AUTH_IDP_ROLE_MAPPINGS`, and matched against the required roles for each route.

---

### Database

| Variable | Default | Description |
|---|---|---|
| `DATABASE_DRIVER` | `sqlite3` | `sqlite3` or `postgres` |
| `DATABASE_DB_PATH` | `./data/api_platform.db` | SQLite file path (ignored for Postgres) |
| `DATABASE_HOST` | `localhost` | Postgres host |
| `DATABASE_PORT` | `5432` | Postgres port |
| `DATABASE_NAME` | `platform_api` | Postgres database name |
| `DATABASE_USER` | _(empty)_ | Postgres username |
| `DATABASE_PASSWORD` | _(empty)_ | Postgres password |
| `DATABASE_SSL_MODE` | `disable` | Postgres SSL mode (`disable`, `require`, `verify-full`) |
| `DATABASE_EXECUTE_SCHEMA_DDL` | `true` | Set to `false` when the DB user lacks DDL privileges |

---

### Encryption

A single key protects all at-rest encryption (secrets, subscription tokens, WebSub HMAC secrets). Provide **exactly one** of `ENCRYPTION_KEY` or `ENCRYPTION_KEY_FILE`.

| Variable | Default | Description |
|---|---|---|
| `ENCRYPTION_KEY` | _(empty)_ | 32-byte AES-256 key as 64 hex chars or base64 (32 bytes). Generate with `openssl rand -hex 32`. |
| `ENCRYPTION_KEY_FILE` | _(empty)_ | Path to a 32-byte binary key file (read on every start). Mutually exclusive with `ENCRYPTION_KEY`. |

In **demo mode** (default), if neither is set a key file is auto-generated next to the SQLite database (`<db-dir>/secret-encryption.key`) and reused on restart. In **production** (`APIP_DEMO_MODE=false`), one of the two must be provided or startup fails — a key is never auto-generated.

---

### Other Settings

| Variable | Default | Description |
|---|---|---|
| `PORT` | `9243` | HTTP/HTTPS server port |
| `LOG_LEVEL` | `DEBUG` | Log verbosity (`DEBUG`, `INFO`, `WARN`, `ERROR`) |
| `TLS_CERT_DIR` | `./data/certs` | Directory for TLS certificates |
| `DEPLOYMENTS_MAX_PER_API_GATEWAY` | `20` | Maximum deployments per API per gateway |
| `DEPLOYMENTS_TRANSITIONAL_STATUS_ENABLED` | `false` | Show `DEPLOYING`/`UNDEPLOYING` status before gateway ack |
| `ARTIFACT_LIMITS_MAX_LLM_PROVIDERS_PER_ORG` | _unlimited_ | Max LLM providers per organization (`0` or unset = unlimited) |
| `ARTIFACT_LIMITS_MAX_LLM_PROXIES_PER_ORG` | _unlimited_ | Max LLM proxies per organization (`0` or unset = unlimited) |
| `ARTIFACT_LIMITS_MAX_MCP_PROXIES_PER_ORG` | _unlimited_ | Max MCP proxies per organization (`0` or unset = unlimited) |
| `ARTIFACT_LIMITS_MAX_WEBSUB_APIS_PER_ORG` | _unlimited_ | Max WebSub APIs per organization (`0` or unset = unlimited) |
| `ARTIFACT_LIMITS_MAX_WEBBROKER_APIS_PER_ORG` | _unlimited_ | Max WebBroker APIs per organization (`0` or unset = unlimited) |
| `GATEWAY_ENABLE_VERSION_VERIFICATION` | `false` | Reject gateway connections with mismatched versions |
| `API_KEY_HASHING_ALGORITHMS` | `sha256` | Comma-separated hash algorithms for API key storage |

## Documentation

See [spec/](spec/) for product, architecture, design, and implementation documentation.
