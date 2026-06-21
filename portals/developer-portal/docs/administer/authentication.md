# Authentication

The Developer Portal supports two authentication modes, controlled by whether `identityProvider.clientId` is set in `config.yaml`.

## Modes

| Mode | When to use | Configured by |
|------|-------------|---------------|
| **Local (Platform API JWT)** | Development — no external IDP required | Leave `identityProvider.clientId` empty |
| **OIDC (External IDP)** | Production — delegates identity to Asgardeo, Keycloak, or any OIDC-compliant IDP | Set `identityProvider.clientId` |

---

## Login Flow

**IDP mode** (`clientId` is set): clicking Login on any portal page redirects the user directly to the IDP's authorization endpoint — no intermediate login page is shown. After the user authenticates at the IDP, they are returned to the page they originally requested.

**Local auth mode** (`clientId` is empty): clicking Login shows a username/password form. Credentials are validated against the Platform API.

---

## OIDC Mode Configuration

### `identityProvider` fields

| Field | Env var | Required | Description |
|-------|---------|----------|-------------|
| `name` | `DP_IDENTITYPROVIDER_NAME` | No | Friendly name used in logs (default: `oauth2`) |
| `issuer` | `DP_IDENTITYPROVIDER_ISSUER` | Yes | IDP token issuer URL — used for issuer claim verification |
| `authorizationURL` | `DP_IDENTITYPROVIDER_AUTHORIZATIONURL` | Yes | OAuth2 authorization endpoint |
| `tokenURL` | `DP_IDENTITYPROVIDER_TOKENURL` | Yes | OAuth2 token endpoint |
| `userInfoURL` | `DP_IDENTITYPROVIDER_USERINFOURL` | No | OIDC userinfo endpoint |
| `clientId` | `DP_IDENTITYPROVIDER_CLIENTID` | Yes | OAuth2 client ID |
| `clientSecret` | `DP_IDENTITYPROVIDER_CLIENTSECRET` | No* | Client secret for confidential clients (Traditional Web App, Keycloak). Leave empty for PKCE-only public clients. |
| `audience` | `DP_IDENTITYPROVIDER_AUDIENCE` | No | JWT `aud` claim to verify — typically the `clientId`. Leave empty to skip audience check. |
| `callbackURL` | `DP_IDENTITYPROVIDER_CALLBACKURL` | Yes | OAuth2 redirect URI — must be registered in the IDP. Pattern: `https://<domain>/<orgName>/callback` |
| `scope` | `DP_IDENTITYPROVIDER_SCOPE` | No | Space-separated OIDC scopes to request (default: `openid profile email`) |
| `logoutURL` | `DP_IDENTITYPROVIDER_LOGOUTURL` | No | IDP logout endpoint — used for end-session redirect |
| `logoutRedirectURI` | `DP_IDENTITYPROVIDER_LOGOUTREDIRECTURI` | No | Post-logout redirect back to the portal |
| `jwksURL` | `DP_IDENTITYPROVIDER_JWKSURL` | No* | JWKS endpoint for token signature verification. Either `jwksURL` or `certificate` is required. |
| `certificate` | `DP_IDENTITYPROVIDER_CERTIFICATE` | No* | X.509 certificate (PEM) as alternative to JWKS |
| `tokenRefreshTimeoutMs` | `DP_IDENTITYPROVIDER_TOKENREFRESHTIMEOUTMS` | No | Token refresh timeout in ms (default: `10000`) |

### Claim mapping and role fields (under `identityProvider` in `config.yaml`)

These tell the portal how to read user identity and roles from the IDP token. They live inside the `identityProvider:` block.

| Field | Env var | Default | Description |
|-------|---------|---------|-------------|
| `orgIDClaim` | `DP_IDENTITYPROVIDER_ORGIDCLAIM` | `organization.uuid` | JWT claim for the organization UUID. Asgardeo B2B uses `org_id`. Supports dot-notation (e.g. `org.id`). |
| `roleClaim` | `DP_IDENTITYPROVIDER_ROLECLAIM` | `roles` | JWT claim for the user's roles. Supports dot-notation (e.g. `realm_access.roles` for Keycloak). |
| `groupsClaim` | `DP_IDENTITYPROVIDER_GROUPSCLAIM` | `groups` | JWT claim for groups |
| `adminRole` | `DP_IDENTITYPROVIDER_ADMINROLE` | `admin` | Role value that grants portal admin access |
| `superAdminRole` | `DP_IDENTITYPROVIDER_SUPERADMINROLE` | `superAdmin` | Role value that grants portal super-admin access |
| `subscriberRole` | `DP_IDENTITYPROVIDER_SUBSCRIBERROLE` | `Internal/subscriber` | Role value for standard subscribers |
| `fidp` | — | `{}` | Map of `?fidp=<key>` query param values to IDP identifiers for federated login hints |

> Claim names can also be overridden per-organization in the database (via the admin API), allowing different orgs to use different IDPs or claim structures.

---

## Local Auth Mode

When `identityProvider.clientId` is empty, the portal uses a built-in login form and authenticates against the Platform API.

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
3. The IDP issues an org-scoped token. On every authenticated request, the portal checks that the token's org claim (`orgIDClaim`) matches the org's `ORGANIZATION_IDENTIFIER`. A mismatch returns a 401.

**User flow with multiple orgs:**

- Public pages are always accessible — no org check is performed.
- Protected pages (applications, subscriptions, API keys) require a token whose org claim matches the org being accessed.
- If a user navigates from Org A to Org B's protected pages while logged in as Org A, they see a 401. They must log out and log in again on Org B.

> The mechanism for passing the org identifier to the IDP is IDP-specific. Asgardeo uses an `org=<identifier>` query parameter on the authorization URL. Other IDPs may use different approaches (e.g. tenant-specific realm URLs in Keycloak). The `ORGANIZATION_IDENTIFIER` mismatch check runs regardless of IDP type.

---

## Keycloak Example

```yaml
identityProvider:
  name: "Keycloak"
  issuer: "https://keycloak.example.com/realms/myrealm"
  authorizationURL: "https://keycloak.example.com/realms/myrealm/protocol/openid-connect/auth"
  tokenURL: "https://keycloak.example.com/realms/myrealm/protocol/openid-connect/token"
  userInfoURL: "https://keycloak.example.com/realms/myrealm/protocol/openid-connect/userinfo"
  clientId: "devportal"
  clientSecret: "<client-secret>"        # env: DP_IDENTITYPROVIDER_CLIENTSECRET
  audience: "devportal"
  callbackURL: "https://<your-domain>/default/callback"
  logoutURL: "https://keycloak.example.com/realms/myrealm/protocol/openid-connect/logout"
  logoutRedirectURI: "https://<your-domain>/default"
  jwksURL: "https://keycloak.example.com/realms/myrealm/protocol/openid-connect/certs"
  scope: "openid profile email"
  orgIDClaim: "organization"          # custom claim — add via Keycloak protocol mapper
  roleClaim: "realm_access.roles"     # Keycloak nests realm roles here
```

**Keycloak setup steps:**
1. Create a Confidential client named `devportal`
2. Set redirect URI to `https://<your-domain>/<orgName>/callback`
3. Enable PKCE (set `PKCE Code Challenge Method` to `S256`)
4. Copy the client secret
5. Add a custom protocol mapper for your organization UUID claim (`orgIDClaim`)
6. Realm roles are exposed at `realm_access.roles` — configure `roleClaim` accordingly

---

## Generic OIDC

Any OIDC-compliant IDP works. You need to set:
- `authorizationURL`, `tokenURL`, `userInfoURL` from the IDP's `.well-known/openid-configuration`
- `jwksURL` from `jwks_uri` in the discovery document
- `issuer` from `issuer` in the discovery document
- `clientId` (and `clientSecret` for confidential clients)
- `callbackURL` registered with the IDP

---

## Guides

- [Asgardeo Setup](asgardeo-setup.md) — end-to-end production walkthrough for WSO2 Asgardeo
