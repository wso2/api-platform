# Quickstart: Validating WebSocket API Support

End-to-end validation scenarios proving the feature works, one per priority user story in
[spec.md](./spec.md). Each is independently runnable and maps to an acceptance scenario already
defined there — see spec.md for the full Given/When/Then; this guide is the runnable shape, not a
restatement of the requirements.

## Prerequisites

- A running API Platform Gateway built from this feature's changes (gateway-controller +
  gateway-runtime/policy-engine), version-gated per `MinWebSocketKindGatewayVersion`
  (research.md §6).
- The `mock-websocket-backend` test service (plan.md's Project Structure) reachable from the
  gateway, acting as a `ws://`/`wss://` echo upstream.
- A WebSocket client capable of setting `Sec-WebSocket-Protocol` and query parameters (`wscat`,
  `websocat`, or the console/API Portal test console itself once built).

## Scenario 1 — Deploy and connect (User Story 1, P1)

```bash
# Deploy a WebSocketApi with one channel, gateway-first, via the gateway management API
# (contracts/gateway-management-api.md) — see tests/framework/suites/it/resources/templates/
# websocket-api.yaml for the fixture this mirrors.
curl -X POST https://<gateway>/websocket-apis -d @websocket-api.yaml

# Connect and echo
websocat ws://<gateway>/chat/v1/rooms/42
> hello
< hello

# Delete, then confirm new handshakes are rejected
curl -X DELETE https://<gateway>/websocket-apis/<id>
websocat ws://<gateway>/chat/v1/rooms/42   # expect connection refused / 404
```

**Expected outcome**: `101` on connect, echoed frames both ways, `404` after delete (SC-001,
SC-002).

## Scenario 2 — Secure the handshake (User Story 2, P1)

```bash
websocat ws://<gateway>/chat/v1/rooms/42                         # no credential → expect 401
websocat "ws://<gateway>/chat/v1/rooms/42?access_token=<token>"  # valid token as query param → connects
websocat ws://<gateway>/chat/v1/rooms/42 --protocol "<token>,chat.v1"  # token in subprotocol → connects, 101 echoes only chat.v1
```

**Expected outcome**: credential-less and invalid-token attempts fail with the identical unified
`401` body REST uses; the token never reaches the mock backend's logs (grep the backend's access
log to confirm — SC-003).

## Scenario 3 — Connection lifetime and limits (User Story 3, P1)

```bash
# Deploy with idleTimeout=10s, maxConnectionDuration=30s, maxConnections=2 (see data-model.md §1)
websocat ws://<gateway>/chat/v1/rooms/42 &   # connection 1
websocat ws://<gateway>/chat/v1/rooms/43 &   # connection 2
websocat ws://<gateway>/chat/v1/rooms/44     # connection 3 — expect immediate 503, no queueing
```

**Expected outcome**: idle connections close within 1s of 10s; all connections close within 1s of
30s; the 3rd concurrent connection is rejected with `503` within 100ms (SC-004, SC-005).

```bash
# Separately, deploy with upstream.main.timeout set to idleTimeout=5s, maxConnectionDuration=15s,
# maxConnections=2 (see data-model.md §1) — these bound the gateway-to-backend pool, not the client
# connection above, and apply independently per upstream slot (FR-015–FR-017).
websocat ws://<gateway>/chat/v1/rooms/42     # connect through `main`; leave the backend side idle
                                              # past the 5s upstream idleTimeout
# expect: Envoy recycles/closes the pooled backend connection at the configured upstream idle
# timeout, independently of whether the client connection above is itself idle or active

websocat ws://<gateway>/chat/v1/rooms/42     # stay connected past the 15s upstream maxConnectionDuration
# expect: the gateway reconnects to the backend at that bound regardless of traffic

websocat ws://<gateway>/chat/v1/rooms/42 &   # drive `main` to its upstream maxConnections ceiling (2)
websocat ws://<gateway>/chat/v1/rooms/43 &
websocat ws://<gateway>/chat/v1/rooms/44 &   # 3rd connection through `main` — exceeds its ceiling
websocat ws://<gateway>/sandbox-rooms/42     # connect through `sandbox` while `main` is saturated
# expect: the sandbox connection succeeds — main's upstream ceiling doesn't affect sandbox's pool
```

**Expected outcome**: each upstream-slot bound (idle timeout, maximum connection duration, maximum
connections) is enforced on that slot's backend connection pool independently of the downstream
client connection's own settings and independently of the other slot (`sandbox` unaffected by
`main`'s ceiling) — FR-015–FR-017, Acceptance Scenario 6. The `maxConnections` ceiling is
best-effort, not exact (Known Limitations #6): a small overshoot is possible if the upstream
resolves to multiple backend hosts.

## Scenario 4 — Control plane (User Story 4, P2)

No CLI command targets platform-api's control-plane endpoints directly for this kind — `ap gateway
websocket-api`/`ap gateway apply` (T054-T057b) operate gateway-first, against the gateway
management API, same as Scenario 1's `curl` calls. This scenario exercises platform-api's own REST
surface instead (contracts/platform-api.md), the same way the console and API Portal will call it.

```bash
# Create from an AsyncAPI document (FR-052; contracts/platform-api.md § Resource endpoints)
curl -X POST https://<control-plane>/websocket-apis \
  -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
  -d '{"source": "url", "definition": "https://backend.example.com/specs/chat-asyncapi.yaml"}'
# → 201 Created: {"id": "<id>", "displayName": "Chat", "channels": [...], "upstream": {...}}

# Deploy to a registered gateway (contracts/platform-api.md § Deployment sub-resource)
curl -X POST https://<control-plane>/websocket-apis/<id>/deployments \
  -H "Authorization: Bearer <token>" -H "Content-Type: application/json" \
  -d '{"gatewayId": "<gateway-id>"}'
# connect through the gateway as in Scenario 1

# Undeploy
curl -X DELETE https://<control-plane>/websocket-apis/<id>/deployments/<deploymentId> \
  -H "Authorization: Bearer <token>"
# confirm new connections fail; existing ones last no longer than maxConnectionDuration
```

**Expected outcome**: matches the `/graphql-apis` control-plane endpoints' behavior exactly, per
contracts/platform-api.md — same scopes (`ap:websocket_api:manage`/`:deployment:manage`), same
response envelope, same deploy/undeploy semantics as `GraphQLApi`.

## Scenario 5 — Console and API Portal (User Story 5, P2)

Manual/UI validation (Cypress E2E once `it/ui/cypress/e2e/websocket-apis/tryout.cy.js`'s skipped
stub is implemented, per research.md §4):
1. Console create wizard → import `chat.asyncapi.yaml` by URL → see derived channels before
   creating.
2. Deploy → open the WebSocket test console → connect directly from the browser to the gateway
   (not proxied through the API Portal server — research.md §4) → send/receive a message.
3. Publish to the API Portal → confirm the AsyncAPI viewer shows live gateway URLs (research.md
   §5).

## Scenario 6 — Analytics (User Story 6, P3)

Open and close four connections: one normal client close, one idle-timeout close, one rejected
(bad credential), one hitting the connection limit. Query the analytics stream and confirm exactly
four events, each with the correct `closeReason`/`handshakeStatus`, and that the normal close is
not counted as an error (SC-008).

## Out of scope — do not attempt to validate

Per spec.md's Out of Scope section: per-message policies, message-level analytics, GraphQL
subscriptions over WebSocket, event-gateway `WebBrokerApi` (WebSocket↔Kafka) behavior. A failure in
any of these is not a regression in this feature.
