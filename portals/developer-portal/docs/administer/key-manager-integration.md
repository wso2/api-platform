# Key Manager Integration

A key manager is the OAuth2 authorization server used to issue access tokens for OAuth2-secured APIs. The Developer Portal does not create or register OAuth applications — developers create their OAuth application directly in the key manager, then link its client ID to an application in the portal. The portal only proxies `client_credentials` token requests to the key manager's token endpoint.

## Prerequisites

No encryption key is required for key manager configuration — the portal never stores a client secret or other key manager credential. (The encryption key in `config.toml` is still used for other secrets, such as webhook signing secrets.)

## Add a Key Manager

> **Authentication:** The examples below use a `$TOKEN` variable. Obtain a Bearer token first:
> ```bash
> TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v0.9/auth/login" \
>   -d "username=admin&password=admin" | jq -r .token)
> ```

Use the `KeyManager` manifest format:

```yaml
# keymanager.yaml
apiVersion: devportal.api-platform.wso2.com/v1alpha1
kind: KeyManager

metadata:
  name: asgardeo-prod

spec:
  displayName: Asgardeo
  enabled: true
  tokenEndpoint: https://api.asgardeo.io/t/myorg/oauth2/token
```

```bash
curl -k -X POST https://localhost:3000/api/v0.9/key-managers \
  -H "Authorization: Bearer $TOKEN" \
  -F "keymanager=@keymanager.yaml"
```

| Field | Required | Description |
|---|---|---|
| `metadata.name` | Yes | Unique handle for the key manager (used internally, e.g. in route segments) |
| `spec.displayName` | Yes | Display name shown to developers |
| `spec.tokenEndpoint` | Yes | OAuth2 token endpoint. The portal proxies `client_credentials` token requests here using the client ID/secret the developer supplies |
| `spec.enabled` | No | Whether the key manager is active. Defaults to `true` |

Every key manager is treated as a generic OAuth2 `client_credentials` provider — there is no `type` to configure.

## List Key Managers

```bash
curl -k https://localhost:3000/api/v0.9/key-managers -H "Authorization: Bearer $TOKEN"
```

## Get a Key Manager

```bash
curl -k https://localhost:3000/api/v0.9/key-managers/{kmId} -H "Authorization: Bearer $TOKEN"
```

## Update a Key Manager

```yaml
# keymanager-update.yaml
apiVersion: devportal.api-platform.wso2.com/v1alpha1
kind: KeyManager

metadata:
  name: Asgardeo

spec:
  tokenEndpoint: https://api.asgardeo.io/t/myorg/oauth2/token
```

```bash
curl -k -X PUT https://localhost:3000/api/v0.9/key-managers/{kmId} \
  -H "Authorization: Bearer $TOKEN" \
  -F "keymanager=@keymanager-update.yaml"
```

## Delete a Key Manager

```bash
curl -k -X DELETE https://localhost:3000/api/v0.9/key-managers/{kmId} \
  -H "Authorization: Bearer $TOKEN"
```

> **Warning:** Deleting a key manager that has applications linked to it will prevent those applications from generating new tokens through it. Existing tokens issued by the key manager remain valid until they expire.

## How Developers Use Key Managers

1. The developer creates an OAuth application directly in the key manager's own console (outside the portal) and obtains a client ID.
2. In the portal, the developer opens their application's **Manage Keys** view, picks a key manager, and pastes the client ID. The portal stores only the client ID — never a secret.
3. To get an access token, the developer clicks **Generate access token**, enters the client secret when prompted, and the portal proxies a `client_credentials` token request to the key manager's token endpoint. The secret is never stored.
4. The **cURL** tab shows the equivalent `curl` command so the developer can request tokens directly without going through the portal.

When an application is deleted, the portal removes the stored client ID mappings. It does not contact the key manager — the OAuth application itself is owned and managed there independently.
