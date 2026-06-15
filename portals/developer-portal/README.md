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
mkdir -p configs && cp sample.config.yaml configs/config.yaml
docker compose up
```

Then open **https://localhost:3000/default/views/default**

> **Browser warning:** A self-signed TLS certificate is generated automatically on first start. Click **Advanced → Proceed** (Chrome) or **Accept the Risk** (Firefox) to continue.

Default local users: `admin` / `admin` and `developer` / `developer`

What happens automatically on first start:
- The DB schema is applied and the database is initialised automatically
- A default **default** org, view, labels, and subscription plans are seeded automatically on startup (controlled by `defaultOrgName` in config)
- A self-signed TLS certificate is generated and stored in the `certs_data` Docker volume

### Test

```bash
# Run Cypress integration tests headlessly inside Docker
make it

# Open Cypress interactive UI — requires the portal running locally first
make it-open
```

For integration test details, see [it/README.md](it/README.md).

### Clean

```bash
# Stop and remove containers and volumes
docker compose down -v

# Remove build artifacts and distribution zips
make clean
```

---

## Development (`npm start`)

Use this for active development, custom IdP configuration, or when you prefer to run Node directly.

### 1. Create config file

```bash
mkdir -p configs && cp sample.config.yaml configs/config.yaml
```

`configs/config.yaml` is your local config file (not committed to git). See [Configuration reference](#configuration-reference) below for all available settings.

### 2. Configure HTTP mode (optional)

Open `configs/config.yaml` and confirm these are set (they are the defaults in `sample.config.yaml`):

```yaml
advanced:
  http: true
baseUrl: "http://localhost:3000"
defaultPort: 3000
```

### 3. Configure the Identity Provider (optional)

The portal's login flow requires a valid OAuth2/OIDC provider. Update the `identityProvider` block in `configs/config.yaml`:

```yaml
identityProvider:
  issuer: "https://<your-idp>/oauth2/token"
  authorizationURL: "https://<your-idp>/oauth2/authorize"
  tokenURL: "https://<your-idp>/oauth2/token"
  userInfoURL: "https://<your-idp>/oauth2/userinfo"
  jwksURL: "https://<your-idp>/oauth2/jwks"
  clientId: "<your-client-id>"
  callbackURL: "http://localhost:3000/<orgHandle>/callback"
```

For local exploration you can skip IdP setup by using the built-in local users instead (see [Local auth](#local-auth)).

### 4. Database setup

#### SQLite (default — no setup required)

The portal uses SQLite out of the box. The database file is created automatically at the path configured by `db.storage` (default: `./devportal.db`). No installation or schema migration step is needed.

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

Then update the `db` block in `configs/config.yaml`:

```yaml
db:
  dialect: "postgres"
  host: "localhost"
  port: 5432
  database: "devportal"
  username: "postgres"
  password: "postgres"
```

In production, set the password via the `DP_DB_PASSWORD` environment variable instead of storing it in the config file.

### 5. Seed default organization

The default organization is seeded automatically on startup when `defaultOrgName` is set in config (or via `DP_DEFAULTORGNAME` env var).
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

**npm start (HTTP):**
```bash
DEVPORTAL_URL=http://localhost:3000 ./seeders/seed-apis.sh
```

**Docker Compose (HTTPS):**
```bash
DEVPORTAL_URL=https://localhost:3000 ./seeders/seed-apis.sh
```

> **Note:**
>
> Use the following command to pass variables to the script.
> ```bash
> DEVPORTAL_URL=https://localhost:3000 ORG_ID=1ba42a09-45c0-40f8-a1bf-e4aa7cde1575 DEVPORTAL_CREDENTIALS=admin:admin ./seeders/seed-apis.sh
> ```

---

## Configuration Reference

All settings live in `configs/config.yaml`. Every setting can also be overridden with a `DP_*` environment variable.

The full annotated list of settings is in [`sample.config.yaml`](sample.config.yaml).

### Local auth

For quick exploration without an IdP, the portal includes built-in local users enabled by default in `sample.config.yaml`:

```yaml
defaultAuth:
  users:
    - username: "admin"
      password: "admin"
      roles: ["admin"]
      orgClaimName: "default"
      organizationIdentifier: "default"
    - username: "developer"
      password: "developer"
      roles: ["Internal/subscriber"]
      orgClaimName: "default"
      organizationIdentifier: "default"
```

Remove or empty the `users` list in production.

### Environment variable overrides

Every config key can be overridden with a `DP_*` environment variable. You can place these in a `.env` file at the project root.

**Convention:**
- Prefix: `DP_`
- `_` separates nesting levels (one token = one config object level)
- `__` represents a literal underscore within a key name
- Tokens are matched case-insensitively against config keys

| Env var | Config path |
|---------|-------------|
| `DP_DB_HOST` | `config.db.host` |
| `DP_DB_PORT` | `config.db.port` |
| `DP_ADVANCED_HTTP` | `config.advanced.http` |
| `DP_IDENTITYPROVIDER_CLIENTID` | `config.identityProvider.clientId` |
| `DP_IDENTITYPROVIDER_ISSUER` | `config.identityProvider.issuer` |
| `DP_BASEURL` | `config.baseUrl` |
| `DP_DEFAULTPORT` | `config.defaultPort` |
| `DP_ADVANCED_DBSSLDIALECTOPTION` | `config.advanced.dbSslDialectOption` |

`.env` example:
```dotenv
DP_DB_HOST=my-postgres-host
DP_SECRETS_DBSECRET=my-secret-password
DP_IDENTITYPROVIDER_CLIENTID=my-client-id
```

---

## Publish your first API

Create an API manifest file and an OpenAPI definition, then upload them:

```yaml
# api.yaml
apiVersion: devportal.api-platform.wso2.com/v1
kind: RestApi

metadata:
  name: ping-api-v1.0

spec:
  type: REST
  displayName: Ping API
  version: v1.0
  description: Sample HTTP echo/probe API. Requires API key authentication. No subscription plans.
  provider: WSO2
  status: PUBLISHED
  gatewayType: wso2/api-platform
  referenceID: ping-api-v1.0

  tags:
    - ping
    - api-key

  labels:
    - default

  subscriptionPolicies: []

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
# Get the default org UUID
ORG_ID=$(curl -sk -u admin:admin https://localhost:3000/devportal/organizations | grep -o '"orgID":"[^"]*"' | head -1 | cut -d'"' -f4)

# Create the API
curl -sk -X POST "https://localhost:3000/devportal/organizations/$ORG_ID/apis" \
  -u admin:admin \
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
