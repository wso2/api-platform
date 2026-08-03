# WSO2 API Platform — API Portal & MCP Hub

A standalone distribution of the API Portal and Platform API, orchestrated with Docker Compose. The API Portal is a Node.js web application for discovering and subscribing to APIs; the Platform API is its local-auth sidecar, validating username/password logins without requiring an external identity provider.

## Contents

```
wso2apip-api-portal-<version>/
├── README.md
├── docker-compose.yaml                          # API Portal + Platform API
├── scripts/
│   ├── setup.sh                                 # One-time TLS + secrets provisioning
│   ├── setup.ps1                                # Same, for Windows (PowerShell)
│   ├── seed-samples.sh                          # Optional: deploy the bundled sample APIs/MCPs
│   └── seed-samples.ps1                         # Same, for Windows (PowerShell)
├── configs/
│   ├── config.toml                              # Unified active config — [api_portal] + [platform_api] sections
│   └── config-template.toml                     # Config reference — both active components, plus optional [ai_workspace] at the bottom
└── resources/
    ├── api-portal/
    │   ├── role-to-scope-mapping.yaml           # API Portal role-to-scope mapping (dp:* scopes; used when auth.authorization.mode = "role")
    │   └── db-scripts/                          # API Portal PostgreSQL schema (reference copy)
    ├── platform-api/
    │   ├── role-to-scope-mapping.yaml           # Platform API role-to-scope mapping (edit to change what a role grants)
    │   └── db-scripts/                          # Platform API database schemas (reference copy)
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

On **Windows**, use the PowerShell script instead — same flags, same generated files
(Git for Windows and Docker Desktop both ship the `openssl` it needs):

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\setup.ps1
# PowerShell 7+ is also fine:  pwsh -File .\scripts\setup.ps1
docker compose up -d
```

`setup.sh` generates everything the stack needs — nothing is auto-generated at runtime:

| Output | Contents                                                                                                                                                                                                                                                                                                                                      |
|---|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `api-platform.env` (git-ignored) | `APIP_CP_ADMIN_USERNAME` / `APIP_CP_ADMIN_PASSWORD_HASH` (bcrypt) only. No JWT signing key, no API Portal or Platform API encryption/session secrets — those are all written to `resources/keys/` as files (read via `{{ file }}`) so they never appear in `docker inspect` or a process environment dump. |
| `resources/certificates/cert.pem` + `key.pem` | Self-signed TLS pair shared by both services                                                                                                                                                                                                                                                                                                  |
| `resources/keys/jwt_private.pem` + `jwt_public.pem` | RS256 JWT keypair — the Platform API signs with the private key, the API Portal verifies with the public one                                                                                                                                                                                                                            |
| `resources/keys/encryption.key` | Platform API's at-rest encryption key                                                                                                                                                                                                                                                                                                          |
| `resources/keys/api-portal-encryption.key` + `api-portal-session-secret` | API Portal's at-rest encryption key and session-signing secret |
| `.env` (git-ignored) | `COMPOSE_PROFILES` and `COMPOSE_PROJECT_NAME` — read by the `docker compose` CLI from this directory |

The admin password is generated and printed once by `setup.sh` — it is not stored anywhere; only its bcrypt hash lands in `api-platform.env`. Re-running `setup.sh` is safe: it only fills in what's missing and never overwrites an existing value — to rotate a value, delete it from `api-platform.env` (or delete `resources/certificates` for the TLS cert) and re-run. `ADMIN_USERNAME` / `ADMIN_PASSWORD` environment variables skip the interactive prompts (used by CI to pin known test credentials).

Verify the Platform API is healthy:

```bash
curl -fk https://localhost:9243/health
```

Open the API Portal in a browser at `https://localhost:9543/default/views/default` and log in with the admin credentials printed by `setup.sh`.

> **Browser trust warning?** Both services use a self-signed TLS certificate by default. Click **Advanced → Proceed** to continue. See [Custom TLS Certificates](#custom-tls-certificates) to remove the warning permanently.

## Seed Sample APIs (optional)

Deploys the sample APIs and MCP servers under `resources/samples/` into the default organization, entirely through the public REST API:

```bash
./scripts/seed-samples.sh
```

On **Windows**:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\seed-samples.ps1
# PowerShell 7+ is also fine:  pwsh -File .\scripts\seed-samples.ps1
```

Prompts for the admin username/password (or set `ADMIN_USERNAME`/`ADMIN_PASSWORD` to skip the prompt). Safe to re-run — entries that already exist are skipped.

## Exposed Ports

| Port | Service | Description |
|------|---------|-------------|
| `9543` | API Portal | HTTPS — browser entry point |
| `9243` | Platform API | HTTPS — local-auth backend |
| `9643` | AI Workspace | HTTPS — only when the `ai-workspace` profile is enabled (see below) |

## AI Workspace (optional)

This package runs the API Portal and the Platform API by default. **AI Workspace** ships in the same `docker-compose.yaml` as an optional component behind the `ai-workspace` [Compose profile](https://docs.docker.com/compose/how-tos/profiles/), sharing the one Platform API — so you can add it without standing up a second Platform API.

AI Workspace mounts the **same** `configs/config.toml` the other services do and reads only its own `[ai_workspace]` section (it ignores `[api_portal]`/`[platform_api]`). It is **off by default**: a plain `docker compose up -d` never starts it. Enabling it takes one one-time step, because that shipped `config.toml` does **not** carry an `[ai_workspace]` section:

1. **Add the `[ai_workspace]` section to `configs/config.toml`.** Copy the `[ai_workspace.*]` tables from the bottom of the shipped `configs/config-template.toml` (the "AI Workspace (optional)" section) and append them to this stack's `configs/config.toml`. The defaults already point at the shared `https://platform-api:9243`.

Then start the stack with the profile enabled:

```bash
docker compose --profile ai-workspace up -d
```

AI Workspace comes up at `https://localhost:9643`, backed by the same Platform API. Omitting `--profile ai-workspace` on a later `docker compose` command neither starts nor stops it — an already-running instance keeps running. To stop it explicitly, run `docker compose stop ai-workspace`, or `docker compose --profile ai-workspace down` to remove it.

## Design Mode (optional)

**Design mode** turns the API Portal into a file-based preview: it renders APIs, MCP servers, layouts, and applications straight from the bundled sample files instead of the database. With it on, the portal **opens no database connection and never calls the Platform API** — it's purely for previewing content and theming. Do **not** enable it in production.

Like AI Workspace, it's an opt-in you turn on by editing `configs/config.toml` — there's no separate config file or Compose profile:

1. **Copy the `[api_portal.design_mode]` block** from the "DESIGN MODE CONFIGURATION" section of the shipped `configs/config-template.toml` into `configs/config.toml` (keep `enabled = true`). The sample paths are already correct for the bundled samples — leave them as-is.
2. **Restart the API Portal:** `docker compose up -d` (or `docker compose restart api-portal`).

The portal then serves from disk at `/views/default` (e.g. `http://localhost:9543/views/default`). Because design mode never touches the database, the accompanying Platform API and its database go unused while it's on — set `enabled` back to `false` and restart to return to the normal, database-backed portal.

The sample content lives in **`resources/samples/`** (`apis/`, `mcps/`, `applications.yaml`, `subscription-plans.yaml`), which the API Portal container mounts at `/app/samples`. To preview **your own** APIs and MCP servers, add or edit files there and restart — no image rebuild needed.

## Configuration

All settings live in the single `configs/config.toml`. It carries two sections — `[api_portal.*]` and `[platform_api.*]` — and the **same file is mounted into both containers**; each service reads only its own section and ignores the other's. Edit it in place — no rebuild required, just restart the affected service.

Each section writes secrets as `'{{ env "..." }}'` tokens, so a key can be set from the environment without editing the file — the token names the variable, by convention the key uppercased and prefixed with `APIP_AP_` (API Portal) or `APIP_CP_` (Platform API), e.g. `APIP_AP_SERVER_HTTPS_ENABLED`, `APIP_CP_DATABASE_HOST`. A key with no token is not settable from the environment: uncomment or add it in the TOML first. To source a value from a mounted file instead — the right choice for secrets — swap the token for `'{{ file "/secrets/..." }}'`. Never write a secret as a raw literal.

Environment overrides go in `api-platform.env` (git-ignored; loaded into both containers via `env_file`, format `raw`, since the bcrypt password hash contains `$`, which must not be treated as a compose interpolation variable).

### API Portal (`[api_portal.*]`)

| Setting | Description | Default |
|---------|-------------|---------|
| `[api_portal.server.https].enabled` | Terminate TLS in the portal itself (vs. behind a proxy) | `true` |
| `[api_portal.database].driver` | `sqlite` (default) or `postgres` | `sqlite` |
| `[api_portal.auth].mode` | `local` (Platform API sidecar) or `idp` (external OIDC IDP via `[api_portal.auth.idp]`) | `local` |
| `[api_portal.auth.local].platform_api_url` | Address of the Platform API local-auth sidecar | `https://platform-api:9243` |
| `[api_portal.auth.local].public_key_path` | Path to the Platform API RS256 public key PEM used to verify login tokens | `/etc/api-portal/keys/jwt_public.pem` |
| `[api_portal.auth.authorization].enabled` | Enforce each REST operation's declared `dp:*` scopes. `false` lets any authenticated caller through — development only | `true` |
| `[api_portal.auth.authorization].mode` | `role` (the default) expands the token's roles claim through the grant table; `scope` reads the token's own scope claim instead | `role` |
| `[api_portal.auth.authorization].role_to_scope_mapping` | Path to the mounted `resources/api-portal/role-to-scope-mapping.yaml` — used in `role` mode; edit that file to change what a role grants | `./resources/role-to-scope-mapping.yaml` |
| `[api_portal.auth.authorization].page_role_validation` | Gate portal pages on the caller's role tier (`portal_roles` below). Separate from `enabled`, which governs REST scopes | `false` |
| `[api_portal.auth.authorization.portal_roles]` | Which role name in the token's roles claim grants each page tier (`admin`, `subscriber`) | `admin`, `Internal/subscriber` |
| `[api_portal.organization].handle` | The single organization this instance serves, bootstrapped on first start. Required — the portal refuses to start without it | `default` |
| `[api_portal.organization].display_name` | Display name applied when the organization is first seeded | `Default` |

### Platform API (`[platform_api.*]`)

| Setting | Description | Default |
|---------|-------------|---------|
| `[platform_api.logging].level` | Log level (`DEBUG`, `INFO`, `WARN`, `ERROR`) | `INFO` |
| `[platform_api.security].encryption_key` | Single 32-byte key (64 hex chars or base64) used for all at-rest encryption. Generate with `openssl rand -hex 32` | _(from `setup.sh`)_ |
| `[platform_api.database].driver` | `sqlite3` or `postgres` | `sqlite3` |
| `[platform_api.auth.jwt].public_key_file` / `.private_key_file` | RS256 keypair — platform-api signs login JWTs with the private key; the portal verifies with the public one | _(from `setup.sh`)_ |
| `[platform_api.auth.idp]` | JWKS-based IDP auth — disabled in quickstart mode | disabled |
| `[[platform_api.auth.file.users]]` | Local user credentials — `username`/`password_hash` resolved from `setup.sh`'s env vars; `roles` names one or more entries in `resources/platform-api/role-to-scope-mapping.yaml`, which is where that user's scopes come from | admin, generated by `setup.sh` |
| `[platform_api.auth.authorization].role_to_scope_mapping` | Path to the mounted `resources/platform-api/role-to-scope-mapping.yaml` — edit that file to change what a role grants | `/etc/platform-api/role-to-scope-mapping.yaml` |

See `configs/config-template.toml` for a fully-commented reference of every available setting across both active components (plus the optional `[ai_workspace]` section at the bottom).

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
2. In `configs/config.toml`, set `[api_portal.auth]` `mode = "idp"` and fill in the `[api_portal.auth.idp]` block — `client_id`, `client_secret`, `issuer`, `authorization_url`, `token_url`, `jwks_url`, `callback_url`, etc.
3. Adjust `[api_portal.auth.claim_mappings]` to match what your IDP puts in the issued token, and `[api_portal.auth.authorization.portal_roles]` to name the IDP roles that grant each portal tier. (`[api_portal.auth.idp.roles]` is retired — leaving it in place fails startup.)
4. To authorize the REST API from those same IDP roles rather than from `dp:*` scopes the IDP has no reason to mint, set `[api_portal.auth.authorization]` `mode = "role"` and point `role_to_scope_mapping` at the mounted `resources/api-portal/role-to-scope-mapping.yaml`.

See `configs/config-template.toml` for the full, per-field reference.

## Custom TLS Certificates

`resources/certificates/` holds the TLS pair shared by both services — `cert.pem` and `key.pem`, generated by `setup.sh`. This one directory is mounted read-only into both containers at their `/etc/<service>/tls` path. To remove the browser trust warning, replace both files with a certificate from your own CA (same file names), then restart:

```bash
docker compose up -d --force-recreate
```

## Compose project name

`setup.sh` pins `COMPOSE_PROJECT_NAME=wso2apip-api-portal-<version>-<6 hex>` in `.env` on its first run and never changes it. Compose prefixes this stack's containers, network, and volumes with it, so unpacking this zip again elsewhere on the host gets its own volumes instead of adopting this copy's APIs, applications, and users. Don't edit that line or delete `.env` — the data lives in `<project>_api-portal-data` and `<project>_platform-api-data`, and a different name starts the portal empty. `down` keeps those volumes; only `down -v` deletes them. To choose the name yourself — including adopting an earlier release's volumes, whose prefix `docker volume ls` shows — set it for the first run only: `COMPOSE_PROJECT_NAME=<name> ./scripts/setup.sh` (PowerShell: `$env:COMPOSE_PROJECT_NAME = '<name>'; .\scripts\setup.ps1`). It must match `^[a-z0-9][a-z0-9_-]*$`. Two portal stacks still can't run at once: both bind ports `9243` and `9543`.

## Database

The API Portal uses **SQLite** by default (data persisted in a Docker volume) — tables are created automatically on first start. To switch to PostgreSQL, update `configs/config.toml`'s `[api_portal.database]` block with `driver = "postgres"` and your connection details.

The Platform API likewise defaults to SQLite; switch it with `configs/config.toml`'s `[platform_api.database]` block.

`resources/api-portal/db-scripts/` and `resources/platform-api/db-scripts/` contain reference copies of each component's schema and query files (also bundled inside the images) — provided for inspection; no manual SQL execution is required.

## License

Copyright (c) 2026, WSO2 LLC. (https://wso2.com)

Licensed under the Apache License, Version 2.0. You may obtain a copy of the License at
http://www.apache.org/licenses/LICENSE-2.0
