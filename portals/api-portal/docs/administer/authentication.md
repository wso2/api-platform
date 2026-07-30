# Authentication

The API Portal supports two authentication modes, controlled by `auth.mode` in `config.toml`.

## Modes

| Mode | When to use | Configured by |
|------|-------------|---------------|
| **Local (Platform API JWT)** | Development — no external IDP required | `auth.mode = "local"` |
| **OIDC (External IDP)** | Production — delegates identity to Asgardeo, Keycloak, or any OIDC-compliant IDP | `auth.mode = "idp"` |

---

## Login Flow

**IDP mode** (`auth.mode = "idp"`): clicking Login on any portal page redirects the user directly to the IDP's authorization endpoint — no intermediate login page is shown. After the user authenticates at the IDP, they are returned to the page they originally requested.

**Local auth mode** (`auth.mode = "local"`): clicking Login shows a username/password form. Credentials are validated against the Platform API.

---

## OIDC Mode Configuration

### `idp` fields

Set these in `config.toml` under `[api_portal.auth.idp]`. None of them is settable from
the environment as shipped — `configs/config.toml` references no `APIP_AP_IDP_*`
variable, and there is no automatic override layer, so a variable no key names does
nothing. Add a `{{ env "..." }}` token to the key yourself if you need one (the client
secret is the usual candidate).

| Field (TOML) | Required | Description |
| ------- | ---------- | ------------- |
| `name` | No | Friendly name used in logs (default: `oauth2`) |
| `issuer` | Yes | IDP token issuer URL — used for issuer claim verification |
| `authorization_url` | Yes | OAuth2 authorization endpoint |
| `token_url` | Yes | OAuth2 token endpoint |
| `user_info_url` | No | OIDC userinfo endpoint |
| `client_id` | Yes | OAuth2 client ID |
| `client_secret` | No* | Client secret for confidential clients (Traditional Web App, Keycloak). Leave empty for PKCE-only public clients. |
| `audience` | No | JWT `aud` claim to verify — typically the `client_id`. Leave empty to skip audience check. |
| `callback_url` | Yes | OAuth2 redirect URI — must be registered in the IDP. Pattern: `https://<domain>/<orgName>/callback` |
| `scope` | No | Space-separated OIDC scopes to request (default: `openid profile email`) |
| `logout_url` | No | IDP logout endpoint — used for end-session redirect |
| `logout_redirect_uri` | No | Post-logout redirect back to the portal |
| `jwks_url` | No* | JWKS endpoint for token signature verification. Either `jwks_url` or `certificate` is required. |
| `certificate` | No* | X.509 certificate (PEM) as alternative to JWKS |
| `token_refresh_timeout_ms` | No | Token refresh timeout in ms (default: `10000`) |

### Claim mapping (`[auth.claim_mappings]` in `config.toml`)

These tell the portal which token claim carries each field. They sit under `auth`, not
under `auth.idp`, because the same mapping applies in local auth mode.

| Field (TOML) | Default | Description |
|-------|---------|-------------|
| `auth.claim_mappings.organization` | `org_name` | JWT claim for the organization. Asgardeo B2B uses `org_name`. Supports dot-notation (e.g. `org.id`). |
| `auth.claim_mappings.roles` | `roles` | JWT claim for the user's roles. Supports dot-notation (e.g. `realm_access.roles` for Keycloak). |
| `auth.claim_mappings.groups` | `groups` | JWT claim for groups |
| `auth.idp.fidp` | `{}` | Map of `?fidp=<key>` query param values to IDP identifiers for federated login hints |

> Claim names can also be overridden per-organization in the database (via the admin API), allowing different orgs to use different IDPs or claim structures.

Which role name grants each portal access tier is **authorization**, not authentication —
see [Authorization](#authorization) below. It used to live here as `[idp.roles]`.

---

## Local Auth Mode

When `auth.mode` is `"local"`, the portal uses a built-in login form and authenticates against the Platform API.

Requirements:
- `auth.local.platform_api_url` must be set (e.g. `https://platform-api:9243`)
- `auth.local.public_key_path` must point to the Platform API's RS256 public key PEM (SPKI) — the counterpart to its `auth.jwt.private_key`. Bearer-token requests fail closed without it.
- Users and passwords are managed in the Platform API's config

This mode is intended for development and local testing only.

---

## Authorization

Authentication settles *who* a caller is; authorization settles *what they may do*. It is
configured in `[api_portal.auth.authorization]` — a section of its own, outside both
`auth.local` and `auth.idp`, because a token carries the same roles claim whether the
portal verified it against a JWKS endpoint or against the Platform API's public key.

Two surfaces are governed separately:

| Surface | Governed by | Decided from |
|---|---|---|
| REST API (`/api/v0.9`) | `enabled` + `mode` | the `dp:*` scopes each operation declares |
| Portal pages (applications, api-keys, subscriptions, settings) | `page_role_validation` + `portal_roles` | the caller's role tier |

### Settings

| Field (TOML) | Default | Description |
|---|---|---|
| `auth.authorization.enabled` | `true` | Enforce each REST operation's declared scopes. `false` lets any *authenticated* caller through (authentication still applies) and logs a startup warning — development only. |
| `auth.authorization.mode` | `role` | `scope` \| `role`. Validated at startup even when `enabled = false`, so a typo surfaces when it is written rather than when enforcement is switched back on. |
| `auth.authorization.role_to_scope_mapping` | `./resources/role-to-scope-mapping.yaml` | Path to the YAML grant table. **Required** in `role` mode, which is the default — hence a real default rather than empty. Points at the copy baked into the image; `docker-compose.yaml` overrides it to the mounted, editable copy. Loaded and validated at startup whenever it is set. |
| `auth.authorization.page_role_validation` | `false` | Require the caller's roles claim to name the tier a page demands. |
| `auth.authorization.portal_roles.admin` | `admin` | Role name granting the admin tier (portal settings, plus everything the subscriber tier allows). |
| `auth.authorization.portal_roles.subscriber` | `Internal/subscriber` | Role name granting the subscriber tier (own applications, api-keys, subscriptions). |

### `role` mode (default)

The scope claim is ignored entirely and the roles claim (`auth.claim_mappings.roles`) is
expanded through the grant table instead. This is the default because it works for every
issuer: an external IDP emits the roles its estate is organized around and has no reason
to mint `dp:*` scopes, and the Platform API mints its own `ap_*` role names, which the
shipped table aliases so the local-auth quickstart works unchanged.

### `scope` mode

Effective scopes are the token's own `scope` claim. Switch to this when the issuer mints
`dp:*` scopes directly:

- **Asgardeo**, set up via `production/scripts/register_asgardeo_scopes.sh` — it registers every `dp:*` scope as an API resource, and you then attach them to an Asgardeo role (see [asgardeo-setup.md](asgardeo-setup.md)). Asgardeo is the role→scope mapper in that setup, so the portal-side grant table is redundant.
- **The Platform API in local auth mode**, which expands a file user's roles into the `scope` claim of the token it issues.

Note that in `scope` mode an IDP *browser session* still bypasses the per-operation check
(`preauthorized`), because the OIDC client would otherwise have to request all `dp:*`
scopes; the only authorization left for it is the page role gate. `role` mode enforces
the operation-level check for those sessions too.

### Multiple roles, and unknown ones

Multiple roles union their scopes — most-permissive wins, duplicates collapsed — and a
role absent from the table grants nothing, so the failure mode is a denied request rather
than an unintended grant.

Ignoring rather than merging the scope claim in `role` mode is deliberate: a caller must
not be able to widen a role's grant by asking their IDP for extra scope values.

```toml
[api_portal.auth]
mode = "idp"

[api_portal.auth.claim_mappings]
roles = "realm_access.roles"   # Keycloak's nested shape; dot-notation is resolved

[api_portal.auth.authorization]
mode                  = "role"
role_to_scope_mapping = "/etc/api-portal/role-to-scope-mapping.yaml"
page_role_validation  = true

# Drive page tiers from the same roles as REST authorization. The shipped defaults are
# the legacy IDP values ("admin", "Internal/subscriber"), so set these explicitly if you
# want pages gated by the grant table's roles.
[api_portal.auth.authorization.portal_roles]
admin      = "dp_admin"
subscriber = "dp_subscriber"
```

### The grant table

`resources/role-to-scope-mapping.yaml` is the shipped sample.

```yaml
roles:
  - name: dp_subscriber
    scopes:
      - dp:application:manage
      - dp:subscription:manage
      - dp:api:read
```

It defines **two** roles, matching the two personas this portal recognises:

| Role | For |
|---|---|
| `dp_admin` | Runs the portal — content and theme, the API/MCP catalogue, views, labels, subscription plans, key managers, webhook subscribers, and every application and subscription in the organization |
| `dp_subscriber` | Consumes APIs — owns their own applications, subscriptions and keys, and browses the catalogue |

The page gate has exactly these two tiers. An earlier `superAdmin` tier gated the
multi-organization pages of the old devportal (`/portal`, `/devportal`); those routes are
not served here, so it guarded nothing and has been removed. Publisher, operator and
viewer are platform-side personas and live in
`platform-api/resources/role-to-scope-mapping.yaml` instead — API publishing reaches this
portal through the Platform API, the management portal or the `ap` CLI, not through a
human browsing here.

Add your own roles freely; nothing in the file is special-cased in code, and the
validation below applies to whatever you add.

The Platform API's grant table names its roles `ap_*`. The two files are read by
different components, so the names need not agree — but to drive both from one set of IDP
groups, either rename these entries to match theirs or add an alias entry carrying the
same scope list:

```yaml
  - name: ap_admin        # alias so one IDP group covers platform-api and this portal
    scopes: [ ... same list as dp_admin ... ]
```

Startup validation is fail-closed:

- Every `dp:*` scope must be declared in `docs/api-portal-openapi-spec-v0.9.yaml`. An undeclared one aborts startup — otherwise the role would authenticate fine and be denied every request.
- Scopes in another component's namespace (`ap:*`, for a table shared with the Platform API) are checked for well-formedness only. This portal mints no `ap:*` scope and enforces none, so it can neither confirm nor deny their existence.
- A duplicate role name is rejected rather than last-wins, and every problem in the file is reported at once.

The file is mounted rather than baked into the image, so operators can edit what a role
grants. It requires a restart to take effect.

### Migrating from the previous shape

Both retired keys govern authorization, and a retired key is *ignored* — so the effective
setting would silently become the default rather than what the file says. Startup fails
instead, naming the replacement:

```toml
# before
[api_portal.auth]
role_validation = true

[api_portal.auth.idp.roles]
admin = "admin"

# after
[api_portal.auth.authorization]
page_role_validation = true

[api_portal.auth.authorization.portal_roles]
admin = "admin"
```

Note that `role_validation` maps to `page_role_validation`, **not** to `enabled`. The two
are not the same switch: `role_validation` only ever gated portal pages, while REST scope
enforcement was unconditional. Renaming it to `enabled` would mean a config that had
`role_validation = false` silently turned off REST scope enforcement that was previously
active.

---

## Multi-Organization Isolation

When multiple devportal organizations share one IDP, the portal enforces per-org isolation using the `ORGANIZATION_IDENTIFIER` field on each organization (stored in the database, set via the admin API).

**How it works:**

1. Each devportal org has an `ORGANIZATION_IDENTIFIER` — the IDP-side identifier for that org (e.g. an Asgardeo sub-org handle).
2. When a user clicks Login, the portal looks up the org's `ORGANIZATION_IDENTIFIER` and passes it to the IDP in the authorization request, scoping the login session to that org.
3. The IDP issues an org-scoped token. On every authenticated request, the portal checks that the token's org claim (`auth.claim_mappings.organization`) matches the org's `ORGANIZATION_IDENTIFIER`. A mismatch returns a 403.

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
# Write the secret as a token rather than a literal — the variable works because
# this key references it, not automatically. {{ file }} is better still.
client_secret = '{{ env "APIP_AP_AUTH_IDP_CLIENT_SECRET" }}'
audience = "devportal"
callback_url = "https://<your-domain>/default/callback"
logout_url = "https://keycloak.example.com/realms/myrealm/protocol/openid-connect/logout"
logout_redirect_uri = "https://<your-domain>/default"
jwks_url = "https://keycloak.example.com/realms/myrealm/protocol/openid-connect/certs"
scope = "openid profile email"

[api_portal.auth.claim_mappings]
organization = "organization"       # custom claim — add via Keycloak protocol mapper
roles        = "realm_access.roles" # Keycloak nests realm roles here
```

**Keycloak setup steps:**
1. Create a Confidential client named `devportal`
2. Set redirect URI to `https://<your-domain>/<orgName>/callback`
3. Enable PKCE (set `PKCE Code Challenge Method` to `S256`)
4. Copy the client secret
5. Add a custom protocol mapper for your organization UUID claim (`auth.claim_mappings.organization`)
6. Realm roles are exposed at `realm_access.roles` — configure `auth.claim_mappings.roles` accordingly

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
