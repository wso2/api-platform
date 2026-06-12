# AI Workspace: Production Setup

This guide covers setting up Asgardeo as the identity provider and configuring both Platform API and AI Workspace for production use.

## 1. Configure Asgardeo IDP

### 1.1 Root Organization

1. Log in to [https://console.asgardeo.io](https://console.asgardeo.io).
2. Create your root organization (e.g. `default`).
3. If multi-tenancy is needed, create sub-organizations at:
   `https://console.asgardeo.io/t/<root-org>/app/organizations`

### 1.2 AI Workspace Application (Single-Page Application)

1. Create a **Single-Page Application** named `AI Workspace` in root organization.
2. Add authorized redirect URLs (e.g. `https://<your-domain>/signin`).
3. Enable **Share with all organizations**.
4. In the **Protocol** tab, set **Access Token Type** to `JWT`.
5. In the **Login Flow** tab, remove Username/Password and set **SSO Authentication**.
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
`/etc/ai-workspace/config.toml`. Environment variables always take priority over values in
the file (see the key-to-variable mapping below).

Open `configs/config.toml` and fill in the values for your deployment:

```toml
# Host shown in the browser address bar — used to derive OIDC redirect URIs automatically.
domain = "<your-domain>"                                           # e.g. app.example.com

# Set to "oidc" for production (Asgardeo or any OIDC-compliant IDP).
auth_mode = "oidc"

# Issuer URL — OIDC endpoints are auto-discovered from {oidc_authority}/.well-known/openid-configuration.
oidc_authority = "https://api.asgardeo.io/t/<your-tenant>/oauth2/token"

# Client ID of the AI Workspace SPA (from Asgardeo Protocol tab).
oidc_client_id = "<ai-workspace-client-id>"

# JWT claim name mappings — must match AUTH_IDP_*_CLAIM_NAME values in Platform API (section 2).
oidc_org_id_claim     = "org_id"
oidc_org_name_claim   = "org_name"
oidc_org_handle_claim = "org_handle"

# Platform API base URL used by the UI to make API calls.
platform_api_base_url = "https://<platform-api-host>/api/v1"

# Externally reachable host:port that deployed gateways use to reach the Platform API.
controlplane_host = "<platform-api-host>"

# Default region assigned to new organizations on first login.
default_org_region = "us"
```

> **Redirect URIs** are derived automatically from `domain`:
> `https://<domain>/signin` (sign-in) and `https://<domain>/login` (post-logout).
> These must be listed as authorized redirect URLs in the Asgardeo application (section 1.2).

### config.toml → environment variable mapping

| config.toml key        | Environment variable           |
|------------------------|-------------------------------|
| `domain`               | `VITE_DOMAIN`                 |
| `auth_mode`            | `VITE_AUTH_MODE`              |
| `oidc_authority`       | `VITE_OIDC_AUTHORITY`         |
| `oidc_client_id`       | `VITE_OIDC_CLIENT_ID`         |
| `oidc_org_id_claim`    | `VITE_OIDC_ORG_ID_CLAIM`      |
| `oidc_org_name_claim`  | `VITE_OIDC_ORG_NAME_CLAIM`    |
| `oidc_org_handle_claim`| `VITE_OIDC_ORG_HANDLE_CLAIM`  |
| `platform_api_base_url`| `VITE_PLATFORM_API_BASE_URL`  |
| `controlplane_host`    | `VITE_CONTROLPLANE_HOST`      |
| `default_org_region`   | `VITE_DEFAULT_ORG_REGION`     |

Environment variables (e.g. passed via `docker run -e` or a Kubernetes `env:` block) always
override the corresponding `config.toml` value.
