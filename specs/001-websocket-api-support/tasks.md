---

description: "Task list template for feature implementation"
---

# Tasks: WebSocket API Support in the API Platform Gateway

**Input**: Design documents from `/specs/001-websocket-api-support/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md (all present)

**Tests**: Included. `FR-090` in spec.md explicitly requires gateway integration test coverage
(deploy/connect, auth, policy validation, limits, undeploy, sandbox, API keys, analytics) as a
functional requirement of the feature, not an optional extra — so test tasks are generated
alongside implementation tasks for every user story.

**Organization**: Tasks are grouped by user story (spec.md's priorities P1–P3) to enable
independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1–US6)
- File paths below are exact, per plan.md's Project Structure.

## Path Conventions

Multi-module monorepo (see plan.md "Structure Decision") — no single `src/`/`tests/` root. Every
task names its real module path explicitly (`gateway/gateway-controller/...`, `platform-api/...`,
`cli/src/...`, `portals/api-control-plane/...`, `portals/api-portal/...`, `tests/...`).

---

## Phase 1: Setup

**Purpose**: Contract surface and dependency decisions every later phase builds on.

- [X] T001 Add `WebSocketAPI`/`WebSocketAPIConfigData` schemas and `/websocket-apis[...]` paths to `gateway/gateway-controller/api/management-openapi.yaml` per contracts/gateway-management-api.md, then regenerate `gateway/gateway-controller/pkg/api/management/generated.go` (codegen output — do not hand-edit)
- [X] T001a (FR-015–FR-017) In the same spec file (T001), extend the existing `UpstreamTimeout` schema with `idleTimeout`/`maxConnectionDuration`/`maxConnections` (alongside its pre-existing `connect` field) and add a new optional `timeout: $ref: '#/components/schemas/UpstreamTimeout'` property to the existing `Upstream` schema (`main`/`sandbox`) — reusing the component already referenced by `RestApi`'s `UpstreamDefinition.timeout`, rather than introducing a new parallel schema. This is an additive, backward-compatible widening of a schema also used by shipped `GraphQLApi`/`RestApi` — the schema name stays `UpstreamTimeout` (no rename) specifically so the existing generated Go struct and its one call site, `resolveUpstreamCluster` (`gateway-controller/pkg/xds/translator.go:1241`, which reads `UpstreamTimeout.Connect`), keep compiling unchanged against the extended struct
- [ ] T002 Add `WebSocketAPI`/`WebSocketAPIDetail` schemas, new scopes, and `/websocket-apis[...]` paths to `platform-api/resources/openapi.yaml` per contracts/platform-api.md
- [ ] T003 [P] Vet the AsyncAPI parsing dependency per research.md §1 and `dependency-management.md`: run `go list -m -versions github.com/MarkRosemaker/asyncapi`, `govulncheck`, and `go-licenses check` against it; if it fails the gate, finalize the in-repo minimal-parser fallback scope (info/servers/channels/operations/parameters only) instead — record the decision in platform-api's go.mod/go.sum or a short decision note
- [ ] T004 [P] Add `MinWebSocketKindGatewayVersion = NoLTSRelease` to `platform-api/internal/gatewaytranslator/gwversion/versions.go` per research.md §6 (follows the `MinWebBrokerKindGatewayVersion` precedent, not `GraphQLApi` which predates this gating mechanism)
- [X] T005 [P] Scaffold `tests/mock-servers/mock-websocket-backend/main.go` and `Dockerfile` as a `ws://` echo backend, mirroring `tests/mock-servers/mock-graphql-backend`

**Checkpoint**: Contract surface and parser/version-gate decisions are fixed; codegen output is current.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core artifact type, storage, and scopes every user story depends on.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [ ] T006 (FR-050) Add `websocket_apis` table to `platform-api/internal/database/schema.sql` (SQLite-dialect base file) — additive new table per `db-schema-changes.md` R0-FROZEN **and R0-UPGRADE-PATH** (guard with `CREATE TABLE IF NOT EXISTS`), columns and types exactly as data-model.md §4, skill-validated against `graphql_apis`'s actual DDL: `uuid VARCHAR(40) PRIMARY KEY` (also FK→`artifacts(uuid)` `ON DELETE CASCADE`), `organization_uuid VARCHAR(40) NOT NULL` (FK→`organizations(uuid)` `ON DELETE CASCADE` — every query MUST filter from verified JWT claims, never request input, per GO-AUTH-005), `project_uuid VARCHAR(40) NOT NULL` (FK→`projects(uuid)` `ON DELETE CASCADE`), `handle VARCHAR(40) NOT NULL`, `display_name VARCHAR(255) NOT NULL`, `version VARCHAR(30) NOT NULL DEFAULT 'v1.0'`, `description VARCHAR(1023)` nullable, `configuration BLOB NOT NULL` (JSON payload), `data_version VARCHAR(20) NOT NULL DEFAULT '1.0'` (**not an integer**), `origin VARCHAR(20) NOT NULL DEFAULT 'control_plane'` (values `control_plane`/`gateway_api` per `constants.OriginCP`/`OriginDP`), `created_by VARCHAR(200)`, `created_at DATETIME DEFAULT CURRENT_TIMESTAMP`, `updated_by VARCHAR(200)`, `updated_at DATETIME DEFAULT CURRENT_TIMESTAMP`; `UNIQUE(organization_uuid, handle)`; only one index needed — `idx_websocket_apis_project` on `project_uuid` (no separate org index — the UNIQUE constraint's leftmost column already covers it, per R6-NO-REDUNDANT-INDEX; do not copy `graphql_apis`'s existing redundant `idx_graphql_apis_org`)
- [ ] T007 [P] (FR-050) Add the identical `websocket_apis` table to `platform-api/internal/database/schema.sqlite.sql` (same types/guard as T006)
- [ ] T008 [P] (FR-050) Add the identical `websocket_apis` table to `platform-api/internal/database/schema.postgres.sql` — same shape as T006 with Postgres types: `configuration BYTEA NOT NULL`, `created_at`/`updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP`; `ON DELETE CASCADE` on all three FKs
- [ ] T009 [P] (FR-050) Add the identical `websocket_apis` table to `platform-api/internal/database/schema.sqlserver.sql`, guarded with an `OBJECT_ID(...) IS NULL` check (R0-UPGRADE-PATH) — Postgres-equivalent types (`configuration VARBINARY(MAX) NOT NULL`, `created_at`/`updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME()`), **except** the `organization_uuid` FK MUST use `ON DELETE NO ACTION` (not `CASCADE`) — SQL Server rejects multiple cascade paths converging on `organizations` via `uuid→artifacts→organizations` and directly, the same divergence `graphql_apis.sqlserver.sql` already carries. No new cleanup code is needed: the row is already guaranteed deleted via the other cascade path (`uuid→artifacts(uuid) ON DELETE CASCADE`) before this FK would otherwise matter — comment this inline as intentional, not an R8 finding
- [X] T010 (FR-001) Add `KindWebSocketApi` to the gateway-controller artifact-kind registry (alongside `KindGraphQLApi`), backed by the `WebSocketAPIConfigData` type regenerated in T001 — fields per data-model.md §1: `displayName` (string, required), `version` (string, required), `context` (string, required, unique per gateway+version), `upstream.main` (object, required, `ws`/`wss` only), `upstream.sandbox` (object, optional, same validation), `connection.idleTimeout`/`connection.maxConnectionDuration` (duration, defaulted — never unlimited), `connection.maxConnections` (integer, defaulted), `channels` (array, required, min 1), `policies` (array, optional). Each of `upstream.main`/`upstream.sandbox` additionally carries its own optional nested `timeout.connect`/`timeout.idleTimeout`/`timeout.maxConnectionDuration`/`timeout.maxConnections` (FR-015–FR-017) via the extended `UpstreamTimeout` type from T001a — independent per slot, same duration/integer types as the API-level fields
- [X] T010a (FR-001) Add a `websocket_apis` table to the gateway controller's **own local database** (separate from platform-api's) per data-model.md §5 — a one-column-identical clone of its existing `graphql_apis` child table: `uuid`, `gateway_id`, `configuration` (JSON text), composite `PRIMARY KEY (gateway_id, uuid)`, `FOREIGN KEY(gateway_id, uuid) REFERENCES artifacts(gateway_id, uuid) ON DELETE CASCADE`. Add it to **all four** of that component's schema files: `gateway/gateway-controller/pkg/storage/gateway-controller-db.sql` (note: `resources/gateway-controller-db.sql` is a **symlink** to this file, not a second copy, so editing one is editing both — verified during implementation), plus `pkg/storage/gateway-controller-db.postgres.sql` and `pkg/storage/gateway-controller-db.sqlserver.sql`, using that component's own existing type convention (`TEXT` / `NVARCHAR(64)`+`NVARCHAR(MAX)`), not platform-api's. Also register the new kind in `gateway/gateway-controller/pkg/storage/sql_store.go`'s kind→table-name mapping and its `builtinResourceTables` list, so the storage layer can read/write it
- [X] T011 (FR-001) Register a `WebSocket` entry in `gateway/gateway-controller/pkg/transform/registry.go` (implementation added in T018)
- [ ] T012 [P] (FR-050) Create `platform-api/internal/model/websocket_api.go` with the field set from data-model.md §4 (same column list as T006)
- [ ] T013 [P] (FR-050) Create `platform-api/internal/dto/websocket_api.go` (request/response DTOs mirroring `dto/graphql_api.go`'s shape)
- [ ] T014 (FR-051) Add `ap:websocket_api:manage`, `ap:websocket_api:read`, `ap:websocket_api:deployment:manage`, `ap:websocket_api:deployment:read` to `platform-api/resources/role-to-scope-mapping.yaml` and the AI Workspace BFF default OIDC scopes, per contracts/platform-api.md

**Checkpoint**: Artifact type, storage, and scopes exist — user story implementation can begin.

---

## Phase 3: User Story 1 - Deploy a WebSocket API to the gateway and connect through it (Priority: P1) 🎯 MVP

**Goal**: Gateway-first deploy of a `WebSocketApi`, with clients able to connect and exchange
frames with the upstream; non-`WebSocketApi` routes still reject upgrades; delete takes effect
immediately for new handshakes.

**Independent Test**: Deploy a `WebSocketApi` with one channel pointing at the mock echo backend,
connect a WebSocket client through the gateway, send messages, receive the echoes; delete the API
and confirm new handshakes return `404` (quickstart.md Scenario 1).

### Tests for User Story 1

- [X] T015 [P] [US1] (SC-002) Wrote `tests/framework/suites/it/features/websocket_deploy.feature` covering 4 scenarios (deploy+connect+echo+ping/pong, upstream update takes effect, non-`101` backend status passed through, handshake to an unconfigured channel path). All 4 scenarios pass cleanly against a real Envoy (sqlite/postgres/sqlserver variants, 68/68 steps each) once T015b's actual blockers were fixed — see below; the HCM/route upgrade mechanism itself was correct all along. **Not yet covered**: TLS/wss upstream, REST-route-rejects-upgrade-with-403 as its own IT scenario (covered today only by the Go unit test `TestWebSocketUpgradeIsNotAppliedToOtherKinds`), subprotocol negotiation as a dedicated scenario (exercised incidentally by the upstream-update scenario), sandbox routing, and the T017a gateway-level-API-key scenario.
- [X] **T015b [US1] (SC-002) Root-caused and fixed — three independent test-infrastructure bugs, NOT a gateway-controller/Envoy product defect.** The initial IT run reported every handshake failing with a bare `403`, which looked like the HCM/route `upgrade_configs` mechanism (T019) didn't work against a real Envoy. It wasn't: live `/config_dump` inspection (`dynamic_route_configs`/`clusters`/`/stats` via the Envoy admin API) and the `/stats` `rds.*`/`cds.*` counters, taken across multiple live runs, confirmed the translator emits exactly the right route (`upgrade_configs: [{upgrade_type: websocket, enabled: true}]`, correct cluster) and Envoy applies it correctly once it propagates. The real blockers, found in order as each was fixed and the next one surfaced:
  1. **The test itself had no propagation-wait.** Every other feature in this suite retries its first post-deploy data-plane call (`... until status N`) to absorb the store→xDS→Envoy propagation gap; `websocket_deploy.feature` fired its handshake once, immediately after deploy, and got the `403` Envoy returns for an Upgrade request against any route that hasn't (yet) enabled that upgrade type. Fixed by adding a retry-capable `I open a WebSocket connection to "..." until status N` step (`tests/framework/suites/it/steps/platformgateway/websocket.go`, mirroring `Base.sendUntilStatus`) and using it for every handshake that follows a deploy/update.
  2. **The testbench's WebSocket mock service was never actually running.** `tests/framework/core/catalog/testbench/definition.go` never registered the service's `Endpoints` entry (only `cmd/testbench/main.go`'s service list did), so the IT framework never rebuilt/validated it; the cached `ghcr.io/wso2/api-platform/testbench:test` image on the dev machine predated the service's existence entirely, and separately its port (3014) collided with the already-registered `mcp-legacy` service, a collision the catalog's own startup validation now catches. Fixed by registering the `websocket` endpoint in `definition.go`, moving the port to the first free slot (3017, with `Dockerfile`'s `EXPOSE` range bumped to match), and rebuilding the image via the documented `make testbench` target (this is a manual step for anyone whose local image predates testbench source changes — not run automatically by the IT suite).
  3. **A real bug in the testbench's own mock, surfaced only once (1) and (2) let traffic actually reach it.** `tests/framework/testbench/services/websocket/websocket.go` hijacked the connection via a raw `w.(http.Hijacker)` type assertion; the testbench's `observability` middleware (`testbench/logging.go`) wraps every handler's `ResponseWriter` in a `responseRecorder` for access logging, and that wrapper exposes `Hijack` only through the `Unwrap() http.ResponseWriter` convention `http.ResponseController` knows to follow — a direct type assertion on the wrapper fails even though the real connection supports hijacking, producing a `500 connection cannot be hijacked` for every real handshake attempt. Fixed by switching to `http.NewResponseController(w).Hijack()`; verified directly against the backend (bypassing Envoy) before and after.

  None of this touched `gateway/gateway-controller/pkg/xds/translator.go` (T019) or any other product code — the shipped HCM-disabled/route-enabled upgrade design is correct as implemented.
  - **Unrelated observation from the same investigation, not a WebSocket issue**: `mcp-legacy-upstream` and `mcp-legacy-upstream-modern-client` intermittently failed with a 60s timeout / "upstream connect error... remote connection failure" during this session's many repeated runs, both before and after reverting every change made in this session (confirmed via two independent revert-and-retest passes — ruled out the `sdk/core` go.mod replace fix below, and ruled out `gateway/build-manifest.yaml`, which the test framework itself auto-syncs from the built image on every local run and is not a dependency bump anyone made). A clean run later passed both tests, consistent with Docker/host resource pressure from this session's unusually high rebuild volume rather than a real defect — flagging for awareness, not as a tracked bug.
- [ ] **T015c [NEW] [US1] (SC-002) Spec-vs-Envoy conflict, unresolved: an unmatched handshake path is specified as `404`, but Envoy's own upgrade-rejection returns `403` first.** spec.md's Edge Cases ("Channel parameters": "unmatched paths return `404`"; "Same path, different methods": "other methods return `404`/`405`") and quickstart.md Scenario 1 ("`404` after delete") both lock in `404` as the required status for a handshake with no matching channel route — whether because the path was never a channel, or because the API was deleted. T015b's investigation (live `/config_dump`/`/clusters` inspection) confirmed Envoy rejects *any* Upgrade request against a route that doesn't have that upgrade type enabled — including the `no-api-found` catch-all, which does carry its own `404` `direct_response` — with a generic `403` **before that route's own response code ever runs**. This was confirmed for the "unconfigured channel path" scenario in `websocket_deploy.feature` (asserts `403`, not the spec's `404`, and passes) but the delete-then-404 case is untested either way: none of the 4 scenarios in that feature re-attempt a handshake after `DeleteWebSocketAPI` to check its status, so whether delete also yields `403` instead of the spec's `404` has not been directly observed — only inferred from the identical catch-all-route mechanism. Needs a decision, not a silent pick: either (a) revise spec.md/quickstart.md's `404` to `403` for both cases, accepting Envoy's upgrade-rejection as the correct, permanent behavior, or (b) find a mechanism that makes an unmatched/deleted channel's Upgrade request actually surface `404` (e.g., a dedicated catch-all route that itself declares `upgrade_configs` enabled so its own `direct_response` can run, or an ext_proc/Lua-level rewrite of the generic `403` for this specific case). Blocks closing out T015/T015b's "not yet covered" list and the `405` half of the "same path, different methods" edge case, which is equally untested.
- [X] T016 [P] [US1] Add `tests/framework/suites/it/resources/templates/websocket-api.yaml` fixture matching the `WebSocketApi` shape from T010/data-model.md §1

### Implementation for User Story 1

- [X] T017 [US1] (FR-002, FR-003) Implement `gateway/gateway-controller/pkg/api/handlers/websocket_api_handler.go`: `CreateWebSocketAPI`/`ListWebSocketAPIs`/`GetWebSocketAPIById`/`UpdateWebSocketAPI`/`DeleteWebSocketAPI` per contracts/gateway-management-api.md, validating per FR-003 — at least one channel (required, min 1), unique channel paths, valid `ws`/`wss` upstream URLs via the shared upstream URL validator (depends on T010)
- [X] T017a [US1] (FR-002) In the same `websocket_api_handler.go` file (T017), implement the gateway-level API-key sub-resource: `CreateWebSocketAPIKey`/`ListWebSocketAPIKeys`/`UpdateWebSocketAPIKey`/`RegenerateWebSocketAPIKey`/`RevokeWebSocketAPIKey` for `/websocket-apis/{id}/api-keys[...]`, mirroring `CreateGraphQLAPIKey`/`ListGraphQLAPIKeys`/`UpdateGraphQLAPIKey`/`RegenerateGraphQLAPIKey`/`RevokeGraphQLAPIKey` in `graphql_api_handler.go` — required for gateway-first deployment and for the API-key policy tests in User Story 2 (depends on T017)
- [X] T018 [US1] (FR-010) Implement `gateway/gateway-controller/pkg/transform/websocket.go` (`NewWebSocketAPITransformer`/`Transform`): one Envoy route per channel and upstream slot, matching `GET` and the full context+version+channel path (with parameters), routed to an HTTP/1.1-only cluster — no upstream `h2` ALPN; register it in T011's registry entry
- [X] T019 [US1] (SC-002) Extend `gateway/gateway-controller/pkg/xds/translator.go`: upgrades disabled by default at the listener HCM (`upgrade_configs: [{upgrade_type: websocket, enabled: false}]`), enabled only on `WebSocketApi` routes (FR-011); add a regression test asserting `NormalizePath`/`MergeSlashes`/`PathWithEscapedSlashesAction` are still set on the shared listener per research.md §7 (no new code needed there, test only). Implemented exactly as specified, the generated xDS config's shape is unit-tested, **and the mechanism is now confirmed correct against a real Envoy** — the `403`s T015b originally reported were three test-infrastructure bugs (propagation-wait, a stale/misconfigured mock image, a mock hijack bug), not a translator defect; see T015b for the full root cause. No change needed here.
- [ ] T019a [US1] (FR-014) Add a regression test asserting the existing Lua filter's header-only path rewrite and host rewrite still fire correctly on a `WebSocketApi` upgrade request, with no body access attempted — confirms enabling per-route upgrades (T019) doesn't change this filter's existing behavior
- [X] T020 [US1] Extend the translator (same file as T019) to set `idle_timeout` from `connection.idleTimeout` and `max_stream_duration.max_stream_duration` from `connection.maxConnectionDuration` on `WebSocketApi` routes, with no retry policy attached (FR-012)
- [X] T021 [US1] Add gateway config fields for WebSocket idle-timeout and max-connection-duration defaults and upper bounds; reject deploy-time values exceeding the configured bound (FR-013)
- [X] T021a [US1] (FR-011) Add a gateway config flag controlling whether WebSockets over HTTP/2 (RFC 8441 extended `CONNECT`) are accepted, defaulting to off; scope and document whether the `h2`→HTTP/1.1 `CONNECT`-to-upgrade conversion itself ships in this feature or is explicitly deferred (flag existence is the FR-011 MUST; the conversion behavior is a separate, currently under-specified piece — raise for an explicit scope decision if deferring)
- [X] T022 [US1] Implement main/sandbox upstream support in T017/T018 with `ws`/`wss` URL validation via the shared upstream URL validator, plus upstream TLS/CA, upstream auth (OAuth2/mTLS), host rewrite, and connect timeout (FR-004)
- [X] T022a [US1] (FR-004) Extend T022's upstream validation to accept and default each slot's nested `timeout.connect`/`timeout.idleTimeout`/`timeout.maxConnectionDuration`/`timeout.maxConnections` (T010, T001a), independently for `main` and `sandbox` — parsing/defaulting only; translator wiring and runtime enforcement are T036a–T036c
- [X] T023 [US1] (FR-001) Create `gateway/gateway-controller/pkg/utils/websocket_deployment.go` (deployment/artifact-push helpers, mirroring `utils/graphql_deployment.go`)
- [X] T024 [US1] Done, but deliberately diverging from the task's literal wording. Rather than wiring the standalone `tests/mock-servers/mock-websocket-backend` module as its own container, ported its RFC 6455 handshake/echo logic into a new `tests/framework/testbench/services/websocket` service (port 3017 — moved from an initial 3014, which collided with `mcp-legacy`; stateless, behavior driven by handshake query params — `rejectStatus`/`subprotocol`/`pushInterval` — mirroring `backend.go`'s own `?statusCode=` convention) and wired it into the shared testbench binary — `tests/framework/CLAUDE.md`'s "do not create custom Docker containers to mock backends" rule forbids the literal instruction here, and `mock-graphql-backend` is the cautionary precedent for not doing so (left as unwired dead code in this same repo). Implemented the full WebSocket client step vocabulary (`suites/it/steps/platformgateway/websocket.go`: open/send-frame/receive-frame/ping/close, publishing the handshake response through the existing `httpx.Funnel` so the generic `the response status code should be X` assertion works unmodified) and wired `WebSocketApi` into the generic resource-kind vocabulary (`resourceKinds` map, cleanup registration, regex alternatives for create/update/get/delete). **Successful connect+echo is now proven end-to-end** (T015b) — the service also needed its `Endpoints` entry added to `core/catalog/testbench/definition.go` (missing initially) and a hijack-handling fix (`http.NewResponseController(w).Hijack()` instead of a raw type assertion, which fails against the testbench's own access-logging `ResponseWriter` wrapper).

**Checkpoint**: User Story 1 is fully functional and independently testable — deploy, connect,
echo, reject REST-route upgrades, delete → `404`.

---

## Phase 4: User Story 2 - Secure the handshake (Priority: P1)

**Goal**: Auth/authz policies enforced at the handshake, including browser-only credential
locations; handshake-only policy chains enforced at deploy time.

**Independent Test**: Attach an API-key policy; connect without a key (`401`), with a revoked key
(rejected), and with a valid key in a header, a query parameter, and `Sec-WebSocket-Protocol` (all
accepted); confirm the backend and access log never see the key (quickstart.md Scenario 2).

### Tests for User Story 2

- [ ] T025 [P] [US2] (SC-003) Write `tests/framework/suites/it/features/websocket_policies.feature` covering spec.md User Story 2 acceptance scenarios 1–6, including SC-009 (deploying a body-processing policy fails validation 100% of the time, naming the policy) and FR-023's full header-phase policy list (auth, rate limits, `subscription-validation`, header/path rewrite, `backend-jwt`, `interceptor-service` — see T026a)

### Implementation for User Story 2

- [ ] T026 [US2] **BLOCKED — architecture gap found during implementation.** `websocket_api_handler.go` runs in `gateway-controller`, which stores policy attachments only as `models.PolicyDefinition` (`gateway/gateway-controller/pkg/models/policy_definition.go`) — a schema with no `Mode`/body-requirement field at all. Whether a policy declares `RequiresRequestBody`/`RequiresResponseBody` is known only inside `gateway-runtime/policy-engine`, computed from each policy's `Mode()` at chain-build time, after deployment has already been accepted by the controller. Implementing this task as written requires first extending `PolicyDefinition` (and whatever populates it from a policy's `policy-definition.yaml`) to carry a body-mode declaration the controller can see at deploy time — a real, separate schema/pipeline change, not something to improvise inline. Not attempted here to avoid a half-correct validator; see T028 for the enforcement that exists today regardless.
- [X] T026a [US2] (FR-023) Audited `subscription-validation`, `set-headers`, `remove-headers`, `host-rewrite`, `request-rewrite`, `backend-jwt`, and `interceptor-service`'s actual `Mode()` in the `gateway-controllers` sibling repo. `subscription-validation`, `remove-headers`, `host-rewrite`, `request-rewrite`, and `backend-jwt` are unconditionally header-only (`BodyModeSkip` on both sides, hardcoded). `set-headers` and `interceptor-service` are **conditionally** header-only: each has a configuration option (`requestBodyPhase` / `hasRequest`) that flips `RequestBodyMode` to `BodyModeBuffer`. Without T026, nothing stops an operator from attaching either in body-mode on a WebSocketApi channel — but T028's fail-safe (below) means the practical effect is the feature silently never fires for that route (Envoy never sends the body callback), not a security hole. Worth fixing before T026 ships, since today it's a silent misconfiguration rather than a validated rejection.
- [X] T027 [US2] (FR-022) Implemented differently from the task text, correctly per this feature's own design: added `APIKindWebSocket` to the `APIKind` enum (`sdk/core/policy/v1alpha2/types.go`) and to `faultformat.knownAPIKinds`. **No resolver.go/matched-channel-field change was made or is needed** — `resolver.go`'s `PreparedResolver`/`ResolvedOperation` mechanism exists specifically for kinds that multiplex many operations onto one Envoy route at runtime (MCP, A2A); FR-010 deliberately gives WebSocketApi one route per channel, fixed at deploy time, so the "matched channel" the spec asks for is already carried the way REST/GraphQL carry their own per-route identity: `SharedContext.OperationPath`, which `pkg/transform/websocket.go` already sets to the channel's real path (not a placeholder) — confirmed existing before this pass, so an auth policy's `OperationPath == ""` fail-closed check (e.g. `api-key-auth`) is satisfied for free.
- [X] T028 [US2] (FR-021) Implemented in `gateway-runtime/policy-engine/internal/kernel/execution_context.go`'s `getModeOverride()`, **not** `fault_entry.go` as the task text says — `fault_entry.go` is fault/error attribution and has no body-mode logic at all; the real per-request wire decision is `getModeOverride()` (confirmed via `body_mode.go`'s own doc comment: "Change getModeOverride instead"). Added an unconditional override: when `SharedContext.APIKind == APIKindWebSocket`, both `RequestBodyMode` and `ResponseBodyMode` are forced to `NONE` regardless of what the chain declares. Covered by two new tests (`TestGetModeOverride_WebSocketForcesBodyModeNone`, `TestGetModeOverride_NonWebSocketUnaffected`); full `policy-engine` suite passes.
- [ ] T029 [US2] **Out of repo scope.** `api-key-auth`/`jwt-auth`/`basic-auth`/`opaque-token-auth` source lives in the external `github.com/wso2/gateway-controllers` module (confirmed: `api-key-auth` already supports header+query; `jwt-auth`/`basic-auth`/`opaque-token-auth` support header only; none support `Sec-WebSocket-Protocol` anywhere in that module). Extending four production auth policies plus their stripping-before-upstream/`101`-response behavior is real, separate engineering with its own versioning/release lifecycle (`gateway/build-manifest.yaml` pins exact tagged versions) — not attempted here. Needs a tracked issue against that repo with an owner and deadline.
- [X] T030 [P] [US2] (FR-033) Added `policies/origin-validation` to the `gateway-controllers` sibling repo (`/Users/nadunipamudika/Documents/WSO2/Platform/gateway-controllers`, local commit `9326349` on the `graphql` branch, **not pushed**): exact-match, case-insensitive `Origin` allowlist per `go-cors-validation.md` (rejects a configured `"*"` at config time; rejects a missing `Origin` by default, with an explicit `allowMissingOrigin` opt-out); sterile 403 on rejection, no echoed origin. 10 unit tests pass, including a lookalike-origin regression test. **Not yet wired into `api-platform-websocket`'s `gateway/build-manifest.yaml`**, since that requires a real published/tagged version of the sibling module — adding a manifest entry pointing at an unpublished version would break the real build.
- [X] T031 [US2] (FR-034) Verified, no new code needed: `pkg/transform/websocket.go`'s `buildPolicyChain` already resolves `channel.Policies` via the same shared `resolvePolicyInstances(..., policyv1alpha.LevelRoute)` helper REST/GraphQL use, so a channel-attached `jwt-auth` instance (scopes param block included) reaches the handshake's chain unmodified. `jwt-auth`'s scope enforcement (`scopes.allOf`/`anyOf`) is itself kind-agnostic.
- [ ] T032 [US2] (SC-003) **Not implemented.** Confirmed no query-string redaction mechanism exists anywhere in `policy-engine` today — only header-name-based masking (`config.MaskedHeaders`, default `{authorization, x-api-key, x-jwt-assertion}`) in `internal/analytics/publishers/log.go` and `global_properties.go`. Adding `Sec-WebSocket-Protocol` to that default list covers the header-carried case for free (once T029 lands), but a query-string-carried credential needs new, genuinely separate logic: a `masked_query_params`-style config list plus a URL-rewrite step before the path reaches log/analytics/tracing sinks. Scoped but not built in this pass.
- [X] T033 [US2] Confirmed, and the premise does not hold: `api-key-auth`, `jwt-auth`, `basic-auth`, and `opaque-token-auth` each build their own distinct 401 body today — `"Unauthorized"` (capitalized, not the rule's lowercase `"unauthorized"`), different default messages per policy, `basic-auth` additionally setting `WWW-Authenticate`, and all three of jwt/basic/opaque-token taking an operator-configurable `errorMessage`/`realm`. This is a **pre-existing gap across every kind** (REST, GraphQL, MCP, WebSub), not something WebSocket introduces — nothing downstream normalizes it except `faultformat`, which is explicitly Agent-only by design. Fixing it for WebSocket alone isn't possible without either forking those four external policies or introducing a shared helper every kind's auth policies would need to adopt — flagging for an explicit decision rather than quietly working around it.

**Checkpoint**: User Story 2 is independently testable — auth enforced at the handshake,
credentials never reach the backend, logs, or analytics.

---

## Phase 5: User Story 3 - Control connection lifetime and limits (Priority: P1)

**Goal**: Idle timeout, maximum connection duration, and a per-API concurrent-connection ceiling
all enforced on the downstream (client-facing) connection, with excess connections rejected
immediately and never queued; the same three bounds enforced independently on each upstream
(backend-facing) connection pool, per `main`/`sandbox` slot (FR-015–FR-017).

**Independent Test**: Set `idleTimeout: 10s`, `maxConnectionDuration: 30s`; an idle client is
closed at 10s, a client sending pings survives past 10s, every client is closed at 30s
(quickstart.md Scenario 3).

### Tests for User Story 3

- [ ] T034 [P] [US3] Extend `websocket_deploy.feature` (T015) with spec.md User Story 3 acceptance scenarios 1–5: idle close at bound, ping keeps connection alive, max-duration close regardless of traffic, gateway-wide default applies when unset, connection N+1 rejected within 100ms and never queued (SC-004, SC-005)
- [ ] T034a [P] [US3] (FR-090) Add upstream connection-pool coverage to `websocket_deploy.feature` (T015) per FR-015–FR-017: an upstream idle timeout below the backend's own send interval closes the pooled connection, `maxConnectionDuration` forces a backend reconnect regardless of traffic, and a `main` slot at its `maxConnections` ceiling still serves `sandbox` traffic independently (confirms per-slot, not per-API, scope)

### Implementation for User Story 3

- [ ] T035 [US3] (FR-012, FR-013) Verify T020's idle/max-duration enforcement meets SC-004 (close within 1s of the configured bound) under test load
- [ ] T035a [US3] (FR-040) Run a short validation spike to choose the exact extension technology for T036's per-API connection counter — Lua filter vs. Envoy's Go HTTP filter support vs. a small purpose-built WASM filter — per research.md §2's explicitly flagged open risk ("needs a short validation spike before `/speckit-tasks` commits to one"). Record the chosen technology and rationale in research.md §2 before starting T036; this gates T036, it doesn't run alongside it
- [ ] T036 [US3] Implement a router-level per-API connection counter using the extension technology T035a selected — **not** Envoy's native `connection_limit` filter (wrong scope — filter-chain-level, not per-route) and **not** cluster circuit breakers (already shown unreliable by the original spike) — enforcing `connection.maxConnections` with an immediate `503` on overflow, never queued (FR-040, SC-005)
- [ ] T036a [US3] (FR-015) Extend `createCluster`/`createWeightedCluster` in `gateway/gateway-controller/pkg/xds/translator.go` to set each upstream slot's connection-pool `idle_timeout`/`max_connection_duration` from `upstream.<main|sandbox>.timeout.idleTimeout`/`.maxConnectionDuration`, via `typed_extension_protocol_options` (`envoy.extensions.upstreams.http.v3.HttpProtocolOptions.common_http_protocol_options`) — note setting this field requires also explicitly configuring HTTP1/HTTP2 `upstream_protocol_options` on the same message, since the cluster stops inferring protocol implicitly once it's present (depends on T022a)
- [ ] T036b [US3] (FR-016) Extend the same two cluster builders (T036a) to set `circuit_breakers.thresholds[].max_connections` from `upstream.<main|sandbox>.timeout.maxConnections` — unlike T036's downstream counter, Envoy's native circuit breaker is the correct primitive here (per-gateway-instance scope matches the bounded resource: outbound connections from *this* instance to the backend)
- [ ] T036b-spike [US3] (FR-016) Before considering T036b verified, run a short spike against a multi-host upstream cluster (several DNS-resolved backend instances behind one `main`/`sandbox` slot) with `maxConnections` set tight: measure actual overshoot from Envoy's documented "at least one connection per selected host" guarantee, which spikes 11/11b/11c never exercised (single-host backend only — research.md §8). Record the measured overshoot in research.md and decide whether it's acceptable as-is or needs a lower effective default to compensate
- [ ] T036c [P] [US3] (FR-017) Add gateway config fields for upstream idle-timeout, max-connection-duration, and max-connections defaults and upper bounds (`router.webSocket.upstream.*`, data-model.md §3); reject deploy-time `upstream.<main|sandbox>.timeout.*` values exceeding the configured bound, mirroring T021's downstream validation
- [ ] T037 [US3] Extend `basic-ratelimit`/`advanced-ratelimit` to count `WebSocketApi` handshakes as connection-rate limiting (FR-041)
- [ ] T038 [P] [US3] Add a `global_downstream_max_connections` resource-monitor config option, gateway-wide (FR-042, SHOULD)
- [ ] T039 [US3] Add a regression test confirming `connection.idleTimeout`/`maxConnectionDuration` never resolve to "unlimited" even when both are omitted from the artifact (FR-013, FR-031)
- [ ] T039a [US3] Add a regression test confirming `upstream.<main|sandbox>.timeout.maxConnectionDuration` never resolves to "unlimited" even when omitted from either upstream slot — same rule as T039, applied independently per slot (FR-015, FR-017)

**Checkpoint**: User Story 3 is independently testable — one API cannot exhaust the gateway, and
one upstream backend cannot exhaust the gateway's outbound connections either.

---

## Phase 6: User Story 4 - Manage WebSocket APIs from the control plane and CLI (Priority: P2)

**Goal**: Create/deploy/undeploy/restore a `WebSocketApi` from platform-api (AsyncAPI import) and
the `ap` CLI, with API keys, mirroring `GraphQLApi`.

**Independent Test**: Create a WebSocket API in platform-api from an AsyncAPI file, deploy it to a
registered gateway, connect through that gateway, undeploy, confirm new connections fail
(quickstart.md Scenario 4).

### Tests for User Story 4

- [ ] T040 [P] [US4] Write `tests/framework/suites/it/features/websocket_api_keys.feature` covering spec.md User Story 4 acceptance scenarios 1–6 (AsyncAPI 2.x/3.x import, non-WebSocket-protocol document rejected, version-gated deploy refusal, undeploy behavior, bulk-sync inclusion, CLI parity)
- [ ] T041 [P] [US4] Write `cli/src/cmd/gateway/websocketapi/commands_test.go` and `cli/src/cmd/gateway/websocketapi/apikey/commands_test.go`

### Implementation for User Story 4 — platform-api

- [ ] T042 [P] [US4] Create `platform-api/internal/repository/websocket_api.go`: `Create`/`GetByHandle`/`GetByUUID`/`List`/`Count`/`CountByProject`/`Update`/`Delete`/`Exists`, `GetAPIGatewaysWithDetails`, `CreateAPIAssociation`, `GetAPIAssociations`, `UpdateAPIAssociation` (mirrors `repository/graphql_api.go`)
- [ ] T043 [US4] Create `platform-api/internal/service/websocket_asyncapi.go`: parse AsyncAPI 2.x and 3.x (JSON or YAML) from file, URL, or inline content — URL fetches go through the shared SSRF-safe client, never a direct fetch (FR-052, per `ssrf-prevention.md`); derive channels, channel parameters, `servers` filtered to `ws`/`wss` protocol, and `info` (name/version/description); reject a document with no `ws`/`wss` server and no explicit upstream override; enforce config-sourced size and parse-time limits (depends on T003's parser decision)
- [ ] T044 [US4] Create `platform-api/internal/service/websocket_api.go`: `Create`/`Get`/`GetDetail`/`List`/`Update`/`Delete`/`Count`/`AddGatewaysToAPI`/`GetAPIGateways` (mirrors `service/graphql_api.go`)
- [ ] T045 [P] [US4] Create `platform-api/internal/service/websocket_mapping.go` (model/DTO mapping helpers)
- [ ] T046 [US4] Create `platform-api/internal/service/websocket_deployment.go` (deploy/undeploy/restore/builds logic, mirrors `service/graphql_deployment.go`)
- [ ] T047 [P] [US4] Create `platform-api/internal/service/artifact_import_websocket.go` (artifact import path)
- [ ] T048 [US4] (FR-051) Create `platform-api/internal/handler/websocket_api.go`: CRUD + definition get/update + validate-definition + gateways endpoints per contracts/platform-api.md — **use a distinct handler type name**; the existing `platform-api/internal/handler/websocket.go` is the unrelated control-plane↔gateway live channel (deployment acks), per plan.md's naming-collision note
- [ ] T049 [P] [US4] (FR-051) Create `platform-api/internal/handler/websocket_api_apikey.go` (`/websocket-apis/{id}/api-keys[...]`)
- [ ] T050 [P] [US4] (FR-051) Create `platform-api/internal/handler/websocket_api_deployment.go` (`/websocket-apis/{id}/deployments[...]`, undeploy/restore, builds)
- [ ] T051 [US4] Create `platform-api/internal/gatewaytranslator/kinds/websocket.go` (kind-specific CP→gateway translation; depends on T004's `MinWebSocketKindGatewayVersion`)
- [ ] T052 [US4] Extend the **existing** control-plane↔gateway live channel (`platform-api/internal/websocket/` — not the handler created in T048) so `WebSocketApi` and its API keys are included in connected-gateway bulk sync, live deployment events, gateway-first artifact push (read-only in CP), and API key sync (FR-054)
- [ ] T053 [US4] Add the version-gate check in `websocket_api_handler.go` (T017): refuse deployment to a gateway below `MinWebSocketKindGatewayVersion` before translation is attempted (FR-053)

### Implementation for User Story 4 — CLI

- [X] T054 [P] [US4] (FR-060) Create `cli/src/cmd/gateway/websocketapi/{root.go,list.go,get.go,delete.go}` mirroring the `graphqlapi` package
- [X] T055 [P] [US4] (FR-060) Create `cli/src/cmd/gateway/websocketapi/apikey/{root.go,create.go,list.go,regenerate.go,update.go,revoke.go}`
- [X] T056 [US4] (FR-060) Register `websocketapi.Root()` in `cli/src/cmd/gateway/root.go` (depends on T054)
- [X] T057 [P] [US4] (FR-060) Add a "## WebSocket API Commands" section to `docs/cli/gateway/README.md` — net-new documentation; no GraphQL section exists there to mirror (confirmed during planning)
- [X] T057b [US4] (FR-060) Register the new kind for `ap gateway apply`: add `GatewayWebSocketAPIsPath = "/websocket-apis"` / `GatewayWebSocketAPIByIDPath = "/websocket-apis/%s"` to `cli/src/utils/constants.go`, and a `WebSocketAPIHandler` (`GetEndpoint`/`CreateEndpoint`/`UpdateEndpoint`) plus a `ResourceKindWebSocketAPI = "WebSocketApi"` case in `GetResourceHandler` in `cli/src/internal/gateway/resources.go`, mirroring `GraphQLAPIHandler`/`ResourceKindGraphQLAPI` exactly, and add the matching case to the existing `cli/src/internal/gateway/resources_test.go`. **Without this, `ap gateway apply -f <file>.yaml` cannot create or update a `WebSocketApi` at all** — `ap gateway websocket-api` (T054) only covers list/get/delete, matching the `graphqlapi` CLI package's own shape (no `create`/`apply` subcommand exists there either); `apply` is the only CLI path that creates/updates any gateway-first resource, for every kind, and is not file-listed in plan.md's Project Structure — add it there too
- [ ] T057a [US4] (FR-054) Verify the bottom-up sync path, once T042-T057b are all implemented: create a `WebSocketApi` **gateway-first** — (a) directly via the gateway management REST API (T017), and (b) separately via `ap gateway apply -f websocket-api.yaml` (T057b) — and confirm each one is pushed up and becomes visible, read-only, in platform-api's `/websocket-apis` listing once the gateway connects or reconnects to the control plane (the gateway-first artifact-push direction of T052's sync extension). This is distinct from, and in addition to, every other test in this phase, which exercises the top-down control-plane-creates-and-deploys-to-gateway direction only — neither direction's test implies the other works

**Checkpoint**: User Story 4 is independently testable — control-plane CRUD, deployment, and CLI
parity with `graphql-api`; gateway-first creation (REST or CLI) is confirmed to sync up to the
control plane, not just control-plane-initiated deploys down to the gateway.

---

## Phase 7: User Story 5 - Console and API Portal experience (Priority: P2)

**Goal**: Create/configure/deploy/test/monitor a WebSocket API from the console; discover,
document, and try it out from the API Portal.

**Independent Test**: Use the console wizard to create a WebSocket API from an AsyncAPI URL,
deploy it, open the WebSocket test console, connect to a channel, send a message and see the
reply, then publish it to the API Portal (quickstart.md Scenario 5).

### Tests for User Story 5

- [ ] T058 [P] [US5] Implement the existing skipped Cypress stub `portals/api-portal/it/ui/cypress/e2e/websocket-apis/tryout.cy.js` for real (connect + send/receive), per research.md §4 — note GraphQL's sibling stub is also unimplemented, so there is no working tryout to copy
- [ ] T059 [P] [US5] Add Vitest + React Testing Library coverage for the new wizard step and `uiConfig.tsx` registration in `portals/api-control-plane`

### Implementation for User Story 5

- [ ] T060 [US5] Register the `'websocket'` kind in `portals/api-control-plane/src/pages/appShell/appShellPages/apis/create/uiConfig.tsx`
- [ ] T061 [US5] Extend `ApiCreationSteps.tsx`/`ContractSourceForm.tsx` for AsyncAPI file/URL/paste or endpoint-based creation, with a channel preview before creating
- [ ] T062 [P] [US5] Add `'websocket'` to the `ApiTypeFilter`/`API_KINDS` map in `portals/api-control-plane/src/pages/appShell/appShellPages/apis/listing/apiTypeFilter.ts`
- [ ] T063 [P] [US5] Create `portals/api-control-plane/src/pages/appShell/appShellPages/apis/utils/websocketApiDisplay.ts`, mirroring `restApiDisplay.ts`
- [ ] T064 [US5] Register WebSocket routes in `portals/api-control-plane/src/routes/{AppRoutes.tsx,paths.ts}`
- [ ] T065 [US5] Restrict the develop-page policy picker (API and channel level) to handshake-only policies, surfacing the FR-020 constraint in the UI
- [ ] T066 [US5] Build the WebSocket test console (environment/channel selection, channel parameter inputs, connect/disconnect, send box, timestamped message log) that connects **directly from the browser to the gateway** — not through `tryoutProxyController.js`'s existing HTTP-only proxy path, per research.md §4 — sending the generated test key/token as a query parameter or `Sec-WebSocket-Protocol` entry (FR-071)
- [ ] T067 [US5] Extend `portals/api-portal/libs/async-api-viewer/src/components/WebsocketViewer/WebsocketViewer.tsx` to show live gateway URLs once a `WebSocketApi` is deployed (FR-055, research.md §5 — extend existing working component, don't rebuild)
- [ ] T068 [P] [US5] Wire the Overview, Deploy, Insights, Compliance, and Observability pages for WebSocket (channels shown in place of REST resources), including navigation, routes, and scope gates (FR-070)

**Checkpoint**: User Story 5 is independently testable end-to-end via console and API Portal.

---

## Phase 8: User Story 6 - Connection analytics and observability (Priority: P3)

**Goal**: One analytics event per connection at close, with correct close-reason classification.

**Independent Test**: Open and close several connections — rejected, throttled, idle-timed-out,
and normally closed — each appears once in analytics with the correct outcome (quickstart.md
Scenario 6).

### Tests for User Story 6

- [ ] T069 [P] [US6] Add analytics-event assertions to a new `.feature` scenario (or extend `websocket_deploy.feature`) covering spec.md User Story 6 acceptance scenarios 1–4, including SC-008 (exactly one event per connection, normal closes never classified as errors)

### Implementation for User Story 6

- [ ] T070 [US6] Emit one analytics event per connection at close — API, channel, application/subscription, status `101`, connection duration, bytes in/out, close reason — in `gateway/gateway-runtime/policy-engine/internal/analytics/fault_classification.go` (FR-080)
- [ ] T071 [US6] Classify a normal client close as success (not an error) and keep WebSocket connection duration out of request-latency metrics (FR-081, SC-008)
- [ ] T072 [P] [US6] Emit a normal request analytics event with the rejecting status for handshakes rejected by policy or backend (FR-082)
- [ ] T073 [P] [US6] Map WebSocket connection events to a sensible shape for the Moesif publisher, consistent with how GraphQL operation properties are mapped today (FR-083)

**Checkpoint**: All six user stories are now independently functional.

---

## Final Phase: Polish & Cross-Cutting Concerns

- [ ] T074 [P] Finalize the AsyncAPI parser dependency decision from T003 (`govulncheck ./...`, `go-licenses check ./...`, transitive `go mod graph` diff) before merging, per `dependency-management.md`
- [ ] T075 [P] Add a sample AsyncAPI WebSocket definition under `portals/api-portal/samples/`, mirroring the existing `countries-api-v1.0` GraphQL sample
- [ ] T076 Run all six quickstart.md validation scenarios end-to-end against a built gateway + platform-api + console + API Portal
- [ ] T076a (SC-006, SC-007) Build and run a load/performance test against the reference perf setup: measure handshake latency added by the gateway with a one-auth-policy chain (target <10ms p95, SC-006), frame-relay overhead (target <1ms p95, SC-006), sustained concurrent idle connections on one gateway instance while confirming the policy engine holds no open processing stream per idle connection (target 10,000, SC-007), and — per spec.md's Edge Cases ("Gateway restart, rollout, or listener change") — restart the gateway under a representative load of open connections and confirm it survives the resulting mass-reconnect burst from clients backing off and retrying
- [ ] T077 [P] Security review pass against `authentication_authorization.md`, `go-cors-validation.md`, `ssrf-prevention.md`, and `go-network-service-hardening.md` checklists for every new handler/translator/policy path touched in this feature
- [ ] T078 Check `kb/**/*.md` for any feature doc whose `sources:` list references a file changed in this feature (per `feature-docs.md`); update it in the same change set, or confirm none exists
- [ ] T079 [P] Final pass on `docs/cli/gateway/README.md` and add a CHANGELOG entry

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately.
- **Foundational (Phase 2)**: Depends on Setup (needs T001/T001a's regenerated types, T003's parser
  decision informs T012/T013's DTO shape) — **BLOCKS all user stories**.
- **User Stories (Phase 3–8)**: All depend on Foundational completion.
  - US1, US2, US3 (all P1) are the MVP core and are easiest to build in that order, since US2 and
    US3 both extend the route/handler US1 creates — but each has its own independent test and can
    be assigned to different engineers once US1's handler/transformer skeleton (T017, T018) exists.
  - US4 (P2) depends on US1's gateway-side artifact existing, but its platform-api/CLI work
    (T042–T057) is otherwise independent and can start as soon as Phase 2 completes.
  - US5 (P2) depends on US4's platform-api endpoints existing (the console/portal call them) —
    sequence US5 after US4, or stub the API client against contracts/platform-api.md to start
    US5's UI work in parallel.
  - US6 (P3) depends on US1's route/connection lifecycle existing (nothing to classify at close
    otherwise) but not on US2/US3/US4/US5.
- **Polish (Final Phase)**: Depends on all desired user stories being complete.

### Within Each User Story

- Tests are written first per story and MUST fail before implementation.
- Gateway artifact/handler before transformer/translator wiring.
- Translator/policy-engine changes before CLI/console/portal work that calls them.
- Story complete (checkpoint reached) before moving to the next priority, if working sequentially.

### Parallel Opportunities

- T003, T004, T005 (Setup) run in parallel — different files, no interdependency.
- T007, T008, T009 (Foundational schema dialects) run in parallel once T006 fixes the column shape.
- T012, T013 (Foundational model/DTO) run in parallel.
- Within US1: T015, T016 (tests) in parallel; T022/T023 can run alongside T019/T020 once T017/T018 land; T022a depends on T022, T010, and T001a.
- Within US4: T042, T045, T047 (platform-api) in parallel; T049, T050 (handlers) in parallel once
  T048 lands; T054, T055, T057b (CLI) in parallel — different files, no interdependency. T057a runs
  last — it depends on T052 (sync extension), T053 (version gate), and T057b (`apply` support,
  needed to create gateway-first via CLI at all), not just one of them.
- Within US5: T062, T063, T068 in parallel.
- Across stories: once Phase 2 is done, US1/US2/US3 (all P1) can be staffed to different engineers
  in parallel, as can US4 and US6 once their respective dependencies (above) are met.

---

## Parallel Example: User Story 1

```bash
# Launch both US1 tests together:
Task: "Write tests/framework/suites/it/features/websocket_deploy.feature"
Task: "Add tests/framework/suites/it/resources/templates/websocket-api.yaml fixture"

# Launch independent US1 implementation tasks together, once T017/T018 exist:
Task: "Add gateway config fields for WebSocket idle-timeout/max-duration bounds (T021)"
Task: "Implement main/sandbox upstream support with TLS/auth/rewrite (T022)"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup.
2. Complete Phase 2: Foundational (critical — blocks every story).
3. Complete Phase 3: User Story 1.
4. **Stop and validate**: run quickstart.md Scenario 1 independently.
5. Demo gateway-first deploy + connect + echo before building anything else.

### Incremental Delivery

1. Setup + Foundational → foundation ready.
2. US1 → validate independently → MVP.
3. US2 + US3 (both P1, both extend US1's route) → validate independently → full P1 scope secured
   and bounded.
4. US4 → validate independently → control-plane/CLI parity.
5. US5 → validate independently → console/portal parity.
6. US6 → validate independently → observability complete.
7. Polish.

### Parallel Team Strategy

1. Team completes Setup + Foundational together.
2. Once Foundational is done: Engineer A takes US1→US2→US3 (P1 chain, same route/handler
   surface); Engineer B takes US4 (platform-api + CLI); Engineer C takes US5 once US4's contract is
   stable (or starts against contracts/platform-api.md directly); Engineer D takes US6 once US1
   lands.
3. Stories integrate at their checkpoints; the P1 chain (US1→US2→US3) is the gating path for an
   MVP demo.

---

## Notes

- [P] tasks touch different files with no completed-task dependency.
- [Story] labels map every Phase 3+ task to its spec.md user story for traceability.
- Every task names an exact file path per plan.md's Project Structure — no task should require
  guessing where code belongs.
- Data-model.md field constraints (required/nullable, defaults, uniqueness) are quoted verbatim in
  the tasks that create those fields (T006–T009, T010, T010a, T012) rather than left to implementation-time
  discretion.
- Two naming collisions are called out explicitly where they matter (T048, T052) — do not merge
  WebSocket-artifact work into the pre-existing, unrelated `handler/websocket.go` control-plane↔
  gateway channel.
- Stop at any checkpoint to validate a story independently before continuing.
