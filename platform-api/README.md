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
```

`-config` is always required — there is no default path, and Platform API fails closed rather than
guessing one. Which config file (and which secrets) you point it at depends on how you're running it:

#### Option A — Fully standalone (no Docker)

The quickest way to run Platform API entirely on its own, with no docker-compose, no other portal
involved, and no mounted `/etc/platform-api/...` paths:

```bash
cd platform-api
make setup-local-dev   # one-time: generates a TLS cert, JWT keypair, encryption key,
                        # and admin credentials, all under ./resources (gitignored)
make run-local          # runs `go run ./cmd/main.go -config config/config.local.toml`
                        # with those secrets exported
```

`setup-local-dev.sh` prints the generated admin username/password once at the end — copy it before
it scrolls away; it's only stored bcrypt-hashed afterward. Re-running the script is safe (idempotent
by default); pass `--force` to rotate the TLS cert/JWT keypair/admin credentials, or
`--rotate-encryption-key` to (destructively) rotate the at-rest encryption key. See
[`scripts/setup-local-dev.sh`](scripts/setup-local-dev.sh) and
[`config/config.local.toml`](config/config.local.toml) for the details — the config file is
otherwise identical to `config/config.toml` below, just pointed at `./resources/...` instead of
`/etc/platform-api/...`.

Listens on `https://localhost:9243` (self-signed cert). If a previous run is still bound to that
port, `lsof -ti:9243 | xargs -r kill -9` before restarting — `go run` spawns a child binary that
outlives a plain `pkill`/Ctrl-C on the wrapper process.

#### Option B — Alongside the AI Workspace / API Portal quickstarts

`config/config.toml` is the config those docker-compose quickstarts mount directly, also using
`platform_api.auth.mode = "file"` (username/password login backed by the organization/user block in
that file). It's the one Platform API config shared by every quickstart, so its admin user is
granted the `ap_admin` role from the mounted
[`resources/role-to-scope-mapping.yaml`](resources/role-to-scope-mapping.yaml), which covers both
the `ap:*` (Platform API) and `dp:*` (API Portal) namespaces. That one role is the whole grant —
replace it, name more roles alongside it, or edit what it grants in `role-to-scope-mapping.yaml`.

There is no default admin credential: `APIP_CP_ADMIN_USERNAME` and `APIP_CP_ADMIN_PASSWORD_HASH` are
**required** in this mode, and startup fails closed if either is unset or empty. `portals/scripts/setup.sh`
provisions both (and the `/etc/platform-api/...`-mounted TLS cert/JWT keypair/encryption key) for the
quickstarts, printing the generated password once — see `portals/ai-workspace/README.md` or
`portals/api-portal/README.md`. To set the admin credential yourself instead, generate a hash with
`htpasswd -nBC 12 "" | tr -d ':\n'`, which prompts for the password instead of taking it as an
argument — a password on the command line lands in shell history, `ps` output, and CI logs.
Alternatively set
`platform_api.auth.mode = "internal_token"`
for locally-signed RS256 JWTs with no local users — see
[`config/config-template.toml`](config/config-template.toml) for the full reference.

```bash
cd platform-api
go run ./cmd/main.go -config config/config.toml
```

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
| `[platform_api.auth.authorization]` | `enabled`, `mode` (`scope` / `role`), `role_to_scope_mapping` — applies in every auth mode |
| `[platform_api.auth.jwt]` | Asymmetric (RS256) token settings: `issuer`, `public_key_file` (**required** — path to a PEM RSA public key, verifies tokens), `private_key_file` (**required in `file` mode** — path to a PEM RSA private key, signs login tokens), `token_ttl` |
| `[platform_api.auth.idp]` / `[platform_api.auth.claim_mappings]` | JWKS endpoint and issuer/audience for `idp` mode; JWT claim-name mappings (all modes) |
| `[platform_api.auth.file.organization]` / `[[platform_api.auth.file.users]]` | Local org + username/password/scope entries for `file` mode |
| `[platform_api.server.http]` / `[platform_api.server.https]` | Listener enablement, ports, and (HTTPS) `cert_file` / `key_file` paths (certificates are always required for HTTPS — no self-signed fallback) |
| `[platform_api.server.timeouts]` | Read/write/idle timeouts |
| `[platform_api.server.cors]` | `allowed_origins` for credentialed cross-origin requests |
| `[platform_api.server.websocket]` | Gateway WebSocket connection limits and rate limiting |
| `[platform_api.deployments]` | Deployment caps and stuck-deployment timeout handling |
| `[platform_api.gateway]` | Gateway registration verification toggles |
| `[platform_api.event_hub]` | Multi-replica event delivery polling/retention |
| `[platform_api.webhook]` | API Portal webhook receiver: `enabled`, `secret` (required when enabled), signature/body limits |

#### Authentication modes

`platform_api.auth.mode` selects exactly one mode; only that mode's section is read:

- **`internal_token`** — verify asymmetrically-signed (RS256) JWTs (`[platform_api.auth.jwt]`); tokens are minted by another trusted platform component and signed with the matching RSA private key, verified here against `public_key_file`. Symmetric (HMAC) and unsigned (`none`) tokens are rejected.
- **`file`** — `internal_token` plus local username/password login: the login endpoint authenticates against `[platform_api.auth.file]` and issues RS256 JWTs signed with `[platform_api.auth.jwt].private_key_file`, verified with the matching `public_key_file`. Used by the AI Workspace and API Portal quickstarts.
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
shipped [`resources/role-to-scope-mapping.yaml`](resources/role-to-scope-mapping.yaml) defines six roles, each granting scopes in
both the `ap:*` (Platform API) and `dp:*` (Developer Portal) namespaces — one role covers a persona
across both components:

| Role | Persona | Access level |
|---|---|---|
| `ap_admin` | Platform administrator | Every resource and operation, both components |
| `ap_operator` | Platform operator / CI-CD service account | Gateways, deployments, subscription plans, key managers, webhooks; reads everything else |
| `ap_publisher` | API publisher | Full API/MCP/LLM lifecycle and its Developer Portal content; reads applications, subscriptions, plans |
| `ap_developer` | API developer | Creates, updates and deploys APIs/proxies in an existing project and calls them through its own application and keys; deletes the APIs and proxies it created, but not projects or secrets, and publishes no portal content |
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
role_to_scope_mapping = "/etc/platform-api/role-to-scope-mapping.yaml"  # required when mode = "role"
```

`mode = "scope"` authorizes from the scope claim directly. `mode = "role"` expands the roles claim
named by `claim_mappings.roles` into platform scopes via the `role_to_scope_mapping` YAML file; both that
claim mapping and the file path are required in role mode, so startup fails rather than falling
back to using role names verbatim as scopes.

The mapping file is operator-owned config, not part of the image: the packs mount their editable
sample (`resources/role-to-scope-mapping.yaml`) at `/etc/platform-api/role-to-scope-mapping.yaml`.

Validation of that file is namespace-scoped. An `ap:` scope must be declared in this server's OpenAPI
spec (plus any its compiled-in plugins declare) — an unknown one fails startup rather than silently
denying requests later. A scope in
another component's namespace (`dp:*`) is checked only for shape: this server mints it into the token
but never enforces it, so it can neither confirm nor deny that it exists. That is what lets one role
describe a persona across the whole platform — and what makes a per-user scope list unnecessary.

##### Granting a file-mode user roles

A `file`-mode user is granted **only** roles — there is no per-user scope list. The login endpoint
expands them through the same `role_to_scope_mapping` file when it mints the token, unioning what
each grants:

```toml
[[platform_api.auth.file.users]]
username      = '{{ env "APIP_CP_ADMIN_USERNAME" }}'
password_hash = '{{ env "APIP_CP_ADMIN_PASSWORD_HASH" }}'
roles         = ["ap_admin"]               # expanded via auth.authorization.role_to_scope_mapping
```

`roles` is a list, so a user whose persona spans two shipped roles names both rather than needing a
seventh role defined for the combination — `roles = ["ap_publisher", "ap_subscriber"]` grants the union
of the two, most-permissive wins, with duplicate scopes collapsed.

The issued token carries **both**: the expanded scopes as the `scope` claim, and the role names as the
`roles` claim. So the same login works under either authorization mode — `scope` (the default) checks
the expanded claim, and flipping `auth.authorization.mode = "role"` re-expands the roles from the same
`role-to-scope-mapping.yaml` on every request instead. `claim_mappings.roles` defaults to the flat `roles` claim the
login endpoint signs, so that switch needs no extra claim wiring.

At least one role is required, and startup fails if a user has none or names one the mapping file
doesn't define — either way that user would authenticate successfully and then be denied every route.
Because the mapping is the only place a grant is expressed, no user can drift out of step with the
roles it names, and widening or narrowing a persona is one edit in one file. To grant something no
combination of shipped roles covers, add a role to `role-to-scope-mapping.yaml`.

This is how the shipped `config/config.toml` grants its admin user: `roles = ["ap_admin"]` and nothing
else. Changing what that user can do means editing the mounted `role-to-scope-mapping.yaml`, which makes that file the
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
