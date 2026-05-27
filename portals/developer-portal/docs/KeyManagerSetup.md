# Key Manager Setup Guide

This guide covers setting up an external Key Manager (KM) in decoupled mode (no WSO2 API Manager control plane).

---

## Asgardeo

### Overview

Asgardeo acts as the Authorization Server. The Developer Portal uses its DCR (Dynamic Client Registration) API to create and manage OAuth2 clients on behalf of developers, and its token endpoint to issue access tokens.

### Step 1 — Create a Standard-Based Application in Asgardeo

1. Log in to the [Asgardeo Console](https://console.asgardeo.io) and select your organization.
2. Go to **Applications → New Application** and choose **Standard-Based Application**.
3. Select **OpenID Connect (OIDC)**, give it a name (e.g. `DevPortal Admin`), and click **Register**.
4. On the **Protocol** tab, under **Allowed grant types**, enable **Client Credentials**.
5. On the **API Authorization** tab, authorize the following API and grant all its scopes:
   - **DCR API** — scopes: `internal_dcr_create`, `internal_dcr_view`, `internal_dcr_update`, `internal_dcr_delete`
6. On the **Protocol** tab, copy the **Client ID** and **Client Secret**. These are your `adminClientId` and `adminClientSecret`.

### Step 2 — Note Your Asgardeo Endpoints

Replace `{org}` with your Asgardeo organization name in all URLs below.

| Field | Value |
|---|---|
| `tokenEndpoint` | `https://api.asgardeo.io/t/{org}/oauth2/token` |
| `clientRegistrationEndpoint` | `https://api.asgardeo.io/t/{org}/api/identity/oauth2/dcr/v1.1/register` |
| `issuer` | `https://api.asgardeo.io/t/{org}/oauth2/token` |
| `jwksURL` | `https://api.asgardeo.io/t/{org}/oauth2/jwks` |
| `authorizeEndpoint` *(additionalProperties)* | `https://api.asgardeo.io/t/{org}/oauth2/authorize` |
| `revokeEndpoint` *(additionalProperties)* | `https://api.asgardeo.io/t/{org}/oauth2/revoke` |
| `logoutEndpoint` *(additionalProperties)* | `https://api.asgardeo.io/t/{org}/oidc/logout` |

### Step 3 — Create the Key Manager YAML

Save the following as `asgardeo-keymanager.yaml`, fill in your values, and place it in your org content directory (e.g. `artifacts/default/orgContent/{orgName}/`):

```yaml
apiVersion: devportal.api-platform.wso2.com/v1
kind: KeyManager

metadata:
  name: Asgardeo

spec:
  type: ASGARDEO
  enabled: true
  tokenEndpoint: https://api.asgardeo.io/t/{org}/oauth2/token
  clientRegistrationEndpoint: https://api.asgardeo.io/t/{org}/api/identity/oauth2/dcr/v1.1/register
  issuer: https://api.asgardeo.io/t/{org}/oauth2/token
  jwksURL: https://api.asgardeo.io/t/{org}/oauth2/jwks
  adminClientId: "<client-id-of-your-devportal-admin-m2m-app>"
  adminClientSecret: "<client-secret-of-your-devportal-admin-m2m-app>"
  grantTypes:
    - client_credentials
    - authorization_code
    - refresh_token
  scopes:
    - openid
    - profile
  additionalProperties:
    authorizeEndpoint: https://api.asgardeo.io/t/{org}/oauth2/authorize
    revokeEndpoint: https://api.asgardeo.io/t/{org}/oauth2/revoke
    logoutEndpoint: https://api.asgardeo.io/t/{org}/oidc/logout
    applicationConfiguration:
      - name: ext_application_token_lifetime
        label: Application Access Token Expiry Time
        type: input
        required: false
        default: "3600"
        tooltip: Type the access token expiry time in seconds
      - name: ext_user_token_lifetime
        label: User Access Token Expiry Time
        type: input
        required: false
        default: "3600"
        tooltip: Type the user access token expiry time in seconds
      - name: ext_refresh_token_lifetime
        label: Refresh Token Expiry Time
        type: input
        required: false
        default: "86400"
        tooltip: Type the refresh token expiry time in seconds
      - name: ext_id_token_lifetime
        label: ID Token Expiry Time
        type: input
        required: false
        default: "3600"
        tooltip: Type the ID token expiry time in seconds
      - name: ext_pkce_mandatory
        label: PKCE Mandatory
        type: checkbox
        required: false
        default: false
        tooltip: Enable to make PKCE mandatory for the application
      - name: ext_pkce_support_plain
        label: PKCE Support Plain
        type: checkbox
        required: false
        default: false
        tooltip: Enable to support plain PKCE challenge method
      - name: ext_public_client
        label: Public Client
        type: checkbox
        required: false
        default: false
        tooltip: Enable if the application is a public client
```

### Application Configuration Fields

The `applicationConfiguration` array under `additionalProperties` defines additional fields that appear in the **Modify Credentials** and **View Credentials** modals on the Manage Keys page. Each entry supports the following properties:

| Property | Description |
|---|---|
| `name` | Field name sent to Asgardeo (must start with `ext_` for Asgardeo extension properties) |
| `label` | Display label shown in the UI |
| `type` | Field type: `input` (text field), `checkbox`, `select` (dropdown) |
| `required` | Whether the field is mandatory |
| `default` | Default value when generating new keys |
| `tooltip` | Help text shown below the field |
| `values` | *(select type only)* Array of options |
| `multiple` | *(select type only)* Allow multiple selections |

### Step 4 — Upload via Admin API

```bash
curl -X POST https://<devportal-host>/devportal/organizations/{orgId}/key-managers \
  -H "Authorization: Bearer <admin-token>" \
  -F "keymanager=@asgardeo-keymanager.yaml"
```

Or place the YAML file directly in your org content directory and restart the server. The file is loaded automatically during organization content sync.
