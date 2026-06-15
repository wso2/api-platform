# Manage Organizations

An organization is the top-level multi-tenant unit in the Developer Portal. Each organization has its own API catalog, applications, subscriptions, and branding. The organization handle appears in every portal URL (`/<orgHandle>/views/<viewName>`).

## Create an Organization

Create an `org.yaml` file using the Organization manifest format:

```yaml
# org.yaml
apiVersion: devportal.api-platform.wso2.com/v1
kind: Organization

metadata:
  name: acme                     # orgHandle — used in portal URLs

spec:
  displayName: Acme Corp
  organizationIdentifier: ACME   # value of the org claim for this org's users
  roleClaimName: roles
  groupsClaimName: groups
  organizationClaimName: organization
  adminRole: admin
  subscriberRole: subscriber
  superAdminRole: superAdmin
  businessOwner: Platform Team
  businessOwnerContact: "+1-202-555-0147"
  businessOwnerEmail: platform-team@acme.com

  labels:
    - name: default
      displayName: Default

  views:
    - name: default
      displayName: Default View
      labels:
        - default
```

```bash
curl -X POST http://localhost:3000/organizations \
  -H "Authorization: Bearer $TOKEN" \
  -F "organization=@org.yaml"
```

| Field | Required | Description |
|---|---|---|
| `metadata.name` | Yes | URL-safe org handle used in all portal URLs (becomes `orgHandle`) |
| `spec.displayName` | Yes | Human-friendly organization name shown in the portal UI |
| `spec.organizationIdentifier` | Yes | Expected value of the org claim for users in this organization |
| `spec.roleClaimName` | Yes | Name of the JWT claim that carries user roles |
| `spec.groupsClaimName` | Yes | Name of the JWT claim that carries user groups |
| `spec.organizationClaimName` | Yes | Name of the JWT claim that identifies the user's organization |
| `spec.adminRole` | Yes | Claim value that grants portal admin rights |
| `spec.subscriberRole` | Yes | Claim value that grants developer/subscriber access |
| `spec.superAdminRole` | Yes | Claim value that grants super-admin access across organizations |
| `spec.businessOwner` | No | Contact name for the organization owner |
| `spec.businessOwnerContact` | No | Business owner's phone or contact string |
| `spec.businessOwnerEmail` | No | Business owner's email address |
| `spec.labels` | No | Labels to create for this org. Defaults to a single `default` label if omitted |
| `spec.views` | No | Views to create for this org. Defaults to a single `default` view if omitted |
| `spec.identityProvider` | No | Inline IdP configuration (see [Identity Provider Configuration](#identity-provider-configuration)) |

After creation, the organization is accessible at `/<orgHandle>/views/<viewName>` once a view is created for it.

## List Organizations

```bash
curl http://localhost:3000/organizations -H "Authorization: Bearer $TOKEN"
```

## Update an Organization

```yaml
# org-update.yaml
apiVersion: devportal.api-platform.wso2.com/v1
kind: Organization

metadata:
  name: acme

spec:
  displayName: Acme Corporation
  businessOwner: New Owner
  businessOwnerEmail: new-owner@acme.com
```

```bash
curl -X PUT http://localhost:3000/organizations/{orgId} \
  -H "Authorization: Bearer $TOKEN" \
  -F "organization=@org-update.yaml"
```

## Delete an Organization

```bash
curl -X DELETE http://localhost:3000/organizations/{orgId} -H "Authorization: Bearer $TOKEN"
```

> **Warning:** Deleting an organization removes all of its views, APIs, subscriptions, and applications. This action is irreversible.

---

## Identity Provider Configuration

Each organization needs an identity provider (IdP) configured so that users can sign in via OAuth2/OIDC. The portal supports any standards-compliant OAuth2/OIDC provider (Asgardeo, Keycloak, Azure AD, Auth0, etc.).

### Add an IdP

```yaml
# idp.yaml
apiVersion: devportal.api-platform.wso2.com/v1
kind: IdentityProvider

metadata:
  name: Asgardeo

spec:
  issuer: https://api.asgardeo.io/t/myorg/oauth2/token
  authorizationURL: https://api.asgardeo.io/t/myorg/oauth2/authorize
  tokenURL: https://api.asgardeo.io/t/myorg/oauth2/token
  userInfoURL: https://api.asgardeo.io/t/myorg/oauth2/userinfo
  clientId: your-client-id
  callbackURL: http://localhost:3000/acme/callback
  scope: "openid profile email"
  logoutURL: https://api.asgardeo.io/t/myorg/oidc/logout
  logoutRedirectURI: http://localhost:3000/acme
  jwksURL: https://api.asgardeo.io/t/myorg/oauth2/jwks
```

```bash
curl -X POST http://localhost:3000/organizations/{orgId}/identityProvider \
  -H "Authorization: Bearer $TOKEN" \
  -F "identityProvider=@idp.yaml"
```

| Field | Description |
|---|---|
| `metadata.name` | Display name for the IdP (shown in admin UIs) |
| `spec.issuer` | Token issuer URL — used to validate the `iss` claim in tokens |
| `spec.authorizationURL` | OAuth2 authorization endpoint |
| `spec.tokenURL` | Token exchange endpoint |
| `spec.userInfoURL` | User info endpoint for fetching profile claims |
| `spec.clientId` | OAuth2 client ID registered with the IdP |
| `spec.callbackURL` | Redirect URI registered with the IdP — must be `<baseUrl>/<orgHandle>/callback` |
| `spec.scope` | Space-separated OAuth2 scopes to request (minimum: `openid profile`) |
| `spec.logoutURL` | IdP logout endpoint |
| `spec.logoutRedirectURI` | URI the IdP redirects back to after logout (typically `<baseUrl>/<orgHandle>`) |
| `spec.jwksURL` | JWKS endpoint for token signature validation |
| `spec.certificate` | Optional PEM certificate for environments that don't expose a JWKS endpoint |

> **Note:** The OAuth2 client secret is not stored in the IdP record. It is configured separately in `config.yaml` or via the environment variable `DP_CLIENT_SECRET`.

### Claim Mapping

The portal uses claims in the IdP token to identify the user's organization and role. These are set when [creating the organization](#create-an-organization) via `spec.roleClaimName`, `spec.groupsClaimName`, `spec.organizationClaimName`, `spec.organizationIdentifier`, `spec.adminRole`, `spec.subscriberRole`, and `spec.superAdminRole`.

Ensure your IdP is configured to include these claims in the ID token or userinfo response.

### Update an IdP

```yaml
# idp-update.yaml
apiVersion: devportal.api-platform.wso2.com/v1
kind: IdentityProvider

metadata:
  name: Asgardeo

spec:
  clientId: new-client-id
  callbackURL: http://localhost:3000/acme/callback
  logoutRedirectURI: http://localhost:3000/acme
```

```bash
curl -X PUT http://localhost:3000/organizations/{orgId}/identityProvider \
  -H "Authorization: Bearer $TOKEN" \
  -F "identityProvider=@idp-update.yaml"
```

### Remove an IdP

```bash
curl -X DELETE http://localhost:3000/organizations/{orgId}/identityProvider -H "Authorization: Bearer $TOKEN"
```

---

## Local Auth (Development Only)

For local development and first-time setup, the portal ships with a built-in username/password login form. Credentials are validated by a Platform API sidecar — the Developer Portal never handles raw passwords directly.

### How it works

1. The user submits the login form.
2. The Developer Portal forwards the credentials to the Platform API (`POST /api/portal/v1/auth/login`).
3. The Platform API verifies the bcrypt-hashed password and returns a signed JWT containing `dp:*` scopes.
4. The Developer Portal stores the JWT in the server-side session and uses the scopes for all subsequent authorization checks.

### Configuration

Users and their scopes are defined in `configs/config-platform-api.toml`. Copy the example file to get started:

```bash
cp configs/config-platform-api.toml.example configs/config-platform-api.toml
```

Add or modify users in the `[[auth.file_based.users]]` sections:

```toml
[[auth.file_based.users]]
username      = "admin"
password_hash = "$2y$10$..."   # bcrypt hash — see below
scopes        = "dp:org_read dp:org_manage dp:api_read dp:api_manage ..."

[[auth.file_based.users]]
username      = "developer"
password_hash = "$2y$10$..."
scopes        = "dp:api_read dp:app_read dp:app_write dp:subscription_read"
```

Generate a bcrypt password hash with:

```bash
htpasswd -bnBC 12 "" <password> | tr -d ':\n'
```

### Scope-based authorization

Every devportal REST API operation requires a specific `dp:*` scope. Users without the required scope receive a `403 Forbidden` response. Common scope sets:

| Access level | Scopes to grant |
|---|---|
| Full admin | All `dp:*_manage` scopes + `dp:*_read` |
| API publisher | `dp:api_manage dp:api_content_manage dp:org_read dp:label_read` |
| Developer / subscriber | `dp:api_read dp:app_read dp:app_write dp:subscription_read dp:subscription_write` |

See `configs/config-platform-api.toml.example` for the complete scope list used by the default admin user.

### Session persistence and scripted access

The Platform API generates a random JWT signing key at startup. Sessions are invalidated when it restarts unless you pin the key. Set the **same value** in both services so the devportal can verify JWTs locally without a network round-trip:

```bash
# In .env (read by both services via docker-compose env_file / DP_* override)
AUTH_JWT_SECRET_KEY=<random-64-char-string>
```

For scripts and CLI tools, get a Bearer token directly from the Platform API and pass it on each request — no session cookie required:

```bash
TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v1/auth/login" \
  -d "username=admin&password=admin" | jq -r .token)

curl -s -H "Authorization: Bearer $TOKEN" http://localhost:3000/organizations
```

The token is verified locally by the Developer Portal using the shared `AUTH_JWT_SECRET_KEY` with no extra call to the Platform API per request.

> **Note:** Local auth is for development only. For production, configure an OIDC identity provider per organization (see [Identity Provider Configuration](#identity-provider-configuration)).

---

## Default Organization

When the portal starts (or via the Docker init scripts), a default organization named `ACME` with a `default` view and `default` label is created automatically. You can rename or reconfigure it after first boot.
