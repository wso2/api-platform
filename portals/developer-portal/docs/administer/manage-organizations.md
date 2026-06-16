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
  -u admin:admin \
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
curl http://localhost:3000/organizations -u admin:admin
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
  -u admin:admin \
  -F "organization=@org-update.yaml"
```

## Delete an Organization

```bash
curl -X DELETE http://localhost:3000/organizations/{orgId} -u admin:admin
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
  -u admin:admin \
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
  -u admin:admin \
  -F "identityProvider=@idp-update.yaml"
```

### Remove an IdP

```bash
curl -X DELETE http://localhost:3000/organizations/{orgId}/identityProvider -u admin:admin
```

---

## Local Auth (Development Only)

For local development and first-time setup, the portal ships with built-in user support. Configure users in `config.yaml`:

```yaml
defaultAuth:
  users:
    - username: "admin"
      password: "admin"
      roles:
        - "admin"
      orgClaimName: "ACME"
      organizationIdentifier: "ACME"
    - username: "developer"
      password: "developer"
      roles:
        - "subscriber"
      orgClaimName: "ACME"
      organizationIdentifier: "ACME"
```

> **Important:** Remove the `defaultAuth` block before deploying to production. It is not a substitute for a real IdP.

---

## Default Organization

When the portal starts (or via the Docker init scripts), a default organization named `ACME` with a `default` view and `default` label is created automatically. You can rename or reconfigure it after first boot.
