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
