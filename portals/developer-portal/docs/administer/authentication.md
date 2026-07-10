# Authentication

The Developer Portal supports two authentication modes, controlled by whether `idp.clientId` is set in `config.toml`.

## Modes

| Mode | When to use | Configured by |
|------|-------------|---------------|
| **Local (Platform API JWT)** | Development — no external IDP required | Leave `idp.clientId` empty |
| **OIDC (External IDP)** | Production — delegates identity to Asgardeo, Keycloak, or any OIDC-compliant IDP | Set `idp.clientId` |

---

## Login Flow

**IDP mode** (`clientId` is set): clicking Login on any portal page redirects the user directly to the IDP's authorization endpoint — no intermediate login page is shown. After the user authenticates at the IDP, they are returned to the page they originally requested.

**Local auth mode** (`clientId` is empty): clicking Login shows a username/password form. Credentials are validated against the Platform API.

---

## OIDC Mode Configuration

### `idp` fields

| Field (TOML) | Env var | Required | Description |
|-------|---------|----------|-------------|
| `name` | `APIP_DP_IDP_NAME` | No | Friendly name used in logs (default: `oauth2`) |
| `issuer` | `APIP_DP_IDP_ISSUER` | Yes | IDP token issuer URL — used for issuer claim verification |
| `authorization_url` | `APIP_DP_IDP_AUTHORIZATIONURL` | Yes | OAuth2 authorization endpoint |
| `token_url` | `APIP_DP_IDP_TOKENURL` | Yes | OAuth2 token endpoint |
| `user_info_url` | `APIP_DP_IDP_USERINFOURL` | No | OIDC userinfo endpoint |
| `client_id` | `APIP_DP_IDP_CLIENTID` | Yes | OAuth2 client ID |
| `client_secret` | `APIP_DP_IDP_CLIENTSECRET` | No* | Client secret for confidential clients (Traditional Web App, Keycloak). Leave empty for PKCE-only public clients. |
| `audience` | `APIP_DP_IDP_AUDIENCE` | No | JWT `aud` claim to verify — typically the `client_id`. Leave empty to skip audience check. |
| `callback_url` | `APIP_DP_IDP_CALLBACKURL` | Yes | OAuth2 redirect URI — must be registered in the IDP. Pattern: `https://<domain>/<orgName>/callback` |
| `scope` | `APIP_DP_IDP_SCOPE` | No | Space-separated OIDC scopes to request (default: `openid profile email`) |
| `logout_url` | `APIP_DP_IDP_LOGOUTURL` | No | IDP logout endpoint — used for end-session redirect |
| `logout_redirect_uri` | `APIP_DP_IDP_LOGOUTREDIRECTURI` | No | Post-logout redirect back to the portal |
| `jwks_url` | `APIP_DP_IDP_JWKSURL` | No* | JWKS endpoint for token signature verification. Either `jwks_url` or `certificate` is required. |
| `certificate` | `APIP_DP_IDP_CERTIFICATE` | No* | X.509 certificate (PEM) as alternative to JWKS |
| `token_refresh_timeout_ms` | `APIP_DP_IDP_TOKENREFRESHTIMEOUTMS` | No | Token refresh timeout in ms (default: `10000`) |

### Claim mapping and role fields (`[idp.claims]` / `[idp.roles]` in `config.toml`)

These tell the portal how to read user identity and roles from the IDP token.

| Field (TOML) | Env var | Default | Description |
|-------|---------|---------|-------------|
| `idp.claims.org_id` | `APIP_DP_IDP_CLAIMS_ORGID` | `org_name` | JWT claim for the organization UUID. Asgardeo B2B uses `org_name`. Supports dot-notation (e.g. `org.id`). |
| `idp.claims.role` | `APIP_DP_IDP_CLAIMS_ROLE` | `roles` | JWT claim for the user's roles. Supports dot-notation (e.g. `realm_access.roles` for Keycloak). |
| `idp.claims.groups` | `APIP_DP_IDP_CLAIMS_GROUPS` | `groups` | JWT claim for groups |
| `idp.roles.admin` | `APIP_DP_IDP_ROLES_ADMIN` | `admin` | Role value that grants portal admin access |
| `idp.roles.super_admin` | `APIP_DP_IDP_ROLES_SUPERADMIN` | `superAdmin` | Role value that grants portal super-admin access |
| `idp.roles.subscriber` | `APIP_DP_IDP_ROLES_SUBSCRIBER` | `Internal/subscriber` | Role value for standard subscribers |
| `idp.fidp` | — | `{}` | Map of `?fidp=<key>` query param values to IDP identifiers for federated login hints |

> Claim names can also be overridden per-organization in the database (via the admin API), allowing different orgs to use different IDPs or claim structures.

---

## Local Auth Mode

When `idp.clientId` is empty, the portal uses a built-in login form and authenticates against the Platform API.

Requirements:
- `platformApi.baseUrl` must be set (e.g. `https://platform-api:9243`)
- Users and passwords are managed in the Platform API's config

This mode is intended for development and local testing only.

---

## Multi-Organization Isolation

When multiple devportal organizations share one IDP, the portal enforces per-org isolation using the `ORGANIZATION_IDENTIFIER` field on each organization (stored in the database, set via the admin API).

**How it works:**

1. Each devportal org has an `ORGANIZATION_IDENTIFIER` — the IDP-side identifier for that org (e.g. an Asgardeo sub-org handle).
2. When a user clicks Login, the portal looks up the org's `ORGANIZATION_IDENTIFIER` and passes it to the IDP in the authorization request, scoping the login session to that org.
3. The IDP issues an org-scoped token. On every authenticated request, the portal checks that the token's org claim (`idp.claims.org_id`) matches the org's `ORGANIZATION_IDENTIFIER`. A mismatch returns a 403.

**User flow with multiple orgs:**

- Public pages are always accessible — no org check is performed.
- Protected pages (applications, subscriptions, API keys) require a token whose org claim matches the org being accessed.
- If a user navigates from Org A to Org B's protected pages while logged in as Org A, they see a 403. They must log out and log in again on Org B.

> The mechanism for passing the org identifier to the IDP is IDP-specific. Asgardeo uses an `org=<identifier>` query parameter on the authorization URL. Other IDPs may use different approaches (e.g. tenant-specific realm URLs in Keycloak). The `ORGANIZATION_IDENTIFIER` mismatch check runs regardless of IDP type.

---

## Keycloak Example

```toml
[idp]
name = "Keycloak"
issuer = "https://keycloak.example.com/realms/myrealm"
authorization_url = "https://keycloak.example.com/realms/myrealm/protocol/openid-connect/auth"
token_url = "https://keycloak.example.com/realms/myrealm/protocol/openid-connect/token"
user_info_url = "https://keycloak.example.com/realms/myrealm/protocol/openid-connect/userinfo"
client_id = "devportal"
client_secret = "<client-secret>"        # env: APIP_DP_IDP_CLIENTSECRET
audience = "devportal"
callback_url = "https://<your-domain>/default/callback"
logout_url = "https://keycloak.example.com/realms/myrealm/protocol/openid-connect/logout"
logout_redirect_uri = "https://<your-domain>/default"
jwks_url = "https://keycloak.example.com/realms/myrealm/protocol/openid-connect/certs"
scope = "openid profile email"

[idp.claims]
org_id = "organization"          # custom claim — add via Keycloak protocol mapper
role = "realm_access.roles"      # Keycloak nests realm roles here
```

**Keycloak setup steps:**
1. Create a Confidential client named `devportal`
2. Set redirect URI to `https://<your-domain>/<orgName>/callback`
3. Enable PKCE (set `PKCE Code Challenge Method` to `S256`)
4. Copy the client secret
5. Add a custom protocol mapper for your organization UUID claim (`idp.claims.org_id`)
6. Realm roles are exposed at `realm_access.roles` — configure `idp.claims.role` accordingly

---

## Generic OIDC

Any OIDC-compliant IDP works. You need to set:
- `authorization_url`, `token_url`, `user_info_url` from the IDP's `.well-known/openid-configuration`
- `jwks_url` from `jwks_uri` in the discovery document
- `issuer` from `issuer` in the discovery document
- `client_id` (and `client_secret` for confidential clients)
- `callback_url` registered with the IDP

---

## Guides

- [Asgardeo Setup](asgardeo-setup.md) — end-to-end production walkthrough for WSO2 Asgardeo
