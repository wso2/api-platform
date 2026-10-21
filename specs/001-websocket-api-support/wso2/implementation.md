# **WebSocket API Support in the API Platform Gateway - Implementation**

# **1. Overview**

This feature adds WebSocket APIs as a new, first-class artifact kind end to end: a data-plane routing and policy change in the gateway, a new artifact and its management endpoints in the control plane, a new command family in the command-line tool, and the matching console and developer-portal surfaces. Every piece follows the exact pattern already established by the existing GraphQL API support feature, so the two are operated identically by anyone already familiar with one.

* **Gateway data-plane changes**: a new artifact kind, a translator that turns it into one route per channel with protocol upgrades enabled only on those routes, and a policy-engine change recognizing the new kind and forcing handshake-only processing.
* **Control-plane changes**: a new table and the matching repository/service/handler layers, AsyncAPI import, and inclusion in the existing deployment and bulk-sync machinery.
* **CLI, console, and developer-portal changes**: a mirrored command family, console pages, a new test console, and extensions to the already-existing developer-portal documentation and try-it surfaces for this API kind (see the UIs document for full screen-level detail).

No precondition blocks the feature from shipping — it is purely additive. The one real gating condition is that a gateway must be running a release that recognizes the new minimum-version constant before this artifact kind can be deployed to it; an operator on an older gateway release simply cannot use the feature yet, with no other action required on their part.

# **2. Gateway Data Plane Implementation**

## **2.1 Artifact Structure**

The new artifact kind carries: a display name, a version, a context path, main and sandbox upstream URLs (`ws://`/`wss://` only, each with its own optional connection timeout settings), downstream connection settings (idle timeout, maximum connection duration, maximum concurrent connections), one or more channels (each with a path, optional path parameters, and an optional channel-level policy list), and an API-level policy list. It is defined the same way the existing GraphQL artifact kind is — as an OpenAPI-described type in the gateway management API's specification, with the corresponding generated data type used throughout the gateway codebase.

Each upstream slot's timeout settings reuse and extend the existing, named `UpstreamTimeout` schema — today reachable only through the REST artifact kind's reusable upstream-definition mechanism — rather than introducing a second, parallel schema for the same concept.

```
WebSocketApi
├── displayName, version, context
├── upstream: { main: {url, timeout?}, sandbox: {url, timeout?} }
│   └── timeout: { connect, idleTimeout, maxConnectionDuration, maxConnections }  # per slot, backend-facing; connect pre-existing, other three new
├── connection: { idleTimeout, maxConnectionDuration, maxConnections }           # API-level, downstream/client-facing
├── channels[]: { path, policies[] }
└── policies[]                      # API-level, handshake-only
```

## **2.2 Interfaces**

The gateway management API exposes create/list/get/update/delete for this artifact and create/list/update/regenerate/revoke for its API keys. The full contract is in the REST APIs document, §2–3.

## **2.3 Behaviour**

* The translator generates one route per channel and upstream slot, matching the handshake method and the channel's full path (context, version, and channel path, including any path parameters), routed to an HTTP/1.1-only upstream cluster — the upstream cluster never negotiates HTTP/2.
* Protocol upgrades are disabled by default at the shared listener and enabled only on this artifact kind's routes, so no other API kind's route can be turned into an upgrade target.
* For each such route, the idle timeout and maximum stream duration are set from the artifact's downstream connection settings, and no retry policy is attached — the ordinary per-request retry/timeout machinery does not apply to an upgraded connection.
* Independently, each upstream slot's cluster gets its own connection-pool idle timeout and maximum connection duration from that slot's own `timeout` settings, and a native circuit breaker enforcing that slot's maximum concurrent connections. Unlike the downstream per-API connection ceiling (§3.5–3.6, which needed a purpose-built counter instead of a circuit breaker), the upstream cluster's native circuit breaker is the right mechanism here: the bounded resource — outbound connections from this gateway instance to the backend — is itself per-instance, so per-instance enforcement is the intended behaviour, not a gap.
* Accepting WebSockets negotiated over HTTP/2 is controlled by a separate gateway configuration flag, defaulting to off (see §4). Whether the corresponding HTTP/2-to-HTTP/1.1 upgrade conversion is implemented in this phase, or only the flag exists pending that work, is an open question — see the specification document, §7. This path (HTTP/2 extended `CONNECT`) is also less exercised in Envoy than the plain HTTP/1.1 upgrade path, which weighs toward shipping the flag off until the conversion itself is deliberately built and tested, not toggled on by default once the flag exists.
* The shared listener's path-canonicalization settings (normalization, slash-merging, and escaped-slash handling) already apply uniformly to every route on that listener, including these new ones — no separate per-kind configuration is needed for that.
* Existing header/path-rewrite request processing continues to work unmodified on upgrade requests: header-only path and host rewrite, with no body access, exactly as it already behaves for every other kind.
* The per-connection buffer limit that already applies to every route applies here too — the gateway never buffers a whole message, only up to that limit. A channel producing data faster than a slow client consumes it will hit backpressure at that limit; the exact client-visible behaviour when it does (the connection stalling vs. being closed) is not established in the inputs and should be confirmed before launch.

# **3. Policy Engine and Control Plane Implementation**

## **3.1 API Changes**

The control plane exposes a parallel set of endpoints for this artifact kind — resource CRUD, AsyncAPI definition get/update, definition validation, gateways, API keys, and deployments — mirroring the equivalent GraphQL API endpoints one for one. The full contract is in the REST APIs document, §4–5.

## **3.2 Request Validation**

| Validation | Expected behavior |
| :---- | :---- |
| No channels supplied | Reject with 400, naming the missing requirement. |
| Duplicate channel path within the same API | Reject with 400. |
| Upstream URL is not a valid `ws`/`wss` URL | Reject with 400. |
| Any policy in the chain (API- or channel-level) requires the request or response message body | Reject with 400, naming the offending policy. |
| A connection-setting value (API-level or upstream-slot-level) exceeds the gateway-configured upper bound | Reject with 400. |
| AsyncAPI document has no WebSocket server and no explicit upstream override | Reject with 400. |
| AsyncAPI document uses only a non-WebSocket protocol | Reject with 400. |
| Unknown API or gateway reference | Reject with 404. |
| Deploying to a gateway below the minimum supported version | Reject before translation is attempted; exact status NOT ANSWERED (see the specification document, §7). |

## **3.3 Storage Model**

The control-plane artifact is stored as one row per API: organization and project scope, identity (handle, display name, version), and the full derived configuration (channels, upstream, policies, connection settings) as an opaque JSON payload, plus standard audit columns. This mirrors the storage shape already used for the equivalent GraphQL artifact. The full schema is in the DB Schema document, §2.

## **3.4 Create, Update, Deploy, and Delete Request Handling**

1. **Create/update**: the control plane parses the supplied AsyncAPI document (file, URL fetched through the shared SSRF-safe client, or inline content), derives channels, upstream, and basic info, validates it against the same rules the gateway itself enforces, and persists it.
2. **Deploy**: the control plane translates the stored artifact into the gateway-native shape and pushes it to each target gateway. A target gateway below the minimum supported version is refused before translation is attempted, not after a partial push.
3. **Delete/Undeploy**: new handshakes fail immediately on the affected gateway. Already-open connections are **not** affected by this operation — this is a deliberate, documented invariant, not an oversight: a live connection continues until its own maximum connection duration elapses, bounding how long a now-revoked or now-undeployed credential's access can remain effective (see §5).
4. **Bulk sync / gateway reconnect**: this artifact kind and its API keys are included in the control plane's existing bulk-synchronization and live-deployment-event channel used for every other artifact kind — no new synchronization channel is introduced for this feature.

## **3.5 Error Handling**

| Condition | Status | Client-facing behaviour | Logged internally |
| :---- | :---- | :---- | :---- |
| Missing/invalid AsyncAPI document or artifact field | 400 | Generic validation error naming the field | Full parse/validation detail |
| Body-processing policy in the chain | 400 | Generic error naming the policy | Same |
| Missing/invalid/revoked credential at handshake | 401 | The platform's identical unified unauthorized body used everywhere else | The specific reason (expired, missing, revoked) — never in the response |
| Caller lacks the required scope | 403 | Generic forbidden body | The missing scope |
| Resource not found | 404 | Generic not-found body | N/A |
| Deploying to a gateway below the minimum version | NOT ANSWERED | Generic error | NOT ANSWERED |
| Per-API connection ceiling exceeded | 503 | Generic body, no retry-after detail established | Connection count, for capacity planning |

## **3.6 Concurrency, Retries and Failure Modes**

* A gateway restart, rollout, or listener change closes every open connection on that instance at once; clients are expected to back off and reconnect. Whether the gateway has been validated to absorb the resulting reconnect burst at a representative scale is an open question (see the specification document, §7).
* Envoy's graceful-drain sequence tries to let in-flight work finish before a listener is actually torn down — but a long-lived WebSocket connection has no natural "finish." In practice it sits open until the configured drain period elapses, then gets forcibly closed along with everything else on that listener. The drain-period tuning for this specific case (long enough to avoid needlessly cutting connections short on an ordinary rolling restart, short enough that a restart still completes promptly) is not established in the inputs and needs an explicit decision, not just reuse of whatever default every other route already uses.
* The per-API connection ceiling is enforced per gateway instance in this phase, not across a replicated gateway cluster — a client spreading connections across replicas could in principle exceed the nominal ceiling cluster-wide. This is a known, accepted limitation of the current design, not an implementation bug.
* If the policy engine becomes unavailable, handshakes fail closed (rejected) rather than being allowed through; already-open connections are unaffected by a policy-engine outage.

# **4. Configuration**

| Key | Type | Default | Component | Description |
| :---- | :---- | :---- | :---- | :---- |
| WebSocket idle-timeout default / upper bound (downstream) | duration | NOT ANSWERED | Gateway | Bounds how long a connection may sit idle before the gateway closes it; an API-level value above the upper bound is rejected at deploy time. |
| WebSocket max-connection-duration default / upper bound (downstream) | duration | NOT ANSWERED | Gateway | Bounds the total lifetime of a connection regardless of activity; never resolves to "unlimited." |
| WebSocket upstream idle-timeout default / upper bound | duration | NOT ANSWERED | Gateway | Bounds how long the gateway keeps a pooled connection open to the backend; an upstream-slot value above the upper bound is rejected at deploy time (FR-017). |
| WebSocket upstream max-connection-duration default / upper bound | duration | NOT ANSWERED | Gateway | Bounds the total lifetime of a connection to the backend regardless of activity; never resolves to "unlimited" (FR-017). |
| WebSocket upstream max-connections default / upper bound | integer | NOT ANSWERED | Gateway | Per-upstream-slot, per-gateway-instance cap on concurrent backend connections, enforced via a native circuit breaker (FR-016, FR-017). |
| Accept WebSocket over HTTP/2 | boolean | off | Gateway | Gates whether an HTTP/2-negotiated WebSocket upgrade is accepted at all. Off by default, so no behavioural change for an operator who does not opt in. |
| Gateway-wide downstream connection ceiling | integer | NOT ANSWERED | Gateway | A SHOULD-level bound on total concurrent connections across every API on one gateway instance. |

No additional operator-facing enable/disable flag beyond the minimum-gateway-version gate (§2.3) is established in the inputs for this feature as a whole — the feature becomes usable simply by upgrading a gateway to a release that recognizes the new kind.

# **5. Security Considerations**

* **Authorization**: the control-plane endpoints are guarded by dedicated scopes — `ap:websocket_api:manage` / `ap:websocket_api:read` for the resource, definition, and API-key operations, and `ap:websocket_api:deployment:manage` / `ap:websocket_api:deployment:read` for deployment operations. A valid token alone is never sufficient; the specific scope is checked on every operation.
* **Tenant isolation**: the organization used to scope every control-plane query and mutation is resolved from verified token claims only, never from a request parameter or body field.
* **Input handling**: AsyncAPI documents are subject to configured size and parse-time limits; a document fetched by URL goes through the platform's shared SSRF-safe client rather than a direct, unvalidated fetch. Credentials accepted at the handshake via a query parameter or the WebSocket subprotocol negotiation header are stripped before the request reaches the upstream service and before any log line or analytics event is written, and an origin-validation policy using an exact-match allowlist is available as the cross-site WebSocket hijacking defence (browsers send cookies on a WebSocket handshake, and the usual cross-origin protections for ordinary requests do not apply here).

The full security questionnaire is in the Security Impact Review document.

# **6. Observability**

* **Logs**: handshake rejections are logged with the specific reason (expired/missing/revoked credential, policy violation, connection-limit rejection); the raw credential value is never logged, only a masked form where correlation is needed.
* **Metrics**: exactly one analytics event is produced per connection, at close, carrying API, channel, application/subscription identity, connection duration, bytes in/out, and a close reason; a normal client close is never classified as an error, and connection duration is kept out of request-latency aggregates. A rejected handshake produces a normal request-analytics event carrying the rejecting status.
* **Close-reason mapping is a closed, but not exhaustive, list.** The specific signals this feature maps to a close reason (client close, idle timeout, max duration, backend close/failure) are a known subset of everything Envoy can report. Any signal outside that mapped set needs an explicit fallback bucket (e.g. "other"/"unknown") rather than being silently miscounted as one of the mapped reasons — this should be verified against the full set of response flags Envoy v1.39.0 can actually emit, not assumed from the four cases already enumerated.

# **7. Testing Plan**

| Test area | Required coverage |
| :---- | :---- |
| Gateway routing and translation | Deploy/connect/echo over the correct protocol; a non-WebSocket route rejects an upgrade attempt; TLS and sandbox upstream routing; subprotocol negotiation; backend-refusal pass-through. |
| Handshake security | Every supported credential location (header, query parameter, subprotocol); rejected/expired/revoked credentials; channel-level scope enforcement; origin-allowlist rejection; credential never reaching the backend, logs, or analytics. |
| Handshake-only policy validation | A chain containing a body-processing policy fails deployment, naming the policy. |
| Connection lifecycle and limits | Idle-timeout and maximum-duration closes at the configured bound; the per-API connection ceiling rejecting the next connection immediately, never queued; the same idle-timeout, maximum-duration, and connection-ceiling bounds enforced independently on each upstream slot's connection pool (FR-015–FR-017). |
| Control plane and CLI | AsyncAPI import from file/URL/inline for both supported document versions; rejection of a non-WebSocket document; version-gated deployment refusal; undeploy behaviour; inclusion in bulk sync; command-line parity with the equivalent GraphQL command family. |
| Console and developer portal | Creation wizard with channel preview; policy picker restricted to handshake-only policies; the test console connecting and exchanging a message; the developer-portal documentation page showing live URLs; the try-it feature connecting and exchanging a message. |
| Analytics | Exactly one event per connection with the correct close reason; a normal close never classified as an error; a rejected handshake recorded as a failed attempt. |
| Performance and resilience | Handshake and frame-relay latency overhead within the stated targets; a high volume of concurrent idle connections sustained on one gateway instance; the gateway's behaviour under a mass-reconnect burst after a restart (see the specification document, §7, for the open question on whether this has been validated). |
| Envoy version-specific verification | Upgrade behaviour, the connection-limit mechanism finally chosen, `max_stream_duration` interaction with an upgraded connection, and the drain-period behaviour for long-lived connections — all re-verified against Envoy v1.39.0 specifically, not assumed from general Envoy documentation, since this behaviour has changed across Envoy releases. |

# **8. Rollout**

* **Phasing**: the gateway-side artifact kind, routing, and policy-engine changes ship together as one coordinated change, since none of the other work streams is usable without them. The control-plane, CLI, console, and developer-portal pieces can follow as subsequent, independently reviewable changes once the gateway-side foundation is in place.
* **Default state**: no gateway advertises support for this artifact kind until it is upgraded to a release that sets the minimum-version constant — there is no separate feature flag beyond that version gate established in the inputs.
* **Rollback**: undeploying an API (or rolling back to a gateway release that predates this feature) removes it from new traffic immediately; already-open connections are unaffected and continue until their own maximum connection duration elapses. No destructive schema rollback is needed, since the new table is purely additive.
* **Restart/rollout drain tuning**: because a WebSocket connection has no natural end for Envoy's graceful-drain sequence to wait for (§3.6), the drain-period value used for a routine gateway rollout needs an explicit decision for this route type specifically, not inherited silently from whatever every other route already uses.

# **9. Future Work**

* GraphQL subscriptions carried over a WebSocket connection, building on this foundation.
* Actively closing already-open connections on revocation, undeploy, subscription change, or policy change, if the bounded-staleness approach (a mandatory finite maximum connection duration) proves insufficient in practice.
* Message-level policy support (validation, transformation, guardrails, authorization, rate limiting) — out of scope for this feature because it would require full WebSocket frame parsing and a long-lived processing stream per connection, a materially different mechanism than the handshake-only policy chain used here.
