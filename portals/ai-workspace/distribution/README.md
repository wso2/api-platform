# WSO2 API Platform — AI Workspace

A standalone distribution of the AI Workspace and Platform API, orchestrated with Docker Compose. The AI Workspace is a React SPA served by a Go BFF (Backend-for-Frontend) that proxies all browser traffic same-origin and owns authentication — tokens live in a server-side session (HttpOnly cookie) and never reach the browser.

## Contents

```
wso2apip-ai-workspace-<version>/
├── README.md
├── docker-compose.yaml                          # AI Workspace + Platform API
├── configs/
│   ├── config.toml                              # AI Workspace active configuration
│   ├── config-template.toml                     # AI Workspace full configuration reference
│   ├── config-platform-api.toml                 # Platform API active configuration
│   └── config-platform-api-template.toml        # Platform API full configuration reference
└── resources/
    ├── roles.yaml                               # Platform API role definitions
    └── platform-api/
        └── db-scripts/                          # Platform API schema scripts (schema.*.sql)
```

## Prerequisites

- Docker Engine 24+
- Docker Compose v2

No other tools are required to run the stack.

## Quick Start

Generate the two required secret keys and write them to a `.env` file alongside `docker-compose.yaml`:

```bash
echo "AUTH_JWT_SECRET_KEY=$(openssl rand -hex 32)" >> .env
echo "ENCRYPTION_KEY=$(openssl rand -hex 32)" >> .env
```

> **Important:** keep these values stable — changing them after first start invalidates all existing signed tokens and encrypted data.

```bash
docker compose up -d
```

Verify both services are healthy:

```bash
curl -fk https://localhost:9243/health    # Platform API
curl -fk https://localhost:5380/healthz   # AI Workspace
```

Open the AI Workspace in a browser at `https://localhost:5380` and log in with the default credentials: **admin / admin**.

> **Browser trust warning?** Both services use a self-signed TLS certificate by default. Click **Advanced → Proceed** to continue. See [Custom TLS Certificates](#custom-tls-certificates) to remove the warning permanently.

## Exposed Ports

| Port | Service | Description |
|------|---------|-------------|
| `5380` | AI Workspace (BFF) | HTTPS — browser entry point |
| `9243` | Platform API | HTTPS — backend REST API |

## Configuration

### AI Workspace (`configs/config.toml`)

| Setting | Description | Default |
|---------|-------------|---------|
| `domain` | Host and port shown in the browser address bar | `localhost:5380` |
| `auth_mode` | `basic` (file-based quickstart) or `oidc` (external IDP) | `basic` |
| `controlplane_host` | Address gateways use to reach the Platform API | `host.docker.internal:9243` |
| `platform_gateway_versions` | Gateway versions shown in the create-gateway selector | _(current release)_ |

### Platform API (`configs/config-platform-api.toml`)

| Setting | Description | Default |
|---------|-------------|---------|
| `log_level` | Log level (`DEBUG`, `INFO`, `WARN`, `ERROR`) | `INFO` |
| `database.driver` | `sqlite3` or `postgres` | `sqlite3` |
| `auth.file_based.users` | Local user credentials (change the password hash before sharing) | `admin` / `admin` |

See `configs/config-template.toml` and `configs/config-platform-api-template.toml` for a fully-commented reference of every available setting.

## Required Secrets

Two secrets must be set before starting the stack. Pass them via environment variables or a `.env` file placed alongside `docker-compose.yaml`:

| Variable | Purpose | Generate with |
|----------|---------|---------------|
| `AUTH_JWT_SECRET_KEY` | Signs login JWTs | `openssl rand -hex 32` |
| `ENCRYPTION_KEY` | Encrypts secrets, subscription tokens, and HMAC keys at rest | `openssl rand -hex 32` |

Both must be **stable** — rotating them invalidates all existing signed tokens and encrypted data.

## Demo Mode vs. Production

The stack ships in **demo mode** (`APIP_DEMO_MODE=true` by default), which enables zero-config startup with file-based auth and self-signed TLS. Set `APIP_DEMO_MODE=false` in your `.env` to enable production-grade startup checks:

| Check | Demo mode (default) | Production (`false`) |
|-------|--------------------|-----------------------|
| Authentication | File-based (`admin` / `admin`) allowed | OIDC required — basic auth rejected |
| Inbound TLS | Self-signed cert auto-generated | Real cert/key must be mounted |
| Upstream TLS | Skip-verify allowed | Skip-verify rejected — CA bundle required |

When `APIP_DEMO_MODE=false`, any missing requirement causes the affected service to exit immediately with a message naming what to provide.

## Authentication Modes

### File-based (default)

No setup required. The default `admin` / `admin` credentials are defined in `configs/config-platform-api.toml`. To change the password, generate a new bcrypt hash:

```bash
htpasswd -bnBC 12 "" <new-password> | tr -d ':\n'
```

Replace the `password_hash` value in `configs/config-platform-api.toml` before starting.

### OIDC (production)

To delegate login to an external OIDC-compliant provider (Asgardeo, Keycloak, Auth0, etc.):

1. Register a **confidential** OIDC application in your IDP with redirect URL `https://localhost:5380/api/auth/callback` and enable the **Authorization Code** and **Refresh Token** grants.
2. Uncomment the `OIDC` environment variable blocks on **both** services in `docker-compose.yaml`.
3. Add your IDP credentials to `.env`:

```bash
OIDC_ISSUER=https://idp.example.com/oauth2/token
OIDC_JWKS_URL=https://idp.example.com/oauth2/jwks
OIDC_CLIENT_ID=<your-client-id>
OIDC_CLIENT_SECRET=<your-client-secret>
```

See the [WSO2 API Platform documentation](https://wso2.com/api-platform/docs/) (AI Workspace section) for a full OIDC setup walkthrough including Asgardeo scope registration.

## Custom TLS Certificates

Mount your own certificate to remove the browser trust warning:

1. Create a `certs/` directory next to `docker-compose.yaml` and place your PEM files there:

   ```
   certs/
   ├── ai-workspace.crt    # PEM certificate (or full chain)
   ├── ai-workspace.key    # PEM private key
   ├── platform-api.crt
   └── platform-api.key
   ```

2. Uncomment the TLS volume lines in `docker-compose.yaml` under each service.
3. Restart: `docker compose up -d`

## Database

The Platform API uses **SQLite** by default (data persisted in a Docker volume). To switch to PostgreSQL, update `configs/config-platform-api.toml`:

```toml
[database]
driver = "postgres"
host   = "your-db-host"
port   = 5432
name   = "platform_api"
user   = "platform_api"
# password via DATABASE_PASSWORD env var
```

The `resources/platform-api/db-scripts/` directory contains the schema scripts (`schema.postgres.sql`, `schema.sqlite.sql`, `schema.sqlserver.sql`). The Platform API applies the appropriate schema automatically on startup — no manual SQL execution is required.

## License

Copyright (c) 2026, WSO2 LLC. (https://wso2.com)

Licensed under the Apache License, Version 2.0. You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
