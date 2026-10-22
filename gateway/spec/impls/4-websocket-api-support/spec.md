# Feature Specification: WebSocket API Support in the API Platform Gateway

**Feature Branch**: `websocket`
**Created**: 2026-10-13
**Status**: Draft
**Input**: User description: "Support WebSocket APIs as a first-class API type in API Platform, proxied through the standalone (Envoy-based) API Platform Gateway — not the event gateway. Allow creating WebSocket APIs the same way GraphQL APIs are created (gateway-first, control-plane and CLI), with QoS features, and Console and API Portal support. Use Bijira's WebSocket support as the reference product behaviour."

**Related**: GraphQL API support ([#3195](https://github.com/wso2/api-platform/issues/3195), backend [#3310](https://github.com/wso2/api-platform/pull/3310), UI/analytics [#3520](https://github.com/wso2/api-platform/pull/3520)) is the template this feature follows end to end. Envoy behaviour that this spec depends on was verified by a spike; see [research.md](research.md).

## Clarifications

### Session 2026-10-13

- Q: Which runtime proxies WebSocket APIs — the standalone gateway (Envoy + policy engine) or the event gateway? → A: The standalone gateway. The event gateway (`WebBrokerApi`, WebSocket ↔ Kafka) is out of scope for this feature.
- Q: Which existing feature is the template for scope and delivery? → A: GraphQL API support (#3310, #3520): a new artifact kind across gateway, platform-api, CLI, console, API Portal, analytics, and tests.

### Session 2026-10-22

- Q: Where do WebSocket connection bounds live? → A: WebSocketApi upstreams follow the RestApi model: `upstreamDefinitions` with a WebSocket-only `connection` block (maxConnectionDuration, maxConnections) and a WebSocket-only `timeout` (`WebSocketUpstreamTimeout`: connect, idleTimeout); inline `url` kept; API-level downstream `connection` removed — client-facing bounds are gateway-level config; shared `Upstream.timeout` and widened `UpstreamTimeout` reverted. FR-040 (per-API downstream limit) is removed and Q2 is moot. *(Revised 2026-10-22: `idleTimeout` moved from `connection` into `timeout`, see research.md decision 9.)*

## Background

A WebSocket API is a long-lived, full-duplex connection set up by an HTTP/1.1 `GET` with `Upgrade: websocket` (RFC 6455). After the server answers `101 Switching Protocols`, the connection carries WebSocket frames, not HTTP requests. A gateway therefore sees exactly one HTTP exchange per connection (the handshake), and then a byte stream that can stay open for hours.

What this means for the API Platform Gateway today (`upstream/main`):

- `gateway-controller/pkg/xds/translator.go` never sets `upgrade_configs`. Envoy rejects every upgrade with `403 upgrade_failed` (spike scenario 01).
- The policy engine (ext_proc) processes HTTP messages. It sees the handshake request and the `101` response; frames after the upgrade arrive, if at all, as raw RFC 6455 bytes.
- `platform-api` already has `APITypeWS`/`APISubTypeWebSocket` and `ws`/`wss` transports, and the API Portal already lists, documents and issues API keys for WebSocket APIs. No gateway artifact kind exists for them.

The product reference is Bijira's WebSocket support: AsyncAPI import, OAuth2 at the handshake (token accepted in `Sec-WebSocket-Protocol` for browsers), connection throttling, connection-level analytics, a WebSocket test console, and per-plan limits on concurrent connections, connection duration and idle time.

### WebSocket Primer

REST is request/response: the client always speaks first, and the server answers once. WebSocket keeps one connection open, and **either side can send a message at any time**. This suits chat, live dashboards, notifications and other server-push use cases.

**Handshake.** A connection starts as an HTTP/1.1 `GET` asking to switch protocols:

```http
GET /chat/v1/rooms/42 HTTP/1.1
Host: api.example.com
Upgrade: websocket
Connection: Upgrade
Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==
Sec-WebSocket-Version: 13
Sec-WebSocket-Protocol: chat.v1
```

The server accepts with `101 Switching Protocols`. From then on, the connection carries only WebSocket frames until one side closes it. The handshake is the only HTTP exchange the gateway sees, so it is the only point where the gateway can authenticate, authorize, route and rewrite headers.

**Frames.** After the handshake, data moves as frames:

| Frame | Purpose |
|---|---|
| Text | UTF-8 message, typically JSON |
| Binary | Raw bytes |
| Ping / Pong | Keep-alive and liveness check; ping frames also reset the gateway idle timer (spike 05) |
| Close | Orderly shutdown with a status code |

How frames behave:
- Client→server frames are masked (XOR with a random key).
- A large message may be split across several frames.
- Common close codes: `1000` normal, `1001` going away, `1008` policy violation, `1006` abnormal. `1006` is never sent on the wire; the client reports it when the connection drops without a close frame. That is what clients see when the gateway enforces an idle or maximum-duration limit.

**Other terms used in this spec:**

| Term | Meaning |
|---|---|
| `ws://`, `wss://` | WebSocket over plain TCP / over TLS (like `http://` / `https://`) |
| Subprotocol (`Sec-WebSocket-Protocol`) | Name of the application message format both sides agree on (e.g. `chat.v1`, `graphql-transport-ws`, `mqtt`). The server picks one of the client's offers. Browsers fail the connection if they offered subprotocols and the server picks none |
| Extension | Negotiated frame-level feature, mainly `permessage-deflate` (compression) |
| Browser limitation | The browser `WebSocket` API cannot set custom headers such as `Authorization`. Browser clients pass credentials in the query string or in `Sec-WebSocket-Protocol` (FR-030) |
| AsyncAPI | The contract format for event-driven APIs (the OpenAPI equivalent). Describes servers, channels, operations and messages. Versions 2.x and 3.x are in use |

### Channel vs Topic

The WebSocket protocol defines neither. Both words describe how messages are addressed, at two levels:

**Path level (visible to the gateway).**
- **Channel** is the AsyncAPI term. With the WebSocket binding, a channel is a URL path a client connects to (e.g. `/rooms/{roomId}`, `/notifications`). One channel is one WebSocket endpoint, and each connection belongs to exactly one channel.
- **Topic** is the term WSO2 API Manager and Bijira use in their UIs for the same thing: the API Manager editor lists a WebSocket API's "topics", and the Bijira test console asks you to "expand the topic you want to test".
- At this level, *channel* and *topic* are synonyms.

**Message level (invisible to the gateway).** Many applications open one connection and multiplex logical topics inside it, naming the topic in the message body, e.g. `{"action":"subscribe","topic":"prices.AAPL"}`. Socket.IO rooms, STOMP destinations, MQTT-over-WebSocket topics and GraphQL subscriptions all work this way. These topics exist only inside frames, and this feature passes them through untouched.

A third meaning, a **broker topic** (Kafka/MQTT), belongs to the event gateway's `WebBrokerApi`, which maps a WebSocket channel to a Kafka topic. It is not part of this feature.

| Concept | Visible to the gateway | What the gateway can do |
|---|---|---|
| Channel (AsyncAPI) = topic (WSO2 UI) = URL path | Yes, at the handshake | Route, authenticate, check per-channel scopes, limit connections, record analytics per channel |
| Topic inside messages (application protocol) | No | Nothing; frames pass through unchanged |
| Broker topic (Kafka/MQTT) | Not applicable | Out of scope (event gateway) |

**Naming in this feature.** The artifact, APIs and code use **channel**, matching AsyncAPI. How the console and API Portal label it is Open Question Q4.

### WebSocket and HTTP/2

Envoy fully supports HTTP/2, including WebSocket over HTTP/2, but that feature is off by default and the gateway doesn't enable it. WebSocket still works without it, because browsers fall back to HTTP/1.1.

**HTTP/2 has no `Upgrade`.** The RFC 6455 handshake relies on the HTTP/1.1 `Upgrade` and `Connection` headers. HTTP/2 removed both, because it multiplexes many requests as streams over one connection, so a single request can't take the connection over. RFC 8441 (2018) adds **extended CONNECT** instead:
- The client opens the WebSocket as one HTTP/2 stream with `:method = CONNECT`, `:protocol = websocket` and the path.
- The server must first announce support with `SETTINGS_ENABLE_CONNECT_PROTOCOL = 1`.
- Chrome and Firefox implement it; Safari support has not been checked.

**Where this shows up in the gateway.** The HTTPS listener prefers HTTP/2 (TLS ALPN offers `h2` first, `translator.go` `createDownstreamTLSContext`), so a browser on `https://` usually talks HTTP/2 to the gateway. When it then opens a WebSocket:

| Gateway setting | What the browser does | Result |
|---|---|---|
| Today: `allow_connect` not set | Sees no extended-CONNECT support and opens a separate HTTP/1.1 connection for the WebSocket | Works; costs one extra TCP + TLS connection |
| `http2_protocol_options.allow_connect: true` on the listener | Opens the WebSocket as a stream on its existing HTTP/2 connection | Works; no extra connection |

On the plain `http://` listener, clients use HTTP/1.1, so none of this applies.

**Backend side.** Envoy translates between the protocols. With `allow_connect` on, a browser can use HTTP/2 to Envoy while Envoy uses a normal HTTP/1.1 `Upgrade` handshake to the backend:

```text
Browser ── HTTP/2 extended CONNECT ──▶ Envoy ── HTTP/1.1 Upgrade ──▶ Backend
```

Most WebSocket backends speak only HTTP/1.1. The gateway's clusters already default to HTTP/1.1, and this spec keeps it that way (FR-010), because an HTTP/2 backend connection would need extended CONNECT support on the backend too.

**What this means for the design:**
- **Not required** for WebSocket to work: browsers fall back to HTTP/1.1 automatically.
- **Optional optimisation:** FR-011 makes `allow_connect` a gateway config flag, so operators can let browsers reuse their HTTP/2 connection.
- **Not tested yet:** the spike used plain HTTP/1.1 without TLS, so both the HTTP/2 path and the fallback need testing during implementation (FR-090).

### Request Flow Through the Gateway

```text
Client                         Gateway (Envoy + policy engine)              Backend
  │ GET /chat/v1/rooms/42                                                     │
  │ Upgrade: websocket                                                        │
  │──────────────────────────▶ match channel route /rooms/{roomId}            │
  │                            policy engine: authenticate, check scopes,     │
  │                            strip credentials ────────────────────────────▶│
  │                                                          101 Switching ◀──│
  │◀───────── 101 ───────────  policy engine done (its ext_proc stream closes)│
  │═══ frames ═══════════════▶ Envoy relays bytes; idle timer resets ════════▶│
  │◀══ frames ════════════════                                     ◀══════════│
  │  … client close, backend close, idle timeout or max duration …           │
  │                            access log / analytics: one event per connection│
```

### WebSocket Compared with REST

| | REST | WebSocket |
|---|---|---|
| Connection | Short, one exchange per request | One long-lived connection (minutes to hours) |
| Who sends | Client asks, server answers | Either side, at any time |
| Contract | OpenAPI (paths + methods) | AsyncAPI (channels + messages) |
| Gateway sees | Every request and response | The handshake, then opaque frames |
| Authentication | Every request | Once, at the handshake |
| Rate limiting | Requests per period | Connections (handshakes); messages are out of scope |
| Analytics | One event per request | One event per connection, at close |
| Config change takes effect | On the next request | Only on new connections (spike 09) |

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Deploy a WebSocket API to the gateway and connect through it (Priority: P1)

As a gateway operator, I deploy a WebSocket API to a standalone API Platform Gateway (YAML through the gateway management REST API, gateway-first, like `GraphQLApi`). Clients then open WebSocket connections to the gateway, and frames flow both ways to my `ws://`/`wss://` backend.

**Why this priority**: This is the minimum useful feature, and every other story builds on it. Without upgrade support in the router, no WebSocket API can be proxied at all.

**Independent Test**: Deploy a `WebSocketApi` with one channel pointing at an echo backend, connect a WebSocket client through the gateway, send messages, and receive the echoes. Then delete the API and check that new handshakes return `404`.

**Acceptance Scenarios**:

1. **Given** a `WebSocketApi` with context `/chat/v1` and channel `/rooms/{roomId}` is deployed, **When** a client opens `ws://<gateway>/chat/v1/rooms/42`, **Then** the gateway returns `101`, forwards the handshake to `<upstream>/rooms/42`, and relays text, binary, ping/pong and close frames both ways unchanged.
2. **Given** the API's upstream is `wss://backend:8443/ws`, **When** a client connects, **Then** the gateway connects to the backend over TLS using HTTP/1.1 and the upstream CA trust configured for the API.
3. **Given** a `RestApi` is deployed on the same gateway, **When** a client sends a WebSocket upgrade to a REST route, **Then** the gateway rejects it (`403`) and the REST route keeps working for normal requests.
4. **Given** the client offers `Sec-WebSocket-Protocol: chat.v1, chat.v2`, **When** the backend selects `chat.v1`, **Then** the client receives `chat.v1` in the `101` response.
5. **Given** the backend refuses the upgrade (any non-`101` status), **When** a client connects, **Then** the client receives that status and the access log records it.
6. **Given** a sandbox upstream is configured, **When** a client connects through the sandbox vhost, **Then** the connection is routed to the sandbox backend (same rules as `GraphQLApi`).

---

### User Story 2 - Secure the handshake (Priority: P1)

As an API publisher, I attach authentication and authorization policies (API key, JWT, OAuth2/opaque token, basic auth, IP and header based policies) to a WebSocket API. The gateway enforces them on the handshake, including for browser clients that cannot set an `Authorization` header.

**Why this priority**: A WebSocket API without access control cannot be exposed. The handshake is the only point where the gateway can authenticate the connection.

**Independent Test**: Attach `api-key-auth`, connect without a key (rejected `401`), with a revoked key (rejected), and with a valid key in a header, a query parameter and `Sec-WebSocket-Protocol` (all accepted). Check that the backend and the access log never see the key.

**Acceptance Scenarios**:

1. **Given** `jwt-auth` is attached at API level, **When** a client connects without a token or with an invalid one, **Then** the handshake fails with `401` and the identical unified auth-failure body used for REST, and the backend is never contacted.
2. **Given** a browser client sends the token as a query parameter (e.g. `?access_token=...`) or as a `Sec-WebSocket-Protocol` entry, **When** the token is valid, **Then** the connection is accepted, the token is removed before the request reaches the backend, and the token is not written to access logs or analytics.
3. **Given** the token was sent in `Sec-WebSocket-Protocol` alongside a real subprotocol, **When** the connection is accepted, **Then** the `101` response echoes only the real subprotocol and never the token.
4. **Given** a channel requires scope `chat:write`, **When** a token without that scope connects to that channel, **Then** the handshake fails with `403`.
5. **Given** an origin-validation policy allows only `https://app.example.com`, **When** a page from another origin opens a connection, **Then** the handshake is rejected. This is the cross-site WebSocket hijacking (CSWSH) defence, since CORS does not apply to WebSockets.
6. **Given** a policy that needs the message body (e.g. a payload transformer or a body guardrail) is attached to a WebSocket API, **When** the API is deployed, **Then** deployment fails with a validation error naming the policy (see FR-020).

---

### User Story 3 - Control connection lifetime and limits (Priority: P1)

As a gateway operator, I configure how long a client connection may stay idle and how long it may live in total (gateway-wide), and as an API publisher I bound each backend connection pool (idle time, lifetime, concurrent connections) on an upstream definition, so that one API cannot exhaust the gateway or its backend.

**Why this priority**: A connection that never closes ties up gateway and backend resources indefinitely, and stale authorization lives as long as the connection does (see US-2, FR-031). Bijira enforces exactly these limits.

**Independent Test**: Set the gateway's `router.websocket.downstream.idle_timeout_default: 10s` and `max_connection_duration_default: 30s`. An idle client is closed at 10s, a client sending pings survives past 10s, and every client is closed at 30s.

**Acceptance Scenarios**:

1. **Given** the downstream idle timeout is 10s, **When** no frame (data or ping/pong) crosses the connection for 10s, **Then** the gateway closes it.
2. **Given** a client sends ping frames more often than the idle timeout, **When** no data frames are sent, **Then** the connection stays open.
3. **Given** the downstream maximum connection duration is 30s, **When** a connection reaches 30s, **Then** the gateway closes it whatever its traffic.
4. **Given** any `WebSocketApi`, **When** it is deployed, **Then** the gateway-wide downstream values apply (FR-013); there is no per-API override. An unlimited lifetime is never the implicit default.
5. *(Removed 2026-10-22 — the per-API downstream connection limit no longer exists; downstream connection bounds are gateway-level configuration.)*
6. **Given** the upstream definition `main` references sets `connection.maxConnections` to N, **When** more than N handshakes need a backend connection, **Then** at most N further handshakes wait for a free backend connection and any beyond that get `503` (FR-016).

---

### User Story 4 - Manage WebSocket APIs from the control plane and CLI (Priority: P2)

As an API publisher, I create a WebSocket API in platform-api from an AsyncAPI definition, deploy it to one or more gateways, undeploy and restore deployments, and manage API keys. I can do the same from the `ap` CLI, as for GraphQL APIs.

**Why this priority**: Gateway-first deployment (US-1) works without the control plane, but most users manage APIs centrally. This is the GraphQL CP-based and CLI-based approaches applied to WebSocket.

**Independent Test**: Create a WebSocket API in platform-api by uploading an AsyncAPI file, deploy it to a registered gateway, connect through that gateway, undeploy, and confirm new connections fail.

**Acceptance Scenarios**:

1. **Given** an AsyncAPI 2.x or 3.x document with `ws`/`wss` servers and channels, **When** I create a WebSocket API from a file, a URL or inline content, **Then** platform-api validates it, derives channels, context and upstream, and stores the API.
2. **Given** an AsyncAPI document using a non-WebSocket protocol only (e.g. `kafka`, `mqtt`), **When** I try to create a WebSocket API from it, **Then** creation fails with a validation error.
3. **Given** a WebSocket API, **When** I deploy it to a gateway whose version does not support the `WebSocketApi` kind, **Then** the deployment is refused before translation (same version gating as `GraphQLApi`).
4. **Given** a WebSocket API is deployed, **When** I undeploy it, **Then** new connections fail, and existing connections end no later than the gateway's downstream maximum connection duration (see FR-031).
5. **Given** a gateway is configured from the control plane, **When** it connects or reconnects, **Then** WebSocket APIs and their API keys are included in bulk sync and live events like every other kind.
6. **Given** the `ap` CLI, **When** I run `ap gateway websocket-api list|get|delete` and `ap gateway websocket-api api-key create|list|regenerate|revoke|update`, **Then** the commands behave like their `graphql-api` counterparts.

---

### User Story 5 - Console and API Portal experience (Priority: P2)

As an API publisher, I create, configure, deploy, test and monitor a WebSocket API from the console like a REST or GraphQL API. As an application developer, I discover it in the API Portal, read its AsyncAPI definition, get API keys and try it out.

**Why this priority**: Feature parity in the UI is what makes WebSocket a first-class API type, as #3520 did for GraphQL.

**Independent Test**: Use the console wizard to create a WebSocket API from an AsyncAPI URL, deploy it, open the WebSocket test console, connect to a channel, send a message and see the reply, then publish it to the API Portal.

**Acceptance Scenarios**:

1. **Given** the console's create flow, **When** I choose WebSocket, **Then** I can import an AsyncAPI definition (file, URL or paste) or start from an endpoint, and I see the derived channels before creating.
2. **Given** a deployed WebSocket API, **When** I open its Test console, **Then** I can pick an environment and channel, fill in channel parameters, connect with a generated test key or token, and send and receive messages.
3. **Given** the API's Develop page, **When** I attach policies, **Then** only policies valid for WebSocket APIs (handshake-only, FR-020) can be added, at API or channel level.
4. **Given** the API is published to the API Portal, **When** a developer opens it, **Then** the AsyncAPI definition shows the live gateway URLs, API keys can be generated, and the try-out connects through the gateway.
5. **Given** the API's Overview, Deploy, Insights, Compliance and Observability pages, **When** I open them, **Then** they show WebSocket-specific information (channels, connection metrics) in place of REST resources.

---

### User Story 6 - Connection analytics and observability (Priority: P3)

As an API publisher or operator, I see how many connections were opened, how long they lasted, how many bytes they carried, which closed abnormally, and which were rejected (auth, throttled, backend refused).

**Why this priority**: Operability matters, but the API works without it. Bijira offers connection counts, errors and throttled requests.

**Independent Test**: Open and close several connections, including a rejected one, a throttled one, an idle-timed-out one and a normally closed one. Each appears once in analytics with the correct outcome.

**Acceptance Scenarios**:

1. **Given** a connection ends, **When** analytics is published, **Then** exactly one event is emitted for it, with API, channel, application/subscription, status `101`, connection duration, bytes in/out, and a close reason (client close, idle timeout, max duration, backend close, error).
2. **Given** a normal client close, **When** the event is classified, **Then** it is not counted as an error. Envoy flags every client close as `DC downstream_remote_disconnect`, which must map to a normal outcome.
3. **Given** a handshake rejected by policy or backend, **When** analytics is published, **Then** it is recorded as a failed connection attempt with the rejecting status.
4. **Given** connection-level events, **When** latency dashboards are shown, **Then** WebSocket connection duration is not mixed into request latency figures.

---

### Edge Cases

- **Policy that needs a body.** Asking the policy engine for the request body on an upgrade deadlocks the handshake: Envoy waits for a body that never comes before the `101` (spike 07, 08, 08b). Buffering the response body has the same effect (08c). Such chains must be rejected at deploy time and must never reach Envoy.
- **Undeploy, key revocation, subscription change, policy change, token expiry** while connected. The live connection is not affected; Envoy applies new routes to new streams only (spike 09). See FR-031.
- **Gateway restart, rollout or listener change.** All connections on that instance close at once and clients reconnect together. Clients must back off and retry; the gateway must survive the reconnect burst.
- **HTTP/2 clients.** The HTTPS listener advertises `h2` first. Without RFC 8441 support, browsers open a separate HTTP/1.1 connection for WebSockets. With it (FR-011), Envoy converts the extended `CONNECT` into an HTTP/1.1 upgrade toward the backend.
- **Backend speaks HTTP/2 only.** Not supported; WebSocket upstreams use HTTP/1.1 (FR-010).
- **`permessage-deflate`.** Negotiated end to end; the gateway passes compressed frames through untouched.
- **Large frames and slow consumers.** Envoy flow control with `per_connection_buffer_limit_bytes` (default 1 MiB) applies; the gateway never buffers whole messages.
- **Close semantics.** When the gateway ends a connection (idle timeout, max duration), the client sees an abnormal close (`1006`), not a close frame, because Envoy does not speak the WebSocket protocol (spike 05, 10).
- **Query string carries a token.** It must not reach access logs, analytics, tracing or the backend (spike 02 showed it in all three without redaction).
- **Channel parameters.** `/rooms/{roomId}` must match one path segment only; unmatched paths return `404`.
- **Same path, different methods.** Only `GET` handshakes are routed for a channel; other methods return `404`/`405`.
- **Many connections from one client.** Bounded by the gateway-wide downstream connection limit and, toward the backend, by the upstream definition's `connection.maxConnections`.

## Requirements *(mandatory)*

### Functional Requirements

#### Artifact and gateway management API

- **FR-001**: The gateway MUST support a new artifact kind `WebSocketApi` (`apiVersion: gateway.api-platform.wso2.com/v1`), stored and versioned like `GraphQLApi`, with the fields in [Key Entities](#key-entities).
- **FR-002**: The gateway management API MUST expose `/websocket-apis` and `/websocket-apis/{id}` (create, list, get, update, delete) and `/websocket-apis/{id}/api-keys[...]` (create, list, regenerate, update, revoke), mirroring `/graphql-apis`.
- **FR-003**: The gateway MUST validate a `WebSocketApi` on create and update: required fields, at least one channel, unique channel paths, valid `ws`/`wss` upstream URLs (checked by the shared upstream URL validator used by REST/MCP/LLM), resolvable policies (like `ValidateGraphQLAPIPolicies`), handshake-only policies (FR-020), `upstream.<slot>` setting exactly one of `url`/`ref` with every `ref` resolving to an `upstreamDefinitions` entry, and valid upstream-definition `timeout`/`connection` settings.
- **FR-004**: `WebSocketApi` MUST support `main` and `sandbox` upstreams with the same vhost rules as `GraphQLApi`, following the `RestApi` upstream model: each slot takes either an inline `ws://`/`wss://` `url` (gateway-wide upstream connection defaults; `tls` on an inline url is rejected) or a `ref` to a `spec.upstreamDefinitions` entry — not both, not neither; main and sandbox may reference the same definition (one shared cluster). A `WebSocketUpstreamDefinition` has the `RestApi` definition's shape — `name` (`^[a-zA-Z0-9\-_]+$`, 1–100 chars, unique), `basePath` (starts with `/`, no trailing `/`), `upstreams[{url, weight}]` (host and port only, one scheme per definition, weight 0–100), `timeout` (WebSocket-only `WebSocketUpstreamTimeout`: `connect` and the backend `idleTimeout`), `tls` (`identity`, `trustedCAs`, `verifyHostName`; every target `wss://`; validated against the gateway certificate store) — plus a WebSocket-only `connection` block: maximum connection duration and maximum concurrent connections to that backend (FR-015–FR-017). Upstream auth (OAuth2/mTLS to the backend) and host rewrite apply as for REST.

#### Router (Envoy) translation

- **FR-010**: The translator MUST generate one Envoy route per channel and upstream slot, matching `:method == GET` and the channel's full path (context + version + channel path, with parameters), routed to an HTTP/1.1 cluster. WebSocket upstream clusters MUST NOT negotiate HTTP/2 (no `h2` ALPN upstream).
- **FR-011**: Upgrades MUST be opt-in per route. The HCM gets `upgrade_configs: [{upgrade_type: websocket, enabled: false}]`, and only `WebSocketApi` routes set route-level `upgrade_configs: [{upgrade_type: websocket, enabled: true}]`, so no REST, GraphQL, MCP or LLM route can be upgraded (spike 02). Accepting WebSockets over HTTP/2 (RFC 8441, `http2_protocol_options.allow_connect`) MUST be controlled by a gateway config flag.
- **FR-012**: For `WebSocketApi` routes, the translator MUST set `idle_timeout` and `max_stream_duration.max_stream_duration` from the gateway config `router.websocket.downstream.idle_timeout_default` / `max_connection_duration_default` (no per-API override), and MUST NOT attach a retry policy. The route `timeout` does not end an upgraded connection (spike 03) and may stay at the gateway default.
- **FR-013**: The gateway config (`[router]`) MUST provide the client-facing WebSocket idle timeout and maximum connection duration applied to every `WebSocketApi`. Client-facing bounds are gateway-level configuration only; the artifact carries no downstream `connection` block.
- **FR-014**: The existing Lua filter MUST keep working on upgrade requests: header-only path and host rewrite, with no body access.
- **FR-015**: For each upstream cluster (one per referenced definition or inline-`url` slot), the translator MUST set the cluster's connection-pool `idle_timeout` and `max_connection_duration` from the referenced definition's `timeout.idleTimeout`/`connection.maxConnectionDuration` (gateway-wide upstream defaults for an inline `url` or an omitted value), via `typed_extension_protocol_options` (`HttpProtocolOptions.common_http_protocol_options`). This is independent of the downstream route settings in FR-012: it bounds how long the gateway keeps a connection open to the backend, not how long a client may stay connected to the gateway. These fields live only in the WebSocket-only `WebSocketUpstreamTimeout` (`connect`, `idleTimeout`) and `WebSocketUpstreamConnection` (`maxConnectionDuration`, `maxConnections`) schemas; the shared `Upstream` schema has no `timeout` property and the shared `UpstreamTimeout` (also used by `RestApi.upstreamDefinitions[].timeout`) stays `connect`-only and unchanged. Each cluster is keyed per API (`upstream_WebSocketApi_<uuid>_<definition-name>`, or `..._<slot>` for an inline url), so two APIs on one backend never share pool bounds.
- **FR-016**: The gateway MUST support a per-upstream-definition cap on concurrent connections to the backend (`connection.maxConnections`), implemented via the cluster's native circuit breaker (`circuit_breakers.thresholds[].max_connections`); at the limit, at most `maxConnections` further handshakes wait for a free backend connection, and beyond that a handshake gets `503`. Envoy's cluster circuit breaker is the correct native primitive here: the bounded resource — outbound connections from this gateway instance to the backend — is itself per-instance, so the per-pod scope and burst tolerance observed in spike 11/11b/11c (research.md) are acceptable behaviour, not a gap to work around. This is a best-effort cap, not an exact one: Envoy guarantees at least one connection per selected host, so a multi-target upstream definition can exceed the configured value by a small amount — not yet exercised by the single-host spike (Known Limitations, research.md Key Decision 7).
- **FR-017**: The gateway config (`[router]`) MUST provide defaults (`router.websocket.upstream.*_default`) and upper bounds (`router.websocket.upstream.*_max`) for upstream idle timeout, maximum connection duration and maximum concurrent connections. Omitted `timeout.idleTimeout`/`connection` values and inline-`url` slots use the defaults; a definition value above the configured bound MUST be rejected at deploy time (`400`).

#### Policy engine

- **FR-020**: A `WebSocketApi` policy chain MUST be handshake-only. Deployment MUST be rejected when any policy in the chain declares request-body or response-body processing (`RequiresRequestBody`/`RequiresResponseBody`). The policy definition schema MUST let a policy declare that it supports `WebSocketApi`.
- **FR-021**: Even if FR-020 is bypassed, the policy engine MUST return `RequestBodyMode`/`ResponseBodyMode` `NONE` for a `WebSocketApi` route (fail-safe against the handshake deadlock). The response side already resolves to `NONE` because `responseHasNoBody()` treats `1xx` as bodiless.
- **FR-022**: The policy SDK MUST add `APIKindWebSocket` (`"WebSocketApi"`) to `policyv1alpha2.APIKind`, and the shared context MUST carry the API kind and the matched channel, so policies can tell a WebSocket handshake apart.
- **FR-023**: Header-phase policies MUST work on the handshake unchanged. This includes auth (`api-key-auth`, `jwt-auth`, `basic-auth`, `opaque-token-auth`), `subscription-validation`, header manipulation (`set-headers`, `remove-headers`), `host-rewrite`, `request-rewrite` (path), `backend-jwt`, `interceptor-service` when configured for headers only, and handshake-counting rate limits (`basic-ratelimit`, `advanced-ratelimit`). Each of these MUST be checked against FR-020 rather than assumed header-only.

#### Security

- **FR-030**: Auth policies used with WebSocket APIs MUST accept the credential from a header, a configurable query parameter, or a `Sec-WebSocket-Protocol` entry. On success the credential MUST be removed from the upstream request, and from `Sec-WebSocket-Protocol` in both the upstream request and the `101` response.
- **FR-031**: Credentials are validated only at the handshake. Because live connections are not affected by undeploy, revocation, subscription or policy changes (spike 09), every `WebSocketApi` MUST have a finite effective `maxConnectionDuration` (FR-013), which bounds how long revoked access can last. Actively closing connections on undeploy or revocation is a follow-up (Open Question Q3).
- **FR-032**: Query-string credentials MUST be redacted from access logs, the analytics/ALS stream, tracing span attributes, and any log line, per GO-AUTH-003.
- **FR-033**: An origin-validation policy (exact-match allowlist per the `go-cors-validation` rule) MUST be available for WebSocket handshakes as the CSWSH defence.
- **FR-034**: Channel-level scopes MUST be enforceable on the handshake (`jwt-auth` scopes per channel), as `graphql-scope-authz` does for GraphQL operations.

#### Connection limits and throttling

- **FR-040**: *(Removed 2026-10-22.)* A per-API downstream concurrent connection limit no longer exists — downstream connection bounds are gateway-level configuration (FR-042); backend-side concurrency is bounded per upstream definition (FR-016).
- **FR-041**: Request-count rate-limit policies on a `WebSocketApi` MUST count handshakes (connection-rate limiting). Message-rate limiting is out of scope (see Out of Scope).
- **FR-042**: The gateway SHOULD set a gateway-wide downstream connection limit (Envoy `global_downstream_max_connections` resource monitor). Envoy currently warns at startup that none is configured.

#### Control plane (platform-api)

- **FR-050**: platform-api MUST support WebSocket APIs as a core artifact kind (`constants.WebSocketApi`, `ValidArtifactKinds`), with a `websocket_apis` table in all four schema dialects (new table, R0-compliant), repository, service and handlers mirroring `graphql_apis`.
- **FR-051**: platform-api MUST expose `/websocket-apis` endpoints mirroring `/graphql-apis`: CRUD, AsyncAPI definition get/update, definition validation, gateways, API keys, deployments (deploy, undeploy, restore), and builds. They MUST be guarded by new scopes `ap:websocket_api:manage`, `ap:websocket_api:read`, `ap:websocket_api:deployment:manage` and `ap:websocket_api:deployment:read`, added to `role-to-scope-mapping.yaml` and the AI Workspace BFF default OIDC scopes.
- **FR-052**: platform-api MUST accept AsyncAPI 2.x and 3.x (JSON or YAML) from a file, a URL (fetched through the shared SSRF-safe client) or inline content. It MUST derive channels (2.x `channels`; 3.x `channels` + `operations`), channel parameters, servers with protocol `ws`/`wss` (upstream), and info (name, version, description). Documents with no `ws`/`wss` server and no explicit upstream MUST be rejected. Size and parse-time limits apply.
- **FR-053**: The gateway translator registry MUST register a `WebSocket` kind with `MinGatewayVersion` set to a new `gwversion.MinWebSocketKindGatewayVersion` (`NoLTSRelease` until a gateway release ships it), so deploying to an older gateway is refused before translation.
- **FR-054**: WebSocket APIs and their API keys MUST be included in connected-gateway bulk sync, live deployment events, gateway-first artifact push (gateway → CP, read-only in CP), and API key sync, as for `GraphQLApi`.
- **FR-055**: Publishing to the API Portal MUST send the AsyncAPI definition with live gateway URLs (the portal already renders WebSocket APIs and issues API keys for them).

#### CLI

- **FR-060**: The `ap` CLI MUST add `ap gateway websocket-api` (`list`, `get`, `delete`) and `ap gateway websocket-api api-key` (`create`, `list`, `regenerate`, `update`, `revoke`), mirroring `graphql-api`. Docs go in `docs/cli/gateway/README.md`.

#### Console

- **FR-070**: The console MUST support WebSocket APIs end to end, mirroring #3520:
  - a creation wizard (AsyncAPI file/URL/paste or endpoint, with channel preview);
  - Overview (channels in place of resources), Deploy, Develop (handshake-only policy picker at API and channel level), Edit, Publish;
  - Insights, Compliance and Observability pages, wired into navigation, routes and scope gates.
- **FR-071**: The console MUST provide a WebSocket test console: environment and channel selection, channel parameter inputs, connect/disconnect, a send box, and a timestamped message log. It sends a generated test key or token as a query parameter or `Sec-WebSocket-Protocol` entry, because a browser cannot set headers on a WebSocket handshake.

#### Analytics and observability

- **FR-080**: Analytics MUST emit one event per connection at close, with API kind `WebSocketApi`, channel, application, subscription, connection duration, bytes in/out, and a close reason derived from Envoy response flags and details (e.g. `DC` → client close, `SI`/`stream_idle_timeout` → idle timeout, `max_duration_timeout` → max duration, `UC`/`UF` → backend close/failure).
- **FR-081**: The fault classifier MUST treat a normal close (`101` + `DC`) as success, and analytics MUST keep WebSocket connection duration out of request-latency metrics.
- **FR-082**: Rejected handshakes MUST produce a normal request analytics event with the rejecting status.
- **FR-083**: The Moesif publisher MUST map WebSocket connection events to a sensible shape (as #3520 did for GraphQL properties).

#### Testing

- **FR-090**: Gateway integration tests (`gateway/it`) and v2 framework tests (`tests/framework/suites/it`) MUST cover: deploy, connect and echo; REST route not upgradable; auth at the handshake (header, query, subprotocol, rejected credentials, token stripped); handshake-only policy validation; idle and max-duration closes; `upstream.<slot>.ref` to `upstreamDefinitions`; upstream connection-pool idle timeout, max duration and max-connections enforcement (FR-015–FR-017); undeploy behaviour; sandbox; API keys; analytics events; `wss://` upstreams; over the HTTPS listener, both the HTTP/1.1 fallback (`allow_connect` off) and WebSocket over HTTP/2 (`allow_connect` on). A WebSocket echo backend MUST be added to the testbench (like `graphqlbackend`).

### Key Entities

- **WebSocketApi** (gateway artifact). Example:

  ```yaml
  apiVersion: gateway.api-platform.wso2.com/v1
  kind: WebSocketApi
  metadata:
    name: chat-websocket-v1
    annotations:
      gateway.api-platform.wso2.com/project-id: default
  spec:
    displayName: Chat
    version: v1
    context: /chat
    upstreamDefinitions:              # RestApi upstreamDefinition shape + WebSocket-only `timeout`/`connection` (FR-004)
      - name: chat-backend
        basePath: /ws
        upstreams:                    # ws:// or wss://, host and port only, one scheme per definition
          - url: wss://chat-backend-1:8443
            weight: 80
          - url: wss://chat-backend-2:8443
            weight: 20
        timeout:
          connect: 5s                 # WebSocketUpstreamTimeout — connect + idleTimeout
          idleTimeout: 120s           # upstream connection-pool idle timeout (FR-015); default/max from gateway config
        connection:                   # WebSocketUpstreamConnection — maxConnectionDuration + maxConnections
          maxConnectionDuration: 30m  # upstream connection lifetime cap (FR-015); always finite
          maxConnections: 500         # concurrent connections to this backend (FR-016), per gateway instance
        tls:                          # requires every target to be wss://
          identity: chat-identity
          trustedCAs: [chat-ca]
    upstream:
      main:
        ref: chat-backend
      sandbox:
        url: ws://chat-sandbox:8080/ws  # inline url: gateway-wide upstream defaults, no tls
    # No API-level `connection`: client-facing idle timeout / max duration are gateway config (FR-013)
    channels:
      - path: /rooms/{roomId}
        policies:
          - name: jwt-auth
            version: v1
            params:
              issuers: [PrimaryIdp]
              scopes: { anyOf: ["chat:write"] }
      - path: /notifications
    policies:                      # API level, handshake-only (FR-020)
      - name: api-key-auth
        version: v1
        params:
          key: X-API-Key
          in: header               # header | query | subprotocol (FR-030)
  ```

- **Channel**: an AsyncAPI channel exposed as one WebSocket endpoint. It has a path (with optional parameters), optional channel-level policies, and optional message schemas (documentation only; not enforced by the gateway).
- **Connection settings**: two independent sets apply. Downstream (client-facing) idle timeout and maximum connection duration are gateway-level configuration only, with no per-API override (FR-012, FR-013). Upstream (backend-facing) idle timeout lives on each `upstreamDefinitions[].timeout` (`WebSocketUpstreamTimeout`, beside `connect`), and maximum connection duration and maximum concurrent connections on each `upstreamDefinitions[].connection` block (`WebSocketUpstreamConnection`) (FR-015–FR-017), all defaulted and bounded by gateway-wide config; the shared `UpstreamTimeout` stays `connect`-only.
- **WebSocket API (control plane)**: a platform-api artifact holding the AsyncAPI definition, derived channels, upstream (definitions and slots) and policies, with deployments, builds and API keys like `GraphQLApi`.
- **Connection event (analytics)**: one record per connection: API, channel, application, subscription, start time, duration, bytes in/out, close reason, and handshake status.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A WebSocket API can be deployed gateway-first, from platform-api, or from the CLI, and a client can exchange messages through the gateway, in under 5 minutes from an AsyncAPI file.
- **SC-002**: 100% of upgrade attempts on non-`WebSocketApi` routes are rejected.
- **SC-003**: Handshakes with missing, invalid or revoked credentials are rejected 100% of the time, and credentials never appear in backend requests, access logs or analytics.
- **SC-004**: Idle connections close within 1s of the configured downstream idle timeout, and all connections close within 1s of the configured downstream maximum connection duration.
- **SC-005**: *(Superseded 2026-10-22 — per-API downstream limit removed.)* With an upstream definition `connection.maxConnections` of N, once N backend connections and N waiting handshakes are in use, a further handshake is rejected with `503`.
- **SC-006**: Handshake latency added by the gateway (policy chain with one auth policy) is under 10 ms at p95 on the reference perf setup. Frame relay adds under 1 ms at p95.
- **SC-007**: One gateway instance sustains at least 10,000 concurrent idle WebSocket connections with the policy engine holding zero ext_proc streams for them (spike 06).
- **SC-008**: Each closed connection yields exactly one analytics event with the correct close reason, and normal closes are never classified as errors.
- **SC-009**: Deploying a `WebSocketApi` with a body-processing policy fails validation 100% of the time, with a message naming the policy.

## Assumptions

- WebSocket support targets the standalone gateway (gateway-controller + gateway-runtime: Envoy v1.39.0 and policy engine). The event gateway and its `WebBrokerApi` (WebSocket ↔ Kafka) are unchanged.
- The gateway proxies WebSocket connections transparently. Message content, AsyncAPI message schemas and subprotocol semantics are not interpreted.
- A WebSocket backend accepts HTTP/1.1 upgrades. HTTP/2-only WebSocket backends (RFC 8441 upstream) are not supported in this phase.
- Clients handle reconnects (with backoff) after gateway restarts, rollouts and enforced closes. Clients that stay idle send pings more often than the idle timeout.
- Policies run once per connection, at the handshake. The policies in the build manifest that work on headers only are reusable as-is, with FR-030 changes for credential location.
- AsyncAPI parsing in platform-api uses a maintained, Apache-2.0-compatible Go library (to be selected per `dependency-management.md`), or a minimal in-repo parser for the subset needed (servers, channels, operations, parameters).
- Gateway version gating starts at `NoLTSRelease`, like GraphQL. The first gateway release that ships the kind sets `MinWebSocketKindGatewayVersion`.

## Dependencies

- GraphQL API support (#3310 merged, #3520 pending): reused patterns for the artifact kind, gateway-version gating, API key sync, portal publication, console pages, analytics properties and IT testbench.
- Policy implementations in `wso2/gateway-controllers` (auth policies for FR-030; an origin-validation policy for FR-033; a `WebSocketApi` support declaration in policy definitions for FR-020).
- Envoy v1.39.0 features: route-level `upgrade_configs`, `max_stream_duration`, HTTP/2 `allow_connect`, `global_downstream_max_connections` resource monitor, upstream `HttpProtocolOptions` (`common_http_protocol_options.idle_timeout`/`.max_connection_duration`) and cluster `circuit_breakers.thresholds[].max_connections` (FR-015–FR-017).
- API Portal: existing WebSocket listing, AsyncAPI rendering and API key issuance.

## Security Considerations

- **Authentication at the handshake only.** Bounded by a mandatory finite gateway-level maximum connection duration (FR-031). Document clearly that revocation is not immediate.
- **Credential exposure.** Browser credentials travel in the query string or `Sec-WebSocket-Protocol`. They must be stripped before the upstream and redacted from every log/analytics sink (FR-030, FR-032).
- **Cross-site WebSocket hijacking.** Browsers send cookies with WebSocket handshakes and CORS does not apply, so an origin allowlist is required for cookie-authenticated APIs (FR-033).
- **Upgrade smuggling.** Upgrades are disabled at HCM level and enabled only per `WebSocketApi` route, so no other route can be turned into a tunnel (FR-011). Path normalization (`NormalizePath`, `MergeSlashes`, `PathWithEscapedSlashesAction`) applies to handshakes as to every request.
- **Resource exhaustion.** Covered by gateway-level downstream idle and duration bounds, the gateway-wide connection limit, and Envoy's per-connection buffer limit (FR-012, FR-013, FR-042), and by idle/duration/connection-count bounds applied to each upstream definition's connection pool (FR-015–FR-017) — a stuck or long-lived backend connection is bounded independently of the downstream client connection.
- **SSRF.** AsyncAPI URL import and `ws`/`wss` upstream URLs go through the shared SSRF-safe validator and client (`ssrf-prevention.md`).
- **Fail closed.** The policy engine stays `failure_mode_allow: false`. If it is unavailable, handshakes fail with `500` (observed in the spike) and existing connections are unaffected.

## Out of Scope

- Per-message policies of any kind: message validation against AsyncAPI schemas, transformation, guardrails, message-level authorization, message-rate throttling. The spike showed client→server frames cannot be processed through ext_proc without deadlocking the handshake, and server→client processing would need RFC 6455 frame parsing and one long-lived ext_proc stream per connection. This may be revisited as a separate feature (Bijira does not offer it either).
- Message-level analytics (per-frame counts by type).
- Event gateway (`WebBrokerApi`, WebSocket ↔ Kafka), WebSub, SSE.
- GraphQL subscriptions over WebSocket (`graphql-ws`). They become possible on this foundation and should be a follow-up.
- Gateway-originated WebSocket close frames (Envoy closes the TCP stream; clients see `1006`).
- Load-balancing rebalance of existing long-lived connections across gateway replicas.

## Known Limitations

From the spike ([research.md](research.md)):

1. Undeploy, key revocation, subscription and policy changes do not affect live connections; only new handshakes see them.
2. Gateway-initiated closes (idle, max duration) reach clients as `1006`, not as a WebSocket close frame with a reason.
3. One analytics event per connection, emitted at close; a gateway crash loses events for connections open at that time.
4. Long-lived connections stay on the gateway replica they first reached; scaling out does not move them.
5. Rate-limit policies count handshakes, not messages.
6. FR-016's upstream `connection.maxConnections` cap (native circuit breaker) can admit more than the configured number when an upstream definition's cluster resolves to multiple backend hosts — Envoy guarantees at least one connection per selected host, which is a real source of overshoot above the configured threshold. Verified against a single-host backend only (spike 11/11b/11c); not yet verified against a multi-host cluster (research.md, Key Decision 7 and "Not Covered by the Spike").

## Open Questions

- **Q1 [NEEDS CLARIFICATION]**: Where should channel-level policies attach in the artifact: a `channels[].policies` list (proposed above), or the REST-style `operations` shape (`method: GET`, `path`) for maximum reuse of existing transformer and validation code?
- **Q2 [MOOT — 2026-10-22]**: How is the per-API connection limit (FR-040) enforced, and with what scope? *(FR-040 was removed: client-facing bounds are gateway-level config; backend concurrency is per upstream definition, FR-016.)*
  - **Per gateway instance:** count open connections in the router (an Envoy-native filter) or in the policy engine. Closes are not visible to ext_proc when body modes are `NONE`, so the router is preferred.
  - **Across the gateway cluster:** needs shared state.
  - Also undecided: should a rejection return `429` or `503`?
- **Q3 [NEEDS CLARIFICATION]**: Is bounding stale access with the maximum connection duration acceptable for the first release, or must undeploy/revocation actively close live connections (for example a policy-engine or controller-driven connection drain per API)?
- **Q4**: Should the console and API Portal label a channel **"Channel"** (AsyncAPI term, consistent with the artifact and APIs), **"Topic"** (familiar to WSO2 API Manager and Bijira users), or "Channel" with "topic" as helper text? Proposal: "Channel", with a tooltip noting that WSO2 API Manager calls these topics. The artifact and API field name stays `channels` either way. See [Channel vs Topic](#channel-vs-topic).
