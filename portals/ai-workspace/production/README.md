# AI Workspace: Production Setup

This guide covers setting up Asgardeo as the identity provider and configuring both Platform API and AI Workspace for production use.

---

## Quick Start: Basic Auth Mode (No IDP Required)

Basic auth mode lets you run the full stack locally without any external identity provider. The Platform API issues HMAC-signed JWTs directly after validating credentials against a built-in user list.

### Step 1 — Hash your password

Use any bcrypt tool at cost factor 10 or higher:

```bash
# Python (most systems)
python3 -c "import bcrypt; print(bcrypt.hashpw(b'yourpassword', bcrypt.gensalt(10)).decode())"

# htpasswd (Apache utils)
htpasswd -bnBC 10 "" yourpassword | tr -d ':\n'
```

> The output looks like `$2a$10$...` — this is the value for `password_hash` below.

### Step 2 — Configure the Platform API

Set these environment variables before starting the Platform API:

```bash
# Enable basic-auth mode (disables IDP)
export AUTH_BASIC_AUTH_ENABLED=true

# Organization seeded on first startup — pick any name and URL-safe handle
export AUTH_BASIC_AUTH_ORGANIZATION_NAME="My Org"
export AUTH_BASIC_AUTH_ORGANIZATION_HANDLE="my-org"
export AUTH_BASIC_AUTH_ORGANIZATION_REGION="us"

# Built-in users — JSON array, scopes are space-separated
# Replace <bcrypt-hash> with the hash from Step 1
# Replace scopes with scopes you wish to assign for the user
export AUTH_BASIC_AUTH_USERS='[{"username":"admin","password_hash":"<bcrypt-hash>","scopes":"ap:organization:read ap:organization:manage ap:organization:subscription:read ap:project:read ap:project:create ap:project:update ap:project:delete ap:project:manage ap:application:read ap:application:create ap:application:update ap:application:delete ap:application:manage ap:application:api_key:read ap:application:api_key:create ap:application:api_key:delete ap:application:api_key:manage ap:application:associations:read ap:application:associations:create ap:application:associations:delete ap:application:associations:manage ap:application:associations:api_key:read ap:gateway:read ap:gateway:create ap:gateway:update ap:gateway:delete ap:gateway:manage ap:gateway:token:read ap:gateway:token:create ap:gateway:token:delete ap:gateway:token:manage ap:gateway:policy:read ap:gateway:policy:create ap:gateway:policy:delete ap:gateway:policy:manage ap:gateway:artifacts:read ap:gateway:manifest:read ap:rest_api:read ap:rest_api:create ap:rest_api:update ap:rest_api:delete ap:rest_api:manage ap:rest_api:import ap:rest_api:gateway:read ap:rest_api:gateway:create ap:rest_api:gateway:manage ap:rest_api:deployment:read ap:rest_api:deployment:create ap:rest_api:deployment:delete ap:rest_api:deployment:manage ap:rest_api:deployment:undeploy ap:rest_api:deployment:restore ap:rest_api:api_key:read ap:rest_api:api_key:create ap:rest_api:api_key:update ap:rest_api:api_key:delete ap:rest_api:api_key:manage ap:rest_api:publication:read ap:rest_api:publication:create ap:rest_api:publication:delete ap:devportal:read ap:devportal:create ap:devportal:update ap:devportal:delete ap:devportal:manage ap:subscription:read ap:subscription:create ap:subscription:update ap:subscription:delete ap:subscription:manage ap:subscription_plan:read ap:subscription_plan:create ap:subscription_plan:update ap:subscription_plan:delete ap:subscription_plan:manage ap:llm_template:read ap:llm_template:create ap:llm_template:update ap:llm_template:delete ap:llm_template:manage ap:llm_provider:read ap:llm_provider:create ap:llm_provider:update ap:llm_provider:delete ap:llm_provider:manage ap:llm_provider:api_key:read ap:llm_provider:api_key:create ap:llm_provider:api_key:delete ap:llm_provider:api_key:manage ap:llm_provider:deployment:read ap:llm_provider:deployment:create ap:llm_provider:deployment:delete ap:llm_provider:deployment:manage ap:llm_provider:deployment:undeploy ap:llm_provider:deployment:restore ap:llm_proxy:read ap:llm_proxy:create ap:llm_proxy:update ap:llm_proxy:delete ap:llm_proxy:manage ap:llm_proxy:api_key:read ap:llm_proxy:api_key:create ap:llm_proxy:api_key:delete ap:llm_proxy:api_key:manage ap:llm_proxy:deployment:read ap:llm_proxy:deployment:create ap:llm_proxy:deployment:delete ap:llm_proxy:deployment:manage ap:llm_proxy:deployment:undeploy ap:llm_proxy:deployment:restore ap:mcp_proxy:read ap:mcp_proxy:create ap:mcp_proxy:update ap:mcp_proxy:delete ap:mcp_proxy:manage ap:mcp_proxy:deployment:read ap:mcp_proxy:deployment:create ap:mcp_proxy:deployment:delete ap:mcp_proxy:deployment:manage ap:mcp_proxy:deployment:undeploy ap:mcp_proxy:deployment:restore ap:websub_api:read ap:websub_api:create ap:websub_api:update ap:websub_api:delete ap:websub_api:manage ap:websub_api:api_key:read ap:websub_api:api_key:create ap:websub_api:api_key:delete ap:websub_api:api_key:manage ap:websub_api:api_key:update ap:websub_api:deployment:read ap:websub_api:deployment:create ap:websub_api:deployment:delete ap:websub_api:deployment:manage ap:websub_api:deployment:undeploy ap:websub_api:deployment:restore ap:websub_api:publication:read ap:websub_api:publication:create ap:websub_api:publication:delete ap:webbroker_api:read ap:webbroker_api:create ap:webbroker_api:update ap:webbroker_api:delete ap:webbroker_api:manage ap:webbroker_api:api_key:read ap:webbroker_api:api_key:create ap:webbroker_api:api_key:delete ap:webbroker_api:api_key:manage ap:webbroker_api:api_key:update ap:webbroker_api:deployment:read ap:webbroker_api:deployment:create ap:webbroker_api:deployment:delete ap:webbroker_api:deployment:manage ap:webbroker_api:deployment:undeploy ap:webbroker_api:deployment:restore ap:webbroker_api:publication:read ap:webbroker_api:publication:create ap:webbroker_api:publication:delete ap:git:read"}]'

# JWT signing key — the Platform API signs tokens with this; keep it secret
export AUTH_JWT_SECRET_KEY="change-me-to-a-random-secret"
export AUTH_JWT_ISSUER="platform-api"

# Enforce scope checks on all protected routes
export ENABLE_SCOPE_VALIDATION=true

# Suppress the "JWT validation disabled" warning in local dev
export DEV_MODE=true
```

> **Multiple users:** add more objects to the JSON array, each with their own `username`, `password_hash`, and `scopes`.

### Step 3 — Configure the AI Workspace

Create or edit `.env.local` in `portals/ai-workspace/`:

```bash
# Switch the frontend to basic-auth login mode
VITE_AUTH_MODE=basic

# Platform API URLs — defaults work when the API runs on localhost:9243
VITE_PLATFORM_API_BASE_URL=https://localhost:9243/api/v1
VITE_PORTAL_API_BASE_URL=https://localhost:9243/api/portal/v1
```

### Step 4 — Trust the self-signed certificate

The Platform API generates a self-signed TLS certificate on first start. Browsers block `fetch()` to untrusted origins, which shows as "Failed to fetch" on the login page.

Fix: open `https://localhost:9243/health` in your browser, click **Advanced → Proceed to localhost**, then return to the workspace. You only need to do this once per browser profile.

### Step 5 — Start both services

```bash
# Terminal 1 — Platform API
cd platform-api
go run ./src/cmd/main.go        # or: make run

# Terminal 2 — AI Workspace
cd portals/ai-workspace
pnpm dev                        # or: npm run dev
```

Navigate to `https://localhost:3009` (or whichever port Vite prints) and sign in with the username and password you configured in Step 1.

---

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

## 2. Platform API Environment Variables

> **Note:** Asgardeo uses `org_id` as the JWT claim for the organization UUID. The Platform API defaults to `organization`, so the mapping overrides below are required.

```bash
# Enable JWKS-based IDP authentication (disables local JWT mode)
export AUTH_IDP_ENABLED=true
export AUTH_IDP_NAME="asgardeo"

# Replace <your-tenant> with your Asgardeo root organization name
export AUTH_IDP_JWKS_URL="https://api.asgardeo.io/t/<your-tenant>/oauth2/jwks"
export AUTH_IDP_ISSUER="https://api.asgardeo.io/t/<your-tenant>/oauth2/token"

# Client ID of the AI Workspace application (from Asgardeo Protocol tab)
export AUTH_IDP_AUDIENCE="<ai-workspace-client-id>"

# Asgardeo-specific claim name overrides
export AUTH_IDP_ORGANIZATION_CLAIM_NAME="org_id"
export AUTH_IDP_ORG_NAME_CLAIM_NAME="org_name"
export AUTH_IDP_ORG_HANDLE_CLAIM_NAME="org_handle"

export ENABLE_SCOPE_VALIDATION=true
```

Optional overrides (defaults shown):

```bash
export AUTH_IDP_VALIDATION_MODE="scope"        # or "role" for role-based auth
export AUTH_IDP_USER_ID_CLAIM_NAME="sub"
export AUTH_IDP_USERNAME_CLAIM_NAME="username"
export AUTH_IDP_EMAIL_CLAIM_NAME="email"
export AUTH_IDP_SCOPE_CLAIM_NAME="scope"
```

---

## 3. AI Workspace Environment Variables

> These can be set as environment variables or in a `.env.local` file at the project root.

```bash
# OIDC authority (issuer URL — OIDC discovery runs from {authority}/.well-known/openid-configuration)
export VITE_OIDC_AUTHORITY="https://api.asgardeo.io/t/<your-tenant>/oauth2/token"

# Client ID of the AI Workspace SPA (from Asgardeo Protocol tab)
export VITE_OIDC_CLIENT_ID="<ai-workspace-client-id>"

# Redirect URIs — must match the authorized URLs set in Asgardeo
export VITE_OIDC_REDIRECT_URI="https://<your-domain>/signin"
export VITE_OIDC_POST_LOGOUT_REDIRECT_URI="https://<your-domain>/login"

# Platform API base URL
export VITE_PLATFORM_API_BASE_URL="https://<platform-api-host>/api/v1"

# JWT claim name overrides — must match AUTH_IDP_*_CLAIM_NAME values in Platform API
export VITE_OIDC_ORG_ID_CLAIM="org_id"
export VITE_OIDC_ORG_NAME_CLAIM="org_name"
export VITE_OIDC_ORG_HANDLE_CLAIM="org_handle"
```

For local development (`VITE_PLATFORM_API_BASE_URL` default: `https://localhost:9243/api/v1`).
