# Configuration Reference

AI Workspace is configured through a `config.toml` file mounted into the container at `/etc/ai-workspace/config.toml`. Every key in `config.toml` can be overridden with a corresponding environment variable — environment variables always take precedence.

Copy `configs/config-template.toml` to `configs/config.toml` and fill in the values for your deployment before starting the stack.

## All Configuration Keys

### Core

| Key | Env variable | Default | Description |
|-----|-------------|---------|-------------|
| `domain` | `VITE_DOMAIN` | `localhost:5380` | Host (and optional port) shown in the browser address bar. Used to derive OIDC redirect URIs automatically. |
| `auth_mode` | `VITE_AUTH_MODE` | `basic` | Authentication mode. `"basic"` for file-based local auth; `"oidc"` for external IDP. |
| `platform_api_base_url` | `VITE_PLATFORM_API_BASE_URL` | `https://localhost:9243/api/v1` | Base URL the browser uses to reach the Platform API. May be a relative path (e.g. `/api-proxy/api/v1`) when nginx reverse-proxying is in use. |
| `controlplane_host` | `VITE_CONTROLPLANE_HOST` | `localhost:9243` | Externally reachable `host:port` that deployed gateways use to reach the Platform API. Shown in gateway setup instructions. Must be an absolute address, not a relative path. |
| `default_org_region` | `VITE_DEFAULT_ORG_REGION` | `us` | Default region label assigned to new organizations on first login. |

### OIDC (only required when `auth_mode = "oidc"`)

| Key | Env variable | Default | Description |
|-----|-------------|---------|-------------|
| `oidc_authority` | `VITE_OIDC_AUTHORITY` | — | OIDC issuer URL. OIDC endpoints (authorization, token, JWKS, etc.) are auto-discovered from `{oidc_authority}/.well-known/openid-configuration`. |
| `oidc_client_id` | `VITE_OIDC_CLIENT_ID` | — | Client ID of the AI Workspace SPA registered in your IDP. |
| `oidc_org_id_claim` | `VITE_OIDC_ORG_ID_CLAIM` | `org_id` | JWT claim name for the organization UUID. Must match `organization_claim_name` in Platform API config. |
| `oidc_org_name_claim` | `VITE_OIDC_ORG_NAME_CLAIM` | `org_name` | JWT claim name for the human-readable organization name. |
| `oidc_org_handle_claim` | `VITE_OIDC_ORG_HANDLE_CLAIM` | `org_handle` | JWT claim name for the organization handle (slug). |

OIDC redirect URIs are derived automatically from `domain`:
- Sign-in: `https://<domain>/signin`
- Post-logout: `https://<domain>/login`

Both must be registered as authorized redirect URLs in your IDP application.

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
| `DATABASE_PASSWORD` | Database password |

## Environment Variable Override

Any `config.toml` key can be overridden by setting the corresponding `VITE_*` environment variable. This is useful in container orchestration environments (Kubernetes `env:` blocks, Docker Compose `environment:` sections) where file mounts are less convenient.

Example — override just the authority for a staging environment:

```bash
docker run \
  -e VITE_OIDC_AUTHORITY=https://api.asgardeo.io/t/staging-tenant/oauth2/token \
  -v ./configs/config.toml:/etc/ai-workspace/config.toml \
  ghcr.io/wso2/api-platform/ai-workspace:<version>
```
