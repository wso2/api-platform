# Asgardeo IDP Setup

This guide walks through configuring WSO2 Asgardeo as the identity provider for a production AI Workspace deployment. It corresponds to the OIDC auth mode described in [OIDC Auth](oidc-auth.md).

A complete reference for Platform API and AI Workspace config values is in [`production/README.md`](../../production/README.md).

## Prerequisites

- An Asgardeo account at [console.asgardeo.io](https://console.asgardeo.io)
- AI Workspace and Platform API containers accessible at known hostnames
- `./scripts/register_asgardeo_scopes.sh` from this repository

---

## 1. Root Organization

1. Log in to [console.asgardeo.io](https://console.asgardeo.io).
2. Create or select your root organization (e.g. `default`).
3. If you need multi-tenancy, create sub-organizations at:
   `https://console.asgardeo.io/t/<root-org>/app/organizations`

---

## 2. AI Workspace Application (Confidential Web Application)

The AI Workspace is served by a **BFF that acts as a confidential OIDC client** — it holds the
client secret and runs the authorization-code + PKCE exchange on the back channel. Register it
as a confidential web app, **not** a Single-Page Application. (An SPA is a public client; the
token endpoint will reject the BFF's exchange with *"The authenticated client is not authorized
to use the requested grant type."*)

1. In the root organization, go to **Applications** → **New Application**.
2. Choose **Standard-Based Application → OpenID Connect** (Traditional Web Application) and name
   it `AI Workspace`.
3. Add the **authorized redirect URL** — the BFF callback, not `/signin`:
   - `https://<your-domain>/api/auth/callback`
4. Enable **Share with all organizations** so sub-org users can log in.
5. Under the **Protocol** tab:
   - **Allowed grant types**: Authorization Code + Refresh Token.
   - **PKCE**: enabled.
   - **Access Token Type**: **JWT**.
6. Under the **Login Flow** tab, configure authentication as desired (e.g. SSO Authentication).
7. Under the **User Attributes** tab, add these attributes to the token:
   - `username`
   - `given_name`
   - `family_name`
   - `roles`
   - `email`
   - `scope` (see section 3 below)

Note the **Client ID** and **Client Secret** from the Protocol tab — the BFF needs both; the
Client ID is also used in the Platform API config (audience).

---

## 3. Custom User Attributes

Create a custom attribute for OAuth2 scopes at:
`https://console.asgardeo.io/t/<root-org>/app/attributes`

| Attribute Name | Purpose |
|----------------|---------|
| `scope` | OAuth2 scopes granted to the user |

Then add OIDC scope mappings at:
`https://console.asgardeo.io/t/<root-org>/app/oidc-scopes`

Map the `scope` OIDC claim to the custom `scope` attribute.

---

## 4. System Application (for Scope Registration)

The `ap:*` Platform API scopes must be registered in Asgardeo before they can be assigned to users. A system application (M2M / OIDC) with management API access is used to do this via the scope registration script.

1. Create a new **OIDC Application** (name it e.g. `AI Platform System`).
2. Under **API Authorization**, add:
   - **API Resource Management API**
   - **Application Management API**
3. Note the **Client ID** and **Client Secret**.
4. Run the scope registration script:

```bash
ASGARDEO_TENANT=<your-tenant> \
ASGARDEO_CLIENT_ID=<system-app-client-id> \
ASGARDEO_CLIENT_SECRET=<system-app-client-secret> \
ASGARDEO_RESOURCE_IDENTIFIER=https://<platform-api-host> \
./scripts/register_asgardeo_scopes.sh
```

This creates an API resource in Asgardeo representing the Platform API, with all `ap:*` scopes registered under it.

For local development, the defaults (`ASGARDEO_RESOURCE_IDENTIFIER=https://localhost:9243`) work without changes.

---

## 5. Link Scopes to the AI Workspace Application

1. Open the **AI Workspace** SPA application.
2. Under **API Authorization**, add the API resource created in step 4.
3. Create an application role (e.g. `ap_admin`).
4. Assign all `ap:*` scopes to the `ap_admin` role.

---

## 6. Sub-Organization Users

In each sub-organization that should have access:

1. Register users under the sub-org.
2. Assign the shared `ap_admin` role to each user.

---

## 7. Platform API Configuration

Update `configs/config-platform-api.toml`:

```toml
[auth.jwt]
enabled = false

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
enabled = false
```

> Asgardeo uses `org_id` as the claim for the organization UUID. The Platform API defaults to `organization`, so the claim name override above is required.

---

## 8. AI Workspace Configuration

Update `configs/config.toml`:

```toml
domain             = "<your-domain>"
auth_mode          = "oidc"
controlplane_host  = "<platform-api-host>"
default_org_region = "us"

[platform_api]
url = "https://<platform-api-host>"

[oidc]
authority = "https://api.asgardeo.io/t/<your-tenant>/oauth2/token"
client_id = "<ai-workspace-client-id>"

# BFF-side redirect URLs — never reach the browser.
redirect_url             = "https://<your-domain>/api/auth/callback"   # the BFF callback (section 2)
post_logout_redirect_url = "https://<your-domain>/login"

# Preferred — a mounted secret file. To read it from the git-ignored api-platform.env instead, swap the
# token for '{{ env "APIP_AIW_OIDC_CLIENT_SECRET" }}': the key needs one token or the other.
client_secret = '{{ file "/secrets/ai-workspace/oidc_client_secret" }}'

# Mirrors [auth.idp.claim_mappings] in config-platform-api.toml — the two must agree.
# Must stay the last table under [oidc]: plain [oidc] keys placed below this header
# would land in [oidc.claim_mappings] instead.
[oidc.claim_mappings]
organization_claim_name = "org_id"
org_name_claim_name     = "org_name"
org_handle_claim_name   = "org_handle"
```

The redirect URLs and the client secret are BFF settings and never reach the browser. The
redirect URLs are ordinary `config.toml` keys; the secret is referenced with an interpolation
token so the raw value never lands in the file.

> `[oidc] redirect_url` must exactly match the authorized redirect URL registered in section 2.
> A missing client secret fails startup — see [Configuration → Secrets](../configuration.md#secrets).

---

## Claim Flow Summary

```
Asgardeo token
  ├── sub          → user identity
  ├── org_id       → organization UUID  (→ organization_claim_name in Platform API)
  ├── org_name     → org display name   (→ org_name_claim_name in Platform API)
  ├── org_handle   → org slug           (→ org_handle_claim_name in Platform API)
  └── scope        → space-separated ap:* scopes validated by Platform API
```

The claim names must be consistent across all three places:
- Asgardeo token mapper output
- `[oidc.claim_mappings]` in `config.toml`
- `*_claim_name` in Platform API `[auth.idp.claim_mappings]`
