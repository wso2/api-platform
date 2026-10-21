# **Feature Capabilities**

## 1. Capability Map by Layer

What this feature delivers, grouped by where each capability actually lives — not every
capability below has its own row in §2's decision log; that log tracks contentious or
noteworthy *decisions*, not an exhaustive inventory of what exists.

### Gateway Level — Standalone Gateway, Gateway Management REST API, CLI

- **Artifact kind support** — `WebSocketApi` as a gateway-first artifact, same lifecycle and
  versioning as `GraphQLApi` (FR-001)
- **Gateway Management REST API** — create/list/get/update/delete, plus an API-key sub-resource
  (FR-002, FR-003)
- **Upstream support** — `main`/`sandbox` slots, TLS/CA, upstream auth (OAuth2/mTLS), host
  rewrite, connect timeout (FR-004)
- **Protocol upgrade handling** — opt-in per route only, HTTP/1.1 upstream, optional HTTP/2
  extended-`CONNECT` (FR-010, FR-011)
- **Downstream connection lifecycle** — idle timeout, maximum connection duration, with
  config-level defaults and upper bounds (FR-012, FR-013)
- **Upstream connection lifecycle** — per-slot idle timeout, max duration, and max concurrent
  connections via the native circuit breaker (FR-015–FR-017)
- **Authentication support** — API key, JWT, OAuth2/opaque token, Basic Auth; credential via
  header, query parameter, or `Sec-WebSocket-Protocol` (FR-030)
- **Authorization support** — channel-level scope enforcement via JWT (FR-034)
- **CSWSH protection** — exact-match origin allowlist, the WebSocket equivalent of CORS, which
  doesn't apply to WebSocket handshakes (FR-033)
- **Rate limiting / connection throttling** — per-API concurrent-connection limit,
  handshake-count rate limiting, gateway-wide downstream connection ceiling (FR-040–FR-042)
- **Credential redaction** — query-string tokens stripped before the upstream request, logs, and
  analytics (FR-032)
- **Policy engine integration** — handshake-only policy chain enforcement, reuse of existing
  header-phase policies (FR-020–FR-023)
- **Analytics emission** — one event per connection, not per request; fault classification
  (FR-080, FR-081)
- **CLI support** — `ap gateway websocket-api` (list/get/delete), `ap gateway apply`
  (create/update, gateway-first), API-key subcommands (FR-060)

### Control Plane Level — platform-api, Publisher Console

- **Core artifact kind** — CRUD, dedicated schema table, repository/service/handler layers
  mirroring `graphql_apis` (FR-050)
- **REST API** — CRUD, AsyncAPI definition get/update/validate, gateways, API keys, deployments,
  builds (FR-051)
- **AsyncAPI import** — file, URL, or inline content; AsyncAPI 2.x and 3.x; channel/upstream
  derivation (FR-052)
- **Gateway version gating** — deployment refused on gateways below the minimum supported version
  (FR-053)
- **Deployment orchestration** — deploy/undeploy/restore to one or more registered gateways
  (FR-051)
- **Gateway sync** — bulk sync, live deployment events, gateway-first artifact push (bottom-up),
  API key sync (FR-054)
- **Scopes/authorization** — `ap:websocket_api:{manage,read}`, `:deployment:{manage,read}`
  (FR-051)
- **Console UI** — creation wizard from AsyncAPI, Overview/Deploy/Develop/Edit/Publish pages
  (FR-070)
- **Console policy picker** — restricted to handshake-only policies at API/channel level (FR-070)
- **Console test console** — connects directly browser-to-gateway, not proxied (FR-071)
- **Console observability** — Insights/Compliance/Observability pages showing connection metrics
  in place of REST request metrics (FR-070)
- **Analytics reporting** — third-party publisher mapping (Moesif), consistent with GraphQL's
  shape (FR-083)

### API Portal Level — Developer-facing (devportal)

- **Discovery/listing** — WebSocket APIs already modeled and listed today; this feature connects
  that listing to a real deployed gateway
- **AsyncAPI definition rendering** — shows live gateway URLs once deployed (FR-055)
- **API key issuance** — generate/manage API keys for a WebSocket API from the portal (already
  exists, extended to deployed APIs)
- **Try-it / tryout** — direct browser-to-gateway WebSocket connection, not proxied through the
  portal server, unlike the existing HTTP tryout path (User Story 5)

## 2. Capability Decisions

| Capability | Final Decision / Agreed Behavior | Decision Source (Meeting / Doc) | Start Date | End Date | Implementation Notes | Open Questions / Risks |
| :---- | :---- | :---- | :---- | :---- | :---- | :---- |
| Runtime scope | Proxied through the standalone (Envoy-based) gateway only. The event-gateway's separate WebSocket-to-message-broker bridging capability is unaffected and explicitly out of scope. | Feature design - 2026-10-13 | 2026-10-13 | 2026-10-13 | Scoping decision only; shapes every other row in this table. | N/A |
| Delivery template | Follows the existing GraphQL API support feature end to end — gateway artifact kind, control-plane artifact, CLI, console, portal, analytics, and tests — as the structural template. | GraphQL API support (PR #3310, PR #3520) - 2026-10-13 | 2026-10-13 | 2026-10-13 | Drives the file-by-file shape of the gateway, control-plane, CLI, console, and portal work. | N/A |
| Channel-level policy attachment shape | Policies attach to a per-channel list on each channel entry (matching the equivalent type already used elsewhere in the product for asynchronous, channel-style endpoints), rather than a REST-style operation list with a method field. | Feature design - 2026-10-14 | 2026-10-14 | 2026-10-14 | Confirmed no generic, reusable per-operation policy validator exists today for either shape, so the code-reuse argument for the alternative did not hold; a method field would also be redundant since a channel handshake is always the same method. | N/A |
| Per-API connection-limit enforcement and rejection status | Enforced per gateway instance (no cross-replica shared state in this phase); excess handshakes are rejected immediately with a 503 response, never queued. | Feature design - 2026-10-13 | 2026-10-13 | 2026-10-14 | A purpose-built per-route connection counter in the gateway's data plane, not the generic upstream connection-pool circuit breaker (wrong scope — it bounds upstream pooling, not downstream accept count) and not a cluster-wide shared counter. | The exact data-plane extension mechanism still needs a short validation spike before implementation commits to one. |
| Per-upstream-slot connection-limit enforcement mechanism (FR-016) | Each upstream slot's (`main`/`sandbox`) concurrent-connection cap IS enforced via the upstream cluster's native circuit breaker — the opposite tool choice from the downstream row above. | Feature design - 2026-10-16 | 2026-10-16 | 2026-10-16 | The bounded resource here (outbound connections from this gateway instance to the backend) is itself per-instance, so the native circuit breaker's per-instance scope matches what's being protected; this is not the same "wrong scope" problem that ruled it out for the downstream per-API limit. | N/A |
| Revocation / undeploy effect on live connections | Live connections are not affected by undeploy, key revocation, subscription changes, or policy changes; bounded instead by a mandatory finite maximum connection duration. Actively closing live connections on revocation is deferred. | Feature design - 2026-10-13 | 2026-10-13 | 2026-10-13 | Matches the gateway's demonstrated connection-handling behaviour; the control-plane and policy-engine sides defer to this bound rather than attempting active draining. | Whether active draining is needed in a later phase is tracked as future work, not a current blocker. |
| AsyncAPI document parsing | Evaluate a maintained, permissively-licensed AsyncAPI parsing library as the primary candidate; fall back to a minimal in-repo parser scoped to only the fields this feature needs if that library does not clear the standard dependency-vetting bar. | Feature design - 2026-10-13 | 2026-10-13 | NOT ANSWERED | The previously-official parsing library for this document format is archived and was ruled out on that basis alone. | Final library choice is pending a full license, vulnerability, and transitive-dependency review. |
| Gateway version gating | Deploying this artifact kind to a gateway is refused before translation if the gateway's version is below a new minimum-version constant, following the pattern used for the most recently added artifact kind (not the older GraphQL kind, which predates this gating mechanism). | Feature design - 2026-10-13 | 2026-10-13 | 2026-10-13 | No shipped gateway release supports this kind until a future release sets the constant. | N/A |
| Browser test-console transport | The in-portal test console connects directly from the browser to the gateway, rather than being proxied through the portal's own server. | Feature design - 2026-10-13 | 2026-10-13 | 2026-10-13 | The existing server-side try-it proxy only handles ordinary HTTP request/response and was never built to carry a long-lived duplex connection; direct-from-browser avoids adding that capability for no compensating benefit, since the gateway already handles browser-origin validation. | N/A |
