# Configuration Reference

AI Workspace is configured through a `config.toml` file mounted into the container at `/etc/ai-workspace/config.toml`.

Each value in the file is written as an interpolation token that is resolved once at startup, so the environment override is visible in place:

```toml
key = '{{ env "APIP_AIW_KEY" "default" }}'
#          ^ environment variable   ^ value used when the variable is unset
```

The variable name is always the key uppercased and prefixed with **`APIP_AIW_`** — `log_level` → `APIP_AIW_LOG_LEVEL`, `platform_api_url` → `APIP_AIW_PLATFORM_API_URL`. (The same convention gives the Platform API `APIP_CP_` and the Developer Portal `APIP_DP_`.) A plain literal (`key = "value"`) works too; setting the variable still overrides it, since environment variables always take precedence over the file.

Three variables are deliberately **not** prefixed, because they are not config keys: `APIP_DEMO_MODE` (a stack-wide runtime flag), `APIP_AIW_CONFIG_FILE`'s target is read before the file exists, and `APIP_CONFIG_FILE_SOURCE_ALLOWLIST` (shared, see below). The bare names inside `{{ env "NAME" }}` tokens are also read unprefixed — such a token names an arbitrary environment variable, not a config key.

Copy `configs/config-template.toml` to `configs/config.toml` and fill in the values for your deployment before starting the stack.

## Secrets

Never write a secret as a literal in `config.toml`, and never hardcode one in `docker-compose.yaml`. There are two supported ways to supply the OIDC client secret:

**Environment variable (default)** — the shipped config references it, with no default value, so an unset variable fails startup rather than running with an empty credential. Keep the value in a git-ignored `.env`:

```toml
oidc_client_secret = '{{ env "APIP_AIW_OIDC_CLIENT_SECRET" }}'
```

**Mounted secret file (preferred in production)** — swap the token so the value never enters the environment at all:

```toml
oidc_client_secret = '{{ file "/secrets/ai-workspace/oidc_client_secret" }}'
```

Both forms fail closed: if the variable is unset, or the file is missing or outside the allowed source directories, the server refuses to start. A `{{ file }}` path must live under `/etc/ai-workspace` or `/secrets/ai-workspace`; override that list with the shared `APIP_CONFIG_FILE_SOURCE_ALLOWLIST` (comma-separated; it replaces the defaults rather than extending them).

## All Configuration Keys

### Core

| Key | Env override | Default | Description |
|-----|-------------|---------|-------------|
| `domain` | `APIP_AIW_DOMAIN` | `localhost:5380` | Host (and optional port) shown in the browser address bar. |
| `auth_mode` | `APIP_AIW_AUTH_MODE` | `basic` | Authentication mode. `"basic"` for file-based local auth; `"oidc"` for external IDP. |
| `platform_api_url` | `APIP_AIW_PLATFORM_API_URL` | — | **Required.** Absolute URL the BFF uses to reach the Platform API server-to-server (e.g. `https://platform-api:9243`). Its scheme decides whether the upstream hop uses TLS. |
| `controlplane_host` | `APIP_AIW_CONTROLPLANE_HOST` | `localhost:9243` | Externally reachable `host:port` that deployed gateways use to reach the Platform API. Shown in gateway setup instructions. Must be an absolute address, not a relative path. |
| `default_org_region` | `APIP_AIW_DEFAULT_ORG_REGION` | `us` | Default region label assigned to new organizations on first login. |
| `log_level` | `APIP_AIW_LOG_LEVEL` | `info` | `debug` \| `info` \| `warn` \| `error`. |

### OIDC (only required when `auth_mode = "oidc"`)

| Key | Env override | Default | Description |
|-----|-------------|---------|-------------|
| `oidc_authority` | `APIP_AIW_OIDC_AUTHORITY` | — | OIDC issuer URL. Endpoints (authorization, token, JWKS, etc.) are auto-discovered from `{oidc_authority}/.well-known/openid-configuration`. |
| `oidc_client_id` | `APIP_AIW_OIDC_CLIENT_ID` | — | Client ID of the AI Workspace confidential application registered in your IDP. |
| `oidc_client_secret` | `APIP_AIW_OIDC_CLIENT_SECRET` | — | Confidential-client secret, held only by the BFF and never sent to the browser. Set it via the env var, or from a mounted file with a `{{ file }}` token — see [Secrets](#secrets). |
| `oidc_redirect_url` | `APIP_AIW_OIDC_REDIRECT_URL` | — | The BFF callback, e.g. `https://<domain>/api/auth/callback`. |
| `oidc_post_logout_redirect_url` | `APIP_AIW_OIDC_POST_LOGOUT_REDIRECT_URL` | — | Post-logout URL, e.g. `https://<domain>/login`. Must be an absolute, pre-registered URL. |
| `oidc_org_id_claim` | `APIP_AIW_OIDC_ORG_ID_CLAIM` | `org_id` | JWT claim name for the organization UUID. Must match `organization_claim_name` in Platform API config. |
| `oidc_org_name_claim` | `APIP_AIW_OIDC_ORG_NAME_CLAIM` | `org_name` | JWT claim name for the human-readable organization name. |
| `oidc_org_handle_claim` | `APIP_AIW_OIDC_ORG_HANDLE_CLAIM` | `org_handle` | JWT claim name for the organization handle (slug). |

`oidc_redirect_url` and `oidc_post_logout_redirect_url` must be registered as authorized redirect
URLs in your IDP application. The sign-in redirect is the **BFF callback** `/api/auth/callback`
(the BFF, not the browser, completes the code exchange) — not a `/signin` route.

The full set of BFF keys (listener, TLS, session, cookie, CSRF, proxy) is documented inline in
[`configs/config-template.toml`](../../portals/ai-workspace/configs/config-template.toml).

## Minimal Quick-Start Config (basic auth)

```toml
domain               = "localhost:8080"
auth_mode            = "basic"
platform_api_base_url = "https://localhost:9243/api/v1"
controlplane_host    = "localhost:9243"
```

## Minimal Production Config (OIDC)

```toml
domain               = "app.example.com"
auth_mode            = "oidc"
oidc_authority       = "https://api.asgardeo.io/t/<your-tenant>/oauth2/token"
oidc_client_id       = "<ai-workspace-client-id>"
oidc_org_id_claim    = "org_id"
oidc_org_name_claim  = "org_name"
oidc_org_handle_claim = "org_handle"
platform_api_base_url = "https://api.example.com/api/v1"
controlplane_host    = "api.example.com"
default_org_region   = "us"
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

Sensitive values (JWT signing key, database password) must be passed as environment variables — never store them in `config.toml`:

| Platform API env variable | Description |
|--------------------------|-------------|
| `AUTH_JWT_SECRET_KEY` | JWT signing key (required when `auth.jwt.enabled = true`) |
| `ENCRYPTION_KEY` | 32-byte key (64 hex / base64) — encrypts secrets & subscription tokens (required) |
| `DATABASE_PASSWORD` | Database password |

## Environment Variable Override

Any `config.toml` key can be overridden with `APIP_AIW_` + the uppercased key. This is useful in container orchestration environments (Kubernetes `env:` blocks, Docker Compose `environment:` sections) where file mounts are less convenient.

Example — override just the authority for a staging environment:

```bash
docker run \
  -e APIP_AIW_OIDC_AUTHORITY=https://api.asgardeo.io/t/staging-tenant/oauth2/token \
  -v ./configs/config.toml:/etc/ai-workspace/config.toml \
  ghcr.io/wso2/api-platform/ai-workspace:<version>
```

A browser-safe key keeps the **same name everywhere** — in `config.toml` (`domain`), as an environment override (`APIP_AIW_DOMAIN`), in Vite's `import.meta.env` at build time, and in the `window.__RUNTIME_CONFIG__` payload the BFF serves to the SPA. (Vite is configured with `envPrefix: 'APIP_AIW_'` for this reason; the legacy `VITE_*` names are gone and setting one has no effect.)

Only keys on the BFF's browser-safe allowlist (`bff/internal/config/runtime_config.go`) are ever emitted to the page — server-side settings and the OIDC client credentials are not.
