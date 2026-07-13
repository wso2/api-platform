# OIDC Authentication

In `oidc` mode, AI Workspace delegates authentication to an external OIDC-compliant identity provider (IDP). The AI Workspace is served by a **BFF that acts as a confidential OIDC client**: it runs the authorization-code + PKCE exchange on the back channel, holds the client secret and tokens in a server-side session, and the browser never sees a token. The IDP issues JWT access tokens that carry organization identity as custom claims. Both the BFF and the Platform API must be configured to agree on which claims carry which values.

## IDP Requirements

Any OIDC-compliant IDP works, subject to the following requirements:

| Requirement | Details |
|-------------|---------|
| OIDC Discovery | IDP must expose `/.well-known/openid-configuration` at the authority URL |
| JWT access tokens | Access tokens must be JWTs (not opaque tokens) |
| JWKS endpoint | IDP must expose a JWKS endpoint so Platform API can verify token signatures |
| Custom claims | Tokens must carry `org_id`, `org_name`, and `org_handle` claims (names are configurable) |
| Confidential client | The AI Workspace app must be registered as a **confidential** client (authorization-code + PKCE, **with** a client secret held by the BFF) — not a public/SPA client |
| Redirect URI | The BFF callback `https://<domain>/api/auth/callback` must be an authorized redirect URI |

Tested IDPs: [Asgardeo](asgardeo-setup.md), Keycloak, Auth0, Okta.

## Configuration

### AI Workspace (`configs/config.toml`)

```toml
domain            = "app.example.com"
auth_mode         = "oidc"
controlplane_host = "api.example.com"

[platform_api]
# The upstream the BFF proxies to (an origin — the API paths are appended by the proxy).
url = "https://api.example.com"

[oidc]
# IDP issuer URL — the discovery doc is fetched from {authority}/.well-known/openid-configuration
authority = "https://idp.example.com/realms/my-realm"

# Confidential client ID registered in your IDP
client_id = "ai-workspace"

# JWT claim names for organization identity. This table mirrors
# [auth.idp.claim_mappings] in the Platform API config (below) key for key — both
# services read the same claims out of the same token, so the two must agree.
[oidc.claim_mappings]
organization_claim_name = "org_id"
org_name_claim_name     = "org_name"
org_handle_claim_name   = "org_handle"
```

The redirect URLs and the client secret are BFF settings — they are **never sent to the browser**.
The redirect URLs are ordinary `config.toml` keys; the secret is *referenced* by the config rather
than written into it, so the raw value never lands in a committed file:

```toml
[oidc]
redirect_url             = "https://<domain>/api/auth/callback"   # the BFF callback
post_logout_redirect_url = "https://<domain>/login"

# Preferred: read from a mounted secret file (the value never enters the environment).
client_secret = '{{ file "/secrets/ai-workspace/oidc_client_secret" }}'
```

For a simpler local setup, swap the token for `'{{ env "APIP_AIW_OIDC_CLIENT_SECRET" }}'` and keep
the value in a git-ignored `.env`. The key must carry one token or the other — a variable set with
no token to read it is ignored. Either token fails closed: a missing secret aborts startup rather
than yielding an empty credential. See [Configuration → Secrets](../configuration.md#secrets).

`[oidc] redirect_url` (the BFF callback `/api/auth/callback`) and `post_logout_redirect_url`
must be registered as allowed redirect URIs in your IDP application. The redirect is **not** the
SPA `/signin` route — the BFF, not the browser, completes the code exchange.

### Platform API (`configs/config-platform-api.toml`)

```toml
# Disable local JWT signing — tokens come from the IDP
[auth.jwt]
enabled = false

# Enable JWKS-based validation of IDP tokens
[auth.idp]
enabled  = true
name     = "my-idp"
jwks_url = "https://idp.example.com/realms/my-realm/protocol/openid-connect/certs"
issuer   = ["https://idp.example.com/realms/my-realm"]
audience = ["ai-workspace"]   # must match [oidc] client_id

# Map IDP-specific claim names to Platform API's expected fields
# These must match the [oidc.claim_mappings] values in config.toml above
[auth.idp.claim_mappings]
organization_claim_name = "org_id"
org_name_claim_name     = "org_name"
org_handle_claim_name   = "org_handle"

# Disable file-based auth
[auth.file_based]
enabled = false
```

Optional claim overrides (defaults shown):

```toml
[auth.idp.claim_mappings]
user_id_claim_name  = "sub"
username_claim_name = "username"
email_claim_name    = "email"
scope_claim_name    = "scope"
```

Validation mode (default `scope`):

```toml
[auth.idp]
validation_mode = "scope"   # "scope" or "role"
```

## Required JWT Claims

Each access token issued to an AI Workspace user must contain:

| Claim | Example value | Description |
|-------|--------------|-------------|
| `sub` | `user-uuid` | User identity |
| `org_id` | `org-uuid` | Organization UUID (claim name configurable) |
| `org_name` | `Acme Corp` | Human-readable org name |
| `org_handle` | `acme-corp` | URL-safe org slug |
| `scope` | `ap:gateway:read ap:provider:write ...` | Space-separated `ap:*` scopes |

The full list of `ap:*` scopes required is defined in [`src/config.env.ts`](../../src/config.env.ts).

## Scopes

The AI Workspace requests a set of `ap:*` scopes from the IDP when the user logs in. The Platform API validates that the access token contains the required scope for each API call.

You must register the `ap:*` scopes as an API resource in your IDP and grant them to the AI Workspace application. For Asgardeo, a helper script automates this — see [Asgardeo Setup](asgardeo-setup.md#14-system-application-for-scope-registration).


## Troubleshooting

**Users see a blank screen or redirect loop after login**
- Verify `domain` in `config.toml` matches the actual host:port in the browser.
- Verify the redirect URI `https://<domain>/api/auth/callback` (the BFF callback) is registered
  in the IDP and matches `[oidc] redirect_url`.

**Token endpoint rejects the BFF with `unauthorized_client` / "not authorized to use the requested grant type"**
- The app is registered as a public/SPA client. Re-register it as a **confidential** client
  (authorization-code + refresh-token grants, PKCE) and set `[oidc] client_secret`.

**Platform API returns 401**
- Check that `jwks_url` and `issuer` in Platform API config match the IDP's discovery doc values.
- Check that `audience` matches the `[oidc] client_id` of the confidential application.
- Ensure `organization_claim_name` matches on both sides — `[auth.idp.claim_mappings]` in the Platform API and `[oidc.claim_mappings]` in AI Workspace.

**"Organization not found" error**
- The `org_id` claim in the token does not match any organization in Platform API's database.
- For Asgardeo, ensure sub-organization users have the shared `ap_admin` role assigned.
