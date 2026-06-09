# Platform API

Backend service that powers the API Platform portals, gateways, and automation flows.

## Quick Start

### Prerequisites

Before using the Platform API, obtain a bearer token for authentication. In local JWT mode (default) you can generate a token using the configured `AUTH_JWT_SECRET_KEY`. In IDP mode, obtain a token from your identity provider.

### Build and Run

```bash
# Build
cd platform-api/src
go build ./cmd/main.go

# Run (TLS with self-signed certificates)
cd platform-api/src
go run ./cmd/main.go
```

### Step-by-Step Workflow

**1. Register an Organization**

```bash
curl -k -X POST https://localhost:9243/api/v1/organizations \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <your-token>' \
  -d '{"id":"<org-uuid>","handle":"acme","name":"ACME Corporation","region":"us-east-1"}'
```

**2. Create a Project**

```bash
curl -k -X POST https://localhost:9243/api/v1/projects \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <your-token>' \
  -d '{
    "name": "Production APIs"
  }'
```

**3. Create a Gateway**

```bash
curl -k -X POST https://localhost:9243/api/v1/gateways \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer <your-token>' \
  -d '{
    "name": "prod-gateway-01",
    "displayName": "Production Gateway 01",
    "vhost": "localhost",
    "functionalityType": "regular"
  }'
```

Response includes the gateway UUID:
```json
{
  "id": "4dac93bd-07ba-417e-aef8-353cebe3ba73",
  "name": "prod-gateway-01",
  "displayName": "Production Gateway 01",
  "createdAt": "2025-10-21T15:12:44.168980842+05:30",
  "updatedAt": "2025-10-21T15:12:44.16898088+05:30"
}
```

**4. Generate Gateway Token**

```bash
curl -k -X POST https://localhost:9243/api/v1/gateways/<gateway-uuid>/tokens \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer <your-token>'
```

Response includes the gateway authentication token:
```json
{
  "id": "7ed55286-66a4-43ae-9271-bd1ead475a55",
  "token": "QY8Rnm9bJ-incsGU0xtFz2vx16I1IVhEf0Ma_4O5F9s",
  "createdAt": "2025-10-21T15:12:57.60936197+05:30",
  "message": "New token generated successfully. Old token remains active until revoked."
}
```

**List Gateway Tokens:**
```bash
curl -k https://localhost:9243/api/v1/gateways/<gateway-uuid>/tokens \
  -H 'Authorization: Bearer <your-token>'
```

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
< {"type":"connection.ack","gatewayId":"4dac93bd-07ba-417e-aef8-353cebe3ba73","connectionId":"3150a8b6-649d-4d12-8512-7d72e8ec7f13","timestamp":"2025-10-21T14:42:13+05:30"}
```

Keep this connection open to receive real-time deployment events.

**6. Create an API**

```bash
curl -k -X POST 'https://localhost:9243/api/v1/rest-apis' \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <your-token>' \
  -d '{
      "id": "weather-api",
      "name": "Weather API",
      "description": "Weather API with main and sandbox upstreams",
      "context": "/weather",
      "version": "v1.0",
      "projectId": "<project-uuid>",
      "lifeCycleStatus": "CREATED",
      "transport": ["http","https"],
      "upstream": {
         "main": { "url": "http://sample-backend:5000" },
         "sandbox": { "url": "http://sample-backend:5000/sandbox" }
       }
    }'
```

**7. Deploy API to Gateway**

```bash
curl -k -X POST 'https://localhost:9243/api/v1/rest-apis/weather-api/deployments' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer <your-token>' \
  -d '{
    "name": "weather-v1-prod",
    "base": "current",
    "gatewayId": "<gateway-uuid>",
    "vhost": {
      "main": "example.wso2.com",
      "sandbox": "sand-example.wso2.com"
    }
}'
```

Expected response:
```json
{
  "deploymentId": "90d10e1c-8560-5c36-9d5a-124ecaa17485",
  "name": "weather-v1-prod",
  "gatewayId": "4dac93bd-07ba-417e-aef8-353cebe3ba73",
  "status": "DEPLOYED",
  "vhost": {
    "main": "example.wso2.com",
    "sandbox": "sand-example.wso2.com"
  },
  "createdAt": "2025-10-21T16:15:18+05:30",
  "updatedAt": "2025-10-21T16:15:18+05:30",
  "baseDeploymentId": null
}
```

The connected gateway will receive a deployment event via WebSocket:
```
< {"type":"api.deployed","payload":{"apiId":"54588845-c860-4a56-8802-c06b03028543","revisionId":"90d10e1c-8560-5c36-9d5a-124ecaa17485","vhost":"mg.wso2.com","environment":"production"},"timestamp":"2025-10-21T16:15:18+05:30","correlationId":"ae7488ec-9559-4a81-bddd-b85e1391d2c0"}
```

## Configuration

All configuration is supplied via environment variables.

### Authentication

Two authentication modes are supported. Exactly one should be active at a time.

```
AUTH_IDP_ENABLED=false (default)  →  Local JWT mode  (HMAC signature verification)
AUTH_IDP_ENABLED=true             →  IDP mode        (JWKS-based verification)
```

---

#### Local JWT Mode (default)

The server validates HMAC-signed tokens using `AUTH_JWT_SECRET_KEY`. Set `AUTH_JWT_SKIP_VALIDATION=true` only in local development environments where you do not have a token issuer available — all bearer values will be accepted without any signature check.

| Variable | Default | Description |
|---|---|---|
| `AUTH_JWT_SECRET_KEY` | `your-secret-key-change-in-production` | HMAC signing key for token verification |
| `AUTH_JWT_ISSUER` | `platform-api` | Expected `iss` claim value |
| `AUTH_JWT_SKIP_VALIDATION` | `false` | Skip signature verification — **development only** |
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
| `AUTH_IDP_AUDIENCE` | _(empty)_ | Accepted JWT audiences (comma-separated). Entries ending with `*` are treated as prefix matches |
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

Per-route scope checks are enforced when `ENABLE_SCOPE_VALIDATION=true`. Three built-in platform roles exist:

| Role | Access level |
|---|---|
| `admin` | Full access to all resources and operations |
| `developer` | CRUD and deploy on APIs, projects, applications, and AI/integration resources |
| `viewer` | Read-only access to all resources |

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
| `DATABASE_SUBSCRIPTION_TOKEN_ENCRYPTION_KEY` | _(empty)_ | 32-byte key (64 hex or 44 base64 chars) for AES-256-GCM token encryption. |

---

### Other Settings

| Variable | Default | Description |
|---|---|---|
| `PORT` | `9243` | HTTP/HTTPS server port |
| `LOG_LEVEL` | `DEBUG` | Log verbosity (`DEBUG`, `INFO`, `WARN`, `ERROR`) |
| `TLS_CERT_DIR` | `./data/certs` | Directory for TLS certificates |
| `DEPLOYMENTS_MAX_PER_API_GATEWAY` | `20` | Maximum deployments per API per gateway |
| `DEPLOYMENTS_TRANSITIONAL_STATUS_ENABLED` | `false` | Show `DEPLOYING`/`UNDEPLOYING` status before gateway ack |
| `GATEWAY_ENABLE_VERSION_VERIFICATION` | `false` | Reject gateway connections with mismatched versions |
| `API_KEY_HASHING_ALGORITHMS` | `sha256` | Comma-separated hash algorithms for API key storage |

## Documentation

See [spec/](spec/) for product, architecture, design, and implementation documentation.
