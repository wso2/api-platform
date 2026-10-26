---
type: Feature
title: WebSocket API support
description: Operators can expose a WebSocket backend through the gateway as a WebSocketApi, with per-channel routing, handshake-time policies, backend connection limits and connection timeouts.
resource: /gateway
tags: [gateway, websocket, routing, policies]
status: draft
generated: { by: claude-code/claude-opus-5-5, at: 2026-10-26T10:00:00Z }
sources:
  - id: contract
    resource: /gateway/gateway-controller/api/management-openapi.yaml
    title: Management API contract (WebSocketApi schemas, /websocket-apis paths and API-key sub-resource)
  - id: handler
    resource: /gateway/gateway-controller/pkg/api/handlers/websocket_api_handler.go
    title: REST handlers for WebSocketApi CRUD and API keys
  - id: validation
    resource: /gateway/gateway-controller/pkg/utils/websocket_deployment.go
    title: Deploy-time parsing, validation and context/vhost conflict checks
  - id: transform
    resource: /gateway/gateway-controller/pkg/transform/websocket.go
    title: Translation of a WebSocketApi into routes, clusters and policy chains
  - id: schema
    resource: /gateway/gateway-controller/pkg/storage/gateway-controller-db.sql
    title: websocket_apis table (other dialects change with it)
  - id: cli
    resource: /cli/src/cmd/gateway/websocketapi/
    title: ap gateway websocket-api commands
  - id: example-basic
    resource: /gateway/examples/chat-websocket.yaml
    title: Minimal WebSocketApi example
  - id: example-definition
    resource: /gateway/examples/chat-websocket-upstream-definition.yaml
    title: WebSocketApi with an upstream definition, connection limits and resilience
  - id: example-api-key
    resource: /gateway/examples/chat-websocket-api-key.yaml
    title: WebSocketApi secured with api-key-auth
  - id: feature
    resource: /tests/framework/suites/it/features/websocket_deploy.feature
    title: End-to-end feature file
---

# WebSocket API support

**Status**: In progress.

Before this feature the gateway refused every WebSocket upgrade, so a WebSocket backend could not be published through it. A `WebSocketApi` is a new artifact kind: it maps a context and a list of channels to a WebSocket backend, runs the normal gateway policies on the opening handshake, and then relays frames unchanged. The boundary to keep in mind: the gateway governs the handshake and the connection's lifetime, never the individual messages. It sits beside [REST APIs](/gateway/rest-api-support.md) and shares their upstream, resilience, vhost and API-key models.

The document has two parts. **Product view** answers what the feature does, why it is shaped this way and what it does not do. **Developer view** answers where the code is and how to try it. Nothing in the product view names a file; nothing in the developer view re-explains a use case.

---

## Product view

### Use cases

| # | Actor | Wants to | So that | Status |
|---|---|---|---|---|
| UC1 | API operator | Publish a WebSocket backend under a versioned context with named channels | Clients reach it through the gateway on `ws://` or `wss://` | Done |
| UC2 | API operator | Add, remove or change channels on a live API | New handshakes use the change while existing channels keep working | Done |
| UC3 | API operator | Require an API key or a JWT and limit how often clients can connect | Only entitled clients open connections, at a bounded rate | Done |
| UC4 | API operator | Cap backend connections, connect time, handshake time and idle time per API or channel | One API or a slow backend cannot exhaust the gateway | Done |
| UC5 | Platform operator | Bound how long any WebSocket connection may live | A revoked key or token stops working on open connections within a known time | Done |
| UC6 | API operator | Reject handshakes from origins it does not trust | A cookie-authenticated API cannot be hijacked from another website | Done (needs the newer cors policy) |
| UC7 | API operator | Serve the same API on chosen hostnames, with a sandbox host | Main and sandbox traffic reach the right backend | Done |
| UC8 | Client developer | Open a WebSocket over an existing HTTP/2 connection | A browser saves a connection when the operator allows it | Done (off by default) |
| UC9 | API operator | Manage WebSocket APIs and their keys from the `ap` CLI | The same tooling covers every API kind | Done |
| UC10 | API publisher | Design, deploy and subscribe to WebSocket APIs from the control plane | WebSocket APIs follow the same lifecycle as REST APIs | Planned |

### Decisions

| Decision | Choice | Why |
|---|---|---|
| Separate kind or an option on REST APIs | A separate `WebSocketApi` kind with channels instead of operations | A REST route that accepted an upgrade would become an unaudited tunnel; channels state the only paths that may upgrade. |
| Which routes may upgrade | Only WebSocket channel routes; every other route refuses an upgrade. Enforced by a test that a REST route never comes out upgradable. | Keeps an upgrade from being smuggled through any other API kind. |
| What policies see | Policies run once, on the handshake; frames are relayed without inspection, and a policy's body processing is switched off. | Header-based authentication, rate limiting and CORS work unchanged; per-message policies are a separate, larger feature. |
| How the backend is described | The same model as REST APIs: an inline URL, or a named upstream definition with targets, connect timeout, TLS and connection limits | One mental model across kinds; definitions can be shared by main and sandbox. |
| Backend connection limits | Each API gets its own backend pool. At the limit, a bounded number of handshakes wait for a free connection; beyond that a handshake is rejected at once with 503. | A long-lived connection holds a backend slot for hours, so an unbounded queue would only delay exhaustion. |
| Timeouts | `resilience.timeout` bounds only the handshake (504 on expiry); `resilience.idleTimeout` closes a connection with no frames either way. Channel overrides API overrides the gateway default. | Matches REST precedence while fitting a long-lived connection. |
| Total connection lifetime | One optional gateway-wide maximum, unset by default. It never shortens the handshake timeout. Enforced by a test that the cap reaches the API's backend clusters and not its routes. | Authorization happens only at the handshake, so a lifetime cap is how revocation reaches open connections; a route-level cap would disable the handshake timeout. |
| Handshake to an unknown channel or API | 404, and nothing is sent to any backend | Clear signal to clients; no accidental backend exposure. |
| Backend rejects the handshake | The backend's status passes through unchanged | Clients see the backend's real answer (for example 401 or 429). |
| WebSocket over HTTP/2 | Supported, off by default | Browsers fall back to HTTP/1.1 on their own, so it saves a connection but adds no capability. |
| Cross-site WebSocket hijacking | Two optional cors policy options reject a disallowed or missing `Origin` with 403, off by default | Browsers do not apply CORS to WebSocket handshakes; header and key authentication are immune, so only cookie-based APIs need it. |
| Hostnames | Same vhost model as REST APIs; one context may be reused on different vhosts, but not on the same vhost (409) | Lets operators split traffic by hostname without new concepts. |
| Upgrading an existing gateway | An existing SQLite database gains the WebSocket table on the first start of the new release | An upgrade must not need a manual migration step. |
| Upstream target weights | 0–100, same rules as REST APIs; weights split new connections only | An open connection stays on the backend it started on. |

### Scope

In scope:

- Create, read, update, delete and list WebSocket APIs through the gateway management API and the `ap` CLI.
- Channel routing with exact and parameterised paths, subprotocol pass-through, and query-string pass-through to the backend.
- Handshake-time policies: API key, JWT, basic auth, rate limiting, CORS with origin rejection, header policies.
- API keys for a WebSocket API: create, list, update, regenerate, revoke.
- Backend connection limits, connect, handshake and idle timeouts, and a gateway-wide lifetime cap.
- Main and sandbox upstreams, vhosts, `ws://` and `wss://` backends, and HTTP/2 extended CONNECT on the client side.

Out of scope:

- Inspecting, transforming, rate limiting or logging individual messages.
- A limit on client connections per API or per gateway.
- Control-plane (Platform API) management of WebSocket APIs, until UC10 is delivered.

### Current limitations

| Limitation | Impact | Tracking |
|---|---|---|
| Authentication reads credentials from headers only | Browsers cannot set headers on a WebSocket handshake, so browser clients cannot use API-key or JWT authentication | Not yet filed |
| Revoking a key or token, or deleting the API, does not close open connections | An open connection lasts until it goes idle, hits the lifetime cap, or either side closes it | Not yet filed |
| Policies that read a body deploy without error but their body processing never runs | An operator may believe a body transformation is active when it is not | Not yet filed |
| Removed or unknown fields in the definition are accepted and dropped | A setting from an older definition (for example an upstream-level idle timeout) silently has no effect | Not yet filed |
| Connection limits are enforced per gateway instance | With several replicas or several backend targets the total can exceed the configured value | Not yet filed |
| A new API key can take over ten seconds to start working | A client using a just-created key may get 401 for a short time | Not yet filed |
| Postgres and SQL Server deployments must create the WebSocket table themselves before upgrading | Without it the controller cannot load any configuration at startup | Not yet filed |
| The origin-rejection options need a cors policy release that is not yet published | Gateways built from released policies cannot use UC6 | Not yet filed |
| The control plane does not yet manage WebSocket APIs, and does not refuse deployment to older gateways | WebSocket APIs are gateway-first only | Not yet filed |
| No connection-level analytics event | Connection counts, durations and close reasons are not reported | Not yet filed |

### Open questions

| Question | Options | Owner | Needed by |
|---|---|---|---|
| How should browser clients authenticate? | A short-lived JWT in a query parameter (recommended); a cookie with origin rejection; a first-message token | Product owner | Control-plane release |
| Should body-reading policies be rejected at deploy time? | Reject when the controller knows a policy's body mode (recommended); keep the silent no-op | Gateway team | Next gateway release |
| Should a lifetime cap be configurable per API? | Keep gateway-wide only (current); add a per-API override bounded by the gateway value | Product owner | After first customer feedback |

---

## Developer view

### Entry points

Ordered the way a reader should explore. Every `sources` entry appears here. Rows marked "(shared, not tracked)" are for orientation only and are not in `sources`.

| Start here | Path | What you will find |
|---|---|---|
| Contract | `gateway/gateway-controller/api/management-openapi.yaml` | WebSocketApi schemas, `/websocket-apis` paths and the API-key sub-resource |
| HTTP | `gateway/gateway-controller/pkg/api/handlers/websocket_api_handler.go` | CRUD and API-key handlers |
| Validation | `gateway/gateway-controller/pkg/utils/websocket_deployment.go` | Field rules, limits against gateway maximums, context/vhost conflicts |
| Translation | `gateway/gateway-controller/pkg/transform/websocket.go` | Channel routes, per-API clusters, resilience resolution, lifetime cap |
| xDS | `gateway/gateway-controller/pkg/xds/translator.go` | (shared, not tracked) Upgrade gate on the listener, unknown-path handshake route, HTTP/2 extended CONNECT, cluster limits and lifetime cap |
| Config | `gateway/gateway-controller/pkg/config/config.go` | (shared, not tracked) `router.websocket.*` keys and defaults |
| Schema | `gateway/gateway-controller/pkg/storage/gateway-controller-db.sql` | `websocket_apis` table (other dialects change with it) |
| Policy engine | `gateway/gateway-runtime/policy-engine/internal/kernel/execution_context.go` | (shared, not tracked) Body processing switched off for WebSocket handshakes |
| CLI | `cli/src/cmd/gateway/websocketapi/` | `ap gateway websocket-api` and its `api-key` subcommands |
| Examples | `gateway/examples/chat-websocket.yaml`, `gateway/examples/chat-websocket-upstream-definition.yaml`, `gateway/examples/chat-websocket-api-key.yaml` | Minimal, upstream-definition and API-key variants |
| Tests | `tests/framework/suites/it/features/websocket_deploy.feature` | Executable specification of deploy, handshake, update and rejection behaviour |
| Storage startup | `gateway/gateway-controller/pkg/storage/sqlite.go` | (shared, not tracked) re-applies the guarded schema so upgraded databases gain new tables |
| Test backend | `tests/framework/testbench/services/websocket/websocket.go` | (shared, not tracked) RFC 6455 echo service with `rejectStatus`, `subprotocol` and `pushInterval` query options |

Reuse the upstream-definition and resilience parsing the transformer already shares with REST APIs rather than adding WebSocket-only variants.

### Examples

```bash
# Deploy, then open a connection and exchange a frame
ap gateway apply --file gateway/examples/chat-websocket.yaml
websocat ws://localhost:8080/chat/v1.0/rooms/general

# Secure it with an API key
ap gateway apply --file gateway/examples/chat-websocket-api-key.yaml
ap gateway websocket-api api-key create --id chat-websocket-secure-v1.0 --name my-key
websocat ws://localhost:8080/chat-secure/v1.0/rooms/general -H "X-API-Key: <generated-key>"

# Raw handshake status without a WebSocket client
curl -si --max-time 3 -H "Connection: Upgrade" -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" -H "Sec-WebSocket-Key: $(openssl rand -base64 16)" \
  http://localhost:8080/chat/v1.0/rooms/general | head -1
```
