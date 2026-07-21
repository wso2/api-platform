# Developer Portal

A multi-organisation API developer portal built on Node.js. It provides a customisable web UI for discovering and subscribing to APIs, and a set of Admin REST APIs for managing organisations, views, API metadata, and portal content.

For end-user documentation, see [docs/](docs/).

## Ports

| Port | Protocol | Description |
|------|----------|-------------|
| `3000` | HTTPS (default) / HTTP | Developer Portal UI and Admin REST API |

## Prerequisites

- **Node.js** v24 (LTS)
- **Make**
- **Docker + Docker Compose** (for the Docker-based workflow)

> **PostgreSQL** is optional. The portal uses SQLite by default. See [Database setup](#4-database-setup) if you need PostgreSQL.

---

## Quick Start (Docker Compose)

The fastest way to get the portal running — no local Node install required. Requires `openssl` and Docker (used by `./scripts/setup.sh` to bcrypt-hash the admin password).

### Run

```bash
./scripts/setup.sh
docker compose up
```

`./scripts/setup.sh` is a one-time step: it generates devportal's and the Platform API's encryption/JWT secrets, a self-signed TLS certificate, and an admin user into `api-platform.env` (git-ignored). It prompts for an admin username/password interactively, or generates a random password if you press Enter; set `ADMIN_USERNAME`/`ADMIN_PASSWORD` env vars to skip the prompts (e.g. in CI). Safe to re-run — it only fills in what's missing and never overwrites an existing value; to build devportal from source instead of using the published image, run `docker compose up --build`.

Then open **https://localhost:3000/default/views/default** and log in with the admin credentials `./scripts/setup.sh` printed.

> **Browser warning:** the TLS certificate is self-signed. Click **Advanced → Proceed** (Chrome) or **Accept the Risk** (Firefox) to continue.

What happens automatically on first start:
- The DB schema is applied and the database is initialised automatically
- A default **default** org, view, labels, and subscription plans are seeded automatically on startup (controlled by `organization.default_name` in config)

### Test

```bash
# Run the Cypress UI E2E suite headlessly inside Docker
make it

# Run the REST API suite (Jest + Supertest)
make -C it test-rest-api

# Open Cypress interactive UI — requires the portal running locally first
make it-open
```

Both suites also run on pull requests via the
[Developer Portal Integration Test](../../.github/workflows/devportal-integration-test.yml)
GitHub Actions workflow. For integration test details, see [it/README.md](it/README.md).

### Clean

```bash
# Stop and remove containers and volumes
docker compose down -v

# Remove build artifacts and distribution zips
make clean
```

---

## Makefile Targets

Run `make help` to see the full list. Summary:

### Build

| Target | Description |
|--------|-------------|
| `make build` | Build the developer-portal Docker image (local, current arch) |
| `make build-and-push-multiarch` | Build and push a multi-arch image (`linux/amd64`, `linux/arm64`) to GHCR |

### Distribution

| Target | Description |
|--------|-------------|
| `make dist` | Build standalone distribution zip (`target/wso2apip-developer-portal-<VERSION>.zip`) |
| `make clean-dist` | Remove distribution staging directory and zip |

### Version Management

| Target | Description |
|--------|-------------|
| `make version-set VERSION=X` | Set version and update all artifacts |
| `make version-bump-patch` | Bump patch version (e.g. `1.0.0` → `1.0.1`) |
| `make version-bump-minor` | Bump minor version (e.g. `1.0.0` → `1.1.0`) |
| `make version-bump-major` | Bump major version (e.g. `1.0.0` → `2.0.0`) |
| `make version-bump-next-dev` | Bump to next minor dev version with `-SNAPSHOT` suffix |
| `make version-get-release` | Print release version (strips `-SNAPSHOT` suffix) |

### Integration Tests

| Target | Description |
|--------|-------------|
| `make it` | Run the Cypress UI E2E suite against SQLite (headless, in Docker) |
| `make it-postgres` | Run the Cypress UI E2E suite against PostgreSQL (headless, in Docker) |
| `make it-open` | Open Cypress interactive UI (requires the portal running locally) |
| `make -C it test-rest-api` | Run the REST API suite (Jest + Supertest) against SQLite |
| `make -C it test-rest-api-postgres` | Run the REST API suite against PostgreSQL |

See [it/README.md](it/README.md) for the full list of test commands and suite details.

### Database

| Target | Description |
|--------|-------------|
| `make generate-ddl` | Generate DDL schema files from Sequelize models for all supported dialects |

### Docs

| Target | Description |
|--------|-------------|
| `make generate-apidocs` | Generate REST API docs from the OpenAPI spec |

### Clean

| Target | Description |
|--------|-------------|
| `make clean` | Remove all build artifacts |

---

## Development (`npm start`)

Use this for active development, custom IdP configuration, or when you prefer to run Node directly.

### 1. Config file

`configs/config.toml` already ships with sensible defaults — edit it directly for custom settings. `configs/config-template.toml` is the full annotated reference of every available setting; see [Configuration reference](#configuration-reference) below.

### 2. Use `npm run start:local`, not `npm start`

`configs/config.toml`'s own defaults are wired for the Docker Compose topology (TLS on, pointing at a cert only the containers have, `platform_api.url` pointing at the `platform-api` hostname that only resolves inside the compose network). Plain `npm start` inherits those as-is and will fail — there's no `/app` filesystem or bind-mounted cert here. `npm run start:local` (`package.json`) overrides all of it in one place: TLS off and `platform_api.url` pointed at `localhost` (see [Local auth](#local-auth) if you're running the Platform API sidecar).

### 3. Configure the Identity Provider (optional)

The portal's login flow requires a valid OAuth2/OIDC provider. Update the `[idp]` block in `configs/config.toml`:

```toml
[idp]
issuer = "https://<your-idp>/oauth2/token"
authorization_url = "https://<your-idp>/oauth2/authorize"
token_url = "https://<your-idp>/oauth2/token"
user_info_url = "https://<your-idp>/oauth2/userinfo"
jwks_url = "https://<your-idp>/oauth2/jwks"
client_id = "<your-client-id>"
callback_url = "http://localhost:3000/<handle>/callback"
```

For local exploration you can skip IdP setup by using the Platform API sidecar instead (see [Local auth](#local-auth)).

### 4. Database setup

#### SQLite (default — no setup required)

The portal uses SQLite out of the box. The database file is created automatically at the path configured by `database.path` (default: `./devportal.db`). No installation or schema migration step is needed.

#### PostgreSQL (optional)

To use PostgreSQL instead, spin up an instance:

```bash
docker run --name devportal-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=devportal \
  -p 5432:5432 \
  -d postgres:16
```

Then update the `[developer_portal.database]` block in `configs/config.toml`:

```toml
[developer_portal.database]
driver = "postgres"
host = "localhost"
port = 5432
name = "devportal"
user = "postgres"
password = "postgres"
```

In production, set the password via the `APIP_DP_DATABASE_PASSWORD` environment variable instead of storing it in the config file.

### 5. Seed default organization

The default organization is seeded automatically on startup when `organization.default_name` is set in config (or via `APIP_DP_ORGANIZATION_DEFAULTNAME` env var).
No manual step is required.

### 6. Install and run

```bash
npm install
npm run start:local
```

Open **http://localhost:3000/default/views/default**

---

## Seed Sample APIs (optional)

Deploys the sample APIs and MCP servers under `samples/` into the default organisation, entirely through the public REST API — devportal itself has no built-in seeding logic. Works with both the Docker Compose and `npm start` workflows.

```bash
./scripts/seed-samples.sh
```

Prompts for the admin username/password (or set `ADMIN_USERNAME`/`ADMIN_PASSWORD` to skip the prompt, e.g. in CI). Safe to re-run — entries that already exist are skipped. Set `DEVPORTAL_URL`/`PLATFORM_API_URL` to override the defaults (`https://localhost:3000` / `https://localhost:9243`) — e.g. `DEVPORTAL_URL=http://localhost:3000` when running against `npm run start:local`.

---

## Configuration Reference

All settings live in `configs/config.toml`. Every setting can also be overridden with an `APIP_DP_*` environment variable.

The full annotated list of settings is in [`configs/config-template.toml`](configs/config-template.toml).

### Local auth

For quick exploration without an IdP, the portal delegates credential validation to a Platform API sidecar. `docker-compose.yaml` mounts the Platform API's own [`../../platform-api/config/config.toml`](../../platform-api/config/config.toml) directly — there is no per-portal copy. Users, bcrypt-hashed passwords, and `dp:*` scopes are defined there, under `[[platform_api.auth.file.users]]`:

```toml
[[platform_api.auth.file.users]]
username      = "admin"
password_hash = "$2y$10$..."   # bcrypt hash — generate with: htpasswd -bnBC 12 "" <pw> | tr -d ':\n'
scopes        = "dp:org_manage dp:api_manage ..."
```

The portal config (or `APIP_DP_PLATFORMAPI_*` env vars) must point to the Platform API. `config.toml`'s own defaults assume Docker Compose, where `platform-api` is a resolvable hostname on the compose network — `npm run start:local` already overrides `url` to `https://localhost:9243` (the sidecar's port published to the host) and `tls_skip_verify = true` (self-signed cert), so no manual edit is needed for that flow:

```toml
[developer_portal.platform_api]
url = "https://localhost:9243"            # env: APIP_DP_PLATFORMAPI_URL
jwt_private_key = ""                       # PEM RSA private key that signs portal-minted tokens; must match the Platform API's auth.jwt.public_key — env: APIP_DP_PLATFORMAPI_JWTPRIVATEKEY
tls_skip_verify = true                    # Platform API uses a self-signed cert
```

Tokens are signed asymmetrically (RS256): the portal signs with the RSA private key above and the Platform API verifies against its `auth.jwt.public_key`. There is no shared HMAC secret — the two sides never exchange signing material.

For production, configure an OIDC identity provider per organization instead of local auth.

### Environment variable overrides

Every config key can be overridden with an `APIP_DP_*` environment variable. You can place these in a `.env` file at the project root.

**Convention:**
- Prefix: `APIP_DP_`
- `_` separates nesting levels (one token = one config object level)
- `__` represents a literal underscore within a key name
- Tokens are matched case-insensitively against config keys (matched against the camelCase struct produced from the TOML's snake_case keys)

| Env var | Config path |
|---------|-------------|
| `APIP_DP_DATABASE_HOST` | `config.database.host` |
| `APIP_DP_DATABASE_PORT` | `config.database.port` |
| `APIP_DP_SERVER_HTTPS_ENABLED` | `config.server.https.enabled` |
| `APIP_DP_IDP_CLIENTID` | `config.idp.clientId` |
| `APIP_DP_IDP_ISSUER` | `config.idp.issuer` |
| `APIP_DP_SERVER_PORT` | `config.server.port` |
| `APIP_DP_DATABASE_SSL_MODE` | `config.database.sslMode` |

`.env` example:
```dotenv
APIP_DP_DATABASE_HOST=my-postgres-host
APIP_DP_DATABASE_PASSWORD=my-secret-password
APIP_DP_IDP_CLIENTID=my-client-id
```

---

## Publish your first API

Create an API manifest file and an OpenAPI definition, then upload them:

```yaml
# api.yaml
apiVersion: devportal.api-platform.wso2.com/v1alpha2
kind: RestApi

metadata:
  name: ping-api-v1.0

spec:
  type: REST
  displayName: Ping API
  version: v1.0
  description: Sample HTTP echo/probe API. Requires API key authentication. No subscription plans.
  status: PUBLISHED
  referenceID: ping-api-v1.0

  tags:
    - ping
    - api-key

  labels:
    - default

  subscriptionPlans: []

  visibility: PUBLIC
  visibleGroups: []

  businessInformation:
    businessOwner: Platform Owner
    businessOwnerEmail: support@example.com
    technicalOwner: API Team
    technicalOwnerEmail: architecture@example.com

  endpoints:
    sandboxUrl: http://localhost:8080/ping
    productionUrl: http://localhost:8080/ping
```

```yaml
# openapi.yaml
openapi: 3.0.1
info:
  title: Ping API
  version: 1.0.0
  description: |
    HTTP echo/probe API secured with an API key (`X-API-Key` header).
    Use this API to inspect requests, test connectivity, and probe status codes.
    No subscription plans are required — just an API key.
servers:
  - url: /ping
security:
  - ApiKeyHeader: []
components:
  securitySchemes:
    ApiKeyHeader:
      type: apiKey
      in: header
      name: X-API-Key
  schemas:
    PingResponse:
      type: object
      description: Response returned by the ping/echo service
      additionalProperties: true

paths:
  /get:
    get:
      summary: Echo a GET request
      description: Returns the query parameters and headers sent with the request.
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/PingResponse'

  /post:
    post:
      summary: Echo a POST request
      description: Echoes the posted JSON body back in the response.
      requestBody:
        required: false
        content:
          application/json:
            schema:
              type: object
              additionalProperties: true
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/PingResponse'

  /status/{code}:
    get:
      summary: Return a specific HTTP status code
      description: Returns the given HTTP status code — useful for testing error handling.
      parameters:
        - name: code
          in: path
          required: true
          schema:
            type: integer
            format: int32
      responses:
        '200':
          description: Proxy response (actual status depends on the `code` path parameter)
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/PingResponse'

```

```bash
# Get a Bearer token (substitute the credentials ./scripts/setup.sh printed)
TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v0.9/auth/login" \
  -d "username=<admin-username>&password=<admin-password>" | jq -r .token)

# Get the default org UUID
ORG_ID=$(curl -sk -H "Authorization: Bearer $TOKEN" \
  https://localhost:3000/organizations | jq -r '.[0].id')

# Create the API
curl -sk -X POST "https://localhost:3000/api/v0.9/apis" \
  -H "Authorization: Bearer $TOKEN" \
  -F "api=@api.yaml;type=application/yaml" \
  -F "apiDefinition=@openapi.yaml;type=application/yaml"
```

Refresh the portal — the Ping API now appears in the catalog. Click it to view the documentation and try-out console.

## What was just created?

| Resource | Value |
|---|---|
| Organization | `default` |
| Default view | `default` |
| Portal URL | `https://localhost:3000/default/views/default` |
| Admin credentials | printed by `./scripts/setup.sh` (local auth) |
| Sample API | `Ping API` visible in the catalog |
