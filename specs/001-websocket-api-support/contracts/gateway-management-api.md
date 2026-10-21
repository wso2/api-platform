# Contract: Gateway Management API — `/websocket-apis`

Mirrors `/graphql-apis` exactly (same auth model, same response envelope, same error shape).
Final source of truth is `gateway/gateway-controller/api/management-openapi.yaml` — this document
fixes the surface `/speckit-tasks` must add there and in the generated Go handlers
(`pkg/api/handlers/websocket_api_handler.go`, per plan.md's Project Structure).

## Resource endpoints

| Method | Path | Notes |
|---|---|---|
| `POST` | `/websocket-apis` | Create. Body: `WebSocketAPIConfigData` (data-model.md §1). Validates per FR-003/FR-020; `400` on validation failure, naming the offending field/policy. |
| `GET` | `/websocket-apis` | List, same pagination/filtering convention as `/graphql-apis`. |
| `GET` | `/websocket-apis/{id}` | Get one. |
| `PUT` | `/websocket-apis/{id}` | Update — re-validates in full (a policy valid at creation may not stay handshake-only after an edit). |
| `DELETE` | `/websocket-apis/{id}` | Delete — new handshakes `404` immediately; live connections unaffected (resolved Clarification — bounded staleness only, per FR-031). |

## API key sub-resource

| Method | Path | Notes |
|---|---|---|
| `POST` | `/websocket-apis/{id}/api-keys` | Create. |
| `GET` | `/websocket-apis/{id}/api-keys` | List. |
| `PUT` | `/websocket-apis/{id}/api-keys/{keyId}` | Update. |
| `POST` | `/websocket-apis/{id}/api-keys/{keyId}/regenerate` | Regenerate. |
| `DELETE` | `/websocket-apis/{id}/api-keys/{keyId}` | Revoke. |

## Shared conventions (inherited, not new)

- Auth/authz: same bearer-token + scope model as every other gateway management endpoint (no new
  auth mechanism — GO-AUTH-007 applies: a valid token alone is not sufficient, the specific scope
  must be checked).
- Error shape: the platform's standard sterile error envelope (`error-handling.md`) — no raw
  validation internals, no stack traces.
- Version gating: deploying a `WebSocketApi` to a gateway below `MinWebSocketKindGatewayVersion`
  (research.md §6) is refused with a clear, generic error before translation is attempted.

## Data-plane behavior this contract implies (not a separate HTTP surface)

- A successful deploy causes the xDS translator to emit: one route per channel with
  `GET`-only matching and opt-in `upgrade_configs` (FR-010, FR-011), idle/max-stream-duration set
  from `connection.*` (FR-012), and — per research.md §2 — a router-level per-API connection
  counter enforcing `connection.maxConnections` with an immediate `503` on overflow, never queued.
- Independently, each upstream slot's cluster gets its own connection-pool idle timeout and
  maximum connection duration from `upstream.<main|sandbox>.timeout.*`, plus a native circuit
  breaker enforcing that slot's `maxConnections` (FR-015–FR-017) — a native Envoy mechanism is the
  right fit here, unlike the downstream case above. `timeout` reuses and extends the existing
  `UpstreamTimeout` schema (today only reachable via `RestApi`'s `UpstreamDefinition.timeout`),
  exposed directly on `Upstream` for the first time rather than a new, parallel schema.
- `NormalizePath`/`MergeSlashes`/`PathWithEscapedSlashesAction` are inherited from the shared
  listener HCM config automatically (research.md §7) — no separate per-kind contract needed for
  this.
