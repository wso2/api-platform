# Phase 0 Research: WebSocket API Support

Resolves every `NEEDS CLARIFICATION` left in [plan.md](./plan.md)'s Technical Context, plus the
open technical risks surfaced while mapping the existing `GraphQLApi` implementation this feature
follows.

## 1. AsyncAPI parsing library for platform-api

- **Decision**: Evaluate `github.com/MarkRosemaker/asyncapi` as the primary candidate (Apache-2.0,
  supports AsyncAPI 2.x and 3.x, most recently updated of the alternatives found); fall back to a
  minimal in-repo parser for the specific subset this feature needs (`info`, `servers`, `channels`,
  `operations`, parameters) if that library doesn't clear full vetting.
- **Rationale**: The official `github.com/asyncapi/parser-go` is **archived** (read-only as of
  2025-08-04) — `dependency-management.md` directive 1 rejects archived repos outright, so it's
  disqualified regardless of license fit. `MarkRosemaker/asyncapi` is a parser/formatter library
  scoped to the AsyncAPI spec itself, not a heavier code-generation framework. GraphQL's own
  "import" equivalent in this codebase (`graphql_sdl.go`, `graphql_introspection.go`) is hand-rolled
  rather than built on a third-party SDL library, so an in-repo parser for the small subset
  AsyncAPI requires here (far smaller surface than full OpenAPI) is a consistent, lower-footprint
  fallback, not a lesser option — it also avoids the exact "new dependency" scrutiny entirely.
- **Alternatives considered**: `github.com/lerenn/asyncapi-codegen` and its fork
  `github.com/dimonoff/asyncapi-codegen` — both are primarily **code-generation** tools (client/
  server scaffolding), a much larger dependency surface than this feature needs (it only has to
  *read* a document, never generate code from it), and the fork is an early, low-adoption variant
  (`v0.1.0`) of the original with no independent maintenance signal.
- **Action before implementation**: whichever path is chosen, it must still run the full
  `dependency-management.md` workflow (`go list -m -versions`, `govulncheck ./...`, `go-licenses
  check ./...`, transitive `go mod graph` diff) before merging — this research only narrows the
  candidate, it doesn't substitute for that gate.

## 2. Per-API concurrent connection limit (FR-040) — enforcement mechanism

- **Decision**: Enforce the limit with a custom, HTTP-route-scoped connection counter in the
  router/data-plane (an atomic increment on handshake accept, decrement on close, keyed by API),
  implemented as a gateway-controller-authored Envoy extension (Lua or Envoy Go HTTP filter) rather
  than a built-in Envoy primitive — **not** Envoy's native `connection_limit` network filter and
  **not** cluster circuit breakers.
- **Rationale**: Envoy's `connection_limit` network filter counts connections at **filter-chain**
  scope (L4/listener level), not per HTTP route — multiple `WebSocketApi`s sharing a listener/vhost
  would share one counter, not get independent per-API limits, unless each API got its own filter
  chain (a much heavier xDS footprint than one route per channel). Cluster circuit breakers bound
  **upstream** connection pooling, not downstream accept count, and the original spike already
  found them "per pod, burst-sensitive, and queue or reject unpredictably" for this purpose. The
  policy engine (ext_proc) cannot see connection closes once request/response body modes are
  `NONE` (FR-021), so counting there would double-count or leak — the router is the only layer that
  reliably sees both open and close.
- **Alternatives considered**: Envoy native `connection_limit` filter (rejected — wrong scope,
  per-listener not per-route); cluster circuit breakers (rejected — upstream-pool scoped, already
  shown unreliable by the spike); counting in the policy engine (rejected — blind to closes under
  `NONE` body mode); cluster-wide/shared-state counting (rejected by the resolved Clarification —
  out of scope for this phase).
- **Open risk, flagged for implementation**: the exact extension mechanism (Lua vs. Envoy's Go
  HTTP filter support vs. a small purpose-built WASM filter) needs a short validation spike before
  `/speckit-tasks` commits to one — this research narrows the *scope* (router, HTTP-level, custom)
  but not the *exact Envoy extension technology*.

## 3. Generic cross-kind policy validator — confirmed absent

- **Finding**: No generic, shared per-operation/per-kind policy validator exists in
  `gateway-controller` today (confirmed: no `func Validate.*Polic` match outside kind-specific
  handler files). `GraphQLApi`'s own "validation" is inline logic in
  `pkg/api/handlers/graphql_api_handler.go` (`mapValidationError`, `isGraphQLAPICreateBadRequest`),
  not a standalone, reusable function.
- **Decision**: `WebSocketApi` validation (FR-003, FR-020) follows the same pattern — inline
  validation in the new `websocket_api_handler.go`, including the handshake-only policy chain check
  (FR-020) — rather than introducing a new generic validator as part of this feature. Refactoring
  REST/GraphQL onto a shared validator is out of scope; this feature adds a kind, it doesn't
  restructure existing ones.

## 4. WebSocket test console / tryout — no existing implementation to copy

- **Finding**: Both the GraphQL and WebSocket browser tryout flows are currently **unimplemented
  placeholders** — `portals/api-portal/it/ui/cypress/e2e/graphql-apis/tryout.cy.js` and
  `.../websocket-apis/tryout.cy.js` each contain only `it.skip(...)` stubs. The shared tryout proxy
  (`tryoutProxyController.js`/`tryoutProxyService.js`/`tryoutProxyRoute.js`) is a plain HTTP
  request/response proxy today — confirmed no `upgrade`/`websocket`/protocol-switching handling
  exists in it at all.
- **Decision**: The WebSocket test console (FR-071) connects **directly from the browser to the
  gateway** (`ws://`/`wss://`, with the test key/token as a query parameter or
  `Sec-WebSocket-Protocol` entry, since the browser cannot set headers on a WebSocket handshake —
  already specified in FR-071). It does **not** route through `tryoutProxyController.js`'s existing
  HTTP-proxy path, because proxying a raw, long-lived duplex WebSocket stream through the
  API Portal's Node server is a materially different (and unnecessary) problem — the browser can
  open the WebSocket connection itself once the gateway URL is known, the same way it would for a
  real client. `tryoutProxyController.js` remains unchanged for REST/GraphQL's HTTP tryout.
- **Alternatives considered**: proxying through the Node server (rejected — adds a hop, a new
  duplex-streaming code path, and session/cookie complexity the direct-from-browser approach
  avoids entirely, with no compensating benefit since the gateway already handles CORS/origin
  validation for browser clients per FR-033).

## 5. `WebsocketViewer.tsx` reuse

- **Finding**: `portals/api-portal/libs/async-api-viewer/src/components/WebsocketViewer/
  WebsocketViewer.tsx` already exists and renders AsyncAPI definitions for WebSocket APIs at the
  data level (platform-api already models `APITypeWS`/`APISubTypeWebSocket`).
- **Decision**: Extend this existing component to show **live gateway URLs** once a `WebSocketApi`
  is actually deployed (FR-055) rather than building a new viewer — it is confirmed working
  infrastructure, not a stub (unlike the tryout flow in item 4).

## 6. Gateway version gating constant

- **Decision**: Add `MinWebSocketKindGatewayVersion = NoLTSRelease` to
  `platform-api/internal/gatewaytranslator/gwversion/versions.go`, following the exact precedent of
  `MinWebBrokerKindGatewayVersion`/`MinAgentKindGatewayVersion` in the same file (both also start at
  `NoLTSRelease` until a gateway release ships the kind). `GraphQLApi` itself predates this gating
  mechanism, so it has no equivalent constant — `WebBrokerApi` is the correct precedent to follow,
  not `GraphQLApi`, for this one specific piece.

## 7. xDS path-canonicalization carry-over (`go-control-plane-xds-security.md` directive 6)

- **Finding, verified (not assumed)**: `NormalizePath`, `MergeSlashes`, and
  `PathWithEscapedSlashesAction` are set once per listener at HTTP Connection Manager creation
  time in `gateway-controller/pkg/xds/translator.go`'s `CreateListener` path, confirmed by existing
  tests (`TestTranslator_CreateListener_PathWithEscapedSlashesAction`,
  `translator_test.go:1464-1468`) asserting `manager.GetNormalizePath()`/`GetMergeSlashes()` are
  non-nil/true. These are listener-level HCM settings, not per-route or per-kind.
- **Decision**: No new code is needed for `WebSocketApi` routes to inherit this hardening — they
  share the same listener and HCM as every other kind. Phase 1's contracts note this explicitly so
  an implementer doesn't duplicate the setting per-kind under a mistaken assumption that it's
  missing.

## 8. Upstream per-slot connection limit (FR-016) — circuit breaker re-read, and a gap it doesn't close

- **Context**: FR-015–FR-017 (added after this document's original Phase 0 pass) use the upstream
  cluster's native `circuit_breakers.thresholds.max_connections` to cap concurrent connections to
  each upstream slot (`main`/`sandbox`). §2 above rejected this identical mechanism for the
  *downstream* per-API limit — this section establishes why the same tool is nonetheless the right
  one for the *upstream* case, using evidence already gathered, and what it still leaves open.
- **Finding, re-reading the original spike for a different purpose**: §2's rejection cites spikes
  11/11b/11c (`gateway/spec/impls/4-websocket-api-support/research.md`) as proof the mechanism is
  "per pod, burst-sensitive, and queue or reject unpredictably" — true, and disqualifying for an
  exact downstream admission count (SC-005). But those same spikes, read for the upstream question
  instead, already confirm something useful: spike 11c's result (3 connected, 1 rejected, 1 queued
  ~8s **until a slot freed**) shows Envoy holds a long-lived upgraded connection counted as active
  against the circuit breaker for its *entire* open duration, not just during the initial handshake
  — so the mechanism does apply to upgraded connections at all, which was not obvious from the
  Envoy documentation alone (the docs don't address protocol-upgrade connections specifically).
- **Decision**: Use the native circuit breaker for FR-016 despite its imprecision, because what it
  protects upstream — outbound connections *this gateway instance* opens to the backend — is
  itself a per-instance quantity. The same per-pod scope that disqualified it downstream (a shared,
  cluster-wide resource needing an exact count) is the *correct* scope here.
- **Remaining gap, not resolved by re-reading**: all three spikes ran against a single-host echo
  backend (`gateway/spec/impls/4-websocket-api-support/research.md`, Spike Setup). Envoy's
  documented behavior — guaranteeing *at least one connection per selected host* even once the
  threshold is hit — is the actual source of overshoot above the configured `max_connections`, and
  it only manifests when the cluster resolves to multiple hosts. A single-host backend can't
  expose it. This needs its own short spike (a multi-host upstream, `max_connections` set tight,
  measuring actual overshoot) before FR-016 is considered fully verified, mirroring how §2's
  mechanism choice itself was spike-verified rather than assumed from documentation.
