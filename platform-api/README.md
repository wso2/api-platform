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

Configuration comes from a TOML config file and/or environment variables (env vars
override the file). **Config-override environment variables are prefixed with `APIP_CP_`**
The prefix is stripped and the remainder mapped to a config key — e.g. `APIP_CP_LOG_LEVEL` → `log_level`,
`APIP_CP_DATABASE_HOST` → `database.host`. The variable names in the tables below are shown
with the prefix.

Two variables are intentionally **not** prefixed: `APIP_DEMO_MODE` (a standalone runtime
flag) and the shared `APIP_CONFIG_FILE_SOURCE_ALLOWLIST`. The `{{ env "NAME" }}` interpolation
tokens in the config file read the literal name via `os.LookupEnv` (independent of the koanf
prefix mechanism); the samples use the same `APIP_CP_`-prefixed names for one consistent
namespace — e.g. `{{ env "APIP_CP_ENCRYPTION_KEY" }}` (see "Providing secrets via the config
file" below).

### Authentication

Two authentication modes are supported. Exactly one should be active at a time.

```
APIP_CP_AUTH_IDP_ENABLED=false (default)  →  Local JWT mode  (HMAC signature verification)
APIP_CP_AUTH_IDP_ENABLED=true             →  IDP mode        (JWKS-based verification)
```

> **Demo mode (`APIP_DEMO_MODE`).** Defaults to `true`; an explicit `false`/`0` opts into
> production-grade startup checks. Note that `APIP_CP_ENCRYPTION_KEY` and `APIP_CP_AUTH_JWT_SECRET_KEY` are **required**.

---

#### Local JWT Mode (default)

The server signs and validates HMAC login tokens using `APIP_CP_AUTH_JWT_SECRET_KEY` — a 32-byte key (64 hex chars or base64). Set `APIP_CP_AUTH_JWT_SKIP_VALIDATION=true` only in local development environments where you do not have a token issuer available — all bearer values will be accepted without any signature check.

| Variable | Default | Description                                                         |
|---|---|---------------------------------------------------------------------|
| `APIP_CP_AUTH_JWT_SECRET_KEY` | _(empty)_ | HMAC key for signing/verifying login JWTs — 32-byte value (64 hex or base64; `openssl rand -hex 32`) |
| `APIP_CP_AUTH_JWT_ISSUER` | `platform-api` | Expected `iss` claim value                                          |
| `APIP_CP_AUTH_JWT_SKIP_VALIDATION` | `false` | Skip signature verification — **development only**                  |
| `DEV_MODE` | `false` | Suppresses the startup warning when `APIP_CP_AUTH_JWT_SKIP_VALIDATION=true` |

Local development with no token issuer:
```bash
export APIP_CP_AUTH_JWT_SKIP_VALIDATION=true
export DEV_MODE=true
go run ./cmd/main.go
```

Production with HMAC verification:
```bash
export APIP_CP_AUTH_JWT_SECRET_KEY=<strong-random-key>
export APIP_CP_AUTH_JWT_ISSUER=https://your-token-issuer
go run ./cmd/main.go
```

**Legacy variable names** (still accepted, deprecated):

| Old name | New name |
|---|---|
| `JWT_SECRET_KEY` | `APIP_CP_AUTH_JWT_SECRET_KEY` |
| `JWT_ISSUER` | `APIP_CP_AUTH_JWT_ISSUER` |
| `JWT_SKIP_VALIDATION` | `APIP_CP_AUTH_JWT_SKIP_VALIDATION` |
| `JWT_SKIP_PATHS` | `APIP_CP_AUTH_SKIP_PATHS` |

---

#### IDP Mode

Tokens are validated against any standards-compliant identity provider (Thunder, Asgardeo, Keycloak, Azure AD, Okta, etc.) using its JWKS endpoint. Set `APIP_CP_AUTH_IDP_ENABLED=true` and supply at minimum `APIP_CP_AUTH_IDP_JWKS_URL` and `APIP_CP_AUTH_IDP_ISSUER`.

| Variable | Default | Description |
|---|---|---|
| `APIP_CP_AUTH_IDP_ENABLED` | `false` | Set to `true` to activate IDP mode |
| `APIP_CP_AUTH_IDP_NAME` | _(empty)_ | Optional label shown in startup logs (e.g. `thunder`, `asgardeo`) |
| `APIP_CP_AUTH_IDP_JWKS_URL` | _(required)_ | IDP's JWKS endpoint for public key retrieval |
| `APIP_CP_AUTH_IDP_ISSUER` | _(required)_ | Accepted JWT issuer |
| `APIP_CP_AUTH_IDP_AUDIENCE` | _(empty)_ | Accepted JWT audience. When set, the token's `aud` claim must contain this value; empty skips the check |
| `APIP_CP_AUTH_IDP_ORGANIZATION_CLAIM_NAME` | `organization` | JWT claim holding the org UUID for the active session |
| `APIP_CP_AUTH_IDP_ORG_NAME_CLAIM_NAME` | `org_name` | JWT claim for the org display name |
| `APIP_CP_AUTH_IDP_ORG_HANDLE_CLAIM_NAME` | `org_handle` | JWT claim for the org URL-safe handle |
| `APIP_CP_AUTH_IDP_USER_ID_CLAIM_NAME` | `sub` | JWT claim used as the canonical user identifier |
| `APIP_CP_AUTH_IDP_USERNAME_CLAIM_NAME` | `username` | JWT claim for the human-readable username |
| `APIP_CP_AUTH_IDP_EMAIL_CLAIM_NAME` | `email` | JWT claim for the user's email address |
| `APIP_CP_AUTH_IDP_SCOPE_CLAIM_NAME` | `scope` | JWT claim carrying granted OAuth2 scopes |
| `APIP_CP_AUTH_IDP_VALIDATION_MODE` | `scope` | Authorization mode: `scope` (validate scope claim directly) or `role` (expand IDP roles to platform roles) |
| `APIP_CP_AUTH_IDP_ROLES_CLAIM_PATH` | _(empty)_ | Dot-notation path to the roles claim (e.g. `realm_access.roles`). Required when `APIP_CP_AUTH_IDP_VALIDATION_MODE=role` |
| `APIP_CP_AUTH_IDP_ROLE_MAPPINGS` | _(empty)_ | Comma-separated `idp-role=platform-role` pairs (e.g. `PLATFORM_ADMIN=admin,PLATFORM_DEV=developer`). When empty, IDP role values are used as-is |

**Example — Asgardeo:**
```bash
export APIP_CP_AUTH_IDP_ENABLED=true
export APIP_CP_AUTH_IDP_NAME=asgardeo
export APIP_CP_AUTH_IDP_JWKS_URL=https://api.asgardeo.io/t/<org>/oauth2/jwks
export APIP_CP_AUTH_IDP_ISSUER=https://api.asgardeo.io/t/<org>/oauth2/token
export APIP_CP_AUTH_IDP_AUDIENCE=<client-id>
export APIP_CP_AUTH_IDP_ORGANIZATION_CLAIM_NAME=organizationId
export APIP_CP_AUTH_IDP_VALIDATION_MODE=scope
export APIP_CP_AUTH_IDP_ROLES_CLAIM_PATH=scope
```

---

#### Skip Paths

Path prefixes listed here bypass authentication entirely. Used for internal gateway traffic and health checks.

| Variable | Default |
|---|---|
| `APIP_CP_AUTH_SKIP_PATHS` | `/health,/metrics,/api/internal/v1/ws/gateways/connect,...` |

To extend the default list:
```bash
export APIP_CP_AUTH_SKIP_PATHS="/health,/metrics,/api/internal/v1/ws/gateways/connect,/my-custom-path"
```

---

### Role-Based Access Control (RBAC)

Per-route scope checks are enforced when `APIP_CP_ENABLE_SCOPE_VALIDATION=true`. Five built-in platform roles exist:

| Role | Persona | Access level |
|---|---|---|
| `admin` | Platform administrator | Full access to all resources and operations |
| `developer` | API designer | Full API lifecycle; cannot manage gateways or subscription plans |
| `publisher` | DevPortal manager | Read APIs and publish/unpublish to DevPortals; cannot create or deploy |
| `operator` | CI/CD service account | Deploy and undeploy operations only; cannot create resources or manage credentials |
| `viewer` | Auditor | Read-only access to all resources |

| Variable | Default | Description |
|---|---|---|
| `APIP_CP_ENABLE_SCOPE_VALIDATION` | `false` | Set to `true` to enforce per-route scope/role checks |

In **local JWT mode**, scopes are read directly from the `scope` claim in the token.  
In **IDP mode with `APIP_CP_AUTH_IDP_VALIDATION_MODE=scope`**, scopes are read from the claim named by `APIP_CP_AUTH_IDP_SCOPE_CLAIM_NAME`.  
In **IDP mode with `APIP_CP_AUTH_IDP_VALIDATION_MODE=role`**, IDP roles are resolved from `APIP_CP_AUTH_IDP_ROLES_CLAIM_PATH`, mapped via `APIP_CP_AUTH_IDP_ROLE_MAPPINGS`, and matched against the required roles for each route.

---

### Database

| Variable | Default | Description |
|---|---|---|
| `APIP_CP_DATABASE_DRIVER` | `sqlite3` | `sqlite3` or `postgres` |
| `APIP_CP_DATABASE_DB_PATH` | `./data/api_platform.db` | SQLite file path (ignored for Postgres) |
| `APIP_CP_DATABASE_HOST` | `localhost` | Postgres host |
| `APIP_CP_DATABASE_PORT` | `5432` | Postgres port |
| `APIP_CP_DATABASE_NAME` | `platform_api` | Postgres database name |
| `APIP_CP_DATABASE_USER` | _(empty)_ | Postgres username |
| `APIP_CP_DATABASE_PASSWORD` | _(empty)_ | Postgres password |
| `APIP_CP_DATABASE_SSL_MODE` | `disable` | Postgres SSL mode (`disable`, `require`, `verify-full`) |
| `APIP_CP_DATABASE_EXECUTE_SCHEMA_DDL` | `true` | Set to `false` when the DB user lacks DDL privileges |

---

### Encryption

`APIP_CP_ENCRYPTION_KEY` protects all at-rest encryption (secrets, subscription tokens, WebSub HMAC secrets). It is **never auto-generated** — the operator must provide it.

| Variable | Default | Description |
|---|---|---|
| `APIP_CP_ENCRYPTION_KEY` | _(empty)_ | **Required.** 32-byte AES-256 key as 64 hex chars or base64 (32 bytes). Generate with `openssl rand -hex 32`. Startup fails if missing or malformed. |

#### Providing secrets via the config file (preferred over raw values)

When the Platform API is configured from a TOML file, do **not** write raw key values into
it and do **not** hardcode them as literal env vars in a compose file. Reference each secret
(`APIP_CP_ENCRYPTION_KEY`, `APIP_CP_AUTH_JWT_SECRET_KEY`, `APIP_CP_DATABASE_PASSWORD`,
`APIP_CP_WEBHOOK_SECRET`, …) with an interpolation token that is resolved at startup,
preferring a mounted file:

```toml
encryption_key = '{{ env "APIP_CP_ENCRYPTION_KEY" }}'            # from an env var
# preferred — from a mounted secret file:
# encryption_key = '{{ file "/secrets/platform-api/encryption_key" }}'
```

For the `{{ env }}` form, supply the value from a git-ignored env file rather than the shell or
the compose file — the samples keep secrets in `keys.env` and start the stack with
`docker compose --env-file keys.env up`, which the compose forwards into the container
via an `environment:` `${APIP_CP_…}` passthrough (never an `env_file:` block or a hardcoded value).

`{{ file }}` reads are restricted to an allowlist (`/etc/platform-api`, `/secrets/platform-api`;
override with the shared `APIP_CONFIG_FILE_SOURCE_ALLOWLIST` env var). Resolution fails closed:
a missing/empty required env var, or a missing/disallowed/oversize file, aborts startup.

---

### Other Settings

| Variable | Default | Description |
|---|---|---|
| `LOG_LEVEL` | `DEBUG` | Log verbosity (`DEBUG`, `INFO`, `WARN`, `ERROR`) |
| `HTTPS_ENABLED` | `true` | Enable the TLS listener. Certificates are read from `HTTPS_CERT_DIR` (or generated in demo mode) |
| `HTTPS_PORT` | `9243` | Port for the TLS listener |
| `HTTPS_CERT_DIR` | `./data/certs` | Directory holding `cert.pem` / `key.pem` (used only when `HTTPS_ENABLED=true`) |
| `HTTP_ENABLED` | `false` | Enable the plain-HTTP listener. Use only behind a TLS-terminating ingress/sidecar or for internal traffic — never expose directly to untrusted networks |
| `HTTP_PORT` | `9080` | Port for the plain-HTTP listener |
| `TIMEOUTS_READ_HEADER` | `10s` | Max time to read request headers, on both listeners (`0` disables) |
| `TIMEOUTS_READ` | `60s` | Max time to read the whole request, including the body (`0` disables) |
| `TIMEOUTS_WRITE` | `120s` | Max time for handler execution plus response write (`0` disables) |
| `TIMEOUTS_IDLE` | `120s` | Max time a keep-alive connection may sit unused (`0` disables) |
| `DEPLOYMENTS_MAX_PER_API_GATEWAY` | `20` | Maximum deployments per API per gateway |
| `DEPLOYMENTS_TRANSITIONAL_STATUS_ENABLED` | `false` | Show `DEPLOYING`/`UNDEPLOYING` status before gateway ack |
| `ARTIFACT_LIMITS_MAX_LLM_PROVIDERS_PER_ORG` | _unlimited_ | Max LLM providers per organization (`0` or unset = unlimited) |
| `ARTIFACT_LIMITS_MAX_LLM_PROXIES_PER_ORG` | _unlimited_ | Max LLM proxies per organization (`0` or unset = unlimited) |
| `ARTIFACT_LIMITS_MAX_MCP_PROXIES_PER_ORG` | _unlimited_ | Max MCP proxies per organization (`0` or unset = unlimited) |
| `ARTIFACT_LIMITS_MAX_WEBSUB_APIS_PER_ORG` | _unlimited_ | Max WebSub APIs per organization (`0` or unset = unlimited) |
| `ARTIFACT_LIMITS_MAX_WEBBROKER_APIS_PER_ORG` | _unlimited_ | Max WebBroker APIs per organization (`0` or unset = unlimited) |
| `GATEWAY_ENABLE_VERSION_VERIFICATION` | `false` | Reject gateway connections with mismatched versions |
| `API_KEY_HASHING_ALGORITHMS` | `sha256` | Comma-separated hash algorithms for API key storage |

> The legacy `PORT`, `TLS_ENABLED`, and `TLS_CERT_DIR` env vars are still honored and map onto the HTTPS listener (`HTTPS_PORT`, `HTTPS_ENABLED`, `HTTPS_CERT_DIR`).

## Documentation

See [spec/](spec/) for product, architecture, design, and implementation documentation.
