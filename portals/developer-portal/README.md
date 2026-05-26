## Quick start (Docker Compose)

The fastest way to explore the portal:

```bash
cp sample_config.yaml config.yaml   # only needed once
docker compose up
```

Then open **https://localhost:3000/ACME/views/default**

> **Browser warning:** The portal uses a self-signed TLS certificate generated automatically on first start. Click **Advanced → Proceed** (Chrome) or **Accept the Risk** (Firefox) to continue.

Default local users: `admin` / `admin` and `developer` / `developer`

What happens automatically:
- PostgreSQL starts and the DB schema is applied (`artifacts/docker-init/01_schema.sql`)
- A default **ACME** org, view, labels, and subscription plans are seeded (`artifacts/docker-init/02_seed_default.sql`)
- A self-signed TLS certificate is generated and stored in the `certs_data` Docker volume
- The devportal image is built from source and connected to the DB
- On first boot the app seeds the default theme assets into the database

To stop and clean up:
```bash
docker compose down -v   # -v also removes the postgres data volume and certs volume
```

---

## Manual setup (npm start)

Use this for production, custom IdP configuration, or local development without Docker.

### Prerequisites

- **Node.js**: v22.0.0
- **PostgreSQL**: 16 (local install or Docker — see [Database setup](#4--database-setup))
- **psql**: PostgreSQL client (required to run schema and seed scripts)

### 1 — Create config file

```bash
cp sample_config.yaml config.yaml
```

`config.yaml` is your local config file (not committed to git). See [Configuration reference](#configuration-reference) below for all available settings.

### 2 — Configure HTTP mode (simplest)

Open `config.yaml` and confirm these are set (they are the defaults in `sample_config.yaml`):

```yaml
advanced:
  http: true
baseUrl: "http://localhost:3000"
defaultPort: 3000
```

### 3 — Configure the Identity Provider

The portal's login flow requires a valid OAuth2/OIDC provider. Update the `identityProvider` block in `config.yaml` with your IdP details:

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

### 4 — Database setup

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

#### Update DB config in `config.yaml`

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

The sample API artifacts are in `artifacts/default/apiContent/`. Load them with:

```bash
PGPASSWORD=<DB_PASSWORD> ./artifacts/api_data.sh
```

To remove the seeded APIs:
```bash
PGPASSWORD=<DB_PASSWORD> ./artifacts/delete_api_data.sh
```

### 5 — Install and run

```bash
npm install
npm start
```

### Verify

Open **http://localhost:3000/ACME/views/default**

---

## Configuration reference

All settings live in `config.yaml`. Every setting can be overridden with a `DP_*` environment variable — see [Environment variable overrides](#environment-variable-overrides).

The full list of settings with comments is in [`sample_config.yaml`](sample_config.yaml).

### Secrets

Sensitive values belong under the `secrets:` key in `config.yaml`, or injected as env vars:

| Key | Env var | Description |
|-----|---------|-------------|
| `secrets.dbSecret` | `DP_SECRETS_DBSECRET` | Database password |
| `secrets.apiKeySecret` | `DP_SECRETS_APIKEYSECRET` | API key secret |
| `secrets.billingKeyEncryptionKey` | `DP_SECRETS_BILLINGKEYENCRYPTIONKEY` | 64-char hex key for billing encryption |
| `secrets.azureInsightsConnectionString` | `DP_SECRETS_AZUREINSIGHTSCONNECTIONSTRING` | Azure Application Insights |

In production, inject secrets as env vars rather than storing them in the file.

### Local auth

For quick exploration without an IdP, the portal includes built-in local users. They are enabled by default in `sample_config.yaml`:

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

---

## Environment variable overrides

Every config key can be overridden with a `DP_*` environment variable. You can place these in a `.env` file at the project root.

**Convention:**
- Prefix: `DP_`
- `_` separates nesting levels (one token = one config object level)
- `__` represents a literal underscore within a key name
- Tokens are matched case-insensitively against config keys

**Examples:**

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

## Add a third-party API (Admin APIs)

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

### Postman collection

[Devportal Postman collection](https://devportal-4432.postman.co/workspace/Devportal-Workspace~9221a728-2c4b-46ec-acc3-095b9debacbc/collection/5029047-61d763dc-d7b9-4436-9a2e-94585c806943?action=share&creator=5029047)
