# **WebSocket API Support in the API Platform Gateway - Specification**

# 1. Scope

**In scope**

* Proxying WebSocket APIs through the standalone (Envoy-based) API Platform Gateway as a new, first-class artifact kind, defined from an AsyncAPI document.
* Managing this artifact kind gateway-first (direct gateway management API), from the control plane (with AsyncAPI import, deployment, and API keys), and from the CLI — mirroring how GraphQL APIs are already managed.
* Authentication and authorization policies enforced at the handshake, including credential locations usable by a browser client that cannot set a custom header.
* Connection lifecycle bounds: idle timeout, maximum connection duration, and a per-API concurrent-connection ceiling.
* Console and developer-portal experience: creation, configuration, a live test console, and publication with live gateway URLs.
* One analytics event per connection, emitted at close, with duration, byte counts, and a close reason.

**Out of scope**

* Per-message policies of any kind: message validation against the AsyncAPI schema, transformation, guardrails, message-level authorization, or message-rate throttling. The gateway proxies connections transparently and does not parse WebSocket frames; message-level processing would require full frame parsing and a long-lived processing stream per connection, which is a materially different feature.
* Message-level analytics (per-frame counts by type).
* The event-gateway's separate WebSocket-to-message-broker bridging capability, WebSub, and Server-Sent Events — all unaffected by this feature.
* GraphQL subscriptions carried over a WebSocket connection — this feature is a foundation that makes that possible as a follow-up, not something it delivers itself.
* Gateway-originated WebSocket close frames with a reason — the gateway closes the underlying connection; it does not speak the WebSocket protocol well enough to send a close frame of its own.
* Rebalancing existing long-lived connections across gateway replicas when the gateway scales out.

# 2. Scenarios

## 2.1 Deploy a WebSocket API to the gateway and connect through it (Priority: P1)

As a gateway operator, I deploy a WebSocket API to a standalone API Platform Gateway (gateway-first, the same way a GraphQL API is deployed). Clients then open WebSocket connections to the gateway, and frames flow both ways to my backend.

**Why this priority**: This is the minimum useful feature, and every other story builds on it. Without upgrade support in the router, no WebSocket API can be proxied at all.

**Independent test**: Deploy an API with one channel pointing at an echo backend, connect a WebSocket client through the gateway, send messages, and receive the echoes. Then delete the API and check that new handshakes return 404.

**Acceptance scenarios**

1. **Given** a WebSocket API with context `/chat/v1` and channel `/rooms/{roomId}` is deployed, **When** a client opens a connection to `/chat/v1/rooms/42`, **Then** the gateway accepts the handshake, forwards it to the equivalent backend path, and relays text, binary, ping/pong, and close frames both ways unchanged.
2. **Given** the API's upstream is a TLS-secured WebSocket endpoint, **When** a client connects, **Then** the gateway connects to the backend over TLS using HTTP/1.1 and the upstream CA trust configured for the API.
3. **Given** a REST API is deployed on the same gateway, **When** a client sends a WebSocket upgrade to a REST route, **Then** the gateway rejects it and the REST route keeps working for normal requests.
4. **Given** the client offers a choice of subprotocols, **When** the backend selects one, **Then** the client receives that selection in the handshake response.
5. **Given** the backend refuses the upgrade, **When** a client connects, **Then** the client receives the backend's refusal status and the access log records it.
6. **Given** a sandbox upstream is configured, **When** a client connects through the sandbox vhost, **Then** the connection is routed to the sandbox backend, the same as for other API kinds.

## 2.2 Secure the handshake (Priority: P1)

As an API publisher, I attach authentication and authorization policies (API key, JWT, OAuth2/opaque token, basic auth, IP- and header-based policies) to a WebSocket API. The gateway enforces them on the handshake, including for browser clients that cannot set an Authorization header.

**Why this priority**: A WebSocket API without access control cannot be exposed. The handshake is the only point where the gateway can authenticate the connection.

**Independent test**: Attach an API-key policy, connect without a key (rejected), with a revoked key (rejected), and with a valid key in a header, a query parameter, and the WebSocket subprotocol negotiation header (all accepted). Confirm the backend and the access log never see the key.

**Acceptance scenarios**

1. **Given** a JWT policy is attached at the API level, **When** a client connects without a token or with an invalid one, **Then** the handshake fails using the platform's identical unified unauthorized response used everywhere else, and the backend is never contacted.
2. **Given** a browser client sends the token as a query parameter or as a WebSocket subprotocol entry, **When** the token is valid, **Then** the connection is accepted, the token is removed before the request reaches the backend, and the token is not written to access logs or analytics.
3. **Given** the token was sent in the subprotocol negotiation alongside a real subprotocol, **When** the connection is accepted, **Then** the handshake response echoes only the real subprotocol and never the token.
4. **Given** a channel requires a specific scope, **When** a token without that scope connects to that channel, **Then** the handshake is rejected as forbidden.
5. **Given** an origin-validation policy allows only one specific origin, **When** a page from another origin opens a connection, **Then** the handshake is rejected — this is the cross-site WebSocket hijacking defence, since the usual cross-origin browser protections for ordinary requests do not apply to WebSockets.
6. **Given** a policy that needs the message body (a payload transformer or a body-inspecting guardrail) is attached to a WebSocket API, **When** the API is deployed, **Then** deployment fails with a validation error naming the policy.

## 2.3 Control connection lifetime and limits (Priority: P1)

As an API publisher or gateway operator, I configure how long a connection may stay idle, how long it may live in total, and how many concurrent connections an API accepts, so that one API cannot exhaust the gateway.

**Why this priority**: A connection that never closes ties up gateway and backend resources indefinitely, and stale authorization lives as long as the connection does, since re-authorization only happens at the handshake.

**Independent test**: Set a 10-second idle timeout and a 30-second maximum connection duration. An idle client is closed at 10 seconds, a client sending pings survives past 10 seconds, and every client is closed at 30 seconds.

**Acceptance scenarios**

1. **Given** the idle timeout is 10 seconds, **When** no frame crosses the connection for 10 seconds, **Then** the gateway closes it.
2. **Given** a client sends ping frames more often than the idle timeout, **When** no data frames are sent, **Then** the connection stays open.
3. **Given** the maximum connection duration is 30 seconds, **When** a connection reaches 30 seconds, **Then** the gateway closes it regardless of its traffic.
4. **Given** neither value is set on the API, **When** it is deployed, **Then** the gateway-wide defaults apply — an unlimited lifetime is never the implicit default.
5. **Given** a per-API concurrent-connection limit of N, **When** connection N+1 arrives, **Then** it is rejected immediately and is never left queued.

## 2.4 Manage WebSocket APIs from the control plane and CLI (Priority: P2)

As an API publisher, I create a WebSocket API in the control plane from an AsyncAPI definition, deploy it to one or more gateways, undeploy and restore deployments, and manage API keys. I can do the same from the command-line tool, as for GraphQL APIs.

**Why this priority**: Gateway-first deployment works without the control plane, but most users manage APIs centrally.

**Independent test**: Create a WebSocket API in the control plane by uploading an AsyncAPI file, deploy it to a registered gateway, connect through that gateway, undeploy, and confirm new connections fail.

**Acceptance scenarios**

1. **Given** an AsyncAPI document (either supported major version) with WebSocket servers and channels, **When** it is created from a file, a URL, or inline content, **Then** the control plane validates it, derives channels, context, and upstream, and stores the API.
2. **Given** an AsyncAPI document using only a non-WebSocket protocol, **When** creation from it is attempted, **Then** creation fails with a validation error.
3. **Given** a WebSocket API, **When** it is deployed to a gateway whose version does not support this artifact kind, **Then** the deployment is refused before translation is attempted.
4. **Given** a WebSocket API is deployed, **When** it is undeployed, **Then** new connections fail, and existing connections end no later than the API's maximum connection duration.
5. **Given** a gateway is configured from the control plane, **When** it connects or reconnects, **Then** WebSocket APIs and their API keys are included in bulk synchronization and live events like every other artifact kind.
6. **Given** the command-line tool, **When** the equivalent list/get/delete and API-key subcommands are run, **Then** they behave like their counterparts for the existing GraphQL command family.

## 2.5 Console and developer-portal experience (Priority: P2)

As an API publisher, I create, configure, deploy, test, and monitor a WebSocket API from the console like a REST or GraphQL API. As an application developer, I discover it in the developer portal, read its AsyncAPI definition, get API keys, and try it out.

**Why this priority**: Feature parity in the user interface is what makes WebSocket a first-class API kind, matching the existing GraphQL experience.

**Independent test**: Use the console's creation wizard to create a WebSocket API from an AsyncAPI URL, deploy it, open the WebSocket test console, connect to a channel, send a message and see the reply, then publish it to the developer portal.

**Acceptance scenarios**

1. **Given** the console's create flow, **When** WebSocket is chosen, **Then** an AsyncAPI definition can be imported (file, URL, or paste) or an endpoint can be entered directly, and the derived channels are shown before creating.
2. **Given** a deployed WebSocket API, **When** its test console is opened, **Then** an environment and channel can be selected, channel parameters filled in, a connection opened with a generated test key or token, and messages sent and received.
3. **Given** the API's develop page, **When** policies are attached, **Then** only policies valid for WebSocket APIs (handshake-only) can be added, at API or channel level.
4. **Given** the API is published to the developer portal, **When** a developer opens it, **Then** the AsyncAPI definition shows the live gateway URLs, API keys can be generated, and the try-out connects through the gateway.
5. **Given** the API's overview, deploy, insights, compliance, and observability pages, **When** they are opened, **Then** they show WebSocket-specific information (channels, connection metrics) in place of REST resources.

## 2.6 Connection analytics and observability (Priority: P3)

As an API publisher or operator, I see how many connections were opened, how long they lasted, how many bytes they carried, which closed abnormally, and which were rejected.

**Why this priority**: Operability matters, but the API works without it.

**Independent test**: Open and close several connections, including a rejected one, a throttled one, an idle-timed-out one, and a normally closed one. Each appears once in analytics with the correct outcome.

**Acceptance scenarios**

1. **Given** a connection ends, **When** analytics is published, **Then** exactly one event is emitted for it, with API, channel, application/subscription identity, connection duration, bytes in/out, and a close reason.
2. **Given** a normal client close, **When** the event is classified, **Then** it is not counted as an error.
3. **Given** a handshake rejected by policy or backend, **When** analytics is published, **Then** it is recorded as a failed connection attempt with the rejecting status.
4. **Given** connection-level events, **When** latency dashboards are shown, **Then** WebSocket connection duration is not mixed into request-latency figures.

## Edge Cases

* **A policy needing the message body.** Requesting the message body on an upgrade would deadlock the handshake, since the gateway's upgrade response arrives before any body would. Such policy chains are rejected at deploy time and must never reach a running gateway.
* **Undeploy, key revocation, subscription change, policy change, or token expiry while connected.** The live connection is not affected; new policy/route state applies to new connections only. This is bounded by the mandatory finite maximum connection duration (see §3, FR-031).
* **Gateway restart, rollout, or listener change.** All connections on that instance close at once and clients reconnect together. Clients are expected to back off and retry; whether the gateway has been validated to survive a resulting reconnect burst at scale is tracked in §7 (Open Questions).
* **A client that only speaks HTTP/2.** Without the config flag described in §3 (FR-011) enabled, a browser opens a separate HTTP/1.1 connection for the WebSocket instead. Whether the actual HTTP/2-to-HTTP/1.1 upgrade conversion is implemented when the flag is enabled, versus the flag existing with no corresponding behaviour yet, is tracked in §7.
* **A backend that only speaks HTTP/2.** Not supported; WebSocket upstreams use HTTP/1.1 only.
* **Per-message compression negotiated by the client and backend.** Passed through end to end; the gateway never interprets compressed frames.
* **Large frames and slow consumers.** A per-connection buffer limit applies; the gateway never buffers a whole message before relaying it.
* **A gateway-initiated close (idle timeout, max duration).** The client observes an abnormal close rather than a close frame with a reason, because the gateway does not speak the WebSocket protocol itself — it proxies frames without parsing them (see Out of Scope).
* **A query string carrying a credential.** It must never reach access logs, analytics, tracing, or the backend.
* **Channel path parameters.** Must match exactly one path segment; an unmatched path returns not-found.
* **The same path with a different HTTP method.** Only the handshake method is routed for a channel; any other method on that path returns not-found/method-not-allowed.
* **Many connections from one client.** Bounded by the per-API connection limit and a gateway-wide downstream connection limit.

# 3. Requirements

| ID | Requirement | Priority | Notes |
| :---- | :---- | :---- | :---- |
| FR-001 | The gateway MUST support a new artifact kind for WebSocket APIs, stored and versioned like the existing GraphQL artifact kind. | Must | |
| FR-002 | The gateway management API MUST expose create/list/get/update/delete for this artifact and create/list/regenerate/update/revoke for its API keys, mirroring the GraphQL API management endpoints. | Must | |
| FR-003 | The gateway MUST validate the artifact on create and update: required fields, at least one channel, unique channel paths, valid WebSocket upstream URLs via the shared upstream validator, resolvable policies, handshake-only policies, and valid connection settings. | Must | |
| FR-004 | The artifact MUST support main and sandbox upstreams with the same vhost rules as the GraphQL kind, and the upstream options REST supports where they apply to a handshake: upstream TLS/CA, upstream auth, host rewrite, and connect timeout. Each upstream slot MUST additionally support its own connection lifecycle settings (FR-015–FR-017). | Must | |
| FR-010 | The translator MUST generate one route per channel and upstream slot, matching the handshake method and the channel's full path (with parameters), routed to an HTTP/1.1-only cluster. | Must | |
| FR-011 | Upgrades MUST be opt-in per route: disabled by default at the listener, enabled only on this artifact kind's routes. Accepting WebSockets over HTTP/2 MUST be controlled by a gateway config flag. | Must | See Edge Cases and §7 for the open question on the HTTP/2 conversion's own scope. |
| FR-012 | For this artifact kind's routes, the translator MUST set the idle timeout and maximum stream duration from the API's connection settings, and MUST NOT attach a retry policy. | Must | |
| FR-013 | The gateway config MUST provide defaults and upper bounds for idle timeout and maximum connection duration. A value above the configured bound MUST be rejected at deploy time. | Must | |
| FR-014 | Existing header/path-rewrite request processing MUST keep working unmodified on upgrade requests: header-only path and host rewrite, with no body access. | Must | |
| FR-015 | For each upstream slot, the translator MUST set that upstream's connection-pool idle timeout and maximum connection duration from the slot's own connection settings, independently of the downstream route settings (FR-012). | Must | |
| FR-016 | The gateway MUST support a per-upstream-slot cap on concurrent connections to the backend, implemented via the upstream cluster's native circuit breaker. | Must | Unlike FR-040's downstream limit, a native circuit breaker is the correct mechanism here: the bounded resource (outbound connections from this gateway instance to the backend) is itself per-instance. |
| FR-017 | The gateway config MUST provide defaults and upper bounds for upstream idle timeout, maximum connection duration, and maximum concurrent connections, mirroring FR-013 for the downstream side. | Must | |
| FR-020 | A policy chain on this artifact kind MUST be handshake-only. Deployment MUST be rejected when any policy in the chain declares request-body or response-body processing. | Must | |
| FR-021 | Even if FR-020 is bypassed, the data plane MUST force no request/response body processing for this artifact kind's routes, as a fail-safe against the handshake deadlock. | Must | |
| FR-022 | The policy framework MUST add a WebSocket API kind, and the shared policy context MUST carry the API kind and the matched channel. | Must | |
| FR-023 | Header-phase policies MUST work on the handshake unchanged: authentication, subscription validation, header manipulation, host/path rewrite, backend-JWT, header-only interceptor use, and handshake-counting rate limits. Each MUST be checked against FR-020 rather than assumed header-only. | Must | |
| FR-030 | Auth policies used with this artifact kind MUST accept the credential from a header, a configurable query parameter, or the WebSocket subprotocol negotiation header. On success the credential MUST be removed from the upstream request and from the subprotocol negotiation in both directions. | Must | |
| FR-031 | Credentials are validated only at the handshake. Because live connections are unaffected by undeploy/revocation/subscription/policy changes, every deployed API MUST have a finite effective maximum connection duration, bounding how long revoked access can last. | Must | |
| FR-032 | Query-string credentials MUST be redacted from access logs, the analytics stream, tracing span attributes, and any log line. | Must | |
| FR-033 | An origin-validation policy (exact-match allowlist) MUST be available for the handshake as the cross-site WebSocket hijacking defence. | Must | |
| FR-034 | Channel-level scopes MUST be enforceable on the handshake. | Must | |
| FR-040 | The gateway MUST support a per-API concurrent-connection limit, enforced per gateway instance, that rejects excess handshakes immediately (no queueing) with a 503 response. | Must | |
| FR-041 | Request-count rate-limit policies on this artifact kind MUST count handshakes (connection-rate limiting); message-rate limiting is out of scope. | Must | |
| FR-042 | The gateway SHOULD set a gateway-wide downstream connection limit. | Should | |
| FR-050 | The control plane MUST support this artifact kind as a core kind, with its own schema table and the matching repository, service, and handler layers mirroring the GraphQL kind. | Must | |
| FR-051 | The control plane MUST expose endpoints mirroring the GraphQL API endpoints: CRUD, AsyncAPI definition get/update, definition validation, gateways, API keys, and deployments (deploy/undeploy/restore/builds), guarded by new, dedicated scopes. | Must | |
| FR-052 | The control plane MUST accept AsyncAPI (both supported major versions, JSON or YAML) from a file, a URL fetched through the shared SSRF-safe client, or inline content; derive channels, parameters, WebSocket servers, and basic info; reject a document with no WebSocket server and no explicit upstream override; and enforce size and parse-time limits. | Must | |
| FR-053 | The gateway translator registry MUST register this kind with a minimum supported gateway version, so deploying to an older gateway is refused before translation. | Must | |
| FR-054 | This artifact kind and its API keys MUST be included in connected-gateway bulk sync, live deployment events, gateway-first artifact push (read-only in the control plane), and API key sync, as for the GraphQL kind. | Must | |
| FR-055 | Publishing to the developer portal MUST send the AsyncAPI definition with live gateway URLs. | Must | |
| FR-060 | The command-line tool MUST add a command family for this artifact kind and its API keys, mirroring the GraphQL command family. | Must | |
| FR-070 | The console MUST support this artifact kind end to end, mirroring the GraphQL console experience: a creation wizard, overview/deploy/develop/edit/publish pages, and insights/compliance/observability pages. | Must | |
| FR-071 | The console MUST provide a WebSocket test console: environment and channel selection, channel parameter inputs, connect/disconnect, a send box, and a timestamped message log, sending a generated test key or token as a query parameter or subprotocol entry. | Must | |
| FR-080 | Analytics MUST emit one event per connection at close, with API kind, channel, application, subscription, connection duration, bytes in/out, and a close reason. | Must | |
| FR-081 | The fault classifier MUST treat a normal close as success, and analytics MUST keep connection duration out of request-latency metrics. | Must | |
| FR-082 | Rejected handshakes MUST produce a normal request analytics event with the rejecting status. | Must | |
| FR-083 | Third-party analytics publishers MUST map connection events to a sensible shape, consistent with how the equivalent GraphQL properties are mapped today. | Must | |
| FR-090 | Gateway integration tests MUST cover deploy/connect/echo, REST-route-not-upgradable, handshake auth (all credential locations, rejected credentials, token stripped), handshake-only policy validation, idle/max-duration closes, the downstream connection limit, upstream connection-pool idle timeout/max duration/max-connections enforcement (FR-015–FR-017), undeploy behaviour, sandbox routing, API keys, and analytics events. | Must | |

# 4. Key Entities

| Entity / Term | Definition |
| :---- | :---- |
| WebSocket API (gateway artifact) | A gateway-managed API definition: display name, version, context path, main/sandbox upstreams (each with its own optional connection lifecycle settings), downstream connection settings, one or more channels, and API-level (handshake-only) policies. |
| Channel | An AsyncAPI channel exposed as one WebSocket endpoint: a path (with optional parameters), optional channel-level policies, and optional message schemas carried for documentation only — never enforced by the gateway. |
| Connection settings | Idle timeout, maximum connection duration, and maximum concurrent connections. Two independent sets apply: the API's own downstream settings, and each upstream slot's own backend-facing settings (FR-015–FR-017) — both bounded by gateway-wide defaults and limits. |
| WebSocket API (control plane) | A control-plane artifact holding the AsyncAPI definition, derived channels, upstream, policies, and connection settings, with deployments, builds, and API keys like the GraphQL kind. |
| Connection event (analytics) | One record per connection: API, channel, application, subscription, start time, duration, bytes in/out, close reason, and handshake status. |

# 5. Success Criteria

| ID | Outcome | Target |
| :---- | :---- | :---- |
| SC-001 | A WebSocket API can be deployed gateway-first, from the control plane, or from the command-line tool, and a client can exchange messages through the gateway. | Under 5 minutes from an AsyncAPI file. |
| SC-002 | Upgrade attempts on routes that are not a deployed WebSocket API channel are rejected. | 100% of attempts. |
| SC-003 | Handshakes with missing, invalid, or revoked credentials are rejected, and credentials never appear in backend requests, access logs, or analytics. | 100% of the time. |
| SC-004 | Connections close in line with their configured bounds. | Idle connections within 1 second of the idle timeout; all connections within 1 second of the maximum connection duration. |
| SC-005 | With a per-API limit of N, the (N+1)th connection is rejected and never queued. | Rejected within 100 ms. |
| SC-006 | Handshake and frame-relay latency added by the gateway are bounded. | Handshake overhead (one auth policy) under 10 ms at p95; frame-relay overhead under 1 ms at p95, on the reference performance setup. |
| SC-007 | One gateway instance sustains a high volume of concurrent idle connections without the policy engine holding an open processing stream per connection. | At least 10,000 concurrent idle connections. |
| SC-008 | Each closed connection yields exactly one analytics event with the correct close reason, and normal closes are never classified as errors. | 100% of closed connections. |
| SC-009 | Deploying an API with a body-processing policy in its chain fails validation, with a message naming the policy. | 100% of such deployment attempts. |

# 6. Assumptions

* WebSocket support targets the standalone gateway only. The separate event-gateway bridging capability is unchanged — if this assumption does not hold, the scope boundary in §1 would need to be renegotiated.
* The gateway proxies WebSocket connections transparently; message content, AsyncAPI message schemas, and subprotocol semantics are not interpreted — if message-level behaviour is required, that is a different feature (see §1, Out of Scope).
* A WebSocket backend accepts HTTP/1.1 upgrades; HTTP/2-only WebSocket backends are not supported in this phase — a backend that only speaks HTTP/2 cannot be fronted by this feature.
* Clients handle reconnects with backoff after gateway restarts, rollouts, and enforced closes, and send pings more often than the idle timeout if they want to stay connected while otherwise idle — a client that does neither will be disconnected or will contribute to a reconnect storm the gateway has not yet been validated against (see §7).
* Policies run once per connection, at the handshake; existing header-only policies are reusable as-is, with credential-location changes — a policy that cannot be made handshake-only cannot be used with this artifact kind at all.
* AsyncAPI parsing depends on either a maintained, permissively-licensed external library or a minimal in-repo parser for the subset this feature needs — if neither is available, AsyncAPI import cannot ship and only inline/manual artifact creation would remain possible.
* Gateway version gating starts at "no shipped release supports this kind yet" — a gateway must be upgraded to a release that sets the minimum-version constant before this artifact kind can be deployed to it.

# 7. Open Questions

| Question | Owner | Target date | Status |
| :---- | :---- | :---- | :---- |
| Whether the actual HTTP/2-to-HTTP/1.1 upgrade conversion (not just the gating config flag) ships in this phase, or is explicitly deferred. | NOT ANSWERED | NOT ANSWERED | Open |
| Which exact gateway data-plane extension mechanism implements the per-API connection counter (FR-040) — this blocks finalizing that part of the implementation. | NOT ANSWERED | NOT ANSWERED | Open |
| Final choice of AsyncAPI parsing approach (external library vs. in-repo parser), pending a license/vulnerability/dependency review — this blocks finalizing the control-plane import path. | NOT ANSWERED | NOT ANSWERED | Open |
| The exact status code returned when deploying this artifact kind to a gateway below the minimum supported version. | NOT ANSWERED | NOT ANSWERED | Open |
| Whether the gateway has been validated to survive a mass-reconnect burst after a restart or rollout at a representative connection count. | NOT ANSWERED | NOT ANSWERED | Open |
