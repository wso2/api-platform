# Platform API

Backend service that powers the API Platform portals, gateways, and automation flows.

## Quick Start

### Prerequisites

Before using the Platform API, obtain a bearer token for authentication. In `file` or `external_token` auth mode you can generate a token using the HMAC key configured at `platform_api.auth.jwt.secret_key`. In `idp` mode, obtain a token from your identity provider. See [Configuration](#configuration) below.

### Build and Run

```bash
# Build
cd platform-api
go build ./cmd/main.go

# Run (TLS with self-signed certificates)
cd platform-api
go run ./cmd/main.go
```

`config/config.toml` is the local-development config, used with `platform_api.auth.mode = "file"`
(username/password login backed by the organization/user block in that file) — the same mode the
AI Workspace and Developer Portal quickstarts use, so it works out of the box with either, with no
env vars set. It's the one Platform API config shared by every quickstart (both docker-compose
setups mount it directly), so its admin user's scopes cover both the `ap:*` (AI Workspace /
platform-admin) and `dp:*` (Developer Portal) namespaces. Set `APIP_CP_ADMIN_USERNAME` /
`APIP_CP_ADMIN_PASSWORD_HASH` to pick your own login credentials (generate a hash with
`htpasswd -bnBC 12 "" <password> | tr -d ':\n'`), or set `platform_api.auth.mode = "external_token"`
for locally-signed HMAC tokens with no local users — see
[`config/config-template.toml`](config/config-template.toml) for the full reference.

### Database Configuration

Platform API supports `sqlite3` (default), `postgres`, and `sqlserver`. Configure the driver
under `[platform_api.database]` in your config file, e.g. for SQL Server:

```toml
[platform_api.database]
driver   = "sqlserver"
host     = "sqlserver.example.internal"
port     = "1433"
name     = "platform_api"
username = "sa"
password = '{{ env "DB_PASSWORD" }}'   # or '{{ file "/secrets/platform-api/db_password" }}'
ssl_mode = "disable"
```

```bash
cd platform-api
go run ./cmd/main.go -config config/config.toml
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

Configuration is read from a TOML config file (`-config <path>`), layered over built-in
defaults. **There are no fixed, prescriptive environment variable names** — a key omitted from
the file simply falls back to its built-in default, a literal value in the file is used as-is,
and the only way an environment variable (or a mounted file) affects a setting is by writing an
explicit interpolation token as that key's value:

```toml
some_key = '{{ env "ANY_VAR_NAME" "optional-default" }}'   # from an env var, with a fallback
some_key = '{{ env "ANY_VAR_NAME" }}'                        # from an env var, no fallback — unset fails config load
some_key = '{{ file "/secrets/platform-api/some-file" }}'    # from a mounted file (preferred for secrets)
```

The name inside the token (`ANY_VAR_NAME`) is a free choice — it's read via `os.LookupEnv` at
load time and isn't tied to any specific naming scheme. [`config/config-template.toml`](config/config-template.toml)
is the authoritative reference: it lists every key the binary reads, each already wrapped in an
`{{ env }}` token using the `APIP_CP_*` naming convention as one consistent example — copy it and
edit the values, or replace the tokens with plain literals. `{{ file }}` reads are restricted to
an allowlisted directory (default `/etc/platform-api`, `/secrets/platform-api`) and fail closed:
a missing/empty required source, or a missing/disallowed/oversize file, aborts startup.

### Key sections

All settings live under `[platform_api]` / `[platform_api.*]`. The main sections:

| Section | Purpose |
|---|---|
| `[platform_api]` | resource paths |
| `[platform_api.logging]` | `level`, `format` |
| `[platform_api.security]` | `encryption_key` (**required** — at-rest AES-256 key, 32 bytes as hex or base64, never auto-generated) |
| `[platform_api.security.api_key]` | `hashing_algorithms` accepted for API key verification |
| `[platform_api.database]` | `driver` (`sqlite3` / `postgres` / `sqlserver`), connection fields, pool sizing |
| `[platform_api.auth]` | `mode` — one of `external_token`, `file`, or `idp`; `scope_validation`; `skip_paths` |
| `[platform_api.auth.jwt]` | Asymmetric (RS256) token settings: `issuer`, `public_key` (**required** — PEM RSA public key, verifies tokens), `private_key` (**required in `file` mode** — PEM RSA private key, signs login tokens), `token_ttl` |
| `[platform_api.auth.idp]` / `[platform_api.auth.claim_mappings]` | JWKS endpoint, issuer/audience, validation mode, and JWT claim-name mappings for `idp` mode |
| `[platform_api.auth.file.organization]` / `[[platform_api.auth.file.users]]` | Local org + username/password/scope entries for `file` mode |
| `[platform_api.server.http]` / `[platform_api.server.https]` | Listener enablement, ports, and (HTTPS) `cert_file` / `key_file` paths (certificates are always required for HTTPS — no self-signed fallback) |
| `[platform_api.server.timeouts]` | Read/write/idle timeouts |
| `[platform_api.server.cors]` | `allowed_origins` for credentialed cross-origin requests |
| `[platform_api.server.websocket]` | Gateway WebSocket connection limits and rate limiting |
| `[platform_api.deployments]` | Deployment caps and stuck-deployment timeout handling |
| `[platform_api.gateway]` | Gateway registration verification toggles |
| `[platform_api.event_hub]` | Multi-replica event delivery polling/retention |
| `[platform_api.webhook]` | Developer Portal webhook receiver: `enabled`, `secret` (required when enabled), signature/body limits |

#### Authentication modes

`platform_api.auth.mode` selects exactly one mode; only that mode's section is read:

- **`external_token`** — verify locally-issued, asymmetrically-signed (RS256) JWTs (`[platform_api.auth.jwt]`); tokens are minted externally (e.g. by the Developer Portal) and signed with the matching RSA private key, verified here against `public_key`. Symmetric (HMAC) and unsigned (`none`) tokens are rejected.
- **`file`** — `external_token` plus local username/password login: the login endpoint authenticates against `[platform_api.auth.file]` and issues RS256 JWTs signed with `[platform_api.auth.jwt].private_key`, verified with the matching `public_key`. Used by the AI Workspace and Developer Portal quickstarts.
- **`idp`** — validate tokens against an external IDP's JWKS endpoint (Thunder, Asgardeo, Keycloak, Azure AD, Okta, etc.) via `[platform_api.auth.idp]`; `jwks_url` and `issuer` are required.

`platform_api.auth.skip_paths` is a structured list (not a scalar), so it's edited directly in
the file rather than through a single token; setting it replaces the built-in default list.

#### Role-Based Access Control (RBAC)

Per-route scope checks are enforced when `platform_api.auth.scope_validation = true`. Five built-in platform roles exist:

| Role | Persona | Access level |
|---|---|---|
| `admin` | Platform administrator | Full access to all resources and operations |
| `developer` | API designer | Full API lifecycle; cannot manage gateways or subscription plans |
| `publisher` | DevPortal manager | Read APIs and publish/unpublish to DevPortals; cannot create or deploy |
| `operator` | CI/CD service account | Deploy and undeploy operations only; cannot create resources or manage credentials |
| `viewer` | Auditor | Read-only access to all resources |

All three modes read identity fields — including scope — through the same
`[platform_api.auth.claim_mappings]` table (`scope` defaults to the `scope` claim); `file` mode's
login endpoint also signs the tokens it issues using these same claim names, so issuance and
validation never drift apart. In **`idp` mode**, `validation_mode` additionally controls whether
authorization uses the scope claim directly or expands IDP roles from `claim_mappings.roles` via
`role_mappings`.

### Providing secrets via the config file

Never write raw secret values into the config file, and never hardcode them as literals in a
compose file. Reference each secret (`security.encryption_key`, `auth.jwt.secret_key`,
`database.password`, `webhook.secret`, …) with an interpolation token, preferring a mounted file
over an env var:

```toml
[platform_api.security]
# Read from a mounted key file provisioned by scripts/setup.sh:
encryption_key = '{{ file "/etc/platform-api/keys/encryption.key" }}'
# Alternatively, from an env var supplied via a git-ignored env file:
# encryption_key = '{{ env "APIP_CP_ENCRYPTION_KEY" }}'
```

For the `{{ env }}` form, supply the value from a git-ignored env file rather than the shell or
a hardcoded literal in the compose file — the samples keep secrets in `api-platform.env` and
mount it into the container via an `env_file:` entry (`format: raw`, since a bcrypt hash can
contain `$`, which must not be treated as compose interpolation):

```yaml
services:
  platform-api:
    env_file:
      - path: api-platform.env
        required: true
        format: raw
```

---

## Documentation

See [spec/](spec/) for product, architecture, design, and implementation documentation.
