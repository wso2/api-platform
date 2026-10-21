# Implementation Plan: WebSocket API Support in the API Platform Gateway

**Branch**: `001-websocket-api-support` (git branch in use: `websocket`) | **Date**: 2026-10-13 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/001-websocket-api-support/spec.md`

**Note**: This template is filled in by the `/speckit-plan` command; its definition describes the execution workflow.

## Summary

Add `WebSocketApi` as a first-class gateway artifact kind, proxied through the standalone
(Envoy-based) API Platform Gateway, following the `GraphQLApi` feature end to end: a new artifact
type in the gateway management API and xDS translator, a new control-plane (`platform-api`)
artifact with AsyncAPI import, new `ap` CLI commands, console pages, and API Portal
publishing/tryout. The technical approach is almost entirely "do what `GraphQLApi` already does,
for a new kind" — this plan's job is to name the exact files that pattern touches and flag the
handful of places where WebSocket's long-lived, handshake-only nature genuinely diverges from
GraphQL's single-request/response shape (connection lifecycle limits, credential-in-query/
subprotocol handling, handshake-only policy validation, and connection-level rather than
request-level analytics).

## Technical Context

**Language/Version**: Go 1.26.5 (`gateway-controller`, `gateway-runtime/policy-engine`,
`platform-api`, `cli`); TypeScript + React 19.2 (`portals/api-control-plane` console); Node.js +
Express + React (library components) (`portals/api-portal`)

**Primary Dependencies**:
- Gateway/control-plane (Go): existing gateway management API codegen stack (OpenAPI → Go via
  `gateway/gateway-controller/api/management-openapi.yaml`), xDS/Envoy config generation, `gorilla/
  websocket` (already vendored in `platform-api`, used for the control-plane↔gateway channel — not
  a template for the new artifact's handshake handling, which is Envoy's job, not platform-api's).
- AsyncAPI parsing (platform-api): **no AsyncAPI/OpenAPI parser library is currently vendored** —
  GraphQL's "import" equivalent (`graphql_sdl.go`, `graphql_introspection.go`) is hand-rolled
  against GraphQL SDL, not reusable. This is a genuine new dependency decision — resolved in
  [research.md](./research.md) per `dependency-management.md`.
- Console: Vite, React 19.2, Vitest, `@testing-library/react`.
- API Portal: Express, EJS-style server rendering, `libs/async-api-viewer` (already renders
  WebSocket AsyncAPI docs — `WebsocketViewer.tsx`), Cypress (UI E2E), Node's built-in `node --test`
  (unit).
- Integration tests: `tests/framework` (godog/Gherkin BDD), `tests/mock-servers/mock-*-backend` Go
  services as test backends.

**Storage**: `platform-api`'s existing multi-dialect SQL schema set (SQLite, PostgreSQL, SQL
Server, and a generic/default dialect) — a new `websocket_apis` table, additive only, following
the exact shape of the existing `graphql_apis` table (same four schema files, same FK pattern to
`artifacts`/`organizations`/`projects`).

**Testing**:
- Go: stdlib `testing` + `testify` assertions, colocated `_test.go` files (repository, service,
  handler layers) — the project has no ginkgo/BDD-style Go unit tests.
- Gateway/control-plane integration: godog Gherkin `.feature` files under
  `tests/framework/suites/it/features/`, run against a real mock backend service.
- Console: Vitest + React Testing Library.
- API Portal: `node --test` (unit), Cypress (E2E, including two pre-existing **skipped** tryout
  stubs for both GraphQL and WebSocket — see research.md).

**Target Platform**: Linux server / Docker containers (gateway, policy engine, platform-api, CLI);
browser (console, API Portal frontend) — unchanged from the rest of the platform.

**Project Type**: Multi-service monorepo feature — no new service/project is introduced; every
change extends an existing module along the same seams `GraphQLApi` already established.

**Performance Goals** (from spec Success Criteria):
- Handshake latency overhead (gateway + one auth policy): <10ms p95 (SC-006).
- Frame relay overhead: <1ms p95 (SC-006).
- 10,000 concurrent idle WebSocket connections per gateway instance, with the policy engine
  holding no open processing stream per idle connection (SC-007).
- Per-API connection-limit rejection: <100ms, never queued (SC-005).
- Idle/max-duration enforcement: within 1s of the configured bound (SC-004).

**Constraints**:
- Policy chains on a `WebSocketApi` MUST be handshake-only; deployment of a body-processing policy
  chain MUST be rejected (FR-020, SC-009).
- Every `WebSocketApi` MUST have a finite effective maximum connection duration — no unlimited
  lifetime, ever, even by omission (FR-013, FR-031).
- Per-API connection limit enforcement is per gateway instance only in this phase — no
  cross-replica shared state (resolved Clarification); rejection returns `503`.
- No active connection draining on revocation/undeploy in this phase — bounded staleness via the
  mandatory max connection duration only (resolved Clarification).
- Credentials accepted via header, query parameter, or `Sec-WebSocket-Protocol` MUST be stripped
  before reaching the upstream and redacted from every log/analytics sink (FR-030, FR-032, per
  `authentication_authorization.md` GO-AUTH-003).
- AsyncAPI import-by-URL MUST go through the platform's shared SSRF-safe client (FR-052, per
  `ssrf-prevention.md` / `js-ssrf-prevention.md`).
- Origin-allowlist policy (FR-033) MUST use exact-match host comparison, never substring/regex
  matching, per `go-cors-validation.md`.

**Scale/Scope**: One new artifact kind end-to-end across ~7 existing modules/services
(gateway-controller, gateway-runtime/policy-engine, platform-api, cli, api-control-plane console,
api-portal, tests/framework + mock backend). No new deployable service.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle / Section | Assessment |
|---|---|
| I. Developer Experience First | **PASS.** Feature parity with `GraphQLApi`'s UX end to end (CLI, console, portal); unified `401` auth-failure body reused verbatim (FR-023, User Story 2 scenario 1), per `error-handling.md`/`js-error-handling.md`. |
| II. Minimal Footprint | **PASS, with a gated decision.** The one new dependency candidate (an AsyncAPI parser) must clear `dependency-management.md`'s full bar (latest maintained version, `govulncheck` clean, Apache-2.0-compatible, transitive diff reviewed) before being added — resolved in research.md Phase 0, not assumed here. |
| III. Component Independence | **PASS.** Additive artifact kind; no new hard dependency between components. The control-plane↔gateway live-sync channel (`platform-api/internal/websocket/`) is an existing integration point this feature's artifact and keys flow through (FR-054), not a new coupling. |
| IV. GitOps-Ready, Spec/Execution Separation | **PASS.** `websocket_apis` is a new table (additive, all four SQL dialects), matching `db-schema-changes.md`'s R0-FROZEN rule — no existing shipped table is altered. |
| V. AI-Ready by Design | **PASS.** AsyncAPI URL import (FR-052) routes through the shared SSRF-safe client; the WebSocket test console and tryout are explicitly in scope (User Story 5, FR-071) so agentic/human callers get the same hardening. |
| VI. Container-First Distribution, Policy-First Gateway | **PASS.** No new xDS/control-plane *surface* is introduced — this feature adds a kind to the existing, already-hardened xDS server (`go-control-plane-xds-security.md` governs that server as a whole, not per-kind). The translator change must still carry forward `NormalizePath`/`MergeSlashes`/`PathWithEscapedSlashesAction` onto WebSocket routes (directive 6) — verified in Phase 1 data-model/contracts, not assumed. |
| Security & Compliance Standards | **PASS, enforced per-FR.** GO-AUTH-005 (tenant isolation on new `websocket_apis` queries), GO-AUTH-007 (new `ap:websocket_api:*` scopes, deny-by-default), `ssrf-prevention.md` (FR-052), `go-cors-validation.md` (FR-033 exact-match only), `go-network-service-hardening.md` (FR-012/013 bounded idle/max-duration, FR-040/042 bounded concurrency, FR-015–FR-017 the same bounds applied to each upstream connection pool). `xxe-xml-processing.md`/`js-xxe-xml-processing.md` are **N/A** — AsyncAPI documents are JSON/YAML, not XML; noted explicitly rather than silently skipped. |
| Development Workflow & Quality Gates | **PASS, tracked.** New table's shape fixed in data-model.md §4 by pattern-matching the existing `graphql_apis` table; the `designing-db-schemas` skill itself is still to be invoked against that shape at T006 (not yet run as of this plan) to catch any R1–R10 gaps pattern-matching alone might miss. Any new Go dependency runs the full `dependency-management.md` workflow before being added (Phase 0 research.md); this spec/plan pair itself *is* the GitOps-committed artifact this gate asks for. |

**No violations requiring Complexity Tracking justification.** This feature fits entirely inside
the architecture `GraphQLApi` already established — no new service, no new xDS/control-plane
surface, no new cross-component dependency.

### Post-Design Re-Check (after Phase 0/1)

Re-evaluated against research.md and data-model.md; no new violations surfaced.

- **VI. Container-First Distribution, Policy-First Gateway**: research.md §2's connection-limit
  mechanism (a custom per-route counter) must be delivered as a standard, xDS-configured Envoy
  extension point (a Lua filter or Envoy's Go HTTP filter support) — both are sanctioned Envoy
  extensibility mechanisms already reachable from this gateway's config, not a hand-patched fork of
  Envoy's own source. **Still PASS**, with this constraint made explicit so `/speckit-tasks` can't
  satisfy FR-040 by patching the vendored Envoy build.
- **II. Minimal Footprint**: the AsyncAPI parser decision (research.md §1) remains conditional on
  passing the full `dependency-management.md` gate at implementation time; if the chosen library
  fails that gate, the in-repo-parser fallback keeps this principle satisfied either way. **Still
  PASS.**
- **Added after initial Phase 0/1 design — FR-015–FR-017 (upstream connection settings)**:
  `research.md` §8 re-reads the original downstream connection-limit spike (§2) for a different
  purpose — confirming the upstream cluster's native circuit breaker is the right primitive for a
  per-upstream-slot `maxConnections` cap, unlike the downstream case. This reuses and extends the
  existing `UpstreamTimeout`/`Upstream` schemas (T001a) rather than introducing new ones — an
  additive, backward-compatible change to a schema also used by shipped `GraphQLApi`/`RestApi`.
  **Still PASS** under III (no hard cross-component dependency introduced) and VI (still the
  existing xDS server, no new surface); flagged here because this work was added after this plan's
  original Constitution Check pass and the row above (Security & Compliance Standards) already
  reflects it.
- All other rows unchanged from the pre-design Constitution Check above.

## Project Structure

### Documentation (this feature)

```text
specs/001-websocket-api-support/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md         # Phase 1 output (/speckit-plan command)
├── quickstart.md         # Phase 1 output (/speckit-plan command)
├── contracts/             # Phase 1 output (/speckit-plan command)
│   ├── gateway-management-api.md
│   └── platform-api.md
├── checklists/
│   └── requirements.md
└── tasks.md              # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

No new project/service is introduced. This is a multi-module Go + TypeScript/Node monorepo
(`go.work` lists the Go modules); the structure below extends existing modules along the exact
seams `GraphQLApi` already established, named file by file so `/speckit-tasks` can generate
concrete tasks directly.

```text
gateway/gateway-controller/                      # Go module: control plane + xDS translator
├── api/management-openapi.yaml                  # add WebSocketAPI/WebSocketAPIConfigData schemas,
│                                                 #   /websocket-apis[...] paths (source of truth);
│                                                 #   ALSO extends the existing, shared UpstreamTimeout
│                                                 #   schema (idleTimeout/maxConnectionDuration/
│                                                 #   maxConnections, alongside its pre-existing
│                                                 #   connect field) and adds a new optional `timeout`
│                                                 #   property to the existing Upstream schema — used
│                                                 #   today by shipped GraphQLApi/RestApi; additive and
│                                                 #   backward-compatible, name kept unchanged (T001a)
├── pkg/api/management/generated.go               # codegen output — regenerate, do not hand-edit
├── pkg/api/handlers/websocket_api_handler.go     # NEW — mirrors graphql_api_handler.go
├── pkg/transform/websocket.go                    # NEW — mirrors transform/graphql.go
├── pkg/transform/registry.go                     # register the WebSocket transformer
├── pkg/utils/websocket_deployment.go             # NEW — mirrors utils/graphql_deployment.go
└── pkg/xds/translator.go                         # extend: per-route upgrade_configs, idle/max
                                                    #   stream duration, confirm NormalizePath/
                                                    #   MergeSlashes carry over unchanged (directive 6)

gateway/gateway-runtime/policy-engine/            # Go module: ext_proc data-plane policy engine
├── internal/resolver/resolver.go                 # recognize APIKindWebSocket, surface matched channel
├── internal/kernel/fault_entry.go                # force request/response body mode NONE (FR-021)
└── internal/analytics/fault_classification.go    # classify normal close vs error (FR-081)

platform-api/                                      # Go module: control-plane REST API
├── internal/database/schema.sql                  # add websocket_apis table (generic dialect)
├── internal/database/schema.sqlite.sql           # same table, SQLite dialect
├── internal/database/schema.postgres.sql         # same table, PostgreSQL dialect
├── internal/database/schema.sqlserver.sql        # same table, SQL Server dialect
├── internal/model/websocket_api.go               # NEW
├── internal/dto/websocket_api.go                 # NEW
├── internal/repository/websocket_api.go          # NEW — mirrors repository/graphql_api.go
├── internal/service/
│   ├── websocket_api.go                          # NEW — mirrors service/graphql_api.go
│   ├── websocket_asyncapi.go                     # NEW — AsyncAPI parse/derive (file/URL/inline)
│   ├── websocket_mapping.go                      # NEW
│   ├── websocket_deployment.go                   # NEW
│   └── artifact_import_websocket.go              # NEW
├── internal/handler/
│   ├── websocket_api.go                          # NEW — CAUTION: existing handler/websocket.go is
│   │                                              #   the control-plane↔gateway live channel
│   │                                              #   (deployment acks), an unrelated feature —
│   │                                              #   pick a non-colliding file/type name
│   ├── websocket_api_apikey.go                   # NEW
│   └── websocket_api_deployment.go               # NEW
├── internal/gatewaytranslator/
│   ├── gwversion/versions.go                     # add MinWebSocketKindGatewayVersion = NoLTSRelease
│   └── kinds/websocket.go                        # NEW — kind-specific CP→gateway translation
├── internal/websocket/                           # EXISTING control-plane↔gateway channel — FR-054
│   │                                              #   (bulk sync, live deployment events) extends
│   │                                              #   this channel's payload; not restructured
├── resources/role-to-scope-mapping.yaml          # add ap:websocket_api:{manage,read},
│                                                  #   ap:websocket_api:deployment:{manage,read}
└── resources/openapi.yaml                        # add WebSocketAPI schemas + /websocket-apis paths

cli/src/cmd/gateway/
├── websocketapi/
│   ├── root.go, list.go, get.go, delete.go       # NEW — mirrors cmd/gateway/graphqlapi/*.go
│   │                                              #   (list/get/delete only — no create/apply
│   │                                              #   subcommand, matching the GraphQL precedent)
│   ├── commands_test.go                          # NEW
│   └── apikey/
│       ├── root.go, create.go, list.go, regenerate.go, update.go, revoke.go
│       └── commands_test.go
└── root.go                                        # register websocketapi.Root()
cli/src/internal/gateway/resources.go              # add ResourceKindWebSocketAPI + WebSocketAPIHandler
                                                    #   (GetEndpoint/CreateEndpoint/UpdateEndpoint) and
                                                    #   its case in GetResourceHandler, mirroring
                                                    #   GraphQLAPIHandler — required for `ap gateway
                                                    #   apply -f <file>.yaml` to support the new kind;
                                                    #   this is the only CLI path that actually
                                                    #   creates/updates a gateway-first resource
cli/src/utils/constants.go                         # add GatewayWebSocketAPIsPath = "/websocket-apis",
                                                    #   GatewayWebSocketAPIByIDPath = "/websocket-apis/%s"
docs/cli/gateway/README.md                         # add "## WebSocket API Commands" section — note:
                                                    #   no GraphQL section exists either, so this is
                                                    #   net-new documentation, not a copy

portals/api-control-plane/src/                     # React + Vite console
├── pages/appShell/appShellPages/apis/create/uiConfig.tsx         # register 'websocket' kind
├── pages/appShell/appShellPages/apis/create/components/
│   ├── ApiCreationSteps.tsx                      # extend wizard steps for WebSocket
│   └── ContractSourceForm.tsx                    # extend contract-source step (AsyncAPI file/URL/paste)
├── pages/appShell/appShellPages/apis/listing/apiTypeFilter.ts    # add 'websocket' to ApiTypeFilter/API_KINDS
├── pages/appShell/appShellPages/apis/utils/websocketApiDisplay.ts # NEW — mirrors restApiDisplay.ts
└── routes/{AppRoutes.tsx,paths.ts}                # register WebSocket routes

portals/api-portal/
├── libs/async-api-viewer/src/components/WebsocketViewer/WebsocketViewer.tsx  # EXISTING — extend to
│                                                                              #   show live gateway
│                                                                              #   URLs (FR-055)
├── src/controllers/tryoutProxyController.js       # extend — currently plain HTTP proxy, no
├── src/services/tryoutProxyService.js             #   upgrade/protocol handling at all (confirmed);
├── src/routes/pages/tryoutProxyRoute.js           #   WebSocket connect/send/receive is new work
├── src/styles/async-tryout.css                    # EXISTING — likely reusable as-is
└── it/ui/cypress/e2e/websocket-apis/tryout.cy.js  # EXISTING — currently one `it.skip(...)` stub;
                                                    #   implement for real (GraphQL's sibling stub is
                                                    #   also unimplemented — no working tryout to copy)

tests/
├── framework/suites/it/features/
│   ├── websocket_deploy.feature                  # NEW — mirrors graphql_deploy.feature
│   ├── websocket_policies.feature                # NEW — mirrors graphql_policies.feature
│   └── websocket_api_keys.feature                # NEW — mirrors graphql_api_keys.feature
├── framework/suites/it/resources/templates/websocket-api.yaml   # NEW fixture
└── mock-servers/mock-websocket-backend/           # NEW Go module — mirrors mock-graphql-backend
    ├── main.go
    └── Dockerfile

# Out of scope — do not modify for this feature:
# - event-gateway/gateway-runtime/internal/connectors/receiver/websocket/  (WebBrokerApi, WS↔Kafka —
#   a different feature entirely; confirmed via broker_api_connector.go)
# - platform-api/internal/handler/websocket.go, websocket_readonly.go,
#   internal/websocket/websocket_transport.go  (control-plane↔gateway channel — integration point
#   per FR-054 only, not restructured)
```

**Structure Decision**: No new project/service. Almost every addition is a new, kind-specific
sibling file next to the `graphql_*`-named file it mirrors, in the exact modules `GraphQLApi`
already touches — this keeps the diff reviewable file-by-file against a known-good precedent and
avoids inventing a structure this monorepo doesn't already use. The one exception is
`management-openapi.yaml`'s `UpstreamTimeout`/`Upstream` schemas (T001a, above): those are existing,
shared schemas already used by `GraphQLApi`/`RestApi`, extended in place rather than duplicated —
deliberately reusing what already exists instead of adding a parallel WebSocket-only schema, even
though that makes it the one file in this feature touching pre-existing shared state. The two "out
of scope" existing `websocket`-named code paths are called out explicitly because their naming
collides with this feature's vocabulary despite being functionally unrelated (control-plane
transport channel, event-gateway broker bridging) — a `/speckit-tasks` author or implementer must
not merge work into either of them by name-matching alone.

## Complexity Tracking

*No entries — Constitution Check reported no violations.*
