# Gateway Integration

The Developer Portal supports two integration modes for propagating API key and subscription state to the gateway:

| Mode | When it activates | How it works |
|---|---|---|
| **Platform API** | API has `GATEWAY_TYPE = wso2/api-platform` **and** `platformApi.baseUrl` is configured | Devportal calls the Platform API directly. The Platform API is responsible for broadcasting state to the gateways. Webhook events are not fired. |
| **Webhook** | All other cases (different gateway type, or `platformApi.baseUrl` not set) | Devportal delivers signed HTTP POST events to configured webhook subscribers. |

The two modes are described in detail below.

---

## Platform API Mode (`wso2/api-platform`)

When both conditions are met — the API's `GATEWAY_TYPE` is `wso2/api-platform` and `platformApi.baseUrl` is set in config — the devportal integrates directly with the Platform API for subscriptions and API keys.

```
Developer Portal
       │
       │  REST calls (Bearer JWT of the current user)
       ▼
  Platform API
       │
       │  broadcasts to connected gateways
       ▼
  Gateway instances
```

### Subscription flow

| Operation | Platform API call |
|---|---|
| Subscribe | `POST /api/v1/subscriptions` — Platform API generates the subscription token and returns it; devportal stores it. |
| Unsubscribe | `GET /api/v1/subscriptions?apiId=…&subscriberId=…` → `DELETE /api/v1/subscriptions/{id}` |
| Update status | `GET /api/v1/subscriptions?apiId=…&subscriberId=…` → `PUT /api/v1/subscriptions/{id}` |

### API key flow

| Operation | Platform API call |
|---|---|
| Generate | Key generated in devportal; pushed to `POST /api/v1/rest-apis/{apiHandle}/api-keys` |
| Regenerate | New key generated in devportal; pushed to `PUT /api/v1/rest-apis/{apiHandle}/api-keys/{name}` |
| Revoke | `DELETE /api/v1/rest-apis/{apiHandle}/api-keys/{name}`; devportal DB status updated |

### Configuration

Set `platformApi.baseUrl` in `config.yaml` (or `DP_PLATFORMAPI_BASEURL` env var). No webhook subscriber configuration is needed for APIs using this mode.

```yaml
platformApi:
  baseUrl: "https://platform-api:9243"
  insecure: false   # set true only for self-signed certs in dev
```

---

## Webhook Mode

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

The portal fires events in the background via a delivery worker with automatic retries. Your subscriber endpoint never needs a reverse connection into the portal — it just needs to be a reachable HTTPS endpoint that accepts the POST and does whatever is appropriate on your side (e.g. registering the change with your gateway).

## Webhook Events

| Event | Description | Sensitive field |
|---|---|---|
| `apikey.generated` | A new API key was generated for a subscription | API key secret (`encrypted_key`) |
| `apikey.regenerated` | An existing API key was rotated | New API key secret (`encrypted_key`) |
| `apikey.revoked` | An API key was revoked | — |
| `subscription.created` | A developer subscribed to an API | Subscription token (`encrypted_key`) |
| `subscription.plan_changed` | A subscription's plan changed | — |
| `subscription.deleted` | A developer unsubscribed | — |

For events that carry a sensitive field (`apikey.generated`, `apikey.regenerated`, `subscription.created`), the value is **envelope-encrypted** with the subscriber's RSA-2048 public key and delivered in `data.encrypted_key`. It is never included in plaintext.

## Configure a Webhook Subscriber

Webhook subscribers are **per-organization** and managed through the Webhook Subscribers API — not through `config.yaml`. Each organization registers its own endpoint(s); secrets and public keys are stored encrypted at rest (AES-256-GCM) in the devportal database, keyed to the organization.

Only delivery (retry/backoff) tuning, which applies globally across all organizations, remains in `config.yaml`:

```yaml
webhooks:
  delivery:
    maxAttempts: 8
    backoff: [60, 300, 900, 1800, 3600, 7200, 14400, 28800]  # seconds: 1m 5m 15m 30m 1h 2h 4h 8h
    pollIntervalMs: 2000
    batchSize: 50
    signatureToleranceSec: 300
```

### Create a subscriber

```bash
curl -X POST "http://localhost:3000/o/{orgId}/devportal/v1/webhook-subscribers" \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Production Listener",
    "url": "https://your-service.example.com/devportal/events",
    "secret": "change-me-minimum-32-chars",
    "gatewayType": "wso2/api-platform",
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
| `publicKey` | Recommended | PEM-encoded RSA-2048 public key for envelope-encrypting sensitive fields in `apikey.generated`, `apikey.regenerated`, and `subscription.created` events |
| `gatewayType` | No | Filter events to APIs with this gateway type. Use `"*"` (default) to match all |
| `events` | No | Event type allowlist. Wildcards supported (`apikey.*`). Omit to receive all |
| `enabled` | No | Defaults to `true`. Disable a subscriber without deleting it |
| `timeoutMs` | No | HTTP request timeout in milliseconds (default: 5000) |

### List, update, and delete subscribers

```bash
# List
curl "http://localhost:3000/o/{orgId}/devportal/v1/webhook-subscribers" -H "Authorization: Bearer $TOKEN"

# Get one
curl "http://localhost:3000/o/{orgId}/devportal/v1/webhook-subscribers/{subscriberId}" -H "Authorization: Bearer $TOKEN"

# Update (only supplied fields are changed; omitted fields keep their stored values)
curl -X PUT "http://localhost:3000/o/{orgId}/devportal/v1/webhook-subscribers/{subscriberId}" \
  -H "Authorization: Bearer $TOKEN" -H "Content-Type: application/json" \
  -d '{"enabled": false}'

# Delete
curl -X DELETE "http://localhost:3000/o/{orgId}/devportal/v1/webhook-subscribers/{subscriberId}" \
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
| `X-Devportal-Signature` | HMAC-SHA256 signature (see [Signature Verification](#signature-verification)) |
| `Content-Type` | `application/json` |

### Envelope structure

All events share this top-level shape:

```json
{
  "event_id": "a1b2c3d4-...",
  "event_type": "apikey.generated",
  "occurred_at": "2026-05-29T10:00:00.000Z",
  "org_id": "1ba42a09-...",
  "gateway_type": "wso2/api-platform",
  "data": { ... }
}
```

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
  "org_id": "1ba42a09-...",
  "gateway_type": "wso2/api-platform",
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
    "encrypted_key": {
      "wrappedKey": "<base64>",
      "iv": "<base64>",
      "tag": "<base64>",
      "ciphertext": "<base64>"
    }
  }
}
```

- `subscription` is present only when the key is bound to a subscription
- `encrypted_key` is present only when a `publicKeyPath` is configured for the subscriber (see [Envelope Encryption](#envelope-encryption))
- `expires_at` is `null` for non-expiring keys

### `apikey.regenerated`

Fired when a developer rotates an existing key. The `key_id` is unchanged; the old secret is invalidated and replaced by the new one in `encrypted_key`.

```json
{
  "event_type": "apikey.regenerated",
  "data": {
    "key_id": "key-uuid",
    "name": "my-key",
    "expires_at": null,
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
    "encrypted_key": { "wrappedKey": "...", "iv": "...", "tag": "...", "ciphertext": "..." }
  }
}
```

### `apikey.revoked`

Fired when a developer revokes a key. Your subscriber should reject any request presenting this `key_id` (typically by propagating the revocation to your gateway).

```json
{
  "event_type": "apikey.revoked",
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

- `subscription` is present only when the key was bound to a subscription
- No `encrypted_key` is included — your subscriber only needs the `key_id` to revoke access.

### `subscription.created`

Fired when a developer subscribes to an API. The subscription token is delivered in `encrypted_key` — it is never included in plaintext.

```json
{
  "event_type": "subscription.created",
  "data": {
    "subscription_id": "sub-uuid",
    "subscription_plan": {
      "ref_id": "plan-uuid",
      "name": "Gold"
    },
    "api": {
      "name": "Order API",
      "version": "v1.0",
      "ref_id": "cp-api-uuid"
    },
    "encrypted_key": {
      "wrappedKey": "<base64>",
      "iv": "<base64>",
      "tag": "<base64>",
      "ciphertext": "<base64>"
    }
  }
}
```

- `encrypted_key` decrypts to the subscription token — the value developers must include as `X-Subscription-Token` on APIs that use token-based subscription enforcement
- `encrypted_key` is present only when a `publicKeyPath` is configured for the subscriber; if no public key is configured, the token is not delivered

### `subscription.plan_changed`

Fired when a subscription's plan changes.

```json
{
  "event_type": "subscription.plan_changed",
  "data": {
    "subscription": {
      "plan_name": "Bronze",
      "plan_ref_id": "plan-uuid",
      "status": "ACTIVE"
    },
    "api": {
      "name": "Order API",
      "version": "v1.0",
      "ref_id": "cp-api-uuid"
    }
  }
}
```

Your subscriber identifies the affected subscription via the `aggregateId` in the event. No token is included.

### `subscription.deleted`

Fired when a developer unsubscribes. Your subscriber should revoke access for the corresponding subscription.

```json
{
  "event_type": "subscription.deleted",
  "data": {
    "subscription_id": "sub-uuid",
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

---

## Event Security

### Signature verification

Every POST includes an `X-Devportal-Signature` header. The format is:

```
t=<unix_seconds>,v1=<hex_hmac>
```

The HMAC-SHA256 is computed over the canonical string `<unix_seconds>.<raw_body>` using the subscriber's `secret`.

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

`apikey.generated`, `apikey.regenerated`, and `subscription.created` events include an `encrypted_key` object when a `publicKeyPath` is configured for the subscriber. The sensitive value (API key secret or subscription token) is never included in plaintext.

**Encryption scheme:** hybrid RSA-OAEP + AES-256-GCM.

```
encrypted_key = {
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

function decryptSecret(privateKeyPem, encryptedKey) {
    const aesKey = crypto.privateDecrypt(
        { key: privateKeyPem, padding: crypto.constants.RSA_PKCS1_OAEP_PADDING, oaepHash: 'sha256' },
        Buffer.from(encryptedKey.wrappedKey, 'base64')
    );
    const decipher = crypto.createDecipheriv(
        'aes-256-gcm', aesKey, Buffer.from(encryptedKey.iv, 'base64')
    );
    decipher.setAuthTag(Buffer.from(encryptedKey.tag, 'base64'));
    return decipher.update(Buffer.from(encryptedKey.ciphertext, 'base64')) + decipher.final('utf8');
}
```

If no `publicKeyPath` is configured for the subscriber, `encrypted_key` is omitted and the sensitive value is not delivered at all — configure a public key before going to production.

## Delivery Retry

If your subscriber endpoint is unavailable or returns a non-2xx response, the portal retries delivery according to this schedule:

| Attempt | Delay |
|---|---|
| 1 | 1 minute |
| 2 | 5 minutes |
| 3 | 15 minutes |
| 4 | 30 minutes |
| 5 | 1 hour |
| 6 | 2 hours |
| 7 | 4 hours |
| 8 | 8 hours |

After all attempts are exhausted, the delivery is marked as failed. You can trigger a manual retry via the admin API.

## Monitoring Event Deliveries

> **Authentication:** The examples below use a `$TOKEN` variable. Obtain a Bearer token first:
> ```bash
> TOKEN=$(curl -sk -X POST "https://localhost:9243/api/portal/v1/auth/login" \
>   -d "username=admin&password=admin" | jq -r .token)
> ```

### List recent events

```bash
curl http://localhost:3000/o/{orgId}/devportal/v1/webhook-events -H "Authorization: Bearer $TOKEN"
```

### Get event details

```bash
curl http://localhost:3000/o/{orgId}/devportal/v1/webhook-events/{eventId} -H "Authorization: Bearer $TOKEN"
```

### Retry a failed delivery

```bash
curl -X POST \
  http://localhost:3000/o/{orgId}/devportal/v1/webhook-deliveries/{deliveryId}/retry \
  -H "Authorization: Bearer $TOKEN"
```

