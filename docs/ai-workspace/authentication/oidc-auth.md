# OIDC Authentication

In `oidc` mode, AI Workspace delegates authentication to an external OIDC-compliant identity provider (IDP). The IDP issues JWT access tokens that carry organization identity as custom claims. Both the AI Workspace UI and the Platform API must be configured to agree on which claims carry which values.

## IDP Requirements

Any OIDC-compliant IDP works, subject to the following requirements:

| Requirement | Details |
|-------------|---------|
| OIDC Discovery | IDP must expose `/.well-known/openid-configuration` at the authority URL |
| JWT access tokens | Access tokens must be JWTs (not opaque tokens) |
| JWKS endpoint | IDP must expose a JWKS endpoint so Platform API can verify token signatures |
| Custom claims | Tokens must carry `org_id`, `org_name`, and `org_handle` claims (names are configurable) |
| SPA client type | The AI Workspace app must be registered as a Public/SPA client (PKCE flow, no client secret) |

Tested IDPs: [Asgardeo](asgardeo-setup.md), Keycloak, Auth0, Okta.

## Configuration

### AI Workspace (`configs/config.toml`)

```toml
domain               = "app.example.com"
auth_mode            = "oidc"

# IDP issuer URL — OIDC discovery doc fetched from {oidc_authority}/.well-known/openid-configuration
oidc_authority       = "https://idp.example.com/realms/my-realm"

# SPA client ID registered in your IDP
oidc_client_id       = "ai-workspace"

# JWT claim names for organization identity
# These must match the claim_mappings in Platform API config (see below)
oidc_org_id_claim    = "org_id"
oidc_org_name_claim  = "org_name"
oidc_org_handle_claim = "org_handle"

platform_api_base_url = "https://api.example.com/api/v1"
controlplane_host    = "api.example.com"
```

Redirect URIs are derived from `domain` automatically:
- Sign-in callback: `https://<domain>/signin`
- Post-logout: `https://<domain>/login`

Register both URLs as allowed redirect URIs in your IDP application.

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
audience = ["ai-workspace"]   # must match oidc_client_id

# Map IDP-specific claim names to Platform API's expected fields
# These must match oidc_org_*_claim values in config.toml above
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
- Verify the redirect URI `https://<domain>/signin` is registered in the IDP.

**Platform API returns 401**
- Check that `jwks_url` and `issuer` in Platform API config match the IDP's discovery doc values.
- Check that `audience` matches the `oidc_client_id` registered for the SPA.
- Ensure `organization_claim_name` in Platform API matches `oidc_org_id_claim` in AI Workspace config.

**"Organization not found" error**
- The `org_id` claim in the token does not match any organization in Platform API's database.
- For Asgardeo, ensure sub-organization users have the shared `ap_admin` role assigned.
