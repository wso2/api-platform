# Feature Specification: WebSocket API Support in the API Platform Gateway

**Feature Branch**: `websocket`

**Created**: 2026-10-13

**Status**: Draft

**Input**: User description: "Support WebSocket APIs as a first-class API type in API Platform, proxied through the standalone (Envoy-based) API Platform Gateway — not the event gateway. Allow creating WebSocket APIs the same way GraphQL APIs are created (gateway-first, control-plane and CLI), with QoS features, and Console and API Portal support. Use Bijira's WebSocket support as the reference product behaviour." (seeded from the existing draft at `gateway/spec/impls/4-websocket-api-support/spec.md`)

**Related**: GraphQL API support (issue #3195, backend PR #3310, UI/analytics PR #3520) is the template this feature follows end to end. Envoy behaviour this spec depends on was verified by a spike; see the original draft's [research.md](../../gateway/spec/impls/4-websocket-api-support/research.md). This feature's own Phase 0 [research.md](./research.md) (same folder as this file) builds on that spike for decisions made during planning — later citations in this document ("research.md §N") refer to that one, not the original draft, unless stated otherwise.

## Clarifications

### Session 2026-10-13

- Q: Which runtime proxies WebSocket APIs — the standalone gateway (Envoy + policy engine) or the event gateway? → A: The standalone gateway. The event gateway (`WebBrokerApi`, WebSocket ↔ Kafka) is out of scope for this feature.
- Q: Which existing feature is the template for scope and delivery? → A: GraphQL API support (PR #3310, PR #3520): a new artifact kind across gateway, platform-api, CLI, console, API Portal, analytics, and tests.
- Q: How is the per-API connection limit (FR-040) enforced, and with what rejection status? → A:
  Per gateway instance (no shared cluster state for v1), rejecting excess handshakes with `503`.
- Q: Should undeploy/key revocation/subscription/policy changes actively close already-open
  connections? → A: No — bounded staleness only, via the mandatory finite maximum connection
  duration (FR-031). Active connection draining is deferred as a follow-up, not built in v1.
- Q: Where should channel-level policies attach — a WebSocket-native `channels[].policies` list,
  or a REST-style `operations[]` shape with a `method` field? → A: `channels[].policies`, mirroring
  the `Channel{Name, Description, Request{Method, Name, Policies}}` type `RestApi` already defines
  for its own async/WebSub-style channels. A code investigation found no generic per-operation
  policy validator, CLI, or UI to reuse either way (each artifact kind gets its own hand-written
  validator; there is no per-operation policy CLI for REST or GraphQL today; the only frontend
  reuse point is structural typing, which the existing `Channel` shape already satisfies) — and
  channels are semantically not "operations," since a channel handshake is always `GET`.

## Background

A WebSocket API is a long-lived, full-duplex connection set up by an HTTP/1.1 `GET` with
`Upgrade: websocket` (RFC 6455). After the server answers `101 Switching Protocols`, the connection
carries WebSocket frames, not HTTP requests. A gateway therefore sees exactly one HTTP exchange per
connection (the handshake), and then a byte stream that can stay open for hours.

What this means for the API Platform Gateway today (`upstream/main`):

- The Envoy translator never enables protocol upgrades, so every upgrade attempt is rejected.
- The policy engine processes HTTP messages. It sees the handshake request and the `101` response;
  frames after the upgrade arrive, if at all, as raw RFC 6455 bytes it cannot parse as HTTP.
- platform-api already models WebSocket APIs at the data level (API type and transport), and the
  API Portal already lists, documents, and issues API keys for them. No gateway artifact kind exists
  for them yet, so none of this can reach a deployed gateway.

The product reference is Bijira's WebSocket support: AsyncAPI import, OAuth2 at the handshake
(token accepted in `Sec-WebSocket-Protocol` for browsers), connection throttling, connection-level
analytics, a WebSocket test console, and per-plan limits on concurrent connections, connection
duration, and idle time.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Deploy a WebSocket API to the gateway and connect through it (Priority: P1)

As a gateway operator, I deploy a WebSocket API to a standalone API Platform Gateway (YAML through
the gateway management REST API, gateway-first, like `GraphQLApi`). Clients then open WebSocket
connections to the gateway, and frames flow both ways to my `ws://`/`wss://` backend.

**Why this priority**: This is the minimum useful feature, and every other story builds on it.
Without upgrade support in the router, no WebSocket API can be proxied at all.

**Independent Test**: Deploy a `WebSocketApi` with one channel pointing at an echo backend, connect
a WebSocket client through the gateway, send messages, and receive the echoes. Then delete the API
and check that new handshakes return `404`.

**Acceptance Scenarios**:

1. **Given** a `WebSocketApi` with context `/chat/v1` and channel `/rooms/{roomId}` is deployed,
   **When** a client opens `ws://<gateway>/chat/v1/rooms/42`, **Then** the gateway returns `101`,
   forwards the handshake to `<upstream>/rooms/42`, and relays text, binary, ping/pong, and close
   frames both ways unchanged.
2. **Given** the API's upstream is `wss://backend:8443/ws`, **When** a client connects, **Then**
   the gateway connects to the backend over TLS using HTTP/1.1 and the upstream CA trust configured
   for the API.
3. **Given** a `RestApi` is deployed on the same gateway, **When** a client sends a WebSocket
   upgrade to a REST route, **Then** the gateway rejects it (`403`) and the REST route keeps working
   for normal requests.
4. **Given** the client offers `Sec-WebSocket-Protocol: chat.v1, chat.v2`, **When** the backend
   selects `chat.v1`, **Then** the client receives `chat.v1` in the `101` response.
5. **Given** the backend refuses the upgrade (any non-`101` status), **When** a client connects,
   **Then** the client receives that status and the access log records it.
6. **Given** a sandbox upstream is configured, **When** a client connects through the sandbox
   vhost, **Then** the connection is routed to the sandbox backend (same rules as `GraphQLApi`).

---

### User Story 2 - Secure the handshake (Priority: P1)

As an API publisher, I attach authentication and authorization policies (API key, JWT,
OAuth2/opaque token, basic auth, IP- and header-based policies) to a WebSocket API. The gateway
enforces them on the handshake, including for browser clients that cannot set an `Authorization`
header.

**Why this priority**: A WebSocket API without access control cannot be exposed. The handshake is
the only point where the gateway can authenticate the connection.

**Independent Test**: Attach an API-key policy, connect without a key (rejected `401`), with a
revoked key (rejected), and with a valid key in a header, a query parameter, and
`Sec-WebSocket-Protocol` (all accepted). Check that the backend and the access log never see the
key.

**Acceptance Scenarios**:

1. **Given** a JWT auth policy is attached at API level, **When** a client connects without a token
   or with an invalid one, **Then** the handshake fails with `401` using the identical unified
   auth-failure body used for REST, and the backend is never contacted.
2. **Given** a browser client sends the token as a query parameter (e.g. `?access_token=...`) or as
   a `Sec-WebSocket-Protocol` entry, **When** the token is valid, **Then** the connection is
   accepted, the token is removed before the request reaches the backend, and the token is not
   written to access logs or analytics.
3. **Given** the token was sent in `Sec-WebSocket-Protocol` alongside a real subprotocol, **When**
   the connection is accepted, **Then** the `101` response echoes only the real subprotocol and
   never the token.
4. **Given** a channel requires scope `chat:write`, **When** a token without that scope connects to
   that channel, **Then** the handshake fails with `403`.
5. **Given** an origin-validation policy allows only `https://app.example.com`, **When** a page
   from another origin opens a connection, **Then** the handshake is rejected. This is the
   cross-site WebSocket hijacking (CSWSH) defence, since CORS does not apply to WebSockets.
6. **Given** a policy that needs the message body (e.g. a payload transformer or a body guardrail)
   is attached to a WebSocket API, **When** the API is deployed, **Then** deployment fails with a
   validation error naming the policy (see FR-020).

---

### User Story 3 - Control connection lifetime and limits (Priority: P1)

As an API publisher or gateway operator, I configure how long a connection may stay idle, how long
it may live in total, and how many concurrent connections an API accepts, so that one API cannot
exhaust the gateway.

**Why this priority**: A connection that never closes ties up gateway and backend resources
indefinitely, and stale authorization lives as long as the connection does (see User Story 2,
FR-031). Bijira enforces exactly these limits.

**Independent Test**: Set an idle timeout of 10s and a maximum connection duration of 30s. An idle
client is closed at 10s, a client sending pings survives past 10s, and every client is closed at
30s. Separately, set a tight upstream connection ceiling on the `main` slot and confirm it bounds
only that slot's backend connections, not the client-facing connection count or the `sandbox` slot.

**Acceptance Scenarios**:

1. **Given** the idle timeout is 10s, **When** no frame (data or ping/pong) crosses the connection
   for 10s, **Then** the gateway closes it.
2. **Given** a client sends ping frames more often than the idle timeout, **When** no data frames
   are sent, **Then** the connection stays open.
3. **Given** the maximum connection duration is 30s, **When** a connection reaches 30s, **Then**
   the gateway closes it regardless of its traffic.
4. **Given** neither value is set on the API, **When** it is deployed, **Then** the gateway-wide
   defaults apply (FR-013). An unlimited lifetime is never the implicit default.
5. **Given** a per-API concurrent connection limit of N, **When** connection N+1 arrives, **Then**
   it is rejected immediately, and is never left queued.
6. **Given** the `main` upstream slot has its own idle timeout, maximum connection duration, and
   maximum concurrent connections set (FR-015–FR-017), **When** the backend connection sits idle
   past that timeout, lives past that duration, or a new connection would exceed that slot's
   concurrency cap, **Then** the gateway enforces each bound on that upstream connection pool
   independently of the downstream client connection's own settings, and independently of the
   `sandbox` slot.

---

### User Story 4 - Manage WebSocket APIs from the control plane and CLI (Priority: P2)

As an API publisher, I create a WebSocket API in platform-api from an AsyncAPI definition, deploy
it to one or more gateways, undeploy and restore deployments, and manage API keys. I can do the
same from the `ap` CLI, as for GraphQL APIs.

**Why this priority**: Gateway-first deployment (User Story 1) works without the control plane, but
most users manage APIs centrally. This applies the GraphQL control-plane and CLI approach to
WebSocket.

**Independent Test**: Create a WebSocket API in platform-api by uploading an AsyncAPI file, deploy
it to a registered gateway, connect through that gateway, undeploy, and confirm new connections
fail.

**Acceptance Scenarios**:

1. **Given** an AsyncAPI 2.x or 3.x document with `ws`/`wss` servers and channels, **When** I
   create a WebSocket API from a file, a URL, or inline content, **Then** platform-api validates it,
   derives channels, context, and upstream, and stores the API.
2. **Given** an AsyncAPI document using a non-WebSocket protocol only (e.g. `kafka`, `mqtt`),
   **When** I try to create a WebSocket API from it, **Then** creation fails with a validation
   error.
3. **Given** a WebSocket API, **When** I deploy it to a gateway whose version does not support the
   `WebSocketApi` kind, **Then** the deployment is refused before translation (same version gating
   as `GraphQLApi`).
4. **Given** a WebSocket API is deployed, **When** I undeploy it, **Then** new connections fail,
   and existing connections end no later than the API's maximum connection duration (see FR-031).
5. **Given** a gateway is configured from the control plane, **When** it connects or reconnects,
   **Then** WebSocket APIs and their API keys are included in bulk sync and live events like every
   other kind.
6. **Given** the `ap` CLI, **When** I run the WebSocket API and API-key subcommands, **Then** they
   behave like their `graphql-api` counterparts.

---

### User Story 5 - Console and API Portal experience (Priority: P2)

As an API publisher, I create, configure, deploy, test, and monitor a WebSocket API from the
console like a REST or GraphQL API. As an application developer, I discover it in the API Portal,
read its AsyncAPI definition, get API keys, and try it out.

**Why this priority**: Feature parity in the UI is what makes WebSocket a first-class API type, as
GraphQL support did before it.

**Independent Test**: Use the console wizard to create a WebSocket API from an AsyncAPI URL, deploy
it, open the WebSocket test console, connect to a channel, send a message and see the reply, then
publish it to the API Portal.

**Acceptance Scenarios**:

1. **Given** the console's create flow, **When** I choose WebSocket, **Then** I can import an
   AsyncAPI definition (file, URL, or paste) or start from an endpoint, and I see the derived
   channels before creating.
2. **Given** a deployed WebSocket API, **When** I open its test console, **Then** I can pick an
   environment and channel, fill in channel parameters, connect with a generated test key or token,
   and send and receive messages.
3. **Given** the API's develop page, **When** I attach policies, **Then** only policies valid for
   WebSocket APIs (handshake-only, FR-020) can be added, at API or channel level.
4. **Given** the API is published to the API Portal, **When** a developer opens it, **Then** the
   AsyncAPI definition shows the live gateway URLs, API keys can be generated, and the try-out
   connects through the gateway.
5. **Given** the API's overview, deploy, insights, compliance, and observability pages, **When** I
   open them, **Then** they show WebSocket-specific information (channels, connection metrics) in
   place of REST resources.

---

### User Story 6 - Connection analytics and observability (Priority: P3)

As an API publisher or operator, I see how many connections were opened, how long they lasted, how
many bytes they carried, which closed abnormally, and which were rejected (auth, throttled, backend
refused).

**Why this priority**: Operability matters, but the API works without it.

**Independent Test**: Open and close several connections, including a rejected one, a throttled
one, an idle-timed-out one, and a normally closed one. Each appears once in analytics with the
correct outcome.

**Acceptance Scenarios**:

1. **Given** a connection ends, **When** analytics is published, **Then** exactly one event is
   emitted for it, with API, channel, application/subscription, status `101`, connection duration,
   bytes in/out, and a close reason (client close, idle timeout, max duration, backend close,
   error).
2. **Given** a normal client close, **When** the event is classified, **Then** it is not counted as
   an error.
3. **Given** a handshake rejected by policy or backend, **When** analytics is published, **Then**
   it is recorded as a failed connection attempt with the rejecting status.
4. **Given** connection-level events, **When** latency dashboards are shown, **Then** WebSocket
   connection duration is not mixed into request latency figures.

---

### Edge Cases

- **Policy that needs a body.** Requesting the message body on an upgrade deadlocks the handshake,
  since the gateway's upgrade response arrives before any body would. Such policy chains must be
  rejected at deploy time and must never reach the running gateway.
- **Undeploy, key revocation, subscription change, policy change, token expiry** while connected.
  The live connection is not affected; new policy/route state applies to new connections only
  (bounded staleness via the mandatory finite maximum connection duration, FR-031).
- **Gateway restart, rollout, or listener change.** All connections on that instance close at once
  and clients reconnect together. Clients must back off and retry; the gateway must survive the
  reconnect burst.
- **HTTP/2 clients.** Without RFC 8441 support, browsers open a separate HTTP/1.1 connection for
  WebSockets. With it (FR-011), the extended `CONNECT` is converted into an HTTP/1.1 upgrade toward
  the backend.
- **Backend speaks HTTP/2 only.** Not supported; WebSocket upstreams use HTTP/1.1 (FR-010).
- **`permessage-deflate`.** Negotiated end to end; the gateway passes compressed frames through
  untouched.
- **Large frames and slow consumers.** A per-connection buffer limit applies; the gateway never
  buffers whole messages.
- **Close semantics.** When the gateway ends a connection (idle timeout, max duration), the client
  sees an abnormal close, not a close frame, because the gateway does not speak the WebSocket
  protocol itself — it proxies frames without parsing them.
- **Query string carries a token.** It must not reach access logs, analytics, tracing, or the
  backend.
- **Channel parameters.** `/rooms/{roomId}` must match one path segment only; unmatched paths
  return `404`.
- **Same path, different methods.** Only `GET` handshakes are routed for a channel; other methods
  return `404`/`405`.
- **Many connections from one client.** Bounded by the per-API connection limit and the
  gateway-wide downstream connection limit.

## Requirements *(mandatory)*

### Functional Requirements

#### Artifact and gateway management API

- **FR-001**: The gateway MUST support a new artifact kind `WebSocketApi`, stored and versioned
  like `GraphQLApi`, with the fields in [Key Entities](#key-entities).
- **FR-002**: The gateway management API MUST expose create/list/get/update/delete for WebSocket
  APIs and create/list/regenerate/update/revoke for their API keys, mirroring the `GraphQLApi`
  management endpoints.
- **FR-003**: The gateway MUST validate a `WebSocketApi` on create and update: required fields, at
  least one channel, unique channel paths, valid `ws`/`wss` upstream URLs (checked by the shared
  upstream URL validator used by REST/MCP/LLM), resolvable policies, handshake-only policies
  (FR-020), and valid connection settings.
- **FR-004**: `WebSocketApi` MUST support `main` and `sandbox` upstreams with the same vhost rules
  as `GraphQLApi`, and the upstream options REST supports where they apply to a handshake: upstream
  TLS/CA, upstream auth (OAuth2/mTLS to the backend), host rewrite, and connect timeout. Each
  upstream slot MUST additionally support its own connection lifecycle settings — idle timeout,
  maximum connection duration, and maximum concurrent connections to that backend (FR-015–FR-017).

#### Router (gateway data plane) translation

- **FR-010**: The translator MUST generate one route per channel and upstream slot, matching
  `GET` and the channel's full path (context + version + channel path, with parameters), routed to
  an HTTP/1.1 cluster. WebSocket upstream clusters MUST NOT negotiate HTTP/2.
- **FR-011**: Upgrades MUST be opt-in per route: disabled by default at the listener level, and
  enabled only on `WebSocketApi` routes, so no REST, GraphQL, MCP, or LLM route can be upgraded.
  Accepting WebSockets over HTTP/2 (RFC 8441) MUST be controlled by a gateway config flag.
- **FR-012**: For `WebSocketApi` routes, the translator MUST set the idle timeout from the API's
  idle-timeout setting and the maximum stream duration from the API's maximum-connection-duration
  setting, and MUST NOT attach a retry policy. The ordinary route timeout does not end an upgraded
  connection and may stay at the gateway default.
- **FR-013**: The gateway config MUST provide defaults and upper bounds for WebSocket idle timeout
  and maximum connection duration. An API value above the configured bound MUST be rejected at
  deploy time.
- **FR-014**: Existing header/path-rewrite request processing MUST keep working on upgrade
  requests: header-only path and host rewrite, with no body access.
- **FR-015**: For each upstream slot (`main`, `sandbox`), the translator MUST set that upstream's
  connection-pool idle timeout and maximum connection duration from the slot's own timeout
  settings (`upstream.<slot>.timeout.idleTimeout`/`.maxConnectionDuration`), independently of the
  downstream route settings in FR-012. This bounds how long the gateway keeps a connection open to
  the backend, not how long a client may stay connected to the gateway. `timeout` reuses the
  existing `UpstreamTimeout` schema already used by `RestApi`'s `UpstreamDefinition.timeout`,
  extended with `idleTimeout`/`maxConnectionDuration`/`maxConnections` and exposed directly on
  `Upstream` for the first time, rather than a new, parallel schema.
- **FR-016**: The gateway MUST support a per-upstream-slot cap on concurrent connections to the
  backend (`upstream.<slot>.timeout.maxConnections`), implemented via the upstream cluster's
  native connection-pool circuit breaker. Unlike the downstream per-API limit (FR-040), a native
  circuit breaker is the correct mechanism here: the bounded resource — outbound connections from
  this gateway instance to the backend — is itself per-instance, so enforcing it per gateway
  instance (the same scope FR-040 resolved to for the downstream case) is the intended behavior,
  not a limitation to work around. This is a best-effort cap, not an exact one: Envoy guarantees at
  least one connection per selected host, so a multi-host upstream slot can exceed the configured
  value by a small amount (Known Limitations, this feature's [research.md](./research.md) §8).
- **FR-017**: The gateway config MUST provide defaults and upper bounds for upstream idle timeout,
  maximum connection duration, and maximum concurrent connections, mirroring FR-013 for the
  downstream side. An API value above the configured bound MUST be rejected at deploy time.

#### Policy engine

- **FR-020**: A `WebSocketApi` policy chain MUST be handshake-only. Deployment MUST be rejected
  when any policy in the chain declares request-body or response-body processing. The policy
  definition schema MUST let a policy declare that it supports `WebSocketApi`.
- **FR-021**: Even if FR-020 is bypassed, the policy engine MUST request no request/response body
  for a `WebSocketApi` route (fail-safe against the handshake deadlock).
- **FR-022**: The policy SDK MUST add a WebSocket API kind, and the shared policy context MUST
  carry the API kind and the matched channel, so policies can tell a WebSocket handshake apart from
  other request types.
- **FR-023**: Header-phase policies MUST work on the handshake unchanged. This includes
  authentication (API key, JWT, basic auth, opaque token), subscription validation, header
  manipulation, host rewrite, path rewrite, backend JWT, header-only interceptor use, and
  handshake-counting rate limits. Each of these MUST be checked against FR-020 rather than assumed
  header-only.

#### Security

- **FR-030**: Auth policies used with WebSocket APIs MUST accept the credential from a header, a
  configurable query parameter, or a `Sec-WebSocket-Protocol` entry. On success the credential MUST
  be removed from the upstream request, and from `Sec-WebSocket-Protocol` in both the upstream
  request and the `101` response.
- **FR-031**: Credentials are validated only at the handshake. Because live connections are not
  affected by undeploy, revocation, subscription, or policy changes, every `WebSocketApi` MUST have
  a finite effective maximum connection duration (FR-013), which bounds how long revoked access can
  last.
- **FR-032**: Query-string credentials MUST be redacted from access logs, the analytics stream,
  tracing span attributes, and any log line, per `authentication_authorization.md` (GO-AUTH-003).
- **FR-033**: An origin-validation policy (exact-match allowlist, per `go-cors-validation.md`) MUST
  be available for WebSocket handshakes as the cross-site WebSocket hijacking (CSWSH) defence.
- **FR-034**: Channel-level scopes MUST be enforceable on the handshake, consistent with
  channel/operation-level scope enforcement for GraphQL operations.

#### Connection limits and throttling

- **FR-040**: The gateway MUST support a per-API concurrent connection limit, enforced per gateway
  instance (no cross-replica shared state in v1), that rejects excess handshakes immediately (no
  queueing) with `503 Service Unavailable`.
- **FR-041**: Request-count rate-limit policies on a `WebSocketApi` MUST count handshakes
  (connection-rate limiting). Message-rate limiting is out of scope (see Out of Scope).
- **FR-042**: The gateway SHOULD set a gateway-wide downstream connection limit, with the gateway
  config providing its default value (mirroring how FR-013/FR-017 require a default for their own
  idle-timeout/duration/connection-count settings) — this is a gateway-wide, cross-API ceiling, not
  per-API, so it has no "unlimited" concern the way a per-API value does, but it SHOULD NOT be left
  unconfigured by default: an operator who never touches it SHOULD still get a sane, documented
  ceiling rather than an implicit absence of one.

#### Control plane (platform-api)

- **FR-050**: platform-api MUST support WebSocket APIs as a core artifact kind, with its own schema
  table (new table, additive per `db-schema-changes.md`), repository, service, and handlers
  mirroring `graphql_apis`.
- **FR-051**: platform-api MUST expose WebSocket API endpoints mirroring the GraphQL API endpoints:
  CRUD, AsyncAPI definition get/update, definition validation, gateways, API keys, deployments
  (deploy, undeploy, restore), and builds. They MUST be guarded by new, dedicated manage/read/
  deployment scopes, added to the role-to-scope mapping and the AI Workspace BFF default OIDC
  scopes.
- **FR-052**: platform-api MUST accept AsyncAPI 2.x and 3.x (JSON or YAML) from a file, a URL
  (fetched through the shared SSRF-safe client, per `ssrf-prevention.md`), or inline content. It
  MUST derive channels, channel parameters, servers with protocol `ws`/`wss` (upstream), and info
  (name, version, description). Documents with no `ws`/`wss` server and no explicit upstream MUST
  be rejected. Size and parse-time limits apply.
- **FR-053**: The gateway translator registry MUST register the WebSocket kind with a minimum
  gateway version gate (no released version supports it until a gateway release ships it), so
  deploying to an older gateway is refused before translation.
- **FR-054**: WebSocket APIs and their API keys MUST be included in connected-gateway bulk sync,
  live deployment events, gateway-first artifact push (gateway → control plane, read-only in the
  control plane), and API key sync, as for `GraphQLApi`.
- **FR-055**: Publishing to the API Portal MUST send the AsyncAPI definition with live gateway
  URLs (the portal already renders WebSocket APIs and issues API keys for them).

#### CLI

- **FR-060**: The `ap` CLI MUST add WebSocket API management and API-key subcommands mirroring the
  `graphql-api` command family, with docs in the CLI gateway reference.

#### Console

- **FR-070**: The console MUST support WebSocket APIs end to end, mirroring the GraphQL console
  experience: a creation wizard (AsyncAPI file/URL/paste or endpoint, with channel preview);
  overview (channels in place of resources), deploy, develop (handshake-only policy picker at API
  and channel level), edit, publish; insights, compliance, and observability pages, wired into
  navigation, routes, and scope gates.
- **FR-071**: The console MUST provide a WebSocket test console: environment and channel selection,
  channel parameter inputs, connect/disconnect, a send box, and a timestamped message log. It sends
  a generated test key or token as a query parameter or `Sec-WebSocket-Protocol` entry, because a
  browser cannot set headers on a WebSocket handshake.

#### Analytics and observability

- **FR-080**: Analytics MUST emit one event per connection at close, with API kind
  `WebSocketApi`, channel, application, subscription, connection duration, bytes in/out, and a
  close reason (client close, idle timeout, max duration, backend close/failure).
- **FR-081**: The fault classifier MUST treat a normal close as success, and analytics MUST keep
  WebSocket connection duration out of request-latency metrics.
- **FR-082**: Rejected handshakes MUST produce a normal request analytics event with the rejecting
  status.
- **FR-083**: Third-party analytics publishers MUST map WebSocket connection events using the same
  field-mapping structure and conventions already used for GraphQL operation properties (property
  naming, nesting, and type conventions) — not a newly-designed shape for this kind alone.

#### Testing

- **FR-090**: Gateway integration tests MUST cover: deploy, connect and echo; REST route not
  upgradable; auth at the handshake (header, query, subprotocol, rejected credentials, token
  stripped); handshake-only policy validation; idle and max-duration closes; downstream connection
  limit; upstream connection-pool idle timeout, max duration, and max-connections enforcement
  (FR-015–FR-017); undeploy behaviour; sandbox; API keys; analytics events. A WebSocket echo
  backend MUST be added to the integration test backend set.

### Key Entities

- **WebSocketApi** (gateway artifact): a gateway-managed API definition with a display name,
  version, context path, main/sandbox upstreams (`ws`/`wss` URLs, each with its own optional
  `timeout` lifecycle settings — FR-015–FR-017, reusing and extending the existing
  `UpstreamTimeout` schema), downstream connection settings (idle timeout, maximum connection
  duration, maximum concurrent connections — always finite per FR-031), a `channels[]` list (see
  Channel below), and API-level policies (handshake-only, per FR-020).
- **Channel**: an AsyncAPI channel exposed as one WebSocket endpoint, carrying its own
  `policies[]` list. Modeled on the `Channel` type `RestApi` already defines for its own
  async/WebSub-style channels (name, description, and a request shape of method — always `GET`
  for WebSocket — path/name, and policies), rather than a new `operations[]`/`Operation` shape.
  It has a path (with optional parameters), optional channel-level policies, and optional message
  schemas (documentation only; not enforced by the gateway).
- **Connection settings**: idle timeout, maximum connection duration, and maximum concurrent
  connections. Two independent sets apply: the API's own downstream (client-facing) settings
  (`connection.*`, FR-012, FR-013, FR-040), and each upstream slot's own backend-facing settings
  (`upstream.<slot>.timeout.*`, FR-015–FR-017 — reusing and extending the existing
  `UpstreamTimeout` schema rather than a new one). Both are bounded by gateway-wide defaults and
  limits.
- **WebSocket API (control plane)**: a platform-api artifact holding the AsyncAPI definition,
  derived channels, upstream, policies, and connection settings, with deployments, builds, and API
  keys like `GraphQLApi`.
- **Connection event (analytics)**: one record per connection: API, channel, application,
  subscription, start time, duration, bytes in/out, close reason, and handshake status.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A WebSocket API can be deployed gateway-first, from platform-api, or from the CLI,
  and a client can exchange messages through the gateway, in under 5 minutes from an AsyncAPI file.
- **SC-002**: 100% of upgrade attempts on non-`WebSocketApi` routes are rejected.
- **SC-003**: Handshakes with missing, invalid, or revoked credentials are rejected 100% of the
  time, and credentials never appear in backend requests, access logs, or analytics.
- **SC-004**: Idle connections close within 1 second of the configured idle timeout, and all
  connections close within 1 second of the configured maximum connection duration.
- **SC-005**: With a per-API limit of N, connection N+1 is rejected within 100 ms and never queued.
- **SC-006**: Handshake latency added by the gateway (policy chain with one auth policy) is under
  10 ms at p95 on the reference performance setup. Frame relay adds under 1 ms at p95.
- **SC-007**: One gateway instance sustains at least 10,000 concurrent idle WebSocket connections
  without the policy engine holding an open processing stream per connection.
- **SC-008**: Each closed connection yields exactly one analytics event with the correct close
  reason, and normal closes are never classified as errors.
- **SC-009**: Deploying a `WebSocketApi` with a body-processing policy fails validation 100% of the
  time, with a message naming the policy.

## Assumptions

- WebSocket support targets the standalone gateway (gateway-controller + gateway-runtime). The
  event gateway and its WebSocket-to-Kafka bridging artifact are unchanged.
- The gateway proxies WebSocket connections transparently. Message content, AsyncAPI message
  schemas, and subprotocol semantics are not interpreted.
- A WebSocket backend accepts HTTP/1.1 upgrades. HTTP/2-only WebSocket backends are not supported
  in this phase.
- Clients handle reconnects (with backoff) after gateway restarts, rollouts, and enforced closes.
  Clients that stay idle send pings more often than the idle timeout.
- Policies run once per connection, at the handshake. Policies that work on headers only are
  reusable as-is, with FR-030 changes for credential location.
- AsyncAPI parsing in platform-api uses a maintained, Apache-2.0-compatible library selected per
  `dependency-management.md`, or a minimal in-repo parser for the subset needed (servers, channels,
  operations, parameters).
- Gateway version gating starts unassigned, like GraphQL's did; the first gateway release that
  ships the kind sets its minimum-version constant.

## Dependencies

- GraphQL API support (PR #3310 merged, PR #3520 pending): reused patterns for the artifact kind,
  gateway-version gating, API key sync, portal publication, console pages, analytics properties,
  and integration-test backend.
- New auth-adjacent policy work: credential-location support for WebSocket (FR-030) and a new
  origin-validation policy (FR-033).
- Data-plane features this spec depends on: route-level protocol-upgrade enablement, maximum
  stream duration, optional HTTP/2 CONNECT-based upgrades, a gateway-wide downstream connection
  limit, and per-upstream-cluster connection-pool idle timeout, maximum connection duration, and
  circuit breaking (FR-015–FR-017).
- API Portal: existing WebSocket listing, AsyncAPI rendering, and API key issuance.

## Security Considerations

- **Authentication at the handshake only.** Bounded by a mandatory finite maximum connection
  duration (FR-031). This must be documented clearly: revocation is not immediate.
- **Credential exposure.** Browser credentials travel in the query string or
  `Sec-WebSocket-Protocol`. They must be stripped before the upstream and redacted from every
  log/analytics sink (FR-030, FR-032).
- **Cross-site WebSocket hijacking.** Browsers send cookies with WebSocket handshakes and CORS does
  not apply, so an origin allowlist is required for cookie-authenticated APIs (FR-033).
- **Upgrade smuggling.** Upgrades are disabled by default and enabled only per `WebSocketApi`
  route, so no other route can be turned into a tunnel (FR-011). Path normalization applies to
  handshakes as to every other request.
- **Resource exhaustion.** Covered by idle and duration bounds, per-API and gateway-wide connection
  limits, and a per-connection buffer limit (FR-012, FR-013, FR-040, FR-042), and by the same
  idle/duration/connection-count bounds applied independently to each upstream connection pool
  (FR-015–FR-017) — a stuck or long-lived backend connection is bounded separately from the
  downstream client connection.
- **SSRF.** AsyncAPI URL import and `ws`/`wss` upstream URLs go through the shared SSRF-safe
  validator and client (`ssrf-prevention.md`).
- **Fail closed.** If the policy engine is unavailable, handshakes must fail closed (not open), and
  existing connections must be unaffected.

## Out of Scope

- Per-message policies of any kind: message validation against AsyncAPI schemas, transformation,
  guardrails, message-level authorization, message-rate throttling. Client-to-server frame
  processing cannot go through the header-phase policy pipeline without deadlocking the handshake,
  and server-to-client processing would need full WebSocket frame parsing and a long-lived
  processing stream per connection. This may be revisited as a separate feature.
- Message-level analytics (per-frame counts by type).
- Event gateway (WebSocket-to-Kafka bridging), WebSub, SSE.
- GraphQL subscriptions over WebSocket. They become possible on this foundation and should be a
  follow-up.
- Gateway-originated WebSocket close frames (the gateway closes the TCP stream; clients see an
  abnormal close).
- Load-balancing rebalance of existing long-lived connections across gateway replicas.

## Known Limitations

1. Undeploy, key revocation, subscription, and policy changes do not affect live connections; only
   new handshakes see them.
2. Gateway-initiated closes (idle, max duration) reach clients as an abnormal close, not as a
   WebSocket close frame with a reason.
3. One analytics event per connection, emitted at close; a gateway crash loses events for
   connections open at that time.
4. Long-lived connections stay on the gateway replica they first reached; scaling out does not move
   them.
5. Rate-limit policies count handshakes, not messages.
6. FR-016's upstream `maxConnections` cap (native circuit breaker) can admit more connections than
   configured when an upstream slot's cluster resolves to multiple backend hosts — Envoy guarantees
   at least one connection per selected host, a real source of overshoot above the configured
   threshold. Verified against a single-host backend only; a multi-host cluster has not been tested
   (this feature's [research.md](./research.md) §8).
