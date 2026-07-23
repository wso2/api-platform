# WSO2 API Platform — Developer Portal

A standalone distribution of the Developer Portal and Platform API, orchestrated with Docker Compose. The Developer Portal is a Node.js web application for discovering and subscribing to APIs; the Platform API is its local-auth sidecar, validating username/password logins without requiring an external identity provider.

## Contents

```
wso2apip-developer-portal-<version>/
├── README.md
├── docker-compose.yaml                          # Developer Portal + Platform API
├── scripts/
│   ├── setup.sh                                 # One-time TLS + secrets provisioning
│   └── seed-samples.sh                          # Optional: deploy the bundled sample APIs/MCPs
├── configs/
│   ├── config.toml                              # Unified active config — [developer_portal] + [platform_api] sections
│   └── config-template.toml                     # Unified full configuration reference (both components)
└── resources/
    ├── developer-portal/
    │   └── db-scripts/                          # Developer Portal PostgreSQL schema (reference copy)
    └── samples/
        ├── apis/                                # Sample REST/GraphQL/SOAP APIs
        └── mcps/                                # Sample MCP servers
```

## Prerequisites

- Docker Engine 24+
- Docker Compose v2
- `openssl` and Docker (used by `setup.sh` to bcrypt-hash the admin password)

## Quick Start

Run the setup script once, from the distribution root, before the first start:

```bash
./scripts/setup.sh
docker compose up -d
```

`setup.sh` generates everything the stack needs — nothing is auto-generated at runtime:

| Output | Contents |
|---|---|
| `api-platform.env` (git-ignored) | `APIP_DP_SECURITY_ENCRYPTION_KEY` / `APIP_DP_SECURITY_SESSION_SECRET` (Developer Portal), `APIP_CP_ENCRYPTION_KEY` (Platform API at-rest encryption), `APIP_CP_ADMIN_USERNAME` / `APIP_CP_ADMIN_PASSWORD_HASH` (bcrypt). No JWT signing key — the RS256 keypair is written to `resources/keys/` as PEM files, since a multi-line PEM cannot live in an env file. |
| `resources/certificates/cert.pem` + `key.pem` | Self-signed TLS pair shared by both services |
| `resources/keys/jwt_private.pem` + `jwt_public.pem` | RS256 JWT keypair — the Platform API signs with the private key, the Developer Portal verifies with the public one |

The admin password is generated and printed once by `setup.sh` — it is not stored anywhere; only its bcrypt hash lands in `api-platform.env`. Re-running `setup.sh` is safe: it only fills in what's missing and never overwrites an existing value — to rotate a value, delete it from `api-platform.env` (or delete `resources/certificates` for the TLS cert) and re-run. `ADMIN_USERNAME` / `ADMIN_PASSWORD` environment variables skip the interactive prompts (used by CI to pin known test credentials).

Verify the Platform API is healthy:

```bash
curl -fk https://localhost:9243/health
```

Open the Developer Portal in a browser at `https://localhost:3000/default/views/default` and log in with the admin credentials printed by `setup.sh`.

> **Browser trust warning?** Both services use a self-signed TLS certificate by default. Click **Advanced → Proceed** to continue. See [Custom TLS Certificates](#custom-tls-certificates) to remove the warning permanently.

## Seed Sample APIs (optional)

Deploys the sample APIs and MCP servers under `resources/samples/` into the default organisation, entirely through the public REST API:

```bash
./scripts/seed-samples.sh
```

Prompts for the admin username/password (or set `ADMIN_USERNAME`/`ADMIN_PASSWORD` to skip the prompt). Safe to re-run — entries that already exist are skipped.

## Exposed Ports

| Port | Service | Description |
|------|---------|-------------|
| `3000` | Developer Portal | HTTPS — browser entry point |
| `9243` | Platform API | HTTPS — local-auth backend |
| `5380` | AI Workspace | HTTPS — only when the `with-ai-workspace` profile is enabled (see below) |

## AI Workspace (optional)

This package runs the Developer Portal and the Platform API by default. **AI Workspace** ships in the same `docker-compose.yaml` as an optional component behind the `with-ai-workspace` [Compose profile](https://docs.docker.com/compose/how-tos/profiles/), sharing the one Platform API — so you can add it without standing up a second Platform API.

AI Workspace mounts the **same** `configs/config.toml` the other services do and reads only its own `[ai_workspace]` section (it ignores `[developer_portal]`/`[platform_api]`). It is **off by default**: a plain `docker compose up -d` never starts it. Enabling it takes one one-time step, because that shipped `config.toml` does **not** carry an `[ai_workspace]` section:

1. **Add the `[ai_workspace]` section to `configs/config.toml`.** Copy the `[ai_workspace.*]` tables from the bottom of the shipped `configs/config-template.toml` (the "AI Workspace (optional)" section) and append them to this stack's `configs/config.toml`. The defaults already point at the shared `https://platform-api:9243`.

Then start the stack with the profile enabled:

```bash
docker compose --profile with-ai-workspace up -d
```

AI Workspace comes up at `https://localhost:5380`, backed by the same Platform API. Omit `--profile with-ai-workspace` on any later `docker compose` command to leave it stopped.

## Configuration

All settings live in the single `configs/config.toml`. It carries two sections — `[developer_portal.*]` and `[platform_api.*]` — and the **same file is mounted into both containers**; each service reads only its own section and ignores the other's. Edit it in place — no rebuild required, just restart the affected service.

Each section writes secrets as `'{{ env "..." }}'` tokens, so a key can be set from the environment without editing the file — the token names the variable, by convention the key uppercased and prefixed with `APIP_DP_` (Developer Portal) or `APIP_CP_` (Platform API), e.g. `APIP_DP_SERVER_HTTPS_ENABLED`, `APIP_CP_DATABASE_HOST`. A key with no token is not settable from the environment: uncomment or add it in the TOML first. To source a value from a mounted file instead — the right choice for secrets — swap the token for `'{{ file "/secrets/..." }}'`. Never write a secret as a raw literal.

Environment overrides go in `api-platform.env` (git-ignored; loaded into both containers via `env_file`, format `raw`, since the bcrypt password hash contains `$`, which must not be treated as a compose interpolation variable).

### Developer Portal (`[developer_portal.*]`)

| Setting | Description | Default |
|---------|-------------|---------|
| `[developer_portal.server.https].enabled` | Terminate TLS in the portal itself (vs. behind a proxy) | `true` |
| `[developer_portal.database].driver` | `sqlite` (default) or `postgres` | `sqlite` |
| `[developer_portal.auth].mode` | `local` (Platform API sidecar) or `idp` (external OIDC IDP via `[developer_portal.auth.idp]`) | `local` |
| `[developer_portal.auth.local].platform_api_url` | Address of the Platform API local-auth sidecar | `https://platform-api:9243` |
| `[developer_portal.auth.local].public_key_path` | Path to the Platform API RS256 public key PEM used to verify login tokens | `/etc/devportal/keys/jwt_public.pem` |
| `[developer_portal.organization].default_name` | Organization bootstrapped automatically on first start | `default` |

### Platform API (`[platform_api.*]`)

| Setting | Description | Default |
|---------|-------------|---------|
| `[platform_api.logging].level` | Log level (`DEBUG`, `INFO`, `WARN`, `ERROR`) | `INFO` |
| `[platform_api.security].encryption_key` | Single 32-byte key (64 hex chars or base64) used for all at-rest encryption. Generate with `openssl rand -hex 32` | _(from `setup.sh`)_ |
| `[platform_api.database].driver` | `sqlite3` or `postgres` | `sqlite3` |
| `[platform_api.auth.jwt].public_key` / `.private_key` | RS256 keypair — platform-api signs login JWTs with the private key; the portal verifies with the public one | _(from `setup.sh`)_ |
| `[platform_api.auth.idp]` | JWKS-based IDP auth — disabled in quickstart mode | disabled |
| `[[platform_api.auth.file.users]]` | Local user credentials — `username`/`password_hash` resolved from `setup.sh`'s env vars, `scopes` is a plain literal | admin, generated by `setup.sh` |

See `configs/config-template.toml` for a fully-commented reference of every available setting across both components.

## Authentication Modes

### File-based (default)

The admin user is generated by `setup.sh` (see [Quick Start](#quick-start)). To set your own password instead, generate a new bcrypt hash:

```bash
htpasswd -bnBC 12 "" NEW_PASSWORD | tr -d ':\n'
```

Put the hash in `api-platform.env` as `APIP_CP_ADMIN_PASSWORD_HASH` (and the username as `APIP_CP_ADMIN_USERNAME`) before starting.

### OIDC (production)

To delegate login to an external OIDC-compliant provider instead of file-based auth:

1. Register an OIDC application in your IDP with redirect URL `https://<your-domain>/<org>/callback`, and enable the **Authorization Code** grant.
2. In `configs/config.toml`, set `[developer_portal.auth]` `mode = "idp"` and fill in the `[developer_portal.auth.idp]` block — `client_id`, `client_secret`, `issuer`, `authorization_url`, `token_url`, `jwks_url`, `callback_url`, etc.
3. Adjust `[developer_portal.auth.claim_mappings]` and `[developer_portal.auth.idp.roles]` to match what your IDP puts in the issued token.

See `configs/config-template.toml` for the full, per-field reference.

## Custom TLS Certificates

`resources/certificates/` holds the TLS pair shared by both services — `cert.pem` and `key.pem`, generated by `setup.sh`. This one directory is mounted read-only into both containers at their `/etc/<service>/tls` path. To remove the browser trust warning, replace both files with a certificate from your own CA (same file names), then restart:

```bash
docker compose up -d
```

## Database

The Developer Portal uses **SQLite** by default (data persisted in a Docker volume) — tables are created automatically on first start. To switch to PostgreSQL, update `configs/config.toml`'s `[developer_portal.database]` block with `driver = "postgres"` and your connection details.

`resources/developer-portal/db-scripts/` contains a reference copy of the Developer Portal's PostgreSQL schema and query files (also bundled inside the image) — provided for inspection; no manual SQL execution is required.

## License

Copyright (c) 2026, WSO2 LLC. (https://wso2.com)

Licensed under the Apache License, Version 2.0. You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
