# Webhook Integration

The Developer Portal publishes real-time webhook events to your configured endpoint(s) whenever API key or subscription state changes. The portal itself does not talk to a gateway — it only delivers a signed HTTP POST to whatever URL you register as a webhook subscriber. What happens next is up to the receiving system: typically that's a small handler you run which reacts to the event by updating the actual API Gateway (or cache, key store, etc.) so access is enforced immediately — for example, rejecting a key the moment a developer revokes it in the portal.

## How It Works

```
Developer Portal
       │
       │  POST (signed + optionally encrypted)
       ▼
Your webhook subscriber endpoint
       │
       │  your handler decides what to do —
       │  e.g. update the API Gateway's routing / key store
       ▼
  Gateway enforces new state on next request
```

The portal fires events in the background via a delivery worker. Each delivery is attempted exactly once — there is no retry. Your subscriber endpoint never needs a reverse connection into the portal — it just needs to be a reachable HTTPS endpoint that accepts the POST and does whatever is appropriate on your side (e.g. registering the change with your gateway).

## Webhook Events

| Event | Description | Sensitive field |
|---|---|---|
| `apikey.generated` | A new API key was generated for a subscription | API key secret (`key`) |
| `apikey.regenerated` | An existing API key was rotated | New API key secret (`key`) |
| `apikey.revoked` | An API key was revoked | — |
| `apikey.application_updated` | A key's application association changed | — |
| `subscription.created` | A developer subscribed to an API | Subscription token (`token`) |
| `subscription.updated` | A subscription's status changed, or its token was regenerated | Subscription token (`token`, only on token regeneration) |
| `subscription.deleted` | A developer unsubscribed | — |
| `application.created` | A developer created an application | — |
| `application.updated` | An application was renamed or its details changed | — |
| `application.deleted` | An application was deleted | — |

For events that carry a sensitive field, the value is **envelope-encrypted** with the subscriber's RSA-2048 public key and placed directly in `data` under its field name (e.g. `data.key`, `data.token`). It is never included in plaintext. The top-level `encrypted_fields` array always lists which `data` fields are encrypted.

## Configure a Webhook Subscriber

Webhook subscribers are **per-organization** and managed through the Webhook Subscribers API — not through `config.yaml`. Each organization registers its own endpoint(s); secrets and public keys are stored encrypted at rest (AES-256-GCM) in the devportal database, keyed to the organization.

Only delivery tuning, which applies globally across all organizations, remains in `config.yaml`. Each delivery is attempted exactly once — there is no retry:

```yaml
webhooks:
  delivery:
    pollIntervalMs: 2000
    batchSize: 50
    signatureToleranceSec: 300
```

### Create a subscriber

```bash
curl -X POST "http://localhost:3000/devportal/v1/webhook-subscribers" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production Listener",
    "url": "https://your-service.example.com/devportal/events",
    "secret": "change-me-minimum-32-chars",
    "events": ["apikey.*", "subscription.*"],
    "timeoutMs": 5000
  }'
```

The response never includes the secret. To set a public key for envelope-encrypting sensitive fields (see [Envelope Encryption](#envelope-encryption)), pass its PEM contents in `publicKey`.

### Subscriber fields

| Field | Required | Description |
|---|---|---|
| `name` | Yes | Unique within the organization |
| `url` | Yes | HTTPS endpoint that receives webhook POSTs (e.g. a handler in front of your gateway). Must be unique within the organization |
| `secret` | No | Minimum 32-character string used to sign each event with HMAC-SHA256. Stored encrypted; never returned in API responses. If omitted, deliveries are sent unsigned (no `X-Devportal-Signature` header) |
| `publicKey` | Recommended | PEM-encoded RSA-2048 public key for envelope-encrypting sensitive fields in `apikey.generated`, `apikey.regenerated`, `subscription.created`, and `subscription.updated` events |
| `events` | No | Event type allowlist. Wildcards supported (`apikey.*`). Omit or leave empty to receive all events |
| `enabled` | No | Defaults to `true`. Disable a subscriber without deleting it |
| `timeoutMs` | No | HTTP request timeout in milliseconds (default: 5000) |

### List, update, and delete subscribers

```bash
# List
curl "http://localhost:3000/devportal/v1/webhook-subscribers" -H "Authorization: Bearer $TOKEN"

# Get one
curl "http://localhost:3000/devportal/v1/webhook-subscribers/{subscriberId}" -H "Authorization: Bearer $TOKEN"

# Update (only supplied fields are changed; omitted fields keep their stored values)
curl -X PUT "http://localhost:3000/devportal/v1/webhook-subscribers/{subscriberId}" \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"enabled": false}'

# Delete
curl -X DELETE "http://localhost:3000/devportal/v1/webhook-subscribers/{subscriberId}" \
  -H "Authorization: Bearer $TOKEN"
```

These endpoints require the `dp:webhook_subscriber_read`, `dp:webhook_subscriber_write`, `dp:webhook_subscriber_delete`, or `dp:webhook_subscriber_manage` OAuth2 scopes (see the OpenAPI spec for the exact scope per operation).

## Webhook Request Format

Every event is delivered as an HTTP POST with a JSON body and the following headers:

| Header | Description |
|---|---|
| `X-Devportal-Event` | Event type (e.g. `apikey.generated`) |
| `X-Devportal-Event-Id` | UUID of the event — use for idempotency |
| `X-Devportal-Delivery-Id` | UUID of this specific delivery attempt |
| `X-Devportal-Signature` | HMAC-SHA256 signature (see [Signature Verification](#signature-verification)). Omitted if the subscriber has no secret configured |
| `Content-Type` | `application/json` |

### Envelope structure

All events share this top-level shape:

```json
{
  "event_id": "a1b2c3d4-...",
  "event_type": "apikey.generated",
  "occurred_at": "2026-05-29T10:00:00.000Z",
  "org": {
    "ref_id": "cp-org-ref-id"
  },
  "encrypted_fields": ["key"],
  "data": { ... }
}
```

- `org.ref_id` is the control-plane reference for the organisation; falls back to the internal org UUID when the org has not yet been linked to the control plane.
- `encrypted_fields` lists the names of fields within `data` that carry an encrypted envelope. Always present — empty (`[]`) for events with no sensitive fields.

The `data` field varies by event type and is described below.

---

## Event Payloads

### `apikey.generated`

Fired when a developer generates a new API key for an API.

```json
{
  "event_id": "a1b2c3d4-...",
  "event_type": "apikey.generated",
  "occurred_at": "2026-05-29T10:00:00.000Z",
  "org": { "ref_id": "cp-org-ref-id" },
  "encrypted_fields": ["key"],
  "data": {
    "key_id": "key-uuid",
    "name": "my-key",
    "expires_at": "2027-01-01T00:00:00.000Z",
    "api": {
      "name": "Order API",
      "version": "v1.0",
      "ref_id": "cp-api-uuid"
    },
    "subscription": {
      "ref_id": "sub-uuid",
      "plan_ref_id": "plan-uuid",
      "plan_name": "Gold"
    },
    "application": {
      "id": "app-uuid",
      "name": "My Mobile App"
    },
    "key": {
      "wrappedKey": "<base64>",
      "iv": "<base64>",
      "tag": "<base64>",
      "ciphertext": "<base64>"
    }
  }
}
```

- `subscription` and `application` are absent when the key is not bound to one
- `key` and its entry in `encrypted_fields` are present only when a public key is configured for the subscriber (see [Envelope Encryption](#envelope-encryption)); if no public key is configured, the secret is not delivered
- `expires_at` is `null` for non-expiring keys

### `apikey.regenerated`

Fired when a developer rotates an existing key. The `key_id` and `name` are unchanged; the new secret is delivered in `key`.

```json
{
  "event_type": "apikey.regenerated",
  "encrypted_fields": ["key"],
  "data": {
    "key_id": "key-uuid",
    "name": "my-key",
    "expires_at": null,
    "api": {
      "name": "Order API",
      "version": "v1.0",
      "ref_id": "cp-api-uuid"
    },
    "key": {
      "wrappedKey": "<base64>",
      "iv": "<base64>",
      "tag": "<base64>",
      "ciphertext": "<base64>"
    }
  }
}
```

- `subscription` and `application` are absent when the key is not bound to one

### `apikey.revoked`

Fired when a developer revokes a key. Your subscriber should reject any request presenting this `key_id` (typically by propagating the revocation to your gateway).

```json
{
  "event_type": "apikey.revoked",
  "encrypted_fields": [],
  "data": {
    "key_id": "key-uuid",
    "name": "my-key",
    "api": {
      "name": "Order API",
      "version": "v1.0",
      "ref_id": "cp-api-uuid"
    },
    "subscription": {
      "ref_id": "sub-uuid",
      "plan_ref_id": "plan-uuid",
      "plan_name": "Gold"
    }
  }
}
```

- `subscription` is absent when the key was not bound to a subscription

### `apikey.application_updated`

Fired whenever a single key's application association changes: the key is associated with an app, dissociated, or its app is renamed or deleted. This is a **per-key** event — like `apikey.generated`/`apikey.regenerated`/`apikey.revoked`, `key_id` identifies the one key affected. The association is optional and exists for analytics attribution only — it has no effect on key validity or authorization.

```json
{
  "event_type": "apikey.application_updated",
  "encrypted_fields": [],
  "data": {
    "key_id": "key-uuid",
    "application": {
      "id": "app-uuid",
      "name": "My App"
    }
  }
}
```

- `application` is `null` when the key's association was removed, or when the key's app was deleted
- Renaming an app fires this event once per key currently associated with it, each with the app's new `name`
- Deleting an app fires this event once per key currently associated with it, each with `application: null` — there is no separate "deleted" variant

### `subscription.created`

Fired when a developer subscribes to an API. The subscription token is delivered in `token` — it is never included in plaintext.

```json
{
  "event_type": "subscription.created",
  "encrypted_fields": ["token"],
  "data": {
    "subscription_id": "sub-uuid",
    "subscriber_id": "user@example.com",
    "subscription_plan": {
      "ref_id": "plan-uuid",
      "name": "Gold"
    },
    "api": {
      "name": "Order API",
      "version": "v1.0",
      "ref_id": "cp-api-uuid"
    },
    "token": {
      "wrappedKey": "<base64>",
      "iv": "<base64>",
      "tag": "<base64>",
      "ciphertext": "<base64>"
    }
  }
}
```

- `subscriber_id` is the identity of the user who created the subscription
- `token` decrypts to the subscription token — the value developers must include as `X-Subscription-Token` on APIs that use token-based subscription enforcement
- `token` and its entry in `encrypted_fields` are present only when a public key is configured for the subscriber; if no public key is configured, the token is not delivered

### `subscription.updated`

Fired when a subscription's status changes, or when the subscription token is regenerated. When the token is regenerated, `token` carries the new value encrypted using the subscriber's RSA public key — use the same decryption steps as `subscription.created`.

```json
{
  "event_type": "subscription.updated",
  "encrypted_fields": ["token"],
  "data": {
    "subscription_id": "sub-uuid",
    "subscriber_id": "user@example.com",
    "subscription_plan": {
      "ref_id": "plan-uuid",
      "name": "Gold"
    },
    "api": {
      "name": "Order API",
      "version": "v1.0",
      "ref_id": "cp-api-uuid"
    },
    "token": {
      "wrappedKey": "<base64>",
      "iv": "<base64>",
      "tag": "<base64>",
      "ciphertext": "<base64>"
    }
  }
}
```

- `token` and its entry in `encrypted_fields` are absent when the update did not involve token regeneration (e.g. a status change only)

### `subscription.deleted`

Fired when a developer unsubscribes. Your subscriber should revoke access for the corresponding subscription.

```json
{
  "event_type": "subscription.deleted",
  "encrypted_fields": [],
  "data": {
    "subscription_id": "sub-uuid",
    "subscriber_id": "user@example.com",
    "subscription_plan": {
      "ref_id": "plan-uuid",
      "name": "Gold"
    },
    "api": {
      "name": "Order API",
      "version": "v1.0",
      "ref_id": "cp-api-uuid"
    }
  }
}
```

Your subscriber identifies the affected subscription via `subscription_id`. No token is included.

### `application.created`

Fired when a developer creates an application.

```json
{
  "event_type": "application.created",
  "encrypted_fields": [],
  "data": {
    "application_id": "app-uuid",
    "name": "My Mobile App",
    "description": "Application used to call Weather APIs."
  }
}
```

### `application.updated`

Fired when a developer renames an application or changes its details. `data` carries the full current representation (not a delta).

```json
{
  "event_type": "application.updated",
  "encrypted_fields": [],
  "data": {
    "application_id": "app-uuid",
    "name": "My Mobile App (renamed)",
    "description": "Application used to call Weather APIs."
  }
}
```

If the application has API keys associated with it (see [`apikey.application_updated`](#apikeyapplication_updated)), one such event is fired per associated key with the new name, alongside this event.

### `application.deleted`

Fired when a developer deletes an application, after the application has been removed.

```json
{
  "event_type": "application.deleted",
  "encrypted_fields": [],
  "data": {
    "application_id": "app-uuid",
    "name": "My Mobile App"
  }
}
```

Your subscriber identifies the affected application via `application_id`. If the application had API keys associated with it, one [`apikey.application_updated`](#apikeyapplication_updated) event (with `application: null`) is fired per associated key alongside this event.

---

## Event Security

### Signature verification

If the subscriber has a secret configured, every POST includes an `X-Devportal-Signature` header. The format is:

```
t=<unix_seconds>,v1=<hex_hmac>
```

The HMAC-SHA256 is computed over the canonical string `<unix_seconds>.<raw_body>` using the subscriber's `secret`. If no secret is configured for the subscriber, the header is omitted and the payload is delivered unsigned.

**Verification steps:**

1. Extract `t` and `v1` from the header.
2. Check that `|now - t| <= 300` seconds (configurable via `delivery.signatureToleranceSec`). Reject if outside the window — this prevents replay attacks.
3. Compute `HMAC-SHA256(secret, "<t>.<raw_request_body>")`.
4. Compare the result with `v1` using a timing-safe comparison. Reject the request if they do not match.

**Example (Node.js):**

```js
const crypto = require('crypto');

function verifySignature(secret, rawBody, signatureHeader) {
    const parts = Object.fromEntries(
        signatureHeader.split(',').map(p => p.split('='))
    );
    const t = parseInt(parts.t, 10);
    if (Math.abs(Date.now() / 1000 - t) > 300) return false; // replay window

    const expected = crypto
        .createHmac('sha256', secret)
        .update(`${t}.${rawBody}`)
        .digest('hex');

    return crypto.timingSafeEqual(
        Buffer.from(expected),
        Buffer.from(parts.v1)
    );
}
```

### Envelope encryption

Events that carry sensitive fields include them directly in `data` under their field name (e.g. `data.key`, `data.token`). The top-level `encrypted_fields` array lists which fields are encrypted — check it before processing `data` so you know which fields need decryption.

**Encryption scheme:** hybrid RSA-OAEP + AES-256-GCM.

```
{
  wrappedKey  — RSA-OAEP(SHA-256) encrypted 256-bit AES key (base64)
  iv          — 12-byte AES-GCM IV (base64)
  tag         — 16-byte AES-GCM authentication tag (base64)
  ciphertext  — AES-256-GCM encrypted secret value (base64)
}
```

**Decryption steps:**

1. RSA-decrypt `wrappedKey` with your private key using OAEP+SHA-256 → `aesKey`
2. AES-256-GCM decrypt `ciphertext` using `aesKey`, `iv`, and `tag` → plaintext secret

**Example (Node.js):**

```js
const crypto = require('crypto');

function decryptField(privateKeyPem, envelope) {
    const aesKey = crypto.privateDecrypt(
        { key: privateKeyPem, padding: crypto.constants.RSA_PKCS1_OAEP_PADDING, oaepHash: 'sha256' },
        Buffer.from(envelope.wrappedKey, 'base64')
    );
    const decipher = crypto.createDecipheriv(
        'aes-256-gcm', aesKey, Buffer.from(envelope.iv, 'base64')
    );
    decipher.setAuthTag(Buffer.from(envelope.tag, 'base64'));
    return Buffer.concat([
        decipher.update(Buffer.from(envelope.ciphertext, 'base64')),
        decipher.final()
    ]).toString('utf8');
}

// Usage — decrypt all encrypted fields from a webhook payload:
for (const fieldName of payload.encrypted_fields) {
    payload.data[fieldName] = decryptField(privateKeyPem, payload.data[fieldName]);
}
```

If no public key is configured for the subscriber, encrypted fields are omitted from `data` and `encrypted_fields` is empty — configure a public key before going to production.

## Delivery Attempts

Each delivery is attempted exactly once. If your subscriber endpoint is unavailable or returns a non-2xx response, the delivery is marked `FAILED` immediately — there is no retry or dead-letter queue. Make sure your endpoint is reliable and fast (see `timeoutMs`), since a missed event is not redelivered automatically.

## Monitoring Event Deliveries

> **Authentication:** The examples below use a `$TOKEN` variable. Obtain a Bearer token first via your configured OIDC provider, or via local auth if enabled.

### List recent events

```bash
curl http://localhost:3000/devportal/v1/webhook-events -H "Authorization: Bearer $TOKEN"
```

### Get event details

```bash
curl http://localhost:3000/devportal/v1/webhook-events/{eventId} -H "Authorization: Bearer $TOKEN"
```
