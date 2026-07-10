# Developer Portal

A multi-organisation API developer portal built on Node.js. It provides a customisable web UI for discovering and subscribing to APIs, and a set of Admin REST APIs for managing organisations, views, API metadata, and portal content.

For end-user documentation, see [docs/](docs/).

## Ports

| Port | Protocol | Description |
|------|----------|-------------|
| `3000` | HTTPS (default) / HTTP | Developer Portal UI and Admin REST API |

## Prerequisites

- **Node.js** v23 (or v22+)
- **Make**
- **Docker + Docker Compose** (for the Docker-based workflow)

> **PostgreSQL** is optional. The portal uses SQLite by default. See [Database setup](#4-database-setup) if you need PostgreSQL.

---

## Quick Start (Docker Compose)

The fastest way to get the portal running — no local Node install required.

### Build

```bash
# Build the developer-portal Docker image from source
make build
```

### Run

```bash
mkdir -p configs && cp configs/config.toml.example configs/config.toml
docker compose up
```

Then open **https://localhost:3000/default/views/default**

> **Browser warning:** A self-signed TLS certificate is generated automatically on first start. Click **Advanced → Proceed** (Chrome) or **Accept the Risk** (Firefox) to continue.

Default credentials: `admin` / `admin` (defined in `configs/config-platform-api.toml`)

What happens automatically on first start:
- The DB schema is applied and the database is initialised automatically
- A default **default** org, view, labels, and subscription plans are seeded automatically on startup (controlled by `organization.default_name` in config)
- A self-signed TLS certificate is generated and stored in the `certs_data` Docker volume

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

### 1. Create config file

```bash
mkdir -p configs && cp configs/config.toml.example configs/config.toml
```

`configs/config.toml` is your local config file (not committed to git). See [Configuration reference](#configuration-reference) below for all available settings.

### 2. Configure HTTP mode (optional)

Open `configs/config.toml` and confirm these are set (they are the defaults in `configs/config.toml.example`):

```toml
[tls]
enabled = false

[server]
base_url = "http://localhost:3000"
port = 3000
```

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

The portal uses SQLite out of the box. The database file is created automatically at the path configured by `database.file` (default: `./devportal.db`). No installation or schema migration step is needed.

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

Then update the `[database]` block in `configs/config.toml`:

```toml
[database]
type = "postgres"
host = "localhost"
port = 5432
name = "devportal"
username = "postgres"
password = "postgres"
```

In production, set the password via the `APIP_DP_DATABASE_PASSWORD` environment variable instead of storing it in the config file.

### 5. Seed default organization

The default organization is seeded automatically on startup when `organization.default_name` is set in config (or via `APIP_DP_ORGANIZATION_DEFAULTNAME` env var).
No manual step is required.

### 6. Install and run

```bash
npm install
npm start
```

Open **http://localhost:3000/default/views/default**

---

## Seed Sample APIs (optional)

Seeds a set of sample APIs into the default organisation. Works with both the Docker Compose and `npm start` workflows.

Get a Bearer token first, then pass it via `DEVPORTAL_TOKEN`:

**npm start (HTTP):**
```bash
TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v0.9/auth/login" \
  -d "username=admin&password=admin" | jq -r .token)
DEVPORTAL_URL=http://localhost:3000 DEVPORTAL_TOKEN=$TOKEN ./seeders/seed-apis.sh
```

**Docker Compose (HTTPS):**
```bash
TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v0.9/auth/login" \
  -d "username=admin&password=admin" | jq -r .token)
DEVPORTAL_URL=https://localhost:3000 DEVPORTAL_TOKEN=$TOKEN ./seeders/seed-apis.sh
```

---

## Configuration Reference

All settings live in `configs/config.toml`. Every setting can also be overridden with an `APIP_DP_*` environment variable.

The full annotated list of settings is in [`configs/config.toml.example`](configs/config.toml.example).

### Local auth

For quick exploration without an IdP, the portal delegates credential validation to a Platform API sidecar. Users, bcrypt-hashed passwords, and `dp:*` scopes are defined in `configs/config-platform-api.toml` (copy from `configs/config-platform-api.toml.example`):

```toml
[[auth.file_based.users]]
username      = "admin"
password_hash = "$2y$10$..."   # bcrypt hash — generate with: htpasswd -bnBC 12 "" <pw> | tr -d ':\n'
scopes        = "dp:org_manage dp:api_manage ..."
```

The portal config (or `APIP_DP_PLATFORMAPI_*` env vars) must point to the Platform API. Docker Compose sets these automatically:

```toml
[platform_api]
base_url = "https://platform-api:9243"   # env: APIP_DP_PLATFORMAPI_BASEURL
jwt_secret = ""                           # same as AUTH_JWT_SECRET_KEY — env: APIP_DP_PLATFORMAPI_JWTSECRET
insecure = false                          # set true when Platform API uses a self-signed cert
```

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
| `APIP_DP_TLS_ENABLED` | `config.tls.enabled` |
| `APIP_DP_IDP_CLIENTID` | `config.idp.clientId` |
| `APIP_DP_IDP_ISSUER` | `config.idp.issuer` |
| `APIP_DP_SERVER_BASEURL` | `config.server.baseUrl` |
| `APIP_DP_SERVER_PORT` | `config.server.port` |
| `APIP_DP_DATABASE_SSL_ENABLED` | `config.database.ssl.enabled` |

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
apiVersion: devportal.api-platform.wso2.com/v1alpha1
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
# Get a Bearer token
TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v0.9/auth/login" \
  -d "username=admin&password=admin" | jq -r .token)

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
| Admin credentials | `admin` / `admin` (local auth) |
| Sample API | `Ping API` visible in the catalog |
