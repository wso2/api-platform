# API Key Management REST API

All API key operations are scoped to a specific API. Replace the placeholders below:

| Placeholder | Description |
|---|---|
| `{orgId}` | Your organization ID |
| `{apiId}` | The UUID of the API |
| `{keyId}` | The UUID of the API key |
| `{appId}` | The UUID of the Developer Portal application |
| `{token}` | A valid Bearer token for the current session |
| `{csrf}` | The `XSRF-TOKEN` cookie value (required for all mutating requests) |

All mutating requests (`POST`) require:
- `Content-Type: application/json`
- `X-CSRF-Token: {csrf}`

Base path: `/o/{orgId}/api/v0.9`

---

## Generate an API key

```bash
curl -X POST \
  "https://{host}/o/{orgId}/api/v0.9/apis/{apiId}/api-keys/generate" \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: {csrf}" \
  -d '{
    "name": "my-key",
    "expiresAt": "2027-01-01T00:00:00Z",
    "subscriptionId": "{subscriptionId}",
    "appId": "{appId}"
  }'
```

`subscriptionId` and `appId` are optional. `expiresAt` is optional; omit for a non-expiring key.

**Response `201`:**
```json
{
  "keyId": "key-xxxxxxxx",
  "keyValue": "sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
  "name": "my-key",
  "status": "active",
  "apiId": "{apiId}",
  "expiresAt": "2027-01-01T00:00:00Z",
  "createdAt": "2026-06-30T10:00:00Z"
}
```

> **Important:** `keyValue` is returned only once. Copy it immediately.

---

## List API keys

```bash
curl -X GET \
  "https://{host}/o/{orgId}/api/v0.9/apis/{apiId}/api-keys" \
  -H "Authorization: Bearer {token}"
```

Optional query parameters:

| Parameter | Description |
|---|---|
| `subscriptionId` | Filter by subscription |
| `appId` | Filter by associated application |
| `status` | Filter by status (`active`, `revoked`) |

**Response `200`:**
```json
{
  "count": 1,
  "list": [
    {
      "keyId": "key-xxxxxxxx",
      "name": "my-key",
      "status": "active",
      "apiId": "{apiId}",
      "appId": "{appId}",
      "appDisplayName": "My App",
      "expiresAt": "2027-01-01T00:00:00Z",
      "createdAt": "2026-06-30T10:00:00Z"
    }
  ]
}
```

---

## Regenerate an API key

Issues a new key value for an existing key. The old value is immediately invalidated.

```bash
curl -X POST \
  "https://{host}/o/{orgId}/api/v0.9/apis/{apiId}/api-keys/regenerate" \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: {csrf}" \
  -d '{"keyId": "{keyId}"}'
```

**Response `200`:** same shape as generate; `keyValue` contains the new value.

---

## Revoke an API key

Permanently revokes a key. This cannot be undone.

```bash
curl -X POST \
  "https://{host}/o/{orgId}/api/v0.9/apis/{apiId}/api-keys/revoke" \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: {csrf}" \
  -d '{"keyId": "{keyId}"}'
```

**Response `204`:** no body.

---

## Associate a key with an application

Links a key to a Developer Portal application for usage analytics. Association does not affect key validity.

```bash
curl -X POST \
  "https://{host}/o/{orgId}/api/v0.9/apis/{apiId}/api-keys/associate" \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: {csrf}" \
  -d '{"keyId": "{keyId}", "appId": "{appId}"}'
```

**Response `200`:** updated key object.

---

## Remove an application association

Removes the link between a key and its associated application.

```bash
curl -X POST \
  "https://{host}/o/{orgId}/api/v0.9/apis/{apiId}/api-keys/dissociate" \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -H "X-CSRF-Token: {csrf}" \
  -d '{"keyId": "{keyId}"}'
```

**Response `204`:** no body.

---

## List API keys associated with an application

Returns all keys (across all APIs) currently associated with a specific application.

```bash
curl -X GET \
  "https://{host}/o/{orgId}/api/v0.9/applications/{applicationId}/api-keys" \
  -H "Authorization: Bearer {token}"
```

**Response `200`:** same paginated list shape as [List API keys](#list-api-keys).

---

## Error responses

| Status | Meaning |
|---|---|
| `400` | Missing or invalid request body field |
| `403` | Authentication or CSRF failure |
| `404` | API, key, or application not found (or key does not belong to the specified API) |
| `500` | Internal server error |
