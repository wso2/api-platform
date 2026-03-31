# Subscription Validation Integration Test — Verification Report

## Summary

**With the reverted gateway-controller (no changes):** The subscription-validation integration test would **fail** because the gateway-controller's `POST /api/v1/subscriptions` handler does not accept or generate `subscriptionToken`. The storage layer requires a non-empty token and fails with "plaintext cannot be empty".

**Solution:** Mock the platform-api event flow. Platform-api propagates subscription creation to the gateway-controller via WebSocket `subscription.created` events. The IT now uses a mock platform-api that injects these events.

---

## Verification Findings

### 1. Gateway-Controller `POST /api/v1/subscriptions` (REST API)

- **OpenAPI:** `SubscriptionCreateRequest` does not define `subscriptionToken`.
- **Handler:** `CreateSubscription` never sets `sub.SubscriptionToken`; it is always empty.
- **Storage:** `SaveSubscription` calls `EncryptSubscriptionToken(key, "")`, which returns error "plaintext cannot be empty".
- **Result:** Subscription creation via REST API returns 500.

### 2. Platform-API Event Flow (per IMPLEMENTATION_NOTES.md)

- Platform-api creates subscriptions and generates the token.
- Platform-api sends `subscription.created` over WebSocket with payload: `apiId`, `subscriptionId`, `subscriptionToken`, `subscriptionPlanId`, `status`.
- Gateway-controller's `handleSubscriptionCreatedEvent` receives the event and persists the subscription with the token.

### 3. What Works Without Changes

- Routes: `/api/v1/subscription-plans`, `/api/v1/subscriptions`, `/api/v1/rest-apis` at gateway-controller
- Auth: Basic auth for gateway-controller requests
- API ID resolution: `resolveAPIIDByHandle` maps handle (metadata.name) to deployment ID
- Encryption key: Set in docker-compose.test.yaml
- Wait step: `I wait for the endpoint ... to return 403`

---

## Implementation: Mock Platform-API

### Components

1. **mock-platform-api** (`tests/mock-servers/mock-platform-api/`)
   - WebSocket server at `/api/internal/v1/ws/gateways/connect` — sends `connection.ack` when gateway connects
   - HTTP `POST /inject-subscription` — accepts `{apiHandle, subscriptionToken, subscriptionPlanId}`, resolves deployment UUID from SQLite, sends `subscription.created` to the connected gateway
   - Sync endpoints return 500 so gateway keeps locally-created plans

2. **Docker Compose**
   - `mock-platform-api` service with shared volume for gateway SQLite
   - Gateway-controller configured with `CONTROLPLANE_HOST`, `CONTROLPLANE_TOKEN`, `INSECURE_SKIP_VERIFY`
   - Gateway depends on mock-platform-api

3. **IT Step**
   - `I create a subscription for API "X" with plan and token "Y"` now POSTs to mock's `/inject-subscription` instead of gateway's `/api/v1/subscriptions`

### Flow

1. Gateway connects to mock WebSocket → receives `connection.ack`
2. Sync runs → mock returns 500 → gateway keeps existing plans
3. IT creates plan via gateway `POST /api/v1/subscription-plans`
4. IT deploys API via gateway `POST /apis`
5. IT calls mock `POST /inject-subscription` with apiHandle, token, planId
6. Mock reads deployment UUID from SQLite, sends `subscription.created` over WebSocket
7. Gateway persists subscription and refreshes xDS snapshot
8. IT invokes API with `Subscription-Key` header → 200, then rate limit → 429

---

## References

- `docs/api-subscriptions/IMPLEMENTATION_PLAN.md`
- `docs/api-subscriptions/IMPLEMENTATION_NOTES.md`
- `gateway/gateway-controller/pkg/controlplane/client.go` — `handleSubscriptionCreatedEvent`
- `platform-api/src/internal/model/subscription_event.go` — event payload structure
