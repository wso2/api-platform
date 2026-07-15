# AI Workspace: Production Setup

This guide covers setting up Asgardeo as the identity provider and configuring both Platform API and AI Workspace for production use.

## 1. Configure Asgardeo IDP

### 1.1 Root Organization

1. Log in to [https://console.asgardeo.io](https://console.asgardeo.io).
2. Create your root organization (e.g. `default`).
3. If multi-tenancy is needed, create sub-organizations at:
   `https://console.asgardeo.io/t/<root-org>/app/organizations`

### 1.2 AI Workspace Application (Confidential Web Application)

The AI Workspace is served by a **BFF that acts as a confidential OIDC client** — it holds the
client secret and runs the authorization-code + PKCE exchange on the back channel. Register it
as a confidential web app, **not** a Single-Page Application (an SPA is a public client and the
token endpoint will reject the BFF's exchange).

1. Create a **Standard-Based Application → OpenID Connect** (Traditional Web Application) named
   `AI Workspace` in the root organization.
2. Add the authorized redirect URL — the **BFF callback**, not `/signin`:
   `https://<your-domain>/api/auth/callback`
3. In the **Protocol** tab:
   - **Allowed grant types**: Authorization Code + Refresh Token.
   - **PKCE**: enabled.
   - **Access Token Type**: `JWT`.
   - Note the **Client ID** and **Client Secret** (the BFF needs both).
4. Enable **Share with all organizations**.
5. In the **Login Flow** tab, configure authentication as desired (e.g. SSO).
6. In the **User Attributes** tab, add the following attributes to the token:
   - `username`
   - `given_name`
   - `family_name`
   - `roles`
   - `email`
   - `scope` (see section 1.3)

### 1.3 Custom User Attributes

Create the following custom user attributes at:
`https://console.asgardeo.io/t/<root-org>/app/attributes`

| Attribute Name | Purpose                            |
|----------------|------------------------------------|
| `scope`        | OAuth2 scopes granted to the user  |

Then add OIDC scope mappings at:
`https://console.asgardeo.io/t/<root-org>/app/oidc-scopes`

### 1.4 System Application (for Scope Registration)

1. Create a new **OIDC Application** and label it as the system application.
2. Add the following API resources to the system application:
   - **API Resource Management API**
   - **Application Management API**
3. Run the scope registration script to create the Platform API resource and all `ap:*` scopes in Asgardeo:

   ```bash
   ASGARDEO_TENANT=<your-tenant> \
   ASGARDEO_CLIENT_ID=<system-app-client-id> \
   ASGARDEO_CLIENT_SECRET=<system-app-client-secret> \
   ASGARDEO_RESOURCE_IDENTIFIER=https://<platform-api-host> \
   ./scripts/register_asgardeo_scopes.sh
   ```

   The script defaults (`ASGARDEO_RESOURCE_IDENTIFIER=https://localhost:9243`) work for local development.

### 1.5 Link Scopes to the AI Workspace Application

1. In the AI Workspace application, add the API resource created in step 1.4.
2. Create an application role (e.g. `ap_admin`) and assign the `ap:*` scopes to it.

### 1.6 Sub-Organization Users

In each sub-organization:

1. Register users.
2. Assign the shared `ap_admin` role to users.

---

## 2. Platform API Configuration

The Platform API reads its configuration from `config-platform-api.toml` (mounted at
`/etc/platform-api/config.toml` in the container). Open `configs/config-platform-api.toml`
and update the `[auth.idp]` section for production:

> **Note:** Asgardeo uses `org_id` as the JWT claim for the organization UUID. The Platform
> API defaults to `organization`, so the claim name overrides below are required.

```toml
# Disable local JWT auth when delegating entirely to an external IDP.
[auth.jwt]
enabled = false

# Enable JWKS-based IDP authentication.
[auth.idp]
enabled  = true
name     = "asgardeo"
jwks_url = "https://api.asgardeo.io/t/<your-tenant>/oauth2/jwks"
issuer   = ["https://api.asgardeo.io/t/<your-tenant>/oauth2/token"]
audience = ["<ai-workspace-client-id>"]   # Client ID from Asgardeo Protocol tab

# Asgardeo-specific claim name overrides.
[auth.idp.claim_mappings]
organization_claim_name = "org_id"
org_name_claim_name     = "org_name"
org_handle_claim_name   = "org_handle"

# Disable file-based auth in production.
[auth.file_based]
enabled = false
```

Optional overrides (defaults shown):

```toml
[auth.idp]
validation_mode = "scope"   # or "role" for role-based auth

[auth.idp.claim_mappings]
user_id_claim_name  = "sub"
username_claim_name = "username"
email_claim_name    = "email"
scope_claim_name    = "scope"
```

---

## 3. AI Workspace Configuration

The AI Workspace container reads its configuration from a `config.toml` file mounted at
`/etc/ai-workspace/config.toml` — the only source of configuration. Keys are grouped into TOML
tables; a value comes from the environment only through an `{{ env }}` token (see below).

Open `configs/config.toml` and fill in the values for your deployment:

```toml
# Host shown in the browser address bar.
domain = "<your-domain>"                                           # e.g. app.example.com

# Set to "oidc" for production (Asgardeo or any OIDC-compliant IDP).
auth_mode = "oidc"

# Externally reachable host:port that deployed gateways use to reach the Platform API.
controlplane_host = "<platform-api-host>"

# Default region assigned to new organizations on first login.
default_org_region = "us"

# Available gateway versions shown in the create-gateway version selector (JSON array string).
# Each entry: version (helm chart minor), latestVersion (image/chart tag), channel ("STS" | "LTS").
platform_gateway_versions = '[{"version":"1.2","latestVersion":"v1.2.0-M1","channel":"STS"}]'

[platform_api]
# The upstream the BFF proxies to, server-to-server. An origin, not a base path — the
# browser never uses it: the SPA calls the same-origin proxy prefix and the BFF forwards.
url = "https://<platform-api-host>"

[oidc]
# Issuer URL — the BFF auto-discovers OIDC endpoints from
# {authority}/.well-known/openid-configuration.
authority = "https://api.asgardeo.io/t/<your-tenant>/oauth2/token"

# Client ID of the AI Workspace confidential application (from the IDP Protocol tab).
client_id = "<ai-workspace-client-id>"

# JWT claim name mappings — this table mirrors [auth.idp.claim_mappings] in the Platform
# API config (section 2) key for key, and the two must agree.
[oidc.claim_mappings]
organization_claim_name = "org_id"
org_name_claim_name     = "org_name"
org_handle_claim_name   = "org_handle"
```

The redirect URLs are ordinary `config.toml` keys. The **client secret is never written into the
file** — it is referenced with an interpolation token resolved at startup. In production, mount it
as a secret file (a Docker/Kubernetes secret) so the value never enters the environment at all:

```toml
[oidc]
# BFF callback registered in the IDP (section 1.2) — NOT the SPA /signin route.
redirect_url             = "https://<your-domain>/api/auth/callback"
post_logout_redirect_url = "https://<your-domain>/login"

# Preferred in production — a mounted secret file under an allowed directory.
client_secret = '{{ file "/secrets/ai-workspace/oidc_client_secret" }}'
```

Mount the secret at that path, e.g. in `docker-compose.yaml`:

```yaml
    volumes:
      - ./secrets/oidc_client_secret:/secrets/ai-workspace/oidc_client_secret:ro
```

Resolution fails closed: a missing or unreadable secret file aborts startup rather than yielding an
empty credential. `{{ file }}` paths must live under `/etc/ai-workspace` or `/secrets/ai-workspace`
(override with `APIP_CONFIG_FILE_SOURCE_ALLOWLIST`). For a simpler local setup, swap the token for
`'{{ env "APIP_AIW_OIDC_CLIENT_SECRET" }}'` and keep the value in the git-ignored `api-platform.env`.

> `[oidc] redirect_url` must exactly match the authorized redirect URL registered in the IDP
> application (section 1.2). The BFF, not the browser, completes the code exchange.

### Setting config.toml keys from the environment

`config.toml` is the only source of configuration — there is no separate environment-override
layer. A key takes its value from the environment when it is written as an `{{ env }}` token, which
names the variable explicitly:

```toml
[oidc]
client_id = '{{ env "APIP_AIW_OIDC_CLIENT_ID" "" }}'
#                  ^ variable read at startup  ^ used when it is unset
```

Setting `APIP_AIW_OIDC_CLIENT_ID` in a `docker run -e` or a Kubernetes `env:` block then sets
`[oidc] client_id`, with no edit to the file. Setting it while the key is absent from the file, or
written as a plain literal, does nothing.

The shipped `config.toml` already writes its keys this way, naming each variable by the same
convention: the key's table path uppercased, dots as underscores, prefixed with **`APIP_AIW_`** (the
Platform API uses `APIP_CP_`, the Developer Portal `APIP_DP_`).

| config.toml key                      | Variable named by its shipped token      |
|--------------------------------------|------------------------------------------|
| `domain`                             | `APIP_AIW_DOMAIN`                        |
| `auth_mode`                          | `APIP_AIW_AUTH_MODE`                     |
| `controlplane_host`                  | `APIP_AIW_CONTROLPLANE_HOST`             |
| `log_level`                          | `APIP_AIW_LOG_LEVEL`                     |
| `[platform_api] url`                 | `APIP_AIW_PLATFORM_API_URL`              |
| `[oidc] authority`                   | `APIP_AIW_OIDC_AUTHORITY`                |
| `[oidc] client_id`                   | `APIP_AIW_OIDC_CLIENT_ID`                |
| `[oidc] client_secret`               | `APIP_AIW_OIDC_CLIENT_SECRET`            |
| `[oidc] redirect_url`                | `APIP_AIW_OIDC_REDIRECT_URL`             |
| `[oidc.claim_mappings] organization_claim_name` | `APIP_AIW_OIDC_CLAIM_MAPPINGS_ORGANIZATION_CLAIM_NAME` |

A token may name any variable, not only the conventional one — that is what lets a key read a
secret that already exists under its own name. For credentials, prefer a mounted secret file
(`{{ file }}`) over the environment.

---

## 4. Production requirements

There is no demo mode: startup checks are always on for **both** the `platform-api` and
`ai-workspace` services and fail fast when a requirement is missing. For production, replace
the quickstart's `setup.sh` outputs with real values: use OIDC (sections 1–3) instead of the
generated file-based admin user, mount TLS certificates from your CA instead of the generated
self-signed pairs, and manage `APIP_CP_ENCRYPTION_KEY` / `APIP_CP_AUTH_JWT_SECRET_KEY` as
stable, real secrets (prefer `{{ file }}` tokens over environment variables).

See [Production hardening](../README.md#production-hardening) in the main README for the
full checklist of what each service requires.
