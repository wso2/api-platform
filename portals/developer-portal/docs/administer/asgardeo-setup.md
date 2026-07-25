# Asgardeo IDP Setup

This guide walks through configuring WSO2 Asgardeo as the identity provider for a production Developer Portal deployment.

A full configuration reference is in [authentication.md](authentication.md).

## Overview

The Developer Portal uses Asgardeo's sub-organization model: each devportal organization maps to one Asgardeo sub-organization. A single Asgardeo application (Traditional Web App) is shared across all devportal orgs, but each login session is scoped to a specific sub-org.

**How it works end-to-end:**

1. A devportal org has an `ORGANIZATION_IDENTIFIER` set to its Asgardeo sub-org handle.
2. When a user clicks Login, the devportal redirects to Asgardeo with `org=<identifier>`, scoping the authorization to that sub-org.
3. Asgardeo issues a JWT with `org_id` set to the sub-org's UUID. The devportal verifies this claim matches the org on every authenticated request.
4. Each login session is bound to one sub-org — accessing a different devportal org's protected pages requires logging out and back in on that org.

**Key design decisions:**
- One Asgardeo application (confidential client) serves all devportal orgs — the callback URL is shared, with per-request routing via session state.
- Public pages (API catalog, docs) are always accessible without authentication.
- Protected pages (applications, subscriptions, API keys) enforce the org claim match.

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

> The `dp:*` scopes are enforced per-operation by the devportal for machine API clients that call `/api/v0.9/*` directly with a Bearer token. Browser sessions (IDP login) are preauthorized — the portal trusts session-level authentication and skips per-operation scope checks for session users. The Platform API uses its own `ap:*` scopes and does not validate `dp:*`.

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

For local development, the default `ASGARDEO_RESOURCE_IDENTIFIER=https://localhost:9543` works without changes.

> The system application is only needed to run this script. Once the `dp:*` API resource is registered in Asgardeo, the system app can be deleted.

---

## 4. Link Scopes to the Application

1. Open the **Developer Portal** application in Asgardeo.
2. Under **API Authorization**, add the API resource created in step 3.
3. Create a role (e.g. `dp_admin`) and assign all `dp:*` scopes to it.
4. Assign the role to users in each sub-organization.

---

## 5. Developer Portal Configuration

Update `configs/config.toml`:

```toml
[idp]
name = "Asgardeo"
issuer = "https://api.asgardeo.io/t/<your-tenant>/oauth2/token"
authorization_url = "https://api.asgardeo.io/t/<your-tenant>/oauth2/authorize"
token_url = "https://api.asgardeo.io/t/<your-tenant>/oauth2/token"
user_info_url = "https://api.asgardeo.io/t/<your-tenant>/oauth2/userinfo"
client_id = "<devportal-app-client-id>"
client_secret = "<devportal-app-client-secret>"   # env: APIP_DP_IDP_CLIENTSECRET
audience = "<devportal-app-client-id>"            # Asgardeo sets client_id as the aud claim
callback_url = "https://<your-domain>/default/callback"
logout_url = "https://api.asgardeo.io/t/<your-tenant>/oidc/logout"
logout_redirect_uri = "https://<your-domain>/default"
jwks_url = "https://api.asgardeo.io/t/<your-tenant>/oauth2/jwks"
scope = "openid profile email"   # dp:* not needed — browser sessions are preauthorized

[idp.claims]
org_id = "org_name"    # Asgardeo B2B: org_name matches ORGANIZATION_IDENTIFIER (sub-org display name)
role = "roles"
```

> **Note:** Set `client_secret` via the `APIP_DP_IDP_CLIENTSECRET` environment variable rather than in the config file.

> **Callback URL:** A single `callback_url` is shared across all devportal organizations. After the callback, the portal uses the session's `returnTo` value to redirect the user to the correct org. Register only the URL you set in `callback_url` with Asgardeo.

---

## 6. Multi-Organization Setup

Each devportal organization maps to one Asgardeo sub-organization. To enable org-scoped login and access isolation, set the `ORGANIZATION_IDENTIFIER` field on each devportal org to the Asgardeo sub-org's **handle** (the URL slug shown in the Asgardeo console).

### How org-scoped login works

When a user clicks Login on a devportal org, the portal appends `org=<ORGANIZATION_IDENTIFIER>` to the Asgardeo authorization URL. Asgardeo scopes the login session to that sub-org and the issued token contains `org_id` set to the sub-org's UUID.

On every authenticated request, the portal verifies that the token's org claim matches the org's `ORGANIZATION_IDENTIFIER`. A mismatch blocks access to protected pages with a 403 — the user must log out and re-login on the correct org.

### Multi-org user flow

- One login session per org — each session is scoped to one Asgardeo sub-org.
- Public pages (API catalog, documentation) remain accessible across orgs without re-authentication.
- Protected pages (applications, subscriptions, API keys) require a token matching the org's `ORGANIZATION_IDENTIFIER`.
- To access a second org's protected pages, the user logs out and logs in again on that org.

---

## Claim Flow Summary

```
Asgardeo token
  ├── sub      → user identity
  ├── org_name → sub-org display name  (→ orgIDClaim → compared to ORGANIZATION_IDENTIFIER)
  ├── org_id   → sub-org UUID          (available but not used for org matching)
  └── roles    → role list             (→ roleClaim → isAdmin check)
```

> Browser sessions are preauthorized — the `scope` claim is not checked against per-operation requirements for session users. Machine API clients calling `/api/v0.9/*` directly with a Bearer token have their `scope` claim enforced as usual.

---

## Relationship to ai-workspace Asgardeo Setup

If you are also running ai-workspace and platform-api with Asgardeo, the setups are independent but use the same root organization:

| Component | App Type | Callback URL | Scopes |
|-----------|----------|--------------|--------|
| **ai-workspace** | Standard-Based SPA (public client, no secret) | `https://<domain>/signin` | `ap:*` (Platform API) |
| **devportal** | Traditional Web Application (confidential client) | `https://<domain>/<orgName>/callback` | `dp:*` (Developer Portal) |
| **platform-api** | — (validates tokens via JWKS; same `ap:*` scopes as ai-workspace) | — | — |

Each application is registered separately in Asgardeo with its own client ID and scopes.

---

## Next Steps

- [Get a Bearer Token via curl](api-token-curl.md) — test the devportal REST API from the terminal once IDP is set up
