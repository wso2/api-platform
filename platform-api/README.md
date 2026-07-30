# Platform API

Backend service that powers the API Platform portals, gateways, and automation flows.

## Quick Start

### Prerequisites

Before using the Platform API, obtain a bearer token for authentication. In `file` auth mode you can obtain a token from the login endpoint. In `internal_token` mode the token is minted by another trusted platform component, signed with the RSA private key matching `platform_api.auth.jwt.public_key_file`. In `idp` mode, obtain a token from your identity provider. See [Configuration](#configuration) below.

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
AI Workspace and Developer Portal quickstarts use. It's the one Platform API config shared by every
quickstart (both docker-compose setups mount it directly), so its admin user is granted the
`ap_admin` role from the mounted [`resources/roles.yaml`](resources/roles.yaml), which covers both
the `ap:*` (Platform API) and `dp:*` (Developer Portal) namespaces. Override the role with
`APIP_CP_ADMIN_ROLE`; the small `scopes` list alongside it carries the few scopes `roles.yaml`
cannot name (see the comment there).

There is no default admin credential: `APIP_CP_ADMIN_USERNAME` and `APIP_CP_ADMIN_PASSWORD_HASH` are
**required** in this mode, and startup fails closed if either is unset or empty. `portals/scripts/setup.sh`
provisions both for the quickstarts, printing the generated password once. To set them yourself,
generate a hash with `htpasswd -nBC 12 "" | tr -d ':\n'`, which prompts for the password instead of
taking it as an argument — a password on the command line lands in shell history, `ps` output, and CI
logs. Alternatively set
`platform_api.auth.mode = "internal_token"`
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
| `[platform_api.auth]` | `mode` — one of `internal_token`, `file`, or `idp` |
| `[platform_api.auth.authorization]` | `enabled`, `mode` (`scope` / `role`), `role_mappings` — applies in every auth mode |
| `[platform_api.auth.jwt]` | Asymmetric (RS256) token settings: `issuer`, `public_key` (**required** — PEM RSA public key, verifies tokens), `private_key` (**required in `file` mode** — PEM RSA private key, signs login tokens), `token_ttl` |
| `[platform_api.auth.idp]` / `[platform_api.auth.claim_mappings]` | JWKS endpoint and issuer/audience for `idp` mode; JWT claim-name mappings (all modes) |
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

- **`internal_token`** — verify asymmetrically-signed (RS256) JWTs (`[platform_api.auth.jwt]`); tokens are minted by another trusted platform component and signed with the matching RSA private key, verified here against `public_key`. Symmetric (HMAC) and unsigned (`none`) tokens are rejected.
- **`file`** — `internal_token` plus local username/password login: the login endpoint authenticates against `[platform_api.auth.file]` and issues RS256 JWTs signed with `[platform_api.auth.jwt].private_key`, verified with the matching `public_key`. Used by the AI Workspace and Developer Portal quickstarts.
- **`idp`** — validate tokens against an external IDP's JWKS endpoint (Thunder, Asgardeo, Keycloak, Azure AD, Okta, etc.) via `[platform_api.auth.idp]`; `jwks_url` and `issuer` are required.

The paths that bypass authentication and scope enforcement — health/metrics probes, the login
endpoint, and the internal routes authenticated by a gateway token instead of a user JWT — are not
configurable: the list is a property of the product's own routing, and a wrong entry in it is an
auth bypass. Plugins declare their own public prefixes through `AuthSkipPaths()`, which are
validated at startup. Use `auth.authorization.enabled` to control authorization enforcement.
A config file still carrying `platform_api.auth.skip_paths` fails startup rather than having the
key silently ignored.

#### Role-Based Access Control (RBAC)

Per-route scope checks are enforced when `platform_api.auth.authorization.enabled = true`. The
shipped [`resources/roles.yaml`](resources/roles.yaml) defines five roles, each granting scopes in
both the `ap:*` (Platform API) and `dp:*` (Developer Portal) namespaces — one role covers a persona
across both components:

| Role | Persona | Access level |
|---|---|---|
| `ap_admin` | Platform administrator | Every resource and operation, both components |
| `ap_operator` | Platform operator / CI-CD service account | Gateways, deployments, subscription plans, key managers, webhooks; reads everything else |
| `ap_publisher` | API publisher | Full API/MCP/LLM lifecycle and its Developer Portal content; reads applications, subscriptions, plans |
| `ap_subscriber` | API consumer | Own applications, subscriptions and keys; reads the API/MCP catalog and plans |
| `ap_viewer` | Auditor | Read-only across both components |

The roles are named after the platform rather than after any one IDP's convention, since the same
file serves every auth mode — map your IDP's groups onto these names via `claim_mappings.roles`.
Edit the file to change what a role grants; it needs a server restart to take effect.

All three modes read identity fields — including scope — through the same
`[platform_api.auth.claim_mappings]` table (`scope` defaults to the `scope` claim); `file` mode's
login endpoint also signs the tokens it issues using these same claim names, so issuance and
validation never drift apart.

Authorization is configured separately from authentication, in
`[platform_api.auth.authorization]`, and applies in **every** auth mode — an enterprise IDP's token
carries the same roles claim whether the platform verifies it against a JWKS endpoint or with a
local public key:

```toml
[platform_api.auth.authorization]
enabled       = true
mode          = "role"                          # "scope" (default) or "role"
role_mappings = "/etc/platform-api/roles.yaml"  # required when mode = "role"
```

`mode = "scope"` authorizes from the scope claim directly. `mode = "role"` expands the roles claim
named by `claim_mappings.roles` into platform scopes via the `role_mappings` YAML file; both that
claim mapping and the file path are required in role mode, so startup fails rather than falling
back to using role names verbatim as scopes.

The mapping file is operator-owned config, not part of the image: the packs mount their editable
sample (`resources/roles.yaml`) at `/etc/platform-api/roles.yaml`.

Validation of that file is namespace-scoped. An `ap:` scope must be declared in this server's OpenAPI
spec (plus any its compiled-in plugins declare) — an unknown one fails startup rather than silently
denying requests later. A scope in another component's namespace (`dp:*`) is checked only for shape:
this server mints it into the token but never enforces it, so it can neither confirm nor deny that it
exists. That is what lets one role describe a persona across the whole platform.

##### Granting a file-mode user a role

A `file`-mode user is granted a role rather than a hand-maintained scope list. The login endpoint
expands that role through the same `role_mappings` file when it mints the token:

```toml
[[platform_api.auth.file.users]]
username      = '{{ env "APIP_CP_ADMIN_USERNAME" }}'
password_hash = '{{ env "APIP_CP_ADMIN_PASSWORD_HASH" }}'
role          = "ap_admin"                 # expanded via auth.authorization.role_mappings
# scopes      = ""                         # optional: unioned with the role's scopes
```

The issued token carries **both**: the expanded scopes as the `scope` claim, and the role name as the
`roles` claim. So the same login works under either authorization mode — `scope` (the default) checks
the expanded claim, and flipping `auth.authorization.mode = "role"` re-expands the role from the same
`roles.yaml` on every request instead. `claim_mappings.roles` defaults to the flat `roles` claim the
login endpoint signs, so that switch needs no extra claim wiring.

A role is normally the whole grant — the mapping file may name scopes in any namespace, so there is
usually nothing left to add. `scopes` remains available for anything the file cannot name (an `ap:`
scope this server's spec doesn't declare, which `roles.yaml` would reject) or on its own instead of a
role; the two are unioned, and one of them is required — a user granted neither would authenticate
and then be denied every route. A role absent from the mapping file fails startup.

This is how the shipped `config/config.toml` grants its admin user: `role = "ap_admin"` and nothing
else. Changing what that user can do means editing the mounted `roles.yaml`, which makes that file the
security-relevant one to review in a pack.

### Providing secrets via the config file

Never write raw secret values into the config file, and never hardcode them as literals in a
compose file. Reference each secret (`security.encryption_key`, `database.password`,
`webhook.secret`, …) with an interpolation token, preferring a mounted file
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
