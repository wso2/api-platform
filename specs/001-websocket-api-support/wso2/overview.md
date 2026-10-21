# **WebSocket API Support in the API Platform Gateway**

| | |
| :---- | :---- |
| Author(s) | naduni@wso2.com |
| Status | Draft |
| Last updated | 2026-10-14 |
| GitHub issue | NOT ANSWERED |
| Related docs | This document set: Feature Capabilities, Specification, Implementation, REST APIs, Proposed DB Schema, UIs, Test Scenarios, Security Impact Review Questionnaire, R&D Task Completion Questionnaire. GraphQL API Support (PR #3310, PR #3520) is the existing feature this one follows as its structural template. |

# What is the problem we are trying to solve?

A WebSocket connection is a long-lived, full-duplex exchange set up by a single HTTP handshake: after the server accepts the upgrade, the connection carries frames, not further HTTP requests, and can stay open for hours. Today, the API Platform Gateway cannot proxy this kind of connection at all, even though the rest of the platform already partly expects to: the control plane already models WebSocket APIs at the data level and the developer portal already lists and documents them, but none of that can reach a deployed gateway.

The problem has three main parts:

* **The gateway's data plane rejects every upgrade attempt.** Nothing in the current routing configuration enables protocol upgrades, so a client attempting to open a WebSocket connection through the gateway is refused outright, regardless of what backend it is meant to reach.

* **The policy engine can only reason about ordinary request/response exchanges.** It sees the handshake request and the upgrade response, but nothing after that — frames exchanged once the connection is open are not HTTP messages it can inspect, which means the entire policy model (how auth, rate limiting, and other checks apply) has to be rethought for a connection that is authorized once and then stays open indefinitely, rather than authorized on every request.

* **No gateway artifact kind exists for WebSocket APIs.** The control plane's existing data-level modeling and the developer portal's existing listing/documentation support have nothing to attach to — there is no way to define, deploy, secure, or manage a WebSocket API at the gateway, so none of that existing groundwork is reachable by an operator or publisher today.

# Why should it be solved?

* **API publishers get a real-time/bidirectional API category they don't have today** - chat, live notifications, and streaming-data backends can be exposed, secured, and governed through the gateway the same way REST and GraphQL APIs already are, instead of being left unmanaged or requiring a separate, ungoverned path to the backend.
* **Operators get first-class policy enforcement on a connection type that previously had none at the gateway** - authentication, rate limiting, and connection-lifetime/concurrency bounds all apply at the handshake, closing a gap where a WebSocket backend would otherwise be reachable with no gateway-level control at all.
* **Application developers get a consistent discovery and try-out experience** - a WebSocket API appears in the developer portal with a live, working connection URL and a try-it feature, the same as every other API kind, rather than documentation with no way to actually connect.
* **The product completes a category it already partially committed to** - the control plane and developer portal already model and display WebSocket APIs; this closes the one missing piece that makes that existing investment usable.

# Proposed Solution

Add WebSocket APIs as a new, first-class gateway artifact kind, proxied transparently (frames are relayed, never parsed or interpreted) through the standalone gateway, and built out end to end — gateway, control plane, command-line tool, console, and developer portal — following exactly the pattern already established by the existing GraphQL API support feature. Policies that apply to a WebSocket API run once, at the handshake, never on a per-message basis; a connection always has a finite maximum lifetime, so that revoking access has a bounded, known worst case even though re-authorization cannot happen mid-connection.

![][image1]

**Gateway responsibilities**

* Recognize the new artifact kind and translate it into one route per channel, with protocol upgrades enabled only on those routes.
* Enforce connection-level bounds (idle timeout, maximum duration, per-API concurrency) and run the handshake-only policy chain.

**Control plane responsibilities**

* Store the artifact (derived from an AsyncAPI definition), expose management endpoints mirroring the existing GraphQL API endpoints, and deploy/undeploy it to one or more gateways.
* Keep this artifact kind and its API keys included in the same bulk-synchronization and live-deployment-event machinery already used for every other kind.

**Command-line tool, console, and developer-portal responsibilities**

* Mirror the existing GraphQL command family, console pages, and developer-portal documentation/try-out experience for this new kind, plus a new in-console test console for verifying a deployment before publishing it.

**Design principles**

* **Handshake-only policies** - a policy chain for this artifact kind must never need the message body; the gateway's upgrade response races ahead of any body that would arrive, so requiring one would deadlock the handshake. This is enforced at deploy time, and as a data-plane fail-safe even if that check is ever bypassed.
* **Bounded connection lifetime, never "forever"** - every deployed API resolves to a finite maximum connection duration, whether configured explicitly or defaulted, because this is what keeps a revoked credential's exposure bounded when re-authorization only happens once, at the handshake.
* **Feature parity with the existing GraphQL API support** - the same artifact-kind pattern, the same control-plane/CLI/console/portal shape, so an operator or publisher already familiar with one operates the other identically.

## **Key Behaviours**

* An upgrade attempt on any route that is not a deployed WebSocket API channel is rejected.
* Credentials accepted via a query parameter or the WebSocket subprotocol negotiation header never reach the upstream service, the access log, or analytics.
* Connections beyond the per-API concurrency ceiling are rejected immediately and never queued.
* Revoking access, undeploying, or changing a policy never affects an already-open connection — only new connections see the change, bounded by the connection's own maximum duration.

# Challenges and solutions

| Challenge | Solution |
| :---- | :---- |
| Asking the policy engine for the request or response body on an upgrade deadlocks the handshake, since the upgrade response arrives before any body would. | Policy chains for this artifact kind are validated as handshake-only at deploy time, and the data plane forces no-body processing for these routes even if that validation is ever bypassed. |
| The gateway's existing connection-counting primitive (its upstream connection-pool circuit breaker) is scoped at the wrong level — it bounds upstream pooling, not how many downstream clients are connected — and was already shown unreliable for this purpose. | A dedicated, purpose-built per-route connection counter enforces the per-API concurrency ceiling instead; the exact data-plane extension mechanism is still an open design question (see the Specification document, §7). |
| Revoking access cannot immediately affect a connection that is already open, because authorization is checked only once, at the handshake. | Every connection carries a mandatory finite maximum duration, bounding how long a revoked credential can remain effective; actively closing live connections is deferred (see the Implementation document, §9). |
| A browser cannot set a custom header on a WebSocket handshake, so the usual bearer-token pattern doesn't work for browser clients. | Credentials are additionally accepted via a query parameter or the WebSocket subprotocol negotiation header, and stripped before they reach the upstream service or any log/analytics sink. |

# Out of Scope

* Any processing of individual messages once a connection is open — validation, transformation, guardrails, authorization, or rate limiting at the message level. The gateway proxies frames transparently and does not parse them. Full detail and rationale are in the Specification document, §1.
* The event-gateway's separate WebSocket-to-message-broker bridging capability, WebSub, and Server-Sent Events.
* GraphQL subscriptions carried over a WebSocket connection — a natural follow-up once this foundation exists, not something this feature delivers.

# Future Works

* GraphQL subscriptions over WebSocket, building on this foundation.
* Active connection draining on revocation/undeploy, if bounded staleness via the maximum connection duration proves insufficient in practice.
* Message-level policy support, if customer demand emerges for it — see the Implementation document, §9, for why it needs a materially different mechanism than what this feature builds.

<!-- [image1]: <path-or-data-uri> -->
