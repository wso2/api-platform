# Asgardeo IDP Setup

This guide walks through configuring WSO2 Asgardeo as the identity provider for a production Developer Portal deployment.

A full configuration reference is in [authentication.md](authentication.md).

## Prerequisites

- An Asgardeo account at [console.asgardeo.io](https://console.asgardeo.io)
- Developer Portal accessible at a known hostname
- `production/scripts/register_asgardeo_scopes.sh` from this repository

---

## 1. Root Organization

1. Log in to [console.asgardeo.io](https://console.asgardeo.io).
2. Create or select your root organization.
3. For multi-tenancy, create sub-organizations at:
   `https://console.asgardeo.io/t/<root-org>/app/organizations`

---

## 2. Developer Portal Application

1. Go to **Applications** → **New Application**.
2. Choose **Traditional Web Application** (confidential client — the devportal is server-side and can hold a secret).
3. Under **Authorized redirect URLs**, add both:
   - `https://<your-domain>/<orgName>/callback` — login callback
   - `https://<your-domain>/<orgName>` — post-logout redirect (Asgardeo validates `post_logout_redirect_uri` against this same list)
4. Enable **Share with all organizations** so sub-org users can log in.
5. Under the **Protocol** tab:
   - Set **Access Token Type** to **JWT**.
6. Under the **Login Flow** tab:
   - Remove the Username/Password authenticator.
   - Add **SSO Authentication** (organization SSO).
7. Under the **User Attributes** tab, add these attributes to the token:
   - `given_name`, `family_name`, `email`, `roles`

Note the **Client ID** and **Client Secret** from the Protocol tab.

> When `clientId` is configured, clicking Login on a devportal page redirects the user directly to Asgardeo — no intermediate login page is shown. The Organization SSO authenticator (step 7) routes the user to their sub-org's login experience.

---

## 3. Register dp:* Scopes

Create a system OIDC application (e.g. `DevPortal System`) and under **API Authorization** add:
- **API Resource Management API** (Management APIs)
- **Application Management API** (Management APIs)

Note its **Client ID** and **Client Secret**, then run:

```bash
ASGARDEO_TENANT=<your-tenant> \
ASGARDEO_CLIENT_ID=<system-app-client-id> \
ASGARDEO_CLIENT_SECRET=<system-app-client-secret> \
ASGARDEO_RESOURCE_IDENTIFIER=https://<your-domain> \
./production/scripts/register_asgardeo_scopes.sh
```

For local development, the default `ASGARDEO_RESOURCE_IDENTIFIER=https://localhost:3000` works without changes.

---

## 4. Link Scopes to the Application

1. Open the **Developer Portal** application in Asgardeo.
2. Under **API Authorization**, add the API resource created in step 3.
3. Create a role (e.g. `dp_admin`) and assign all `dp:*` scopes to it.
4. Assign the role to users in each sub-organization.

---

## 5. Developer Portal Configuration

Update `configs/config.yaml`:

```yaml
identityProvider:
  name: "Asgardeo"
  issuer: "https://api.asgardeo.io/t/<your-tenant>/oauth2/token"
  authorizationURL: "https://api.asgardeo.io/t/<your-tenant>/oauth2/authorize"
  tokenURL: "https://api.asgardeo.io/t/<your-tenant>/oauth2/token"
  userInfoURL: "https://api.asgardeo.io/t/<your-tenant>/oauth2/userinfo"
  clientId: "<devportal-app-client-id>"
  clientSecret: "<devportal-app-client-secret>"   # env: DP_IDENTITYPROVIDER_CLIENTSECRET
  audience: "<devportal-app-client-id>"           # Asgardeo sets clientId as the aud claim
  callbackURL: "https://<your-domain>/default/callback"
  logoutURL: "https://api.asgardeo.io/t/<your-tenant>/oidc/logout"
  logoutRedirectURI: "https://<your-domain>/default"
  jwksURL: "https://api.asgardeo.io/t/<your-tenant>/oauth2/jwks"
  scope: "openid profile email dp:org_manage dp:api_read dp:app_manage dp:subscription_manage"
  orgIDClaim: "org_id"    # Asgardeo B2B sub-org claim
  roleClaim: "roles"
```

> **Note:** Set `clientSecret` via the `DP_IDENTITYPROVIDER_CLIENTSECRET` environment variable rather than in the config file.

> **Callback URL:** A single `callbackURL` is shared across all devportal organizations. After the callback, the portal uses the session's `returnTo` value to redirect the user to the correct org. Register only the URL you set in `callbackURL` with Asgardeo.

---

## 6. Multi-Organization Setup

Each devportal organization maps to one Asgardeo sub-organization. To enable org-scoped login and access isolation, set the `ORGANIZATION_IDENTIFIER` field on each devportal org to the Asgardeo sub-org's **handle** (the URL slug shown in the Asgardeo console).

### How org-scoped login works

When a user clicks Login on a devportal org, the portal appends `org=<ORGANIZATION_IDENTIFIER>` to the Asgardeo authorization URL. Asgardeo scopes the login session to that sub-org and the issued token contains `org_id` set to the sub-org's UUID.

On every authenticated request, the portal verifies that the token's `org_id` matches the org's `ORGANIZATION_IDENTIFIER`. A mismatch blocks access to protected pages with a 401 — the user must log out and re-login on the correct org.

### Multi-org user flow

- One login session per org — each session is scoped to one Asgardeo sub-org.
- Public pages (API catalog, documentation) remain accessible across orgs without re-authentication.
- Protected pages (applications, subscriptions, API keys) require a token matching the org's `ORGANIZATION_IDENTIFIER`.
- To access a second org's protected pages, the user logs out and logs in again on that org.

---

## 7. Platform API Configuration

When the devportal forwards subscription and API key operations to the Platform API, it sends the user's Asgardeo access token as `Authorization: Bearer <token>`. The Platform API must be configured to validate these IDP-issued tokens via JWKS rather than its own local HMAC JWTs.

Add the following to your `config-platform-api.toml`:

```toml
[auth.idp]
enabled  = true
jwks_url = "https://api.asgardeo.io/t/<your-tenant>/oauth2/jwks"
issuer   = ["https://api.asgardeo.io/t/<your-tenant>/oauth2/token"]
audience = ["<devportal-app-client-id>"]

[auth.idp.claim_mappings]
scope_claim_name        = "scope"   # Asgardeo uses "scope" (space-separated)
organization_claim_name = "org_id"  # Asgardeo B2B sub-org UUID
org_handle_claim_name   = "org_id"  # Asgardeo does not emit "org_handle"; use org_id
user_id_claim_name      = "sub"
```

Or via environment variables:

```bash
AUTH_IDP_ENABLED=true
AUTH_IDP_JWKS_URL=https://api.asgardeo.io/t/<your-tenant>/oauth2/jwks
AUTH_IDP_ISSUER=https://api.asgardeo.io/t/<your-tenant>/oauth2/token
AUTH_IDP_AUDIENCE=<devportal-app-client-id>
AUTH_IDP_CLAIM_MAPPINGS_SCOPE_CLAIM_NAME=scope
AUTH_IDP_CLAIM_MAPPINGS_ORGANIZATION_CLAIM_NAME=org_id
AUTH_IDP_CLAIM_MAPPINGS_ORG_HANDLE_CLAIM_NAME=org_id
```

> The devportal's `platformApi.jwtSecret` setting is only used in local auth mode (to verify Platform API-issued HMAC tokens on the devportal side). It has no effect when an external IDP is configured — leave it empty.

---

## Claim Flow Summary

```
Asgardeo token
  ├── sub      → user identity
  ├── org_id   → organization UUID  (→ orgIDClaim)
  ├── roles    → role list          (→ roleClaim → isAdmin check)
  └── scope    → space-separated dp:* scopes enforced per API operation
```

---

## Relationship to ai-workspace Asgardeo Setup

If you are also running ai-workspace and platform-api with Asgardeo, the setups are independent but use the same root organization:

| Component | App Type | Callback URL | Scopes |
|-----------|----------|--------------|--------|
| **ai-workspace** | Standard-Based SPA (public client, no secret) | `https://<domain>/signin` | `ap:*` (Platform API) |
| **devportal** | Traditional Web Application (confidential client) | `https://<domain>/<orgName>/callback` | `dp:*` (Developer Portal) |
| **platform-api** | — (validates tokens via JWKS; same `ap:*` scopes as ai-workspace) | — | — |

Each application is registered separately in Asgardeo with its own client ID and scopes.
