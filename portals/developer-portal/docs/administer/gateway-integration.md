# Gateway Integration

The Developer Portal integrates with the API Gateway by delivering real-time webhook events whenever API key or subscription state changes. This allows the gateway to enforce access immediately — for example, revoking a key at the gateway the moment a developer revokes it in the portal.

## How It Works

```
Developer Portal
       │
       │  POST (signed + optionally encrypted)
       ▼
API Gateway webhook endpoint
       │
       │  updates internal routing / key store
       ▼
  Enforces new state on next request
```

The portal fires events in the background via a delivery worker with automatic retries. The gateway never needs a reverse connection into the portal.

## Webhook Events

| Event | Description | Payload |
|---|---|---|
| `apikey.generated` | A new API key was generated for a subscription | API key value (encrypted) |
| `apikey.regenerated` | An existing API key was rotated | Old + new key value (encrypted) |
| `apikey.revoked` | An API key was revoked | API key reference (encrypted) |
| `subscription.created` | A developer subscribed an application to an API | Subscription details |
| `subscription.updated` | A subscription's plan or state changed | Updated subscription details |
| `subscription.deleted` | A developer unsubscribed | Subscription reference |

API key payloads are **envelope-encrypted** with the subscriber's RSA-2048 public key so that sensitive key material is never exposed in transit.

## Configure a Webhook Subscriber

Add subscriber configuration to `config.yaml` under the `webhooks` block:

```yaml
webhooks:
  subscribers:
    - id: my-gateway                           # stable identifier — used in logs and the DB
      gatewayType: "wso2/api-platform"         # matches the API's gateway type; use "*" for all
      url: "https://gateway.example.com/devportal/events"
      secret: "change-me-minimum-32-chars"     # HMAC-SHA256 signing key
      publicKey: |                             # RSA-2048 PEM — for encrypting apikey.* payloads
        -----BEGIN PUBLIC KEY-----
        MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEA...
        -----END PUBLIC KEY-----
      events:                                  # event type filter (omit to receive all events)
        - apikey.*
        - subscription.*
      timeoutMs: 5000

  delivery:
    maxAttempts: 8
    backoff: [60, 300, 900, 1800, 3600, 7200, 14400, 28800]  # seconds: 1m 5m 15m 30m 1h 2h 4h 8h
    pollIntervalMs: 2000
    batchSize: 50
```

### Subscriber fields

| Field | Required | Description |
|---|---|---|
| `id` | Yes | Stable identifier used in logs and the database |
| `gatewayType` | No | Filter events to APIs with this gateway type. Use `"*"` to match all |
| `url` | Yes | HTTPS endpoint on your gateway that receives webhook POSTs |
| `secret` | Yes | Minimum 32-character string used to sign each event with HMAC-SHA256 |
| `publicKey` | Recommended | RSA-2048 public key (PEM) for envelope-encrypting API key payloads |
| `events` | No | Event type allowlist. Wildcards supported (`apikey.*`). Omit to receive all |
| `timeoutMs` | No | HTTP request timeout in milliseconds (default: 5000) |

### Providing secrets via environment variables

To avoid storing secrets in `config.yaml`, use environment variables. The ID is uppercased and non-alphanumeric characters are replaced with `_`:

```bash
# For subscriber id: my-gateway
export DP_WEBHOOK_SECRET_MY_GATEWAY="your-hmac-secret"
export DP_WEBHOOK_PUBKEY_PATH_MY_GATEWAY="/run/secrets/gateway-pubkey.pem"
```

## Event Security

### Signature verification

Every webhook POST includes an `x-signature` header containing the HMAC-SHA256 signature of the request body, signed with the subscriber's `secret`. Your gateway should verify this signature on every request and reject any request that fails verification.

### Envelope encryption for API key events

API key payloads (`apikey.*` events) are encrypted with the subscriber's RSA-2048 public key using envelope encryption. Only the holder of the corresponding private key can decrypt the payload. If no public key is configured, API key payloads are sent unencrypted.

## Delivery Retry

If the gateway endpoint is unavailable or returns a non-2xx response, the portal retries delivery according to this schedule:

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

### List recent events

```bash
curl http://localhost:3000/organizations/{orgId}/events -u admin:admin
```

### Get event details

```bash
curl http://localhost:3000/organizations/{orgId}/events/{eventId} -u admin:admin
```

### Retry a failed delivery

```bash
curl -X POST \
  http://localhost:3000/organizations/{orgId}/deliveries/{deliveryId}/retry \
  -u admin:admin
```
