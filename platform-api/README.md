# Platform API

Backend service that powers the API Platform portals, gateways, and automation flows.

## Quick Start

### Prerequisites

**Setup OAuth2 Authentication (STS)**

Before using the Platform API, set up the Security Token Service (STS) for authentication:

1. Follow the instructions in [sts/README.md](../sts/README.md) to start the STS service
2. Run the sample OAuth application and log in
3. Copy the access token displayed after successful login
4. Use this token in the `Authorization: Bearer <token>` header for all Platform API requests

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
  -H 'Authorization: Bearer <your-oauth2-token>' \
  -d '{"id":"<org-uuid>","handle":"acme","name":"ACME Corporation","region":"us-east-1"}'
```

**2. Create a Project**

```bash
curl -k -X POST https://localhost:9243/api/v1/projects \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <your-oauth2-token>' \
  -d '{
    "name": "Production APIs"
  }'
```

**3. Create a Gateway**

```bash
curl -k -X POST https://localhost:9243/api/v1/gateways \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json' \
  -H 'Authorization: Bearer <your-oauth2-token>' \
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
  -H 'Authorization: Bearer <your-oauth2-token>'
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
curl -k -s https://localhost:9243/api/v1/gateways/<gateway-uuid>/tokens \
  -H 'Authorization: Bearer <your-oauth2-token>'
```

Response:
```json
[
  {
    "id": "7ed55286-66a4-43ae-9271-bd1ead475a55",
    "status": "active",
    "createdAt": "2025-10-21T15:12:57.60936197+05:30"
  }
]
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
  -H 'Authorization: Bearer <your-oauth2-token>' \
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
  -H 'Authorization: Bearer <your-oauth2-token>' \
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

All configuration is supplied via environment variables. The sections below cover authentication, RBAC, and other key settings.

### Authentication Modes

Authentication mode is controlled by `IDP_ENABLED` and `IDP_TYPE`:

```
IDP_ENABLED=false (default)  →  Simple JWT (parse token, check org claim)
IDP_ENABLED=true
  IDP_TYPE=thunder            →  Thunder JWKS validation
  IDP_TYPE=external           →  External IDP JWKS validation (Asgardeo, Keycloak, …)
```

#### Simple JWT — default

`IDP_ENABLED` is `false` by default. The token is parsed and the organization claim is checked, but no JWKS endpoint is contacted. Signature verification is controlled by `JWT_SKIP_VALIDATION`.

| Variable | Default | Description |
|---|---|---|
| `IDP_ENABLED` | `false` | Keep `false` (or omit) to use this mode |
| `JWT_SKIP_VALIDATION` | `true` | Skip signature verification. Set to `false` to verify HMAC signatures using `JWT_SECRET_KEY` |
| `JWT_SECRET_KEY` | `your-secret-key-change-in-production` | HMAC key used when `JWT_SKIP_VALIDATION=false` |
| `THUNDER_ORGANIZATION_CLAIM_NAME` | `organization` | JWT claim that must be present and non-empty |

No changes needed for local development — the defaults work out of the box:
```bash
go run ./cmd/main.go
```

---

#### IDP_TYPE=thunder

Tokens are validated against Thunder's JWKS endpoint. Requires `IDP_ENABLED=true` and `IDP_TYPE=thunder`.

| Variable | Default | Description |
|---|---|---|
| `IDP_ENABLED` | `false` | Set to `true` |
| `IDP_TYPE` | _(empty)_ | Set to `thunder` |
| `THUNDER_JWKS_URL` | _(required)_ | Thunder's JWKS endpoint (e.g. `https://thunder.example.com/oauth2/jwks`) |
| `THUNDER_ISSUER` | `thunder` | Expected `iss` claim value in incoming JWTs |
| `THUNDER_BASE_URL` | `http://localhost:8090` | Root URL of the Thunder service |
| `THUNDER_CLIENT_ID` | _(empty)_ | OAuth2 client ID for system-level token requests (client_credentials grant) |
| `THUNDER_CLIENT_SECRET` | _(empty)_ | OAuth2 client secret paired with `THUNDER_CLIENT_ID` |
| `THUNDER_ORGANIZATION_CLAIM_NAME` | `organization` | JWT claim that holds the organization/tenant ID |

```bash
export IDP_ENABLED=true
export IDP_TYPE=thunder
export THUNDER_JWKS_URL=https://thunder.example.com/oauth2/jwks
export THUNDER_ISSUER=https://thunder.example.com/oauth2/token
export THUNDER_CLIENT_ID=platform-api-client
export THUNDER_CLIENT_SECRET=<secret>
```

---

#### IDP_TYPE=external

Tokens are validated against a third-party IDP's JWKS endpoint. Requires `IDP_ENABLED=true` and `IDP_TYPE=external`.

| Variable | Default | Description |
|---|---|---|
| `IDP_ENABLED` | `false` | Set to `true` |
| `IDP_TYPE` | _(empty)_ | Set to `external` |
| `EXTERNAL_IDP_JWKS_URL` | _(required)_ | IDP's JWKS endpoint for public key retrieval |
| `EXTERNAL_IDP_ISSUER` | _(required)_ | Comma-separated list of accepted JWT issuers |
| `EXTERNAL_IDP_AUDIENCE` | _(empty)_ | Comma-separated accepted audiences. Entries ending with `*` are treated as prefixes |
| `EXTERNAL_IDP_ORGANIZATION_CLAIM_NAME` | `organization` | JWT claim holding the org/tenant ID |
| `EXTERNAL_IDP_USER_ID_CLAIM_NAME` | `sub` | JWT claim used as the user identifier |
| `EXTERNAL_IDP_USERNAME_CLAIM_NAME` | `username` | JWT claim for the username |
| `EXTERNAL_IDP_EMAIL_CLAIM_NAME` | `email` | JWT claim for the user's email |
| `EXTERNAL_IDP_SCOPE_CLAIM_NAME` | `scope` | JWT claim for granted scopes |
| `EXTERNAL_IDP_ROLES_CLAIM_PATH` | `roles` | Dot-notation path to the roles claim (e.g. `realm_access.roles` for Keycloak) |
| `EXTERNAL_IDP_ROLE_MAPPINGS` | _(empty)_ | Comma-separated `idp-value=platform-role` pairs (see RBAC section below) |

**Example — Asgardeo:**
```bash
export IDP_ENABLED=true
export IDP_TYPE=external
export EXTERNAL_IDP_JWKS_URL=https://api.asgardeo.io/t/<org>/oauth2/jwks
export EXTERNAL_IDP_ISSUER=https://api.asgardeo.io/t/<org>/oauth2/token
export EXTERNAL_IDP_AUDIENCE=<client-id>
export EXTERNAL_IDP_ORGANIZATION_CLAIM_NAME=organizationId
export EXTERNAL_IDP_ROLES_CLAIM_PATH=roles
export EXTERNAL_IDP_ROLE_MAPPINGS=platform-admin=admin,platform-dev=developer,platform-viewer=viewer
```

**Example — Keycloak:**
```bash
export IDP_ENABLED=true
export IDP_TYPE=external
export EXTERNAL_IDP_JWKS_URL=https://keycloak.example.com/realms/<realm>/protocol/openid-connect/certs
export EXTERNAL_IDP_ISSUER=https://keycloak.example.com/realms/<realm>
export EXTERNAL_IDP_ROLES_CLAIM_PATH=realm_access.roles
export EXTERNAL_IDP_ROLE_MAPPINGS=platform-admin=admin,platform-developer=developer
```

---

### Role-Based Access Control (RBAC)

RBAC enforces per-route permission checks on all authenticated requests. Three built-in roles exist:

| Role | Access level |
|---|---|
| `admin` | Full access to all resources and operations |
| `developer` | CRUD and deploy on APIs, projects, applications, and AI/integration resources; no gateway admin or subscription plan management |
| `viewer` | Read-only access to all resources |

#### Enabling and disabling RBAC

| Variable | Default | Description |
|---|---|---|
| `RBAC_ENABLED` | `true` | Set to `false` to disable all permission checks (all authenticated requests are allowed) |

**Disable RBAC** (e.g. for initial deployment or local development without roles configured):
```bash
export RBAC_ENABLED=false
```

**Enable RBAC** (default — no action needed):
```bash
export RBAC_ENABLED=true
```

> **Note:** In simple JWT mode and Thunder IDP mode, role resolution uses the `scope` claim from the JWT directly. In external IDP mode, roles are resolved from `EXTERNAL_IDP_ROLES_CLAIM_PATH` and mapped to platform roles via `EXTERNAL_IDP_ROLE_MAPPINGS`.

---

### JWT Skip Paths

Paths listed here bypass JWT authentication entirely (used for internal/gateway traffic):

| Variable | Default |
|---|---|
| `JWT_SKIP_PATHS` | `/health,/metrics,/api/internal/v1/ws/gateways/connect,...` |

To add extra paths:
```bash
export JWT_SKIP_PATHS="/health,/metrics,/api/internal/v1/ws/gateways/connect,/my-custom-path"
```

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
| `DATABASE_SUBSCRIPTION_TOKEN_ENCRYPTION_KEY` | _(empty)_ | 32-byte key (64 hex or 44 base64 chars) for AES-256-GCM token encryption. Falls back to `JWT_SECRET_KEY` when empty |

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
