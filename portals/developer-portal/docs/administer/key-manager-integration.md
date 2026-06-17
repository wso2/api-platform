# Key Manager Integration

A key manager is the OAuth2 authorization server used by the Developer Portal to issue and validate credentials for OAuth2-secured APIs. When a developer generates OAuth2 keys for their application, the portal communicates with the configured key manager to create the consumer key/secret pair.

## Prerequisites

Before adding a key manager, set the encryption key in `config.yaml`. This key protects sensitive data stored in the database, including key manager credentials and subscription tokens:

```yaml
advanced:
  encryptionKey: ""   # 64-character hex string (AES-256-GCM)
```

Generate a suitable key:

```bash
openssl rand -hex 32
```

Set the output as `encryptionKey`. You can also provide it via the environment variable `DP_ADVANCED_ENCRYPTIONKEY`.

> **Important:** Store this key securely. If it is lost, all encrypted data in the database cannot be decrypted.

## Add a Key Manager

> **Authentication:** The examples below use a `$TOKEN` variable. Obtain a Bearer token first:
> ```bash
> TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v1/auth/login" \
>   -d "username=admin&password=admin" | jq -r .token)
> ```

Use the `KeyManager` manifest format:

```yaml
# keymanager.yaml
apiVersion: devportal.api-platform.wso2.com/v1
kind: KeyManager

metadata:
  name: Asgardeo

spec:
  type: ASGARDEO
  enabled: true
  tokenEndpoint: https://api.asgardeo.io/t/myorg/oauth2/token
  clientRegistrationEndpoint: https://api.asgardeo.io/t/myorg/api/identity/oauth2/dcr/v1.1/register
  issuer: https://api.asgardeo.io/t/myorg/oauth2/token
  jwksURL: https://api.asgardeo.io/t/myorg/oauth2/jwks
  adminClientId: "<admin-client-id>"
  adminClientSecret: "<admin-client-secret>"
  grantTypes:
    - client_credentials
    - authorization_code
    - refresh_token
  scopes:
    - openid
    - profile
  additionalProperties:
    authorizeEndpoint: https://api.asgardeo.io/t/myorg/oauth2/authorize
    revokeEndpoint: https://api.asgardeo.io/t/myorg/oauth2/revoke
    logoutEndpoint: https://api.asgardeo.io/t/myorg/oidc/logout
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

```bash
curl -X POST http://localhost:3000/o/{orgId}/devportal/v1/key-managers \
  -H "Authorization: Bearer $TOKEN" \
  -F "keymanager=@keymanager.yaml"
```

| Field | Required | Description |
|---|---|---|
| `metadata.name` | Yes | Unique display name shown to developers when generating keys |
| `spec.type` | Yes | Key manager type: `ASGARDEO`, `WSO2IS`, `KEYCLOAK`, or `GENERIC_OIDC` |
| `spec.tokenEndpoint` | Yes | OAuth2 token endpoint |
| `spec.clientRegistrationEndpoint` | Yes | Dynamic Client Registration (DCR) endpoint |
| `spec.adminClientId` | Yes | Admin client ID used to call the key manager's admin APIs (stored encrypted) |
| `spec.adminClientSecret` | Yes | Admin client secret (stored encrypted; never returned in responses) |
| `spec.enabled` | No | Whether the key manager is active. Defaults to `true` |
| `spec.issuer` | No | Issuer URL used to validate `iss` claim in tokens |
| `spec.jwksURL` | No | JWKS endpoint for token signature validation |
| `spec.grantTypes` | No | OAuth2 grant types the key manager supports |
| `spec.scopes` | No | Scopes to request when registering clients |
| `spec.additionalProperties` | No | AS-specific extra config (e.g. `revokeEndpoint`, `logoutEndpoint`, `realm`) |

## List Key Managers

```bash
curl http://localhost:3000/o/{orgId}/devportal/v1/key-managers -H "Authorization: Bearer $TOKEN"
```

## Get a Key Manager

```bash
curl http://localhost:3000/o/{orgId}/devportal/v1/key-managers/{kmId} -H "Authorization: Bearer $TOKEN"
```

## Discover Available Key Managers

Developers can use this endpoint to see which key managers are available for their organization:

```bash
curl http://localhost:3000/o/{orgId}/devportal/v1/key-managers/discover
```

## Update a Key Manager

```yaml
# keymanager-update.yaml
apiVersion: devportal.api-platform.wso2.com/v1
kind: KeyManager

metadata:
  name: Asgardeo

spec:
  adminClientId: new-admin-client-id
  adminClientSecret: new-admin-client-secret
```

```bash
curl -X PUT http://localhost:3000/o/{orgId}/devportal/v1/key-managers/{kmId} \
  -H "Authorization: Bearer $TOKEN" \
  -F "keymanager=@keymanager-update.yaml"
```

## Delete a Key Manager

```bash
curl -X DELETE http://localhost:3000/o/{orgId}/devportal/v1/key-managers/{kmId} \
  -H "Authorization: Bearer $TOKEN"
```

> **Warning:** Deleting a key manager that has active applications associated with it will prevent those applications from generating new credentials. Existing tokens issued by the key manager remain valid until they expire.

## How Developers Use Key Managers

When a developer clicks **Generate Keys** for their application in the portal:

1. The portal calls the key manager's DCR endpoint to register the application and obtain a consumer key/secret.
2. The consumer key and secret are displayed to the developer (once — store them securely).
3. The developer uses these credentials to obtain access tokens from the key manager's token endpoint.

If multiple key managers are configured for an organization, the developer can choose which one to use when generating keys.

When an application is deleted, the portal automatically revokes all OAuth clients registered with their respective key managers and removes all stored key mappings.
