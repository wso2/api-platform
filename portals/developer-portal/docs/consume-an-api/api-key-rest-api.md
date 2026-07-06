# API Key Management REST API

All API key operations are scoped to a specific API. Replace the placeholders below:

| Placeholder | Description |
|---|---|
| `{apiId}` | The Developer Portal ID of the API |
| `{applicationId}` | The Developer Portal ID of the application (used in URL paths) |
| `{appId}` | The Developer Portal ID of the application to associate the key with (used in request/response bodies) |
| `{subscriptionId}` | The ID of the subscription to bind the key to |
| `{keyId}` | The key's handle — the `id` you chose when generating it (not the `keyId` returned in responses) |
| `{token}` | A valid Bearer token for the current session |
| `{csrf}` | The `XSRF-TOKEN` cookie value (required for all mutating requests) |

All mutating requests (`POST`) require:
- `Content-Type: application/json`
- `X-CSRF-Token: {csrf}`

Base path: `/api/v0.9`

---

## Generate an API key

```bash
curl -X POST \
  "https://{host}/api/v0.9/apis/{apiId}/api-keys/generate" \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: {csrf}" \
  -d '{
    "id": "weather_prod_key",
    "expiresAt": "2027-01-01T00:00:00Z",
    "subscriptionId": "{subscriptionId}",
    "appId": "{appId}"
  }'
```

`id` is required and must match `^[a-z0-9][a-z0-9_-]{0,127}$` — this becomes the key's handle used by `{keyId}` in other operations. `displayName`, `subscriptionId`, `appId`, and `expiresAt` are all optional; omit `expiresAt` for a non-expiring key.

**Response `201`:**
```json
{
  "keyId": "key-12345",
  "id": "weather_prod_key",
  "displayName": "Weather Prod Key",
  "key": "ak_dGhpcyBpcyBub3QgYSByZWFsIGtleQ",
  "expiresAt": "2026-12-31T23:59:59Z",
  "status": "ACTIVE"
}
```

> **Important:** `key` is the plaintext secret and is returned only once. Copy it immediately.

---

## List API keys

```bash
curl -X GET \
  "https://{host}/api/v0.9/apis/{apiId}/api-keys" \
  -H "Authorization: Bearer {token}"
```

Optional query parameters:

| Parameter | Description |
|---|---|
| `appId` | Filter by associated application |
| `limit` | Page size |
| `offset` | Page offset |

**Response `200`:**
```json
{
  "list": [
    {
      "keyId": "key-12345",
      "id": "weather_prod_key",
      "displayName": "Weather Prod Key",
      "apiId": "{apiId}",
      "appId": "{appId}",
      "appDisplayName": "My App",
      "status": "ACTIVE",
      "expiresAt": "2026-12-31T23:59:59Z",
      "createdAt": "2026-06-30T10:00:00Z",
      "revokedAt": null
    }
  ],
  "pagination": {
    "total": 1,
    "limit": 20,
    "offset": 0
  }
}
```

---

## Regenerate an API key

Issues a new secret for an existing key. The old secret is immediately invalidated.

```bash
curl -X POST \
  "https://{host}/api/v0.9/apis/{apiId}/api-keys/regenerate" \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: {csrf}" \
  -d '{"keyId": "{keyId}"}'
```

`expiresAt` may also be included to update the key's expiry at the same time. `id`/`displayName` cannot be changed by this operation.

**Response `200`:** same shape as generate; `key` contains the new plaintext secret. Returns `409` if the key has already been revoked.

---

## Revoke an API key

Permanently revokes a key. This cannot be undone.

```bash
curl -X POST \
  "https://{host}/api/v0.9/apis/{apiId}/api-keys/revoke" \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: {csrf}" \
  -d '{"keyId": "{keyId}"}'
```

**Response `204`:** no body. Returns `409` if the key was already revoked.

---

## Associate a key with an application

Links a key to a Developer Portal application for usage analytics. Association does not affect key validity or authorization.

```bash
curl -X POST \
  "https://{host}/api/v0.9/apis/{apiId}/api-keys/associate" \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: {csrf}" \
  -d '{"keyId": "{keyId}", "appId": "{appId}"}'
```

**Response `200`:**
```json
{
  "keyId": "key-12345",
  "application": {
    "id": "{appId}",
    "displayName": "My Mobile App"
  }
}
```

Returns `409` if the key has already been revoked.

---

## Remove an application association

Removes the link between a key and its associated application, if any.

```bash
curl -X POST \
  "https://{host}/api/v0.9/apis/{apiId}/api-keys/dissociate" \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: {csrf}" \
  -d '{"keyId": "{keyId}"}'
```

**Response `204`:** no body.

---

## List API keys associated with an application

Returns all keys (across all APIs) currently associated with a specific application. No `apiId` filter is required.

```bash
curl -X GET \
  "https://{host}/api/v0.9/applications/{applicationId}/api-keys" \
  -H "Authorization: Bearer {token}"
```

**Response `200`:** same paginated `{list, pagination}` shape as [List API keys](#list-api-keys).

---

## Error responses

| Status | Meaning |
|---|---|
| `400` | Missing or invalid request body field |
| `403` | Authentication or CSRF failure |
| `404` | API, key, or application not found (or key does not belong to the specified API) |
| `409` | Key already revoked — cannot regenerate or associate |
| `500` | Internal server error |
