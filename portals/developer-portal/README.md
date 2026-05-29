# Developer Portal

A multi-organisation API developer portal built on Node.js. It provides a customisable web UI for discovering and subscribing to APIs, and a set of Admin REST APIs for managing organisations, views, API metadata, and portal content.

For end-user documentation, see [docs/](docs/).

## Ports

| Port | Protocol | Description |
|------|----------|-------------|
| `3000` | HTTPS (default) / HTTP | Developer Portal UI and Admin REST API |

## Prerequisites

- **Node.js** v22.0.0
- **Make**
- **PostgreSQL** 16
- **Docker + Docker Compose** (for the Docker-based workflow)
- **psql** (required to run schema/seed scripts manually)

---

## Quick Start (Docker Compose)

The fastest way to get the portal running — no local Node or PostgreSQL install required.

### Build

```bash
# Build the developer-portal Docker image from source
make build
```

### Run

```bash
cp sample_config.yaml configs/config.yaml   # optional — omit to rely entirely on environment variables
docker compose up
```

Then open **https://localhost:3000/ACME/views/default**

> **Browser warning:** A self-signed TLS certificate is generated automatically on first start. Click **Advanced → Proceed** (Chrome) or **Accept the Risk** (Firefox) to continue.

Default local users: `admin` / `admin` and `developer` / `developer`

What happens automatically on first start:
- PostgreSQL starts and the DB schema is applied (`artifacts/docker-init/01_schema.sql`)
- A default **ACME** org, view, labels, and subscription plans are seeded (`artifacts/docker-init/02_seed_default.sql`)
- A self-signed TLS certificate is generated and stored in the `certs_data` Docker volume
- On first boot the app seeds the default theme assets into the database

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
# Stop and remove containers, volumes, and the postgres data volume
docker compose down -v

# Remove build artifacts and distribution zips
make clean
```

---

## Development (`npm start`)

Use this for active development, custom IdP configuration, or when you prefer to run Node directly.

### 1. Create config file

```bash
cp sample_config.yaml configs/config.yaml
```

`configs/config.yaml` is your local config file (not committed to git). See [Configuration reference](#configuration-reference) below for all available settings.

### 2. Configure HTTP mode (optional)

Open `configs/config.yaml` and confirm these are set (they are the defaults in `sample_config.yaml`):

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

#### Create the database

```bash
createdb -h <HOST> -U <USER> devportal
```

Or spin up PostgreSQL with Docker:

```bash
docker run --name devportal-postgres \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=devportal \
  -p 5432:5432 \
  -d postgres:16
```

#### Update DB config in `configs/config.yaml`

```yaml
db:
  host: "localhost"
  port: 5432
  database: "devportal"
  username: "postgres"
  dialect: "postgres"

secrets:
  dbSecret: "postgres"   # DB password
```

#### Apply the schema

> ⚠️ This drops and recreates all tables — don't run against a database you can't reset.

```bash
psql -h <HOST> -p <PORT> -U <USER> -d devportal -f artifacts/script.sql
```

#### Seed default org and theme (optional)

```bash
chmod +x artifacts/*.sh
./artifacts/org_data.sh        # ACME org + default view + org layout assets
./artifacts/theme_data.sh      # Default theme/styling assets
```

Pass the DB password non-interactively:
```bash
PGPASSWORD=<DB_PASSWORD> ./artifacts/org_data.sh
PGPASSWORD=<DB_PASSWORD> ./artifacts/theme_data.sh
```

#### Add sample APIs (optional)

```bash
PGPASSWORD=<DB_PASSWORD> ./artifacts/api_data.sh

# To remove the seeded APIs:
PGPASSWORD=<DB_PASSWORD> ./artifacts/delete_api_data.sh
```

### 5. Install and run

```bash
npm install
npm start
```

Open **http://localhost:3000/ACME/views/default**

---

## Configuration Reference

All settings live in `configs/config.yaml`. Every setting can also be overridden with a `DP_*` environment variable.

The full annotated list of settings is in [`sample_config.yaml`](sample_config.yaml).

### Secrets

Sensitive values belong under the `secrets:` key in `configs/config.yaml`, or injected as env vars. In production, prefer env vars over storing secrets in the file.

| Key | Env var | Description |
|-----|---------|-------------|
| `secrets.dbSecret` | `DP_SECRETS_DBSECRET` | Database password |
| `secrets.apiKeySecret` | `DP_SECRETS_APIKEYSECRET` | API key secret |
| `secrets.billingKeyEncryptionKey` | `DP_SECRETS_BILLINGKEYENCRYPTIONKEY` | 64-char hex key for billing encryption |
| `secrets.azureInsightsConnectionString` | `DP_SECRETS_AZUREINSIGHTSCONNECTIONSTRING` | Azure Application Insights connection string |

### Local auth

For quick exploration without an IdP, the portal includes built-in local users enabled by default in `sample_config.yaml`:

```yaml
defaultAuth:
  users:
    - username: "admin"
      password: "admin"
      roles: ["admin"]
      orgClaimName: "ACME"
      organizationIdentifier: "ACME"
    - username: "developer"
      password: "developer"
      roles: ["Internal/subscriber"]
      orgClaimName: "ACME"
      organizationIdentifier: "ACME"
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
| `DP_SECRETS_DBSECRET` | `config.secrets.dbSecret` |
| `DP_ADVANCED_HTTP` | `config.advanced.http` |
| `DP_IDENTITYPROVIDER_CLIENTID` | `config.identityProvider.clientId` |
| `DP_IDENTITYPROVIDER_ISSUER` | `config.identityProvider.issuer` |
| `DP_BASEURL` | `config.baseUrl` |
| `DP_DEFAULTPORT` | `config.defaultPort` |
| `DP_SEEDDEFAULTS` | `config.seedDefaults` |
| `DP_ADVANCED_DBSSLDIALECTOPTION` | `config.advanced.dbSslDialectOption` |

`.env` example:
```dotenv
DP_DB_HOST=my-postgres-host
DP_SECRETS_DBSECRET=my-secret-password
DP_IDENTITYPROVIDER_CLIENTID=my-client-id
```

---

## Add a Third-Party API (Admin APIs)

Use the Devportal Admin REST APIs to publish APIs without using seed scripts.

### 1 — Configure a provider (control plane)

```bash
curl --location 'http://localhost:3000/devportal/organizations/{orgId}/provider' \
  --header 'Content-Type: application/json' \
  --header 'Authorization: Bearer <access_token>' \
  --data '{
    "name": "MuleSoft",
    "providerURL": "https://anypoint.mulesoft.com/login/signin?apintent=generic"
  }'
```

### 2 — Create an API (metadata + definition file)

- `apiType` can be `REST`, `AsyncAPI`, `GraphQL`, or `SOAP`
- This is a multipart request: JSON metadata + an API definition file

```bash
curl --location 'http://localhost:3000/devportal/organizations/{organizationID}/apis' \
  --header 'Authorization: Bearer <access_token>' \
  --form 'api-metadata="{
    \"apiInfo\": {
      \"apiName\": \"NavigationAPI\",
      \"provider\": \"MuleSoft\",
      \"orgName\": \"ACME\",
      \"apiType\": \"REST\",
      \"apiVersion\": \"1.0.0\",
      \"apiDescription\": \"<description>\",
      \"visibility\": \"PUBLIC\"
    }
  }"; type=application/json' \
  --form 'apiDefinition=@"{path-to-apiDefinition.json}"'
```

### 3 — Upload API landing page content (optional)

Create a zip with this structure:

```text
{API NAME}/
  content/
    api-content.hbs
    apiContent.md
  images/
    icon.svg
    product.png
```

Then upload it:

```bash
curl --location --request POST 'http://localhost:3000/devportal/organizations/{organizationID}/apis/{apiID}/template' \
  --header 'Authorization: Bearer <access_token>' \
  --form 'apiContent=@"{path-to-zip-file}"' \
  --form 'imageMetadata="{
    \"api-icon\": \"icon.svg\",
    \"api-product\": \"product.png\"
  }"; type=application/json'
```

---

## Postman Collection

[Devportal Postman collection](https://devportal-4432.postman.co/workspace/Devportal-Workspace~9221a728-2c4b-46ec-acc3-095b9debacbc/collection/5029047-61d763dc-d7b9-4436-9a2e-94585c806943?action=share&creator=5029047)
