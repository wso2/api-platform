# Configuration Reference

AI Workspace is configured through a `config.toml` file mounted into the container at `/etc/ai-workspace/config.toml`.

All AI Workspace settings live under a single top-level `[ai_workspace]` table — the same namespacing convention the Platform API uses for its own `[platform_api]` table — so one `config.toml` can hold both services' sections side by side without their keys colliding. Keys are grouped into TOML tables (`[ai_workspace.logging]`, `[ai_workspace.control_plane]`, `[ai_workspace.server.https]`, `[ai_workspace.session]`, `[ai_workspace.auth]`, `[ai_workspace.auth.oidc]`); deployment-identity keys such as `domain` sit directly under `[ai_workspace]`. The session cookie's name, `Secure`, and `SameSite` attributes are not configurable — they are internal details of the BFF's session mechanism.

The file is the **only** source of configuration. Each value in it is written as an interpolation token that is resolved once at startup, so where the value comes from is visible in place:

```toml
[ai_workspace.auth.oidc]
client_id = '{{ env "APIP_AIW_AUTH_OIDC_CLIENT_ID" "default" }}'
#                  ^ environment variable      ^ value used when the variable is unset
```

A key written this way can be set from the environment without editing the file. That token is the *only* thing that lets an environment variable reach a config key — there is no implicit override, so a key written as a plain literal (`key = "value"`), or absent from the file, ignores the variable entirely. Add the key with a token to make it settable that way.

By convention the variable a token names is the key's path **under `[ai_workspace]`** (the `ai_workspace` segment itself is not part of the name) — table and key, uppercased, dots as underscores — prefixed with **`APIP_AIW_`**: `[ai_workspace.auth.oidc] client_id` → `APIP_AIW_AUTH_OIDC_CLIENT_ID`, `[ai_workspace.control_plane] url` → `APIP_AIW_CONTROL_PLANE_URL`, and `[ai_workspace.logging] level` → `APIP_AIW_LOGGING_LEVEL`. (The same prefix convention gives the Platform API `APIP_CP_` and the Developer Portal `APIP_DP_`.) It is only a convention: a token may name any variable, which is what lets a key read an existing secret under its own name.

The file's own location is not a config key — it cannot be, since it is needed before the file is read. The server reads its mount, `/etc/ai-workspace/config.toml`, unless `-config` names another path (`bff -config ../configs/config.toml`, which is what `make bff-run` does). One variable is likewise read directly by the server rather than through a token: `APIP_CONFIG_FILE_SOURCE_ALLOWLIST`, which bounds where `{{ file }}` tokens may read from (see below).

Copy `configs/config-template.toml` to `configs/config.toml` and fill in the values for your deployment before starting the stack.

## Secrets

Never write a secret as a literal in `config.toml`, and never hardcode one in `docker-compose.yaml`. There are two supported ways to supply the OIDC client secret:

**Environment variable (default)** — the key's token names the variable and has no default value, so an unset variable fails startup rather than running with an empty credential. Keep the value in the git-ignored `api-platform.env` (loaded into both services via `env_file`):

```toml
[ai_workspace.auth.oidc]
client_secret = '{{ env "APIP_AIW_AUTH_OIDC_CLIENT_SECRET" }}'
```

**Mounted secret file (preferred in production)** — swap the token so the value never enters the environment at all:

```toml
[ai_workspace.auth.oidc]
client_secret = '{{ file "/secrets/ai-workspace/oidc_client_secret" }}'
```

Both forms fail closed: if the variable is unset, or the file is missing or outside the allowed source directories, the server refuses to start. A `{{ file }}` path must live under `/etc/ai-workspace` or `/secrets/ai-workspace`; override that list with the shared `APIP_CONFIG_FILE_SOURCE_ALLOWLIST` (comma-separated; it replaces the defaults rather than extending them).

## All Configuration Keys

Every key's `{{ env }}` token and default value are written inline in
[`configs/config-template.toml`](../../portals/ai-workspace/configs/config-template.toml) — that
file is the single source of truth for both, so they are not duplicated here. What follows is a
map of what each table is for.

### `[ai_workspace]` (deployment identity)

| Key | Description |
|-----|-------------|
| `domain` | Host (and optional port) shown in the browser address bar. |
| `default_org_region` | Default region label assigned to new organizations on first login. |

### `[ai_workspace.logging]`

| Key | Description |
|-----|-------------|
| `level` | `debug` \| `info` \| `warn` \| `error` (matched case-insensitively). |
| `format` | `text` \| `json`. |

### `[ai_workspace.auth]` — login mode

| Key | Description |
|-----|-------------|
| `mode` | Authentication mode. `"basic"` for file-based local auth; `"oidc"` for external IDP. |

### `[ai_workspace.control_plane]` — the upstream hop

| Key | Description |
|-----|-------------|
| `url` | **Required.** Absolute URL the BFF uses to reach the Platform API server-to-server (e.g. `https://platform-api:9243`) — an origin, not a base path; the API paths are appended by the proxy. Its scheme decides whether the upstream hop uses TLS. |
| `tls_skip_verify` | Skip upstream certificate verification entirely (local development only) — prefer `ca_file`. |
| `ca_file` | PEM bundle trusted for the upstream certificate, appended to the system roots. Prefer this over `tls_skip_verify`. |

### `[ai_workspace.gateway]` — gateway deployment info shown to the browser

Distinct from `[ai_workspace.control_plane]` above: that table is the BFF's own
server-to-server hop, while this one is what an externally deployed gateway needs to
reach the Platform API itself.

| Key | Description |
|-----|-------------|
| `controlplane_host` | Externally reachable `host:port` that deployed gateways use to reach the Platform API. Shown in gateway setup instructions. Must be an absolute address, not a relative path. |
| `platform_gateway_versions` | Gateway versions offered in the create-gateway version selector (JSON array string). |

### `[ai_workspace.auth.oidc]` (only required when `[ai_workspace.auth] mode = "oidc"`)

| Key | Description |
|-----|-------------|
| `authority` | OIDC issuer URL. Endpoints (authorization, token, JWKS, etc.) are auto-discovered from `{authority}/.well-known/openid-configuration`. |
| `client_id` | Client ID of the AI Workspace confidential application registered in your IDP. |
| `client_secret` | Confidential-client secret, held only by the BFF and never sent to the browser. Referenced from an env var or a mounted file — see [Secrets](#secrets). |
| `redirect_url` | The BFF callback, e.g. `https://<domain>/api/auth/callback`. |
| `post_logout_redirect_url` | Post-logout URL, e.g. `https://<domain>/login`. Must be an absolute, pre-registered URL. |

### `[ai_workspace.auth.claim_mappings]` — which token claim carries each field

A sibling of `[ai_workspace.auth.oidc]`, not nested inside it: this table applies to **both** auth modes. In basic mode the Platform API's file-based login endpoint signs its JWTs using these same mapped claim names, so the BFF reads basic-mode tokens by this mapping too. It mirrors the Platform API's `[platform_api.auth.claim_mappings]` key for key, and the two must agree: both services read the same claims out of the same token.

| Key | Description |
|-----|-------------|
| `organization` | Claim carrying the organization UUID. |
| `org_name` | Claim carrying the human-readable organization name. |
| `org_handle` | Claim carrying the organization handle (slug). |
| `username` | Claim carrying the display name. |
| `email` | Claim carrying the email address. |
| `scope` | Claim carrying the space-separated scope string. |
| `roles` | Claim carrying the platform role. Server-side only — not published to the browser. |

`[ai_workspace.auth.oidc] redirect_url` and `post_logout_redirect_url` must be registered as authorized redirect
URLs in your IDP application. The sign-in redirect is the **BFF callback** `/api/auth/callback`
(the BFF, not the browser, completes the code exchange) — not a `/signin` route.

The remaining tables (`[ai_workspace.server.https]`, `[ai_workspace.session]`) and the `[ai_workspace]` listener keys are documented inline in
[`configs/config-template.toml`](../../portals/ai-workspace/configs/config-template.toml).

## Minimal Quick-Start Config (basic auth)

```toml
[ai_workspace]
domain = "localhost:8080"

[ai_workspace.control_plane]
url = "https://localhost:9243"

[ai_workspace.gateway]
controlplane_host = "localhost:9243"

[ai_workspace.auth]
mode = "basic"
```

## Minimal Production Config (OIDC)

```toml
[ai_workspace]
domain             = "app.example.com"
default_org_region = "us"

[ai_workspace.control_plane]
url = "https://api.example.com"

[ai_workspace.gateway]
controlplane_host = "api.example.com"

[ai_workspace.auth]
mode = "oidc"

[ai_workspace.auth.oidc]
authority     = "https://api.asgardeo.io/t/<your-tenant>/oauth2/token"
client_id     = "<ai-workspace-client-id>"
client_secret = '{{ file "/secrets/ai-workspace/oidc_client_secret" }}'
redirect_url  = "https://app.example.com/api/auth/callback"

# Mirrors [platform_api.auth.claim_mappings] in the Platform API config — the two must agree.
# Applies to both auth modes, so it's a sibling of [ai_workspace.auth.oidc], not nested in it.
[ai_workspace.auth.claim_mappings]
organization = "org_id"
org_name     = "org_name"
org_handle   = "org_handle"
```

## Platform API Configuration

The Platform API has its own config file: `configs/config-platform-api.toml` (mounted at `/etc/platform-api/config.toml`). Key sections:

```toml
[auth]
mode = "idp"   # "external_token", "file", or "idp" — exactly one mode is active

[auth.idp]
name     = "asgardeo"
jwks_url = "https://api.asgardeo.io/t/<your-tenant>/oauth2/jwks"
issuer   = ["https://api.asgardeo.io/t/<your-tenant>/oauth2/token"]
audience = ["<ai-workspace-client-id>"]

[auth.claim_mappings]
organization = "org_id"
org_name     = "org_name"
org_handle   = "org_handle"
```

Never write sensitive values (JWT signing key, encryption key, database password) as raw
literals in `config-platform-api.toml`, and never hardcode them in `docker-compose.yaml`.
Instead, reference each in the TOML with an interpolation token that is resolved at
startup — from an environment variable, or preferably from a mounted secret file:

```toml
encryption_key = '{{ env "APIP_CP_ENCRYPTION_KEY" }}'            # from an env var
secret_key     = '{{ file "/secrets/platform-api/jwt_secret" }}' # from a file (preferred)
```

Supply the env values from a git-ignored `api-platform.env` and start with `docker compose up`
Or mount a secret file and use `{{ file }}`. The variable each token names, and which keys
require one, are documented inline in `config-platform-api-template.toml`.

## Environment Variable Override

A key can be overridden from the environment only when its value carries an `{{ env }}` token naming the variable — there is no implicit environment overlay, so a literal or absent key ignores the environment entirely. Every key in the shipped `config-template.toml` carries such a token, conventionally named `APIP_AIW_` + the uppercased dotted key path under `[ai_workspace]`. This is useful in container orchestration environments (Kubernetes `env:` blocks, Docker Compose `environment:` sections) where file mounts are less convenient.

Example — override just the authority for a staging environment:

```bash
docker run \
  -e APIP_AIW_AUTH_OIDC_AUTHORITY=https://api.asgardeo.io/t/staging-tenant/oauth2/token \
  -v ./configs/config.toml:/etc/ai-workspace/config.toml \
  ghcr.io/wso2/api-platform/ai-workspace:<version>
```

A browser-safe key keeps the **same name everywhere** — in `config.toml`, as an environment override, in Vite's `import.meta.env` at build time, and in the `window.__RUNTIME_CONFIG__` payload the BFF serves to the SPA. (Vite's `envPrefix` is configured with an explicit allowlist of these browser-safe names — mirroring the BFF allowlist — so secrets sharing the namespace never reach the bundle.)

Only keys on the BFF's browser-safe allowlist (`bff/internal/config/runtime_config.go`) are ever emitted to the page — server-side settings and the OIDC client credentials are not.
