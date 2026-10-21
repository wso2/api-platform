# **Proposed DB Schema**

# 1. Summary of Changes

| Component | Object | Change | Shipped table? | Upgrade path |
| :---- | :---- | :---- | :---- | :---- |
| Control plane | `websocket_apis` | New table | No | Guarded CREATE (no ALTER needed) |
| Control plane | `idx_websocket_apis_project` | New index | No | Guarded CREATE (no ALTER needed) |
| Gateway controller | `websocket_apis` | New table | No | Guarded CREATE (no ALTER needed) |

No shipped table is modified by this feature. Every object above is brand-new and additive. The control plane and the gateway controller each keep their **own, separate** database — this feature adds a `websocket_apis` table to both, independently.

# 2. Control Plane

This table holds one row per WebSocket API artifact: its identity, its organization/project scope, and its full derived configuration (channels, upstream, policies, connection settings) as an opaque JSON payload — the same storage shape already used for the equivalent GraphQL artifact, so the control plane's artifact-registration, bulk-sync, and deployment machinery can treat every artifact kind uniformly.

**New table**

```sql
CREATE TABLE IF NOT EXISTS websocket_apis (
    uuid                VARCHAR(40)  PRIMARY KEY,
    organization_uuid   VARCHAR(40)  NOT NULL,
    handle              VARCHAR(40)  NOT NULL,
    display_name        VARCHAR(255) NOT NULL,
    version             VARCHAR(30)  NOT NULL DEFAULT 'v1.0',
    project_uuid        VARCHAR(40)  NOT NULL,
    description         VARCHAR(1023),
    configuration       BYTEA        NOT NULL,  -- JSON: channels + upstream + policies + connection settings
    data_version        VARCHAR(20)  NOT NULL DEFAULT '1.0',
    origin              VARCHAR(20)  NOT NULL DEFAULT 'control_plane',
    created_by          VARCHAR(200),
    created_at          TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP,
    updated_by          VARCHAR(200),
    updated_at          TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);
```

`uuid` is also a foreign key to the shared artifact-registration table — this row is a 1:1 subtype of that polymorphic registration, not an independently-generated identity. `origin` records whether the row was created in the control plane or pushed up from a gateway (`control_plane` / `gateway_api`); a gateway-pushed row is read-only in the control plane. `configuration` is opaque storage — it is never queried with structured JSON operators, so it is stored as a large binary payload rather than a structured JSON column type.

**Indexes**

```sql
CREATE INDEX IF NOT EXISTS idx_websocket_apis_project ON websocket_apis(project_uuid);
```

This serves lookups and listings scoped to a single project. No separate index on `organization_uuid` alone is needed — the `UNIQUE(organization_uuid, handle)` constraint already covers organization-scoped lookups through its leading column, and a second index on the same column would be redundant.

# 3. Gateway Controller

The standalone gateway keeps its **own local database**, entirely separate from the control plane's — it holds whatever artifacts have actually been deployed to that one gateway instance. Every artifact kind there already follows the same, much simpler pattern: one shared `artifacts` table carries every kind's common identity and lifecycle fields (gateway id, display name, version, kind, handle, lifecycle state), and each kind gets a minimal child table holding nothing but that kind's own configuration payload, keyed back to the shared table.

**New table**

```sql
CREATE TABLE IF NOT EXISTS websocket_apis (
    uuid          TEXT NOT NULL,
    gateway_id    TEXT NOT NULL,
    configuration TEXT NOT NULL,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES artifacts(gateway_id, uuid) ON DELETE CASCADE
);
```

This is a one-column-identical clone of the existing `graphql_apis`/`rest_apis` child tables there — the entire derived channel/upstream/policy/connection-setting payload is stored as JSON text in `configuration`, the same shape the control plane's `configuration` column holds in §2, just a separate copy of it local to this gateway.

**No change to the shared `artifacts` table.** Its `kind` column has no database-level `CHECK` constraint — enum validation for it lives in the Go layer, per this project's enum-validation convention — so making this a valid artifact kind is a Go-layer registration change (the kind-to-table-name mapping, and the built-in resource-table list, both need the new kind added), not a schema change. See the Implementation document for where that registration lives.

# 4. Dialect Coverage

## Control Plane

| Dialect | Schema file | Updated |
| :---- | :---- | :---- |
| SQLite (default) | `internal/database/schema.sql` | Yes |
| SQLite | `internal/database/schema.sqlite.sql` | Yes |
| PostgreSQL | `internal/database/schema.postgres.sql` | Yes |
| SQL Server | `internal/database/schema.sqlserver.sql` | Yes |
| MySQL | Not supported by this component. | N/A |

Type divergences across dialects: `configuration` is `BYTEA` (PostgreSQL), `BLOB` (SQLite), `VARBINARY(MAX)` (SQL Server); timestamp columns are `TIMESTAMPTZ` (PostgreSQL), `DATETIME` (SQLite), `DATETIME2(7) DEFAULT SYSUTCDATETIME()` (SQL Server). These are the platform's standard, intentional per-engine equivalents, not a divergence specific to this feature.

One further SQL Server-specific divergence: the `organization_uuid` foreign key uses `ON DELETE NO ACTION` on SQL Server instead of `ON DELETE CASCADE`, because SQL Server rejects two cascade paths converging on the same parent table (here, via the artifact-registration foreign key and this direct one). No application-level cleanup code is needed for this: the row is already guaranteed to be deleted through the artifact-registration cascade path before this foreign key would otherwise matter. The equivalent GraphQL table carries the same documented divergence.

## Gateway Controller

| Dialect | Schema file | Updated |
| :---- | :---- | :---- |
| SQLite (default) | `resources/gateway-controller-db.sql` | Yes |
| SQLite (duplicate copy, kept in sync) | `pkg/storage/gateway-controller-db.sql` | Yes |
| PostgreSQL | `pkg/storage/gateway-controller-db.postgres.sql` | Yes |
| SQL Server | `pkg/storage/gateway-controller-db.sqlserver.sql` | Yes |
| MySQL | Not supported by this component. | N/A |

This component keeps **two identical copies** of its SQLite schema (one under `resources/`, one under `pkg/storage/`) — both need the identical new table, not just one.

Type note: this component uses its own, already-established type convention — `TEXT` for identity and configuration columns in both SQLite and PostgreSQL (not the control plane's `VARCHAR`/`BYTEA` split), and `NVARCHAR(64)` / `NVARCHAR(MAX)` in SQL Server. This is that schema's own prior convention, not a deviation to correct; the new table matches it exactly rather than the control plane's conventions in §2.

# 5. Data Model Notes

* **Org / gateway scoping**: the control-plane table is scoped by `organization_uuid`, enforced by the application layer filtering every query from the caller's verified identity claims, never from request input; `UNIQUE(organization_uuid, handle)` also scopes handle uniqueness to the organization rather than globally. The gateway-controller table is scoped by `gateway_id` instead — it is that one gateway instance's local store, not a multi-tenant table, so there is no organization column there at all.
* **Cardinality / growth**: one row per WebSocket API artifact in each store — bounded by how many WebSocket APIs an organization creates (control plane) or how many are actually deployed to one gateway instance (gateway controller), the same order of magnitude as the existing GraphQL/REST artifact tables in each.
* **Retention / cleanup**: in both stores, rows are removed via cascade when the owning artifact-registration row (and, in the control plane, the organization or project) is deleted — there is no independent deletion path or background cleanup job for either table.
* **Derived values**: nothing is computed on read in either store; the full derived configuration (channels, upstream, policies, connection settings) is persisted at write time inside `configuration` and read back as-is.
* **Sensitive columns**: none in either table. Credentials and API keys are not stored here — they live in each component's existing, generic API-key storage, keyed by this artifact's UUID.

# 6. Upgrade and Migration Impact

* **Fresh install**: covered entirely by the guarded `CREATE TABLE IF NOT EXISTS` / dialect-equivalent guard, in both the control plane's and the gateway controller's databases — no separate step needed.
* **Existing deployment**: none needed, for either table. A brand-new table's guarded `CREATE` statement is a no-op if the table already exists and otherwise creates it — there is no column being added to a table that might already exist, so no `ALTER TABLE` path applies here.
* **Rollback**: an older release that does not know about either table is unaffected by its presence; rolling back simply means the new table sits unused.
* **Tested against**: NOT ANSWERED — which dialects this was actually exercised against (versus only having their schema files edited), for either component, is not established in the inputs.

# 7. Accepted Legacy Deviations

While designing the control-plane table, one pre-existing, non-conforming pattern was found on the already-shipped equivalent GraphQL table: it carries both the `UNIQUE(organization_uuid, handle)` constraint and a separate, redundant single-column index on `organization_uuid` alone — the constraint's leading column already serves that lookup, making the extra index unnecessary (no correctness impact). This is accepted legacy on that shipped table and has not been changed; the new table in this feature does not repeat the pattern. It has also been recorded in the project's schema-rules deviation register.

| Table | Rule | Deviation | Recorded |
| :---- | :---- | :---- | :---- |
| `graphql_apis` (control plane, existing, unrelated to this feature) | Redundant indexing | Carries both an organization-scoped unique constraint and a separate single-column index on the same leading column | 2026-10-14 |
