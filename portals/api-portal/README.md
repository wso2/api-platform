# API Portal & MCP Hub

A multi-organisation API Portal built on Node.js. It provides a customisable web UI for discovering and subscribing to APIs, and a set of Admin REST APIs for managing organisations, views, API metadata, and portal content.

For end-user documentation, see [docs/](docs/).

## Ports

| Port | Protocol | Description |
|------|----------|-------------|
| `9543` | HTTPS (default) / HTTP | API Portal UI and Admin REST API |

## Prerequisites

- **Node.js** v24 (LTS)
- **Make**
- **Docker + Docker Compose** (for the Docker-based workflow)

> **PostgreSQL** is optional. The portal uses SQLite by default. See [Database setup](#4-database-setup) if you need PostgreSQL.

---

## Quick Start (Docker Compose)

The fastest way to get the portal running — no local Node install required. Requires `openssl` and Docker (used by `../scripts/setup.sh` to bcrypt-hash the admin password).

### Run

```bash
../scripts/setup.sh
docker compose up
```

`../scripts/setup.sh` is a one-time step: it generates API Portal's and the Platform API's encryption/session/JWT secrets (written to `resources/keys/` as files, read via `{{ file }}` — never as env vars), a self-signed TLS certificate, and an admin user into `api-platform.env` (git-ignored). It prompts for an admin username/password interactively, or generates a random password if you press Enter; set `ADMIN_USERNAME`/`ADMIN_PASSWORD` env vars to skip the prompts (e.g. in CI). Safe to re-run — it only fills in what's missing and never overwrites an existing value; to build API Portal from source instead of using the published image, run `docker compose up --build`.

Then open **https://localhost:9543/default/views/default** and log in with the admin credentials `../scripts/setup.sh` printed.

> **Browser warning:** the TLS certificate is self-signed. Click **Advanced → Proceed** (Chrome) or **Accept the Risk** (Firefox) to continue.

What happens automatically on first start:
- The DB schema is applied and the database is initialised automatically
- The organization this instance serves (**default** out of the box, set by `organization.handle` in config) is seeded on startup, along with its view, labels, and subscription plans

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
[API Portal Integration Test](../../.github/workflows/devportal-integration-test.yml)
GitHub Actions workflow. For integration test details, see [it/README.md](it/README.md).

### Clean

```bash
# Stop and remove containers and volumes
docker compose down -v

# Remove build artifacts and distribution zips
make clean
```

`docker compose down` only tears down services matching the profiles currently active for the project (from `COMPOSE_PROFILES` in `.env`, or an explicit `--profile` flag) — it does not remember what was passed to `docker compose up` earlier. If you separately started the optional `ai-workspace` profile (`docker compose --profile ai-workspace up -d`), a bare `docker compose down` leaves it running. Either tear that down explicitly first (`docker compose --profile ai-workspace down`, or `docker compose stop ai-workspace` to stop without removing), or use `docker compose --profile all down -v` to remove every service regardless of which profiles are currently set — every service in this file carries the `all` profile for exactly this case.

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
| `make dist` | Build standalone distribution zip (`target/wso2apip-api-portal-<VERSION>.zip`) |
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

Schema is maintained per dialect in `database/schema.{sqlite,postgres,sqlserver}.sql` — see
[src/db/driver.js](src/db/driver.js) for the query layer that targets these files.

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

`configs/config.toml`'s own defaults are wired for the Docker Compose topology (TLS on, pointing at a cert only the containers have, `auth.local.platform_api_url` pointing at the `platform-api` hostname that only resolves inside the compose network). Plain `npm start` inherits those as-is and will fail — there's no `/app` filesystem or bind-mounted cert here. `npm run start:local` (`package.json`) overrides all of it in one place: TLS off, `auth.local.platform_api_url` pointed at `localhost`, and `auth.local.public_key_path` pointed at the host-side `resources/keys/` that `../scripts/setup.sh` writes rather than the container mount path (see [Local auth](#local-auth) if you're running the Platform API sidecar).

`security.encryption_key`/`security.session_secret`, unlike `public_key_path`, are read via `{{ file "/etc/api-portal/keys/..." }}` directly in `config.toml` — there's no env-var override for that path, so it always looks under `/etc/api-portal/keys` even for `npm run start:local`, which doesn't exist outside the containers. For local (non-Docker) runs, point `configs/config.toml` at the host-side files `../scripts/setup.sh` already generated instead:

```toml
[api_portal.security]
encryption_key = '{{ file "resources/keys/api-portal-encryption.key" }}'
session_secret = '{{ file "resources/keys/api-portal-session-secret" }}'
```

`{{ file }}` only reads from an allowlisted directory (`/etc/api-portal`, `/secrets/api-portal` by default), so also set `APIP_CONFIG_FILE_SOURCE_ALLOWLIST=resources/keys` when running `npm run start:local` (or add it to the `start:local` script in `package.json`). Don't commit the `config.toml` path change — it's a local-only edit for the Docker-free flow.

### 3. Configure the Identity Provider (optional)

The portal's login flow requires a valid OAuth2/OIDC provider. Set `[api_portal.auth]` `mode = "idp"` and fill in the `[api_portal.auth.idp]` block in `configs/config.toml`:

```toml
[api_portal.auth]
mode = "idp"

[api_portal.auth.idp]
issuer = "https://<your-idp>/oauth2/token"
authorization_url = "https://<your-idp>/oauth2/authorize"
token_url = "https://<your-idp>/oauth2/token"
user_info_url = "https://<your-idp>/oauth2/userinfo"
jwks_url = "https://<your-idp>/oauth2/jwks"
client_id = "<your-client-id>"
callback_url = "http://localhost:9543/<handle>/callback"
```

For local exploration you can skip IdP setup by using the Platform API sidecar instead (see [Local auth](#local-auth)).

### 4. Database setup

#### SQLite (default — no setup required)

The portal uses SQLite out of the box. The database file is created automatically at the path configured by `database.path` (default: `./api-portal.db`). No installation or schema migration step is needed.

#### PostgreSQL (optional)

To use PostgreSQL instead, spin up an instance:

```bash
docker run --name api-portal-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=api_portal \
  -p 5432:5432 \
  -d postgres:16
```

Then update the `[api_portal.database]` block in `configs/config.toml`:

```toml
[api_portal.database]
driver = "postgres"
host = "localhost"
port = 5432
name = "api_portal"
user = "postgres"
password = "postgres"
```

In production, set the password via the `APIP_AP_DATABASE_PASSWORD` environment variable instead of storing it in the config file.

### 5. Choose the organization

A API Portal instance serves exactly one organization, named by `organization.handle`
(or the `APIP_AP_ORGANIZATION_HANDLE` env var). It is **required** — the portal refuses to
start without it — and the packaged `configs/config.toml` sets it to `default`. The
organization is seeded on startup if it doesn't exist yet, so no manual step is required.

Under local auth this must match the Platform API's `[platform_api.auth.file.organization]`
`id`: that value is what its tokens carry in the `org_handle` claim, and a login naming any
other organization is rejected. See [Manage the Organization](docs/administer/manage-organizations.md).

### 6. Install and run

```bash
npm install
npm run start:local
```

Open **http://localhost:9543/default/views/default**

---

## Seed Sample APIs (optional)

Deploys the sample APIs and MCP servers under `samples/` into the default organisation, entirely through the public REST API — API Portal itself has no built-in seeding logic. Works with both the Docker Compose and `npm start` workflows.

```bash
./scripts/seed-samples.sh
```

Prompts for the admin username/password (or set `ADMIN_USERNAME`/`ADMIN_PASSWORD` to skip the prompt, e.g. in CI). Safe to re-run — entries that already exist are skipped. Set `API_PORTAL_URL`/`PLATFORM_API_URL` to override the defaults (`https://localhost:9543` / `https://localhost:9243`) — e.g. `API_PORTAL_URL=http://localhost:9543` when running against `npm run start:local`.

---

## Configuration Reference

All settings live in `configs/config.toml`. There is no automatic `APIP_AP_*` override layer: an environment variable takes effect only for a setting whose `config.toml` entry explicitly references it with a `{{ env "NAME" "fallback" }}` token (see [Environment variable overrides](#environment-variable-overrides)).

The full annotated list of settings is in [`configs/config-template.toml`](configs/config-template.toml).

### Local auth

For quick exploration without an IdP, the portal delegates credential validation to a Platform API sidecar. `docker-compose.yaml` mounts the Platform API's own [`../../platform-api/config/config.toml`](../../platform-api/config/config.toml) directly — there is no per-portal copy. Users and bcrypt-hashed passwords are defined there, under `[[platform_api.auth.file.users]]`; each names one or more roles from the [`role-to-scope-mapping.yaml`](../../platform-api/resources/role-to-scope-mapping.yaml) mounted alongside it, and those roles are where the `dp:*` scopes the portal enforces come from:

```toml
[[platform_api.auth.file.users]]
username      = "admin"
password_hash = "$2y$10$..."   # bcrypt hash — generate with: htpasswd -bnBC 12 "" <pw> | tr -d ':\n'
roles         = ["ap_admin"]   # grants dp:organization:manage, dp:api:manage, … — see platform-api's role-to-scope-mapping.yaml
```

To change what a portal user may do, edit that role's entry in [platform-api's `role-to-scope-mapping.yaml`](../../platform-api/resources/role-to-scope-mapping.yaml) — or name a second role alongside it — rather than listing scopes on the user block.

Note there are two files with this name, read by different components in different modes:

| File | Read by | When |
|---|---|---|
| [`platform-api/resources/role-to-scope-mapping.yaml`](../../platform-api/resources/role-to-scope-mapping.yaml) | Platform API | Local auth — expands a file user's `roles` into the `scope` claim of the token it issues (roles named `ap_*`) |
| [`resources/role-to-scope-mapping.yaml`](resources/role-to-scope-mapping.yaml) | This portal | `auth.authorization.mode = "role"` (the default) — expands an incoming token's roles claim on every request to authorize the portal's REST surface. Recognises `dp_admin`/`dp_subscriber`, and **aliases** `ap_admin`/`ap_subscriber` so local-auth tokens (which carry `ap_*` roles) are authorized unchanged |

Because role mode is the default, the local-auth quickstart uses **both** files, each governing a different surface: the Platform API expands the file user's `roles` into the token it issues (and authorizes its own surface with that mapping), and the portal then expands that same token's roles claim on every request to authorize its REST surface — which is why the portal file aliases `ap_admin`/`ap_subscriber`. An external IDP in role mode drives the portal file directly with its own groups. So to change what a role may do, edit the file for the surface you mean — the portal file for the portal's REST permissions, the Platform API file for the Platform API's — and both if the change must hold across both components. See [Authorization](docs/administer/authentication.md#authorization).

The portal config (or `APIP_AP_AUTH_LOCAL_*` env vars) must point to the Platform API. `config.toml`'s own defaults assume Docker Compose, where `platform-api` is a resolvable hostname on the compose network — `npm run start:local` already overrides `platform_api_url` to `https://localhost:9243` (the sidecar's port published to the host) and `tls_skip_verify = true` (self-signed cert), so no manual edit is needed for that flow:

```toml
[api_portal.auth.local]
platform_api_url = "https://localhost:9243"  # env: APIP_AP_AUTH_LOCAL_PLATFORM_API_URL
public_key_path = "/etc/api-portal/keys/jwt_public.pem"  # path to the Platform API's auth.jwt.public_key PEM — env: APIP_AP_AUTH_LOCAL_PUBLIC_KEY_PATH
tls_skip_verify = true                    # Platform API uses a self-signed cert
```

Tokens are signed asymmetrically (RS256): the Platform API mints them with its `auth.jwt.private_key` and the portal verifies them against the matching public key above. There is no shared HMAC secret, and the private key never leaves the Platform API — `../scripts/setup.sh` generates the keypair into `resources/keys/`, and `docker-compose.yaml` mounts only the `jwt_public.pem` file into the portal (at `/etc/api-portal/keys/jwt_public.pem`); the Platform API's own `jwt_private.pem` and `encryption.key` (its at-rest encryption key, `resources/keys/encryption.key`) are never mounted into the portal container. The portal has its own, separate secrets — `resources/keys/api-portal-encryption.key` and `resources/keys/api-portal-session-secret` — which `docker-compose.yaml` mounts into the portal container at `/etc/api-portal/keys/encryption.key` and `/etc/api-portal/keys/session-secret` respectively; these never leave the portal container either way.

For production, configure an OIDC identity provider instead of local auth. Its tokens must
assert this instance's organization — a login whose organization claim resolves to any other
organization is refused.

### Environment variable overrides

There is **no** automatic `APIP_AP_*` override layer. A variable takes effect only where
`configs/config.toml` explicitly references it with a `{{ env "NAME" "fallback" }}` token
— the same design platform-api uses (see `src/config/configLoader.js`). Setting a
variable no key references does nothing, silently.

Following `platform-api/config/config.toml`, tokens are used sparingly: a key gets one
only where something actually drives it — the Compose database overrides,
`npm run start:local`, `docker-entrypoint.sh`, or a secret. Everything else is a plain
literal, so the file states its own effective configuration.

These are the variables the shipped `configs/config.toml` honours:

| Env var | Config path |
|---------|-------------|
| `APIP_AP_SERVER_PORT` | `config.server.port` |
| `APIP_AP_SERVER_HTTPS_ENABLED` | `config.server.https.enabled` |
| `APIP_AP_LOGGING_LEVEL` | `config.logging.level` |
| `APIP_AP_DATABASE_DRIVER` | `config.database.driver` |
| `APIP_AP_DATABASE_PATH` | `config.database.path` |
| `APIP_AP_DATABASE_HOST` | `config.database.host` |
| `APIP_AP_DATABASE_PORT` | `config.database.port` |
| `APIP_AP_DATABASE_NAME` | `config.database.name` |
| `APIP_AP_DATABASE_USER` | `config.database.user` |
| `APIP_AP_DATABASE_PASSWORD` | `config.database.password` |
| `APIP_AP_AUTH_LOCAL_PLATFORM_API_URL` | `config.auth.local.platformApiUrl` |
| `APIP_AP_AUTH_LOCAL_PUBLIC_KEY_PATH` | `config.auth.local.publicKeyPath` |
| `APIP_AP_AUTH_LOCAL_TLS_SKIP_VERIFY` | `config.auth.local.tlsSkipVerify` |
| `APIP_AP_ORGANIZATION_HANDLE` | `config.organization.handle` |
| `APIP_AP_ORGANIZATION_DISPLAY_NAME` | `config.organization.displayName` |

To make any other key settable from the environment, add the token to `config.toml`
yourself. To change something without an environment variable — including the
`[api_portal.auth.authorization]` block and the IDP settings — edit `config.toml`, or
layer a thin overlay with a second `--config` flag.

`.env` example (loaded from `api-platform.env` at the project root):
```dotenv
APIP_AP_DATABASE_HOST=my-postgres-host
APIP_AP_DATABASE_PASSWORD=my-secret-password
```

---

## Publish your first API

Create an API manifest file and an OpenAPI definition, then upload them:

```yaml
# api.yaml
apiVersion: API Portal.api-platform.wso2.com/v1alpha2
kind: RestApi

metadata:
  name: reading-list-api-v1.0

spec:
  type: REST
  displayName: Reading-List-API
  version: v1.0
  description: Sample reading-list API for tracking books and their reading status. Open access — no API key or subscription required.
  status: PUBLISHED
  referenceID: reading-list-api-v1.0

  tags:
    - reading-list
    - books

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
    sandboxUrl: http://localhost:8080/reading-list/v1.0
    productionUrl: http://localhost:8080/reading-list/v1.0
```

```yaml
# openapi.yaml
openapi: 3.0.1
info:
  title: Reading-List-API
  version: v1.0
  description: |
    Track a personal reading list — add books, update their reading status,
    and remove them when you are done.
    Open access: no API key or subscription token is required.
servers:
  - url: /reading-list/v1.0
components:
  schemas:
    Book:
      type: object
      properties:
        id:
          type: string
          format: uuid
          readOnly: true
        title:
          type: string
          example: The Great Gatsby
        author:
          type: string
          example: F. Scott Fitzgerald
        status:
          type: string
          enum: [to_read, reading, read]
      required: [title, author, status]
    BookList:
      type: object
      properties:
        books:
          type: array
          items:
            $ref: '#/components/schemas/Book'
      required: [books]
    Error:
      type: object
      properties:
        error:
          type: string
      required: [error]

paths:
  /books:
    get:
      summary: List books
      description: Returns every book on the reading list.
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/BookList'
    post:
      summary: Add a book
      description: Adds a book to the reading list and returns it with its assigned id.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Book'
      responses:
        '201':
          description: Created
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Book'

  /books/{id}:
    parameters:
      - name: id
        in: path
        required: true
        schema:
          type: string
          format: uuid
    get:
      summary: Get a book
      description: Returns a single book by id.
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Book'
        '404':
          description: Not Found
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Error'
    put:
      summary: Update a book
      description: Replaces a book's details — commonly used to move it between reading statuses.
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/Book'
      responses:
        '200':
          description: OK
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/Book'
    delete:
      summary: Remove a book
      description: Removes a book from the reading list.
      responses:
        '204':
          description: No Content

```

```bash
# Get a Bearer token from the Platform API (substitute the credentials
# ../scripts/setup.sh printed)
TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v0.9/auth/login" \
  -d "username=<admin-username>&password=<admin-password>" | jq -r .token)

# Create the API
curl -sk -X POST "https://localhost:9543/api/v0.9/apis" \
  -H "Authorization: Bearer $TOKEN" \
  -F "metadata=@api.yaml;type=application/yaml" \
  -F "definition=@openapi.yaml;type=application/yaml"
```

Refresh the portal — the Reading-List-API now appears in the catalog. Click it to view the documentation and try-out console.

## What was just created?

| Resource | Value |
|---|---|
| Organization | `default` |
| Default view | `default` |
| Portal URL | `https://localhost:9543/default/views/default` |
| Admin credentials | printed by `../scripts/setup.sh` (local auth) |
| Sample API | `Reading-List-API` visible in the catalog |
