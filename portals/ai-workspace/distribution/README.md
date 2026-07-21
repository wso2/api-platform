# WSO2 API Platform — AI Workspace

A standalone distribution of the AI Workspace and Platform API, orchestrated with Docker Compose. The AI Workspace is a React SPA served by a Go BFF (Backend-for-Frontend) that proxies all browser traffic same-origin and owns authentication — tokens live in a server-side session (HttpOnly cookie) and never reach the browser.

## Contents

```
wso2apip-ai-workspace-<version>/
├── README.md
├── docker-compose.yaml                          # AI Workspace + Platform API
├── scripts/
│   └── setup.sh                                 # One-time TLS + secrets provisioning
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
- `openssl`, and either `htpasswd` or Docker (used by `setup.sh` to bcrypt-hash the admin password)

## Quick Start

Run the setup script once, from the distribution root, before the first start:

```bash
./scripts/setup.sh
docker compose up -d
```

`setup.sh` generates everything the stack needs — nothing is auto-generated at runtime:

| Output | Contents |
|---|---|
| `api-platform.env` (git-ignored) | `APIP_CP_ENCRYPTION_KEY` (at-rest encryption), `APIP_CP_AUTH_JWT_SECRET_KEY` (signs login JWTs), `APIP_CP_ADMIN_USERNAME`, `APIP_CP_ADMIN_PASSWORD_HASH` (bcrypt) |
| `resources/certificates/cert.pem` + `key.pem` | Self-signed TLS pair shared by both services (SAN: `localhost`, `platform-api`, `ai-workspace`) |

The admin password is generated and printed once by `setup.sh` — it is not stored anywhere; only its bcrypt hash lands in `api-platform.env`. Re-running `setup.sh` keeps existing files; pass `--force` to rotate keys and credentials, or `--certs-only` to (re)generate just the TLS pair. `ADMIN_USERNAME` / `ADMIN_PASSWORD` environment variables skip the interactive prompts (used by CI to pin known test credentials).

For production, prefer mounting secret files and referencing them from the config TOMLs with `{{ file "..." }}` instead of `api-platform.env` — see [Configuration](#configuration) below.

Verify both services are healthy:

```bash
curl -fk https://localhost:9243/health    # Platform API
curl -fk https://localhost:5380/healthz   # AI Workspace
```

Open the AI Workspace in a browser at `https://localhost:5380` and log in with the admin credentials printed by `setup.sh`.

> **Browser trust warning?** Both services use a self-signed TLS certificate by default. Click **Advanced → Proceed** to continue. See [Custom TLS Certificates](#custom-tls-certificates) to remove the warning permanently.

## Exposed Ports

| Port | Service | Description |
|------|---------|-------------|
| `5380` | AI Workspace (BFF) | HTTPS — browser entry point |
| `9243` | Platform API | HTTPS — backend REST API |

## Configuration

Edit `configs/config.toml` for AI Workspace settings and `configs/config-platform-api.toml` for Platform API settings. Both are read directly by the running containers — no rebuild required, just restart the affected service.

Each config TOML writes its values as `'{{ env "..." }}'` tokens, so a key can be set from the environment without editing the file — the token names the variable, by convention the key uppercased and prefixed with `APIP_AIW_` (AI Workspace) or `APIP_CP_` (Platform API), e.g. `APIP_AIW_LOG_LEVEL`, `APIP_CP_DATABASE_HOST`. A key with no token is not settable from the environment: uncomment or add it in the TOML first. To source a value from a mounted file instead — the right choice for secrets — swap the token for `'{{ file "/secrets/..." }}'`. Never write a secret as a raw literal in either file.

Environment overrides go in `api-platform.env` (git-ignored; loaded into both containers via `env_file`, format `raw`, since the bcrypt password hash contains `$`, which must not be treated as a compose interpolation variable). This is also where OIDC mode's `APIP_AIW_OIDC_CLIENT_SECRET` belongs — it's the only file compose passes into the containers, so a separate `.env` alongside it would never reach the app.

### AI Workspace (`configs/config.toml`)

| Setting | Description |
|---------|-------------|
| `domain` | Host and port shown in the browser address bar |
| `auth_mode` | `basic` (file-based quickstart) or `oidc` (external IDP) |
| `[ai_workspace.control_plane].url` | Base URL of the upstream Platform API hop |
| `[ai_workspace.control_plane].ca_file` | PEM bundle trusted for the upstream's TLS cert (appended to system roots). Fixed to the mounted path — not env-overridable; edit the TOML if you change the volume mount in `docker-compose.yaml` |
| `[ai_workspace.control_plane].tls_skip_verify` | Skip upstream cert verification — local dev only |
| `[ai_workspace.gateway].controlplane_host` | Address gateways use to reach the Platform API |
| `[ai_workspace.gateway].platform_gateway_versions` | Gateway versions shown in the create-gateway selector |
| `[ai_workspace.tls].cert_file` / `key_file` | Listener certificate pair — required when `[ai_workspace.tls].enabled` is `true`. Fixed to the mounted path, same as `ca_file` above |
| `[ai_workspace.oidc].*` | Used only when `auth_mode = "oidc"` — see [OIDC](#oidc-production) below |

### Platform API (`configs/config-platform-api.toml`)

| Setting | Description |
|---------|-------------|
| `log_level` | Log level (`DEBUG`, `INFO`, `WARN`, `ERROR`) |
| `encryption_key` | Single 32-byte key (64 hex chars or base64) used for all at-rest encryption (secrets, subscription tokens, WebSub HMAC secrets). Generate with `openssl rand -hex 32` |
| `[database].driver` | `sqlite3` or `postgres` |
| `[auth.jwt].secret_key` | 32-byte HMAC key signing login JWTs |
| `[auth.idp]` | JWKS-based IDP auth — disabled in quickstart mode; enable for Asgardeo, Keycloak, Auth0, etc. |
| `[auth.file_based.users]` | Local user credentials (change the password hash before sharing) |
| `[https]` | Listener on `:9243`; `cert_dir` holds `cert.pem`/`key.pem` |

Each key's default value is written inline in `configs/config-template.toml` and
`configs/config-platform-api-template.toml` — those files are a fully-commented reference of
every available setting and its default, so defaults are not restated here.

## Authentication Modes

### File-based (default)

The admin user is generated by `setup.sh` (see [Quick Start](#quick-start)). To set your own password instead, generate a new bcrypt hash:

```bash
htpasswd -bnBC 10 "" NEW_PASSWORD | tr -d ':\n'
```

Replace the `password_hash` value in `configs/config-platform-api.toml` before starting.

### OIDC (production)

To delegate login to an external OIDC-compliant provider (Asgardeo, Keycloak, Auth0, etc.) instead of file-based auth, both services need to be reconfigured — the AI Workspace to send users to the IDP, and the Platform API to trust the tokens it issues.

1. Register a **confidential** OIDC application in your IDP with redirect URL `https://<your-domain>/api/auth/callback` (use `https://localhost:5380/api/auth/callback` for local development), a post-logout redirect URL, and enable the **Authorization Code** and **Refresh Token** grants.
2. **AI Workspace** (`configs/config.toml`): set `auth_mode = "oidc"`. Every `[ai_workspace.oidc]` key except `scope` defaults to empty and the server refuses to start in OIDC mode until each is set — either directly in the TOML or via its `APIP_AIW_OIDC_*` token in `api-platform.env`:

   ```bash
   APIP_AIW_OIDC_AUTHORITY=https://idp.example.com
   APIP_AIW_OIDC_CLIENT_ID=<your-client-id>
   APIP_AIW_OIDC_CLIENT_SECRET=<your-client-secret>
   APIP_AIW_OIDC_REDIRECT_URL=https://<your-domain>/api/auth/callback
   APIP_AIW_OIDC_POST_LOGOUT_REDIRECT_URL=https://<your-domain>/login
   ```

   Leaving `APIP_AIW_OIDC_SCOPE` unset requests the full `ap:*` scope set.

3. **Platform API** (`configs/config-platform-api.toml`): the `[auth.idp]` fields have no env-var tokens in the quickstart file, so edit the TOML directly — set `enabled = true` and fill in `jwks_url` and `issuer` for your IDP. Then set `[auth.file_based].enabled = false`: while file-based auth is enabled it takes priority, and the IDP is not used regardless of `[auth.idp]`. Align `[auth.idp.claim_mappings]` with `[ai_workspace.oidc.claim_mappings]` in `configs/config.toml` — both services must read the same claims out of the same token.

See `configs/config-template.toml` and `configs/config-platform-api-template.toml` for the full, per-field reference, and the [WSO2 API Platform documentation](https://wso2.com/api-platform/docs/) (AI Workspace section) for a full OIDC setup walkthrough including Asgardeo scope registration.

## Custom TLS Certificates

`resources/certificates/` holds the TLS pair shared by both services — `cert.pem` (certificate or full chain) and `key.pem` (private key), generated by `setup.sh`. This one directory is mounted read-only into both containers at their `/etc/<service>/tls` path. To remove the browser trust warning, replace both files with a certificate from your own CA (same file names) whose SAN list covers all three hostnames (`localhost`, `platform-api`, `ai-workspace`), then restart:

```bash
docker compose up -d
```

## Database

The Platform API uses **SQLite** by default (data persisted in a Docker volume). To switch to PostgreSQL, update `configs/config-platform-api.toml`:

```toml
[database]
driver = "postgres"
host   = "your-db-host"
port   = 5432
name   = "platform_api"
user   = "platform_api"
password = '{{ file "/secrets/platform-api/postgres_password" }}'
ssl_mode = "disable"
```

The `resources/platform-api/db-scripts/` directory contains the schema scripts (`schema.postgres.sql`, `schema.sqlite.sql`, `schema.sqlserver.sql`). The Platform API applies the appropriate schema automatically on startup — no manual SQL execution is required.

## License

Copyright (c) 2026, WSO2 LLC. (https://wso2.com)

Licensed under the Apache License, Version 2.0. You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
