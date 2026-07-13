# Configuration Reference

AI Workspace is configured through a `config.toml` file mounted into the container at `/etc/ai-workspace/config.toml`.

Keys are grouped into TOML tables (`[platform_api]`, `[tls]`, `[session]`, `[cookie]`, `[oidc]`); deployment-identity keys such as `domain` and `auth_mode` sit at the top level.

The file is the **only** source of configuration. Each value in it is written as an interpolation token that is resolved once at startup, so where the value comes from is visible in place:

```toml
[oidc]
client_id = '{{ env "APIP_AIW_OIDC_CLIENT_ID" "default" }}'
#                  ^ environment variable      ^ value used when the variable is unset
```

A key written this way can be set from the environment without editing the file. That token is the *only* thing that lets an environment variable reach a config key — there is no implicit override, so a key written as a plain literal (`key = "value"`), or absent from the file, ignores the variable entirely. Add the key with a token to make it settable that way.

By convention the variable a token names is the key's full path — table and key, uppercased, dots as underscores — prefixed with **`APIP_AIW_`**: `[oidc] client_id` → `APIP_AIW_OIDC_CLIENT_ID`, `[platform_api] url` → `APIP_AIW_PLATFORM_API_URL`, and a top-level `log_level` → `APIP_AIW_LOG_LEVEL`. (The same prefix convention gives the Platform API `APIP_CP_` and the Developer Portal `APIP_DP_`.) It is only a convention: a token may name any variable, which is what lets a key read an existing secret under its own name.

The file's own location is not a config key — it cannot be, since it is needed before the file is read. The server reads its mount, `/etc/ai-workspace/config.toml`, unless `-config` names another path (`bff -config ../configs/config.toml`, which is what `make bff-run` does). Two variables are likewise read directly by the server rather than through a token: `APIP_DEMO_MODE` (a stack-wide runtime flag) and `APIP_CONFIG_FILE_SOURCE_ALLOWLIST`, which bounds where `{{ file }}` tokens may read from (see below).

Copy `configs/config-template.toml` to `configs/config.toml` and fill in the values for your deployment before starting the stack.

## Secrets

Never write a secret as a literal in `config.toml`, and never hardcode one in `docker-compose.yaml`. There are two supported ways to supply the OIDC client secret:

**Environment variable (default)** — the key's token names the variable and has no default value, so an unset variable fails startup rather than running with an empty credential. Keep the value in a git-ignored `.env`:

```toml
[oidc]
client_secret = '{{ env "APIP_AIW_OIDC_CLIENT_SECRET" }}'
```

**Mounted secret file (preferred in production)** — swap the token so the value never enters the environment at all:

```toml
[oidc]
client_secret = '{{ file "/secrets/ai-workspace/oidc_client_secret" }}'
```

Both forms fail closed: if the variable is unset, or the file is missing or outside the allowed source directories, the server refuses to start. A `{{ file }}` path must live under `/etc/ai-workspace` or `/secrets/ai-workspace`; override that list with the shared `APIP_CONFIG_FILE_SOURCE_ALLOWLIST` (comma-separated; it replaces the defaults rather than extending them).

## All Configuration Keys

The "Env var" column is the variable each key's shipped `{{ env }}` token names — set it and the key picks the value up. It works only while the key is present (uncommented) in `config.toml`.

### Top level (deployment identity)

| Key | Env var | Default | Description |
|-----|-------------|---------|-------------|
| `domain` | `APIP_AIW_DOMAIN` | `localhost:5380` | Host (and optional port) shown in the browser address bar. |
| `auth_mode` | `APIP_AIW_AUTH_MODE` | `basic` | Authentication mode. `"basic"` for file-based local auth; `"oidc"` for external IDP. |
| `controlplane_host` | `APIP_AIW_CONTROLPLANE_HOST` | `localhost:9243` | Externally reachable `host:port` that deployed gateways use to reach the Platform API. Shown in gateway setup instructions. Must be an absolute address, not a relative path. |
| `default_org_region` | `APIP_AIW_DEFAULT_ORG_REGION` | `us` | Default region label assigned to new organizations on first login. |
| `log_level` | `APIP_AIW_LOG_LEVEL` | `info` | `debug` \| `info` \| `warn` \| `error`. |

### `[platform_api]` — the upstream hop

| Key | Env var | Default | Description |
|-----|-------------|---------|-------------|
| `url` | `APIP_AIW_PLATFORM_API_URL` | — | **Required.** Absolute URL the BFF uses to reach the Platform API server-to-server (e.g. `https://platform-api:9243`) — an origin, not a base path; the API paths are appended by the proxy. Its scheme decides whether the upstream hop uses TLS. |
| `tls_skip_verify` | `APIP_AIW_PLATFORM_API_TLS_SKIP_VERIFY` | `false` | Accept the upstream's self-signed certificate. Rejected when `APIP_DEMO_MODE=false`. |
| `ca_file` | `APIP_AIW_PLATFORM_API_CA_FILE` | — | PEM bundle trusted for the upstream certificate, appended to the system roots. Prefer this over `tls_skip_verify`. |

### `[oidc]` (only required when `auth_mode = "oidc"`)

| Key | Env var | Default | Description |
|-----|-------------|---------|-------------|
| `authority` | `APIP_AIW_OIDC_AUTHORITY` | — | OIDC issuer URL. Endpoints (authorization, token, JWKS, etc.) are auto-discovered from `{authority}/.well-known/openid-configuration`. |
| `client_id` | `APIP_AIW_OIDC_CLIENT_ID` | — | Client ID of the AI Workspace confidential application registered in your IDP. |
| `client_secret` | `APIP_AIW_OIDC_CLIENT_SECRET` | — | Confidential-client secret, held only by the BFF and never sent to the browser. Set it via the env var, or from a mounted file with a `{{ file }}` token — see [Secrets](#secrets). |
| `redirect_url` | `APIP_AIW_OIDC_REDIRECT_URL` | — | The BFF callback, e.g. `https://<domain>/api/auth/callback`. |
| `post_logout_redirect_url` | `APIP_AIW_OIDC_POST_LOGOUT_REDIRECT_URL` | — | Post-logout URL, e.g. `https://<domain>/login`. Must be an absolute, pre-registered URL. |

### `[oidc.claim_mappings]` — which token claim carries each field

This table mirrors the Platform API's `[auth.idp.claim_mappings]` key for key, and the two must agree: both services read the same claims out of the same token. The variables line up one-to-one too — `APIP_AIW_OIDC_CLAIM_MAPPINGS_ORGANIZATION_CLAIM_NAME` against `APIP_CP_AUTH_IDP_CLAIM_MAPPINGS_ORGANIZATION_CLAIM_NAME`.

| Key | Env var | Default | Description |
|-----|-------------|---------|-------------|
| `organization_claim_name` | `APIP_AIW_OIDC_CLAIM_MAPPINGS_ORGANIZATION_CLAIM_NAME` | `org_id` | Claim carrying the organization UUID. |
| `org_name_claim_name` | `APIP_AIW_OIDC_CLAIM_MAPPINGS_ORG_NAME_CLAIM_NAME` | `org_name` | Claim carrying the human-readable organization name. |
| `org_handle_claim_name` | `APIP_AIW_OIDC_CLAIM_MAPPINGS_ORG_HANDLE_CLAIM_NAME` | `org_handle` | Claim carrying the organization handle (slug). |
| `username_claim_name` | `APIP_AIW_OIDC_CLAIM_MAPPINGS_USERNAME_CLAIM_NAME` | `username` | Claim carrying the display name. |
| `email_claim_name` | `APIP_AIW_OIDC_CLAIM_MAPPINGS_EMAIL_CLAIM_NAME` | `email` | Claim carrying the email address. |
| `scope_claim_name` | `APIP_AIW_OIDC_CLAIM_MAPPINGS_SCOPE_CLAIM_NAME` | `scope` | Claim carrying the space-separated scope string. |
| `role_claim_name` | `APIP_AIW_OIDC_CLAIM_MAPPINGS_ROLE_CLAIM_NAME` | `platform_role` | Claim carrying the platform role. Server-side only — not published to the browser. |

`[oidc.claim_mappings]` must be the **last** table under `[oidc]`: in TOML a sub-table header ends the parent table's key section, so a plain `[oidc]` key written below it would land in the sub-table instead.

`[oidc] redirect_url` and `post_logout_redirect_url` must be registered as authorized redirect
URLs in your IDP application. The sign-in redirect is the **BFF callback** `/api/auth/callback`
(the BFF, not the browser, completes the code exchange) — not a `/signin` route.

The remaining tables (`[tls]`, `[session]`, `[cookie]`) and the top-level listener keys are documented inline in
[`configs/config-template.toml`](../../portals/ai-workspace/configs/config-template.toml).

## Minimal Quick-Start Config (basic auth)

```toml
domain            = "localhost:8080"
auth_mode         = "basic"
controlplane_host = "localhost:9243"

[platform_api]
url = "https://localhost:9243"
```

## Minimal Production Config (OIDC)

```toml
domain             = "app.example.com"
auth_mode          = "oidc"
controlplane_host  = "api.example.com"
default_org_region = "us"

[platform_api]
url = "https://api.example.com"

[oidc]
authority     = "https://api.asgardeo.io/t/<your-tenant>/oauth2/token"
client_id     = "<ai-workspace-client-id>"
client_secret = '{{ file "/secrets/ai-workspace/oidc_client_secret" }}'
redirect_url  = "https://app.example.com/api/auth/callback"

# Mirrors [auth.idp.claim_mappings] in the Platform API config — the two must agree.
[oidc.claim_mappings]
organization_claim_name = "org_id"
org_name_claim_name     = "org_name"
org_handle_claim_name   = "org_handle"
```

## Platform API Configuration

The Platform API has its own config file: `configs/config-platform-api.toml` (mounted at `/etc/platform-api/config.toml`). Key sections:

```toml
[auth.jwt]
enabled = false   # Disable local JWT signing when using an external IDP

[auth.idp]
enabled  = true
name     = "asgardeo"
jwks_url = "https://api.asgardeo.io/t/<your-tenant>/oauth2/jwks"
issuer   = ["https://api.asgardeo.io/t/<your-tenant>/oauth2/token"]
audience = ["<ai-workspace-client-id>"]

[auth.idp.claim_mappings]
organization_claim_name = "org_id"
org_name_claim_name     = "org_name"
org_handle_claim_name   = "org_handle"

[auth.file_based]
enabled = false   # Disable file-based auth in production
```

Never write sensitive values (JWT signing key, encryption key, database password) as raw
literals in `config-platform-api.toml`, and never hardcode them in `docker-compose.yaml`.
Instead, reference each in the TOML with an interpolation token that is resolved at
startup — from an environment variable, or preferably from a mounted secret file:

```toml
encryption_key = '{{ env "APIP_CP_ENCRYPTION_KEY" }}'            # from an env var
secret_key     = '{{ file "/secrets/platform-api/jwt_secret" }}' # from a file (preferred)
```

Supply the env values from a git-ignored `keys.env` and start with `docker compose --env-file keys.env up`
Or mount a secret file and use `{{ file }}`.

The `APIP_CP_`-prefixed names referenced by the tokens above:

| Key referenced by the token | Description                                                                       |
|--------------------------|-----------------------------------------------------------------------------------|
| `APIP_CP_AUTH_JWT_SECRET_KEY` | JWT signing key (required when `auth.jwt.enabled = true`)                         |
| `APIP_CP_ENCRYPTION_KEY` | 32-byte key (64 hex / base64) — encrypts secrets & subscription tokens (required) |
| `APIP_CP_DATABASE_PASSWORD` | Database password                                                                 |

## Environment Variable Override

A key can be overridden from the environment only when its value carries an `{{ env }}` token naming the variable — there is no implicit environment overlay, so a literal or absent key ignores the environment entirely. Every key in the shipped `config-template.toml` carries such a token, conventionally named `APIP_AIW_` + the uppercased dotted key path (`[oidc] authority` → `APIP_AIW_OIDC_AUTHORITY`). This is useful in container orchestration environments (Kubernetes `env:` blocks, Docker Compose `environment:` sections) where file mounts are less convenient.

Example — override just the authority for a staging environment:

```bash
docker run \
  -e APIP_AIW_OIDC_AUTHORITY=https://api.asgardeo.io/t/staging-tenant/oauth2/token \
  -v ./configs/config.toml:/etc/ai-workspace/config.toml \
  ghcr.io/wso2/api-platform/ai-workspace:<version>
```

A browser-safe key keeps the **same name everywhere** — in `config.toml` (`domain`), as an environment override (`APIP_AIW_DOMAIN`), in Vite's `import.meta.env` at build time, and in the `window.__RUNTIME_CONFIG__` payload the BFF serves to the SPA. (Vite's `envPrefix` is configured with an explicit allowlist of these browser-safe `APIP_AIW_*` names — mirroring the BFF allowlist — so secrets sharing the namespace never reach the bundle; the legacy `VITE_*` names are gone and setting one has no effect.)

Only keys on the BFF's browser-safe allowlist (`bff/internal/config/runtime_config.go`) are ever emitted to the page — server-side settings and the OIDC client credentials are not.
