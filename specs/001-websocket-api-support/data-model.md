# Phase 1 Data Model: WebSocket API Support

Entities below expand [spec.md](./spec.md)'s Key Entities section into concrete fields,
validation rules, and relationships, following the `GraphQLApi`/`graphql_apis` precedent found in
[plan.md](./plan.md)'s Project Structure. §4's column shape has been validated against the
`designing-db-schemas` skill's R1–R10 rules directly against `graphql_apis`'s actual DDL (all four
dialects) — this document fixes the shape precisely; writing the literal DDL into the four schema
files is still a `/speckit-tasks` implementation task (T006–T009).

## 1. `WebSocketApi` (gateway artifact)

The gateway-side artifact, defined via the gateway management API (`gateway-controller/api/
management-openapi.yaml` → `WebSocketAPIConfigData`), mirroring `GraphQLAPIConfigData`'s shape.

| Field | Type | Required | Notes |
|---|---|---|---|
| `displayName` | string | yes | Human-readable name. |
| `version` | string | yes | API version segment (path component). |
| `context` | string | yes | Base path segment, unique per gateway + version (FR-003). |
| `upstream.main` | object `{url, timeout?}` | yes | `ws://`/`wss://` only; validated by the shared upstream URL validator (FR-004). |
| `upstream.sandbox` | object `{url, timeout?}` | no | Same validation as `main` when present. |
| `upstream.main.timeout.connect` | duration | no (defaulted) | Pre-existing field, reused as-is (FR-004) — upstream connect timeout. |
| `upstream.main.timeout.idleTimeout` | duration | no (defaulted) | New field (FR-015) — upstream connection-pool idle timeout; falls back to gateway config default. |
| `upstream.main.timeout.maxConnectionDuration` | duration | **yes, effectively** | New field (FR-015) — upstream connection lifetime cap; gateway-config default if unset, never "unlimited" — mirrors the downstream rule. |
| `upstream.main.timeout.maxConnections` | integer | no (defaulted) | New field (FR-016) — concurrent connections to this backend, enforced per gateway instance via the cluster's native circuit breaker — unlike `connection.maxConnections` below, per-instance scope is the intended behaviour here, not a gap. |
| `upstream.sandbox.timeout.*` | same four fields | no | Independent of `upstream.main.timeout` — each upstream slot defaults and bounds separately. |
| `connection.idleTimeout` | duration | no (defaulted) | Falls back to gateway config default (FR-013); rejected at deploy time if it exceeds the configured upper bound. |
| `connection.maxConnectionDuration` | duration | **yes, effectively** | Always resolves to a finite value — gateway-config default if unset, never "unlimited" (FR-013, FR-031). |
| `connection.maxConnections` | integer | no (defaulted) | Per-API concurrent connection ceiling (FR-040); gateway-instance-scoped per the resolved Clarification. |
| `channels` | array of `Channel` | yes, min 1 | See §2. Channel paths unique within the API (FR-003). |
| `policies` | array of `Policy` | no | API-level, handshake-only — every entry validated against FR-020 at deploy time. |
| `metadata.deploymentState` | enum | system-managed | Mirrors `GraphQLAPIConfigData.DeploymentState`. |
| `status` | `ResourceStatus` | system-managed | Mirrors the existing `ResourceStatus` shape used by every kind. |

`timeout` is **not** a new schema — it reuses and extends `UpstreamTimeout`, the existing
component today only reachable via `RestApi`'s `UpstreamDefinition.timeout` (where it carries just
`connect`). This is its first direct use from an inline `Upstream` object (`main`/`sandbox`), and
the first use of its new `idleTimeout`/`maxConnectionDuration`/`maxConnections` fields by any kind.
The schema's name stays `UpstreamTimeout` to avoid an unrelated rename of a component already
referenced by shipped `RestApi`/generated Go code (`UpstreamTimeout.Connect` in
`gateway-controller/pkg/xds/translator.go`), even though it now carries more than pure timeouts.

**Validation rules** (enforced inline in `websocket_api_handler.go`, per research.md §3 — no
generic cross-kind validator exists to hook into):
- At least one channel; channel paths unique.
- Every upstream URL is a valid `ws`/`wss` URL per the shared validator.
- Every policy in `policies` and in every `channels[].policies` MUST declare no request/response
  body requirement (FR-020) — reject the whole deployment, naming the offending policy (SC-009).
- `connection.maxConnectionDuration`, once defaulted, MUST be `> 0` and finite.
- `connection.idleTimeout`/`maxConnectionDuration`, if explicitly set, MUST NOT exceed the
  gateway-config upper bound (FR-013).
- `upstream.<main|sandbox>.timeout.maxConnectionDuration`, once defaulted, MUST be `> 0` and
  finite — same rule as the downstream field, applied independently per upstream slot (FR-015).
- `upstream.<main|sandbox>.timeout.idleTimeout`/`maxConnectionDuration`, if explicitly set,
  MUST NOT exceed the gateway-config upper bound (FR-017).

## 2. `Channel`

Modeled on `RestApi`'s existing `Channel` type (confirmed in the repo, per the Q1 clarification),
not a new `operations[]`/`Operation` shape.

| Field | Type | Required | Notes |
|---|---|---|---|
| `path` | string | yes | AsyncAPI channel path, may include `{param}` path parameters; matches one path segment per parameter (Edge Cases). |
| `policies` | array of `Policy` | no | Channel-level, handshake-only — same FR-020 validation as API-level policies. |
| `messageSchemas` | opaque/documentation-only | no | Carried through from AsyncAPI for display; never enforced by the gateway (Assumptions). |

## 3. Connection settings

Not a separate stored entity — the `connection.*` fields on `WebSocketApi` (§1), downstream and
upstream alike. Documented separately here only because the spec calls them out as a distinct
concept with its own bounds (gateway-wide defaults/upper limits), which live in gateway config,
not the artifact.

Keys as implemented in `config.WebSocketConfig` (`gateway-controller/pkg/config/config.go`), under
`router.websocket`:

| Gateway config key | Applies to |
|---|---|
| `router.websocket.downstream.idle_timeout_default` / `.._max` | `connection.idleTimeout` default/ceiling (FR-013) |
| `router.websocket.downstream.max_connection_duration_default` / `.._max` | `connection.maxConnectionDuration` default/ceiling (FR-013) |
| `router.websocket.downstream.max_connections_default` / `.._max` | `connection.maxConnections` default/ceiling (FR-040) |
| `router.websocket.upstream.idle_timeout_default` / `.._max` | `upstream.<main\|sandbox>.timeout.idleTimeout` default/ceiling (FR-017) |
| `router.websocket.upstream.max_connection_duration_default` / `.._max` | `upstream.<main\|sandbox>.timeout.maxConnectionDuration` default/ceiling (FR-017) |
| `router.websocket.upstream.max_connections_default` / `.._max` | `upstream.<main\|sandbox>.timeout.maxConnections` default/ceiling (FR-017) |
| `router.websocket.allow_http2_connect` | RFC 8441 extended CONNECT, off by default (FR-011) |
| `router.websocket.global_downstream_max_connections` | Gateway-wide downstream ceiling (FR-042, SHOULD) |

A `*_default` of 0 means "no gateway default configured". For the two duration bounds the
transformer then falls back to a built-in finite value rather than leaving the connection
unbounded, so FR-031's guarantee holds even against a config that omits these keys.

## 4. `WebSocketApi` (control plane — platform-api)

New table `websocket_apis`, additive only per `db-schema-changes.md` R0-FROZEN (this is a new
table, not a change to a shipped one). Validated against the `designing-db-schemas` skill's R1–R10
rules directly on the existing `graphql_apis` table's actual DDL (all four dialects) — column
shape below is now exact, not approximate.

| Column | Type (Postgres / SQLite / SQL Server) | Notes |
|---|---|---|
| `uuid` | `VARCHAR(40) PRIMARY KEY` | **Also a FK** to `artifacts(uuid) ON DELETE CASCADE` — this is the shared artifact UUID (1:1 subtype of `artifacts`), not a second independently-generated UUID. Matches `graphql_apis` exactly (R1-UUID). |
| `organization_uuid` | `VARCHAR(40) NOT NULL` | FK → `organizations(uuid)`. **`ON DELETE CASCADE`** on Postgres/SQLite; **`ON DELETE NO ACTION`** on SQL Server, because SQL Server rejects two cascade paths converging on `organizations` (via `uuid→artifacts→organizations` and directly) — the same divergence `graphql_apis.sqlserver.sql` already carries. **No new application cleanup code is needed for this**: verified against `OrganizationRepo.DeleteOrganization`'s actual implementation — the row is already guaranteed deleted via the other cascade path (`uuid→artifacts(uuid) ON DELETE CASCADE`, then `artifacts.organization_uuid→organizations ON DELETE CASCADE`) by the time this FK would otherwise fire; `NO ACTION` here is purely a SQL Server DDL-ordering technicality, never actually relied upon for cleanup. Tenant scope — every query MUST filter on this from verified JWT claims, never request input (GO-AUTH-005, R2-ORG-FK). |
| `project_uuid` | `VARCHAR(40) NOT NULL` | FK → `projects(uuid) ON DELETE CASCADE`. Non-leading FK — needs its own index (R6-FK-INDEX). |
| `handle` | `VARCHAR(40) NOT NULL` | URL-safe slug, immutable after creation (R1-HANDLE-NAME, R7-HANDLE-IMMUTABLE). |
| `display_name` | `VARCHAR(255) NOT NULL` | Display string — no uniqueness constraint (R1-HANDLE-NAME). |
| `version` | `VARCHAR(30) NOT NULL DEFAULT 'v1.0'` | |
| `description` | `VARCHAR(1023)`, nullable | |
| `configuration` | `BYTEA NOT NULL` (Postgres) / `BLOB NOT NULL` (SQLite) / `VARBINARY(MAX) NOT NULL` (SQL Server) | Stores the derived channels, upstream, policies, connection settings as JSON — same large-payload pattern as `graphql_apis.configuration` (R3-LARGE-PAYLOAD). Opaque storage only — no JSON-operator queries against it, so **not** `JSONB` (R3-JSONB needs concrete query evidence, which doesn't exist here, matching `graphql_apis`). |
| `data_version` | `VARCHAR(20) NOT NULL DEFAULT '1.0'` | Optimistic-concurrency / sync versioning (R5-DATA-VERSION). **Not an integer** — this is a string version tag, matching `graphql_apis` exactly. |
| `origin` | `VARCHAR(20) NOT NULL DEFAULT 'control_plane'` | One of `constants.OriginCP` (`"control_plane"`) or `constants.OriginDP` (`"gateway_api"`) — control-plane-created vs. gateway-first-pushed (read-only in CP per FR-054). Enum validated in the service layer, never a DB `CHECK` (R4-NO-ENUM-CHECK). |
| `created_by`, `updated_by` | `VARCHAR(200)`, nullable | User email/subject from verified JWT claims (GO-AUTH-005), never request input. |
| `created_at`, `updated_at` | `TIMESTAMPTZ` (Postgres) / `DATETIME` (SQLite) / `DATETIME2(7) DEFAULT SYSUTCDATETIME()` (SQL Server) | `created_at`/`created_by` never appear in an `UPDATE` (R7-CREATED-IMMUTABLE). |

**Constraint**: `UNIQUE(organization_uuid, handle)` — org-scoped, never a bare `UNIQUE(handle)`
(R2-ORG-UNIQUE).

**Indexes**: `idx_websocket_apis_project` on `project_uuid` (R6-FK-INDEX, non-leading FK). **No**
separate index on `organization_uuid` alone — `UNIQUE(organization_uuid, handle)` already covers
org-scoped lookups via its leftmost column (R6-ORG-INDEX, R6-NO-REDUNDANT-INDEX). Note:
`graphql_apis` itself carries a redundant `idx_graphql_apis_org` alongside the same UNIQUE
constraint — accepted legacy on that shipped table (R0-LEGACY-ACCEPTED, recorded separately in
`db-schema-changes.md` Appendix A), but `websocket_apis` is new and shouldn't copy it.

**Relationships**: `websocket_apis` → `artifacts` (polymorphic artifact registration, as every
other kind does, via the shared `uuid`), → `organizations`, → `projects`; API keys and deployments
reuse the existing generic `api_keys`/`deployments`-style join tables keyed by artifact UUID, not
new WebSocket-specific join tables (consistent with `GraphQLApi`'s approach — confirmed no
kind-specific key/deployment tables exist for GraphQL beyond the generic ones).

**AsyncAPI-derived fields** (not separately columned — stored inside `configuration`): parsed
`info.title`/`version`/`description`, `servers[]` filtered to `ws`/`wss` protocol (the resolved
upstream), `channels[]` → the gateway `Channel` shape in §2, channel `parameters[]`.

## 5. `WebSocketApi` (gateway controller — local store)

**Separate from §4.** The gateway controller keeps its own local database, entirely independent
of platform-api's — it persists whatever artifacts have actually been deployed to that one gateway
instance. This is a second, parallel new table, not the same table as §4.

That store already follows a much simpler shape: one shared `artifacts` table (keyed by
`(gateway_id, uuid)`) carries every kind's common identity/lifecycle fields, and each kind gets a
minimal child table holding only its own JSON configuration. The existing `graphql_apis` child
table there is a one-column clone of `rest_apis`:

```sql
CREATE TABLE IF NOT EXISTS graphql_apis (
    uuid          TEXT NOT NULL,
    gateway_id    TEXT NOT NULL,
    configuration TEXT NOT NULL,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES artifacts(gateway_id, uuid) ON DELETE CASCADE
);
```

`WebSocketApi`'s new table there is identical in shape — `uuid`, `gateway_id`, `configuration`
(the whole derived channel/upstream/policy/connection-setting payload, as JSON text), composite
`PRIMARY KEY (gateway_id, uuid)`, `FOREIGN KEY(gateway_id, uuid) REFERENCES artifacts(gateway_id,
uuid) ON DELETE CASCADE`. Column types there are that schema's own existing convention (`TEXT` in
SQLite/PostgreSQL, `NVARCHAR(64)`/`NVARCHAR(MAX)` in SQL Server) — not the platform-api types in
§4 — since gateway-controller owns its own type choices (R0-SCOPE exempts it from R3).

No change to the shared `artifacts` table itself: `kind` has no DB-level `CHECK` constraint, so
registering a new valid kind value is a Go-layer change (the kind→table-name mapping and the
built-in resource-table list both need the new kind added), not a schema change.

This table's SQLite schema lives in `pkg/storage/gateway-controller-db.sql`;
`resources/gateway-controller-db.sql` is a **symlink** to it, not a second copy, so one edit covers
both paths (verified during implementation — an earlier draft of this document described them as
two files needing separate edits). The PostgreSQL and SQL Server dialect files under
`pkg/storage/` are separate real files and each needs its own change.

## 6. Connection event (analytics)

Not a database table — an analytics/event-stream record, one per connection, emitted at close
(FR-080). Shape:

| Field | Notes |
|---|---|
| `apiId`, `channelPath` | Identify the `WebSocketApi` and matched channel. |
| `applicationId`, `subscriptionId` | Same identity fields other API-kind analytics events carry. |
| `handshakeStatus` | `101` on success; the rejecting status otherwise (FR-082). |
| `connectionDurationMs` | Kept out of request-latency aggregates (FR-081, SC-008). |
| `bytesIn`, `bytesOut` | |
| `closeReason` | One of: client close, idle timeout, max duration, backend close/failure, policy/auth rejection — derived from the data-plane's close signal (FR-080). |

## State transitions

`WebSocketApi` (control plane) follows the same deployment state machine as `GraphQLApi`: `draft →
deployed → undeployed` (restorable) `→ deleted`, with `deployed` fanning out to per-gateway
deployment records. No WebSocket-specific states are introduced — connection-level state (open,
idle-timed-out, closed) is transient router/data-plane state, never persisted to platform-api.
