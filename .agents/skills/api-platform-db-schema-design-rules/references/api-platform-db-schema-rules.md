# API Platform DB Schema Rules (R1–R9)

Reference for the `api-platform-db-schema-design-rules` skill. These are the WSO2 API Platform conventions for relational database schemas. Step A2 and Step B2 of the skill read this file and evaluate each rule against the schema.

**Scope:** applies to every `schema*.sql` file in the repository. For `gateway/gateway-controller/` schemas, **skip R3** (type rules) — the gateway controller team owns their type choices. All other rules (R1–R2, R4–R9) apply to every schema file including gateway controller.

> **Recording blanket-missing findings.** When a rule is violated uniformly across many tables (e.g. a convention that no tables follow), record **one representative finding** that names the pattern and gives a couple of examples, rather than one finding per table.

---

## R1 · Primary Key & Identity

Every table must have a single UUID primary key and, where it is a named resource, the standard identity triple.

**R1-UUID** — Primary key must be `uuid VARCHAR(40) PRIMARY KEY`. Do not use `SERIAL`, `BIGINT`, or `INTEGER` as a primary key for domain entities. Junction/mapping tables must use a composite PK (see R1-COMPOSITE-PK).

**R1-COMPOSITE-PK** — Pure junction/mapping tables (those whose only purpose is to link two or more entities) must use a composite `PRIMARY KEY` over their FK columns — not a surrogate UUID, and not a bare `UNIQUE` constraint.

Use a composite PK when **all** of the following are true:
- Every query hits the table via the composite key (no query looks up a row by a single generated ID)
- No other table holds a FK reference to a row in this table by a surrogate ID
- The table has no independent lifecycle (rows are inserted or deleted, never updated in place by identity)

Do **not** use a composite PK (use a UUID PK instead) when:
- Another table references individual rows by ID (e.g. an audit log or event stream that stores a FK to this table's row)
- The table is exposed as a standalone resource in an API with its own URL (e.g. `/associations/{id}`)

**Why composite PK over UNIQUE-only** — A bare `UNIQUE` constraint without a `PRIMARY KEY` breaks Postgres logical replication for `UPDATE` and `DELETE` operations. Postgres `REPLICA IDENTITY DEFAULT` uses the PK to identify rows in the WAL stream; without a PK it falls back to `REPLICA IDENTITY FULL` (logs entire old row on every write — high WAL volume) or replication fails entirely. CDC tools (Debezium, AWS DMS) have the same requirement. Distributed SQL engines (CockroachDB, YugabyteDB) silently add a hidden PK if you omit one, with unpredictable sharding consequences.

**Column order** — Put the most common filter/scope column first (typically `organization_uuid`), then the remaining FKs. The leading column is covered by the PK index; add separate indexes only for the non-leading FK columns.

```sql
-- Correct composite PK pattern
CREATE TABLE IF NOT EXISTS <junction_table> (
    organization_uuid  VARCHAR(40) NOT NULL,
    entity_a_uuid      VARCHAR(40) NOT NULL,
    entity_b_uuid      VARCHAR(40) NOT NULL,
    created_at         TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (organization_uuid, entity_a_uuid, entity_b_uuid),
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (entity_a_uuid)     REFERENCES entity_a(uuid)      ON DELETE CASCADE,
    FOREIGN KEY (entity_b_uuid)     REFERENCES entity_b(uuid)      ON DELETE CASCADE
);

-- Indexes for non-leading FK columns only (leading column covered by PK)
CREATE INDEX IF NOT EXISTS idx_<junction_table>_entity_a_uuid ON <junction_table>(entity_a_uuid);
CREATE INDEX IF NOT EXISTS idx_<junction_table>_entity_b_uuid ON <junction_table>(entity_b_uuid);
```

**R1-IDENTITY** — Tables representing named resources (APIs, gateways, providers, applications, subscriptions, or any domain entity with a stable slug and a display name) must carry the full identity triple directly:

```sql
handle  VARCHAR(40)  UNIQUE NOT NULL,          -- url-safe slug, immutable once set
name    VARCHAR(255) NOT NULL,                -- human-readable display name
version VARCHAR(30)  NOT NULL DEFAULT '1.0', -- semver or opaque version string
```

Identity must be denormalised onto the table itself so queries against a single table are self-contained. Do not rely on a parent record for identity fields.

**R1-HANDLE-NAME** — `handle` and `name` are distinct:
- `handle` is the URL-safe slug used in API paths (e.g. `/resources/{handle}`). It must be unique within its scope and treated as immutable after creation.
- `name` is the human-readable display string. It may change.

Conflating them (using `name` as the slug, or allowing `handle` to contain spaces) is a finding.

---

## R2 · Organisation Scoping

**R2-ORG-FK** — Every domain table that belongs to an organisation must carry `organization_uuid VARCHAR(40) NOT NULL` with:

```sql
FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE
```

**R2-ORG-UNIQUE** — Uniqueness constraints on named resources must include `organization_uuid`:

```sql
UNIQUE(organization_uuid, handle)   -- not UNIQUE(handle) alone
UNIQUE(organization_uuid, name)
```

A `UNIQUE(name)` without the org scope is a critical data-isolation bug.

---

## R3 · Column Types

**R3-NO-TEXT** — Do not use the bare `TEXT` type for any column. Use either a bounded `VARCHAR(N)`, a binary type, or (Postgres only) `JSONB` when content is queried. A `TEXT` column is always a finding at MEDIUM severity.

**R3-LARGE-PAYLOAD** — Any payload that can grow large or is variable-length must use `BYTEA` (Postgres) / `BLOB` (SQLite) / `VARBINARY(MAX)` (SQL Server). Do **not** use wide VARCHAR for: `openapi_spec`, `model_list`, `content`, `configuration`, `properties`, `manifest`, `policy_definition`, `metadata`, `api_key_hashes`, or any future column whose value can exceed a few hundred bytes.

**R3-JSONB** — In PostgreSQL, use `JSONB` only when the application queries inside the JSON using Postgres JSON operators. Evidence that a column is actively queried inside:
1. The `DEFAULT` is a JSON literal like `'{}'`
2. The column name implies structure: `settings`, `event_data`
3. The same column in a sibling table already uses `JSONB`

SQLite and SQL Server equivalents (`TEXT` / `NVARCHAR(MAX)`) are intentional type-level divergences — not findings.

**R3-JSONB-SCAN-COMPAT** — Do not use `JSONB` if the application layer scans it into a plain `string` variable and calls `json.Unmarshal` manually. Postgres drivers return JSONB as binary, which breaks `string` scan targets at runtime. Only use `JSONB` when the scan target implements `sql.Scanner` (e.g. `pgtype.JSONB`, `json.RawMessage`, or a custom struct).

**R3-BOOLEAN-AS-INT** — Do not use the `BOOLEAN` type. Represent boolean flags as:
- `SMALLINT DEFAULT 0` / `SMALLINT NOT NULL DEFAULT 1` — Postgres
- `INTEGER DEFAULT 0` / `INTEGER NOT NULL DEFAULT 1` — SQLite / SQL Server

**R3-TIMESTAMPTZ** — Use `TIMESTAMPTZ` for **all** timestamp columns in PostgreSQL. `TIMESTAMP` (without timezone) is a bare clock reading with no timezone attached — a finding at MEDIUM. Use `DATETIME` in SQLite and `DATETIME2(7) DEFAULT SYSUTCDATETIME()` in SQL Server.

**R3-VARCHAR-SIZES** — Standard widths:

| Purpose | Width |
|---|---|
| UUID / foreign key | `VARCHAR(40)` |
| User identity (email / sub) | `VARCHAR(200)` |
| Handle / name / display string | `VARCHAR(255)` |
| Version string | `VARCHAR(30)` |
| Lifecycle / status enum | `VARCHAR(20)` |
| Description / reason | `VARCHAR(1023)` |
| Token / hash | `VARCHAR(512)` or `VARCHAR(64)` as appropriate |

Any width above `VARCHAR(1023)` is a strong signal that the column should be `BYTEA`/`BLOB` instead.

**R3-VARCHAR-ENGINE-LIMITS** — Key engine limits for indexed/unique columns:

| Usage | Safe max width |
|---|---|
| Plain storage, no index | `VARCHAR(1023)` |
| Appears in a UNIQUE constraint or any index | `VARCHAR(255)` (safe across all engines with utf8mb4) |
| Oracle target (any index or non-extended) | `VARCHAR(255)` |
| MySQL target (utf8mb4, default prefix limit) | `VARCHAR(191)` |

When a column must be unique but its value can be large, store the value in `BYTEA`/`BLOB` and put a SHA-256 hash in a separate `VARCHAR(64)` column — index and unique-constrain the hash, not the value.

---

## R4 · Constraints

**R4-NO-ENUM-CHECK** — **Do NOT add `CHECK (col IN (...))` constraints for enum or status columns.** Enum validation belongs in application code (Go constants, service-layer validation). DB-layer enum checks require a DDL migration just to add a new valid value.

The only `CHECK` constraints that belong in the schema are structural/cross-column invariants:
```sql
-- Cross-column consistency: both NULL or both non-NULL
CONSTRAINT chk_throttle_pair CHECK (
  (throttle_limit_count IS NULL AND throttle_limit_unit IS NULL) OR
  (throttle_limit_count IS NOT NULL AND throttle_limit_unit IS NOT NULL)
)
-- Temporal consistency
CHECK (revoked_at IS NULL OR status = 'revoked')
```

**R4-NOT-NULL** — Columns that are always required must be `NOT NULL`. Common offenders: `organization_uuid`, `name`, `handle`, `version`, `status`.

**R4-FK-BEHAVIOR** — Every foreign key must declare an explicit `ON DELETE` action:

| Relationship | Rule |
|---|---|
| Child owned by parent (cascade delete is safe) | `ON DELETE CASCADE` |
| Reference that must not dangle but parent cannot be deleted | `ON DELETE RESTRICT` |
| Optional reference — row survives if target is deleted | `ON DELETE SET NULL` |

Omitting `ON DELETE` means the DB default (`RESTRICT` in most engines) applies silently — flag it.

---

## R5 · Audit Columns

**R5-AUDIT-SET** — Only tables written as a **direct consequence of a user-initiated action** (e.g. a REST API call) carry `created_by` / `updated_by`. Tables written by background sync, callbacks, or internal system processes must **not** include these columns.

Rule of thumb: ask "does a human-initiated request cause this row to be inserted or updated?" If yes → include the full audit set. If no → omit `created_by` and `updated_by`.

```sql
data_version VARCHAR(20)  NOT NULL DEFAULT '1.0',
created_by   VARCHAR(200),
created_at   TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP,  -- DATETIME on SQLite / DATETIME2 on SQL Server
updated_by   VARCHAR(200),
updated_at   TIMESTAMPTZ  DEFAULT CURRENT_TIMESTAMP,
```

**R5-IMMUTABLE-CREATED** — `created_by` and `created_at` must never be updated after insert.

**R5-REVOKE-PATTERN** — Tables with soft-revocation add:

```sql
revoked_by VARCHAR(200),
revoked_at TIMESTAMPTZ,
CHECK (revoked_at IS NULL OR status = 'revoked')
```

**R5-DATA-VERSION** — Every domain entity table must include `data_version VARCHAR(20) NOT NULL DEFAULT '1.0'`. Place it immediately before `created_by`. Excluded tables: junction/mapping tables and pure system/event tables (e.g. `events`, `gateway_states`, `audit`, `deployment_status`, `gateway_custom_policy_usages`, `application_api_keys`, `application_artifacts`, `gateway_association_mappings`).

---

## R6 · Indexing

Index every column (or compound) that appears in a `WHERE`, `JOIN`, or `ORDER BY` clause at production query volume.

**R6-FK-INDEX** — Every foreign key column must have an index unless it is already the leftmost column of the PK or a covering UNIQUE constraint.

```sql
CREATE INDEX IF NOT EXISTS idx_<table>_<fk_col> ON <table>(<fk_col>);
```

**R6-ORG-INDEX** — Every org-scoped table must have an index on `organization_uuid`:

```sql
CREATE INDEX IF NOT EXISTS idx_<table>_org ON <table>(organization_uuid);
```

**R6-STATUS-INDEX** — Tables with a `status` column that is filtered in list queries need a status index:

```sql
CREATE INDEX IF NOT EXISTS idx_<table>_status ON <table>(status);
```

**R6-COMPOUND-INDEX** — When the common query is `WHERE a = ? AND b = ?`, a single compound index `(a, b)` outperforms two single-column indexes. Most-selective filter first.

**R6-PARTIAL-INDEX** — Use a partial index when the filter discards most rows:

```sql
CREATE INDEX IF NOT EXISTS idx_<table>_expires_at
  ON <table>(expires_at) WHERE expires_at IS NOT NULL;
```

**R6-UNIQUE-PARTIAL** — For "at most one default per org" patterns:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_<table>_default_per_org
  ON <table>(organization_uuid) WHERE is_default = 1;
```

**R6-NO-REDUNDANT-INDEX** — Do not create an index that is a prefix of an existing UNIQUE constraint or PK.

**R6-GIN-JSONB** — Add a GIN index only when the application concretely queries inside a JSONB column with JSONB operators — do not pre-emptively GIN-index every JSONB column.

---

## R7 · Application Logic Safety

**R7-NO-SELECT-STAR** — Schema changes break `SELECT *` callers. All queries must select named columns.

**R7-JSONB-SCAN** — A `JSONB` column must be scanned into a type that implements `sql.Scanner`. Scanning into `string` bypasses Postgres validation.

**R7-HANDLE-IMMUTABLE** — `handle` must be set on INSERT and never appear in `UPDATE` statements.

**R7-CREATED-IMMUTABLE** — `created_at` and `created_by` must not appear in `UPDATE` statements.

**R7-SOFT-DELETE** — Prefer status transitions to terminal states (`REVOKED`, `ARCHIVED`, `RETIRED`) over soft-delete (`deleted_at`) columns. Only add `deleted_at` when the design explicitly requires row-level tombstones.

**R7-OPTIMISTIC-LOCK** — For resources that are concurrently edited, compare `updated_at` before issuing an UPDATE:

```sql
UPDATE <table>
SET ..., updated_at = NOW(), updated_by = $n
WHERE uuid = $1 AND updated_at = $expected_updated_at
```

If 0 rows are affected, the caller lost the race and must retry.

---

## R8 · Multi-Engine Schema Alignment

When the project maintains schema files for multiple database engines (Postgres + SQLite + SQL Server), the files must remain structurally in sync except for the known intentional type-level divergences below.

**Intentional divergences — not findings:**

| Feature | SQLite | PostgreSQL | SQL Server |
|---|---|---|---|
| JSON-valued columns (queried with JSON operators) | `TEXT` | `JSONB` | `NVARCHAR(MAX)` |
| JSON-valued columns (opaque storage only) | `TEXT` | `VARCHAR(N)` or `BYTEA` | `VARCHAR(N)` or `NVARCHAR(MAX)` |
| Binary columns | `BLOB` | `BYTEA` | `VARBINARY(MAX)` |
| Large text payloads | `TEXT` | `BYTEA` | `NVARCHAR(MAX)` |
| All timestamps | `DATETIME` | `TIMESTAMPTZ` | `DATETIME2(7) DEFAULT SYSUTCDATETIME()` |
| Boolean flags | `INTEGER` (0/1) | `SMALLINT` (0/1) | `SMALLINT` (0/1) |
| JSON literal defaults | `'{}'` | `'{}'::jsonb` | `'{}'` |

**R8-SYNC-STRUCTURE** — Table definitions (columns, order, constraints, CHECK values, FK targets) not in the intentional-divergence list must be identical across all schema files. Verify:
- Same columns, same order
- Same `NOT NULL` / nullable on each column
- Same `DEFAULT` values (modulo type syntax)
- Same `CHECK` constraint values
- Same index definitions

**R8-SQLITE-NO-JSONB** — SQLite does not support `JSONB`. Any `JSONB` in a SQLite schema file is a bug — all JSON columns must be `TEXT`.

---

## R9 · Idempotent DDL

Every `CREATE TABLE` and `CREATE INDEX` statement must be safe to re-run without errors.

**R9-TABLE** — Use the engine-specific existence guard:

```sql
-- PostgreSQL / SQLite
CREATE TABLE IF NOT EXISTS <table> (...);

-- SQL Server
IF OBJECT_ID(N'dbo.<table>', N'U') IS NULL
CREATE TABLE dbo.<table> (...);
```

**R9-INDEX** — Use `IF NOT EXISTS` (Postgres/SQLite) or a `sys.indexes` check (SQL Server):

```sql
-- PostgreSQL / SQLite
CREATE INDEX IF NOT EXISTS idx_... ON <table>(...);

-- SQL Server
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_...' AND object_id = OBJECT_ID(N'dbo.<table>'))
CREATE INDEX idx_... ON dbo.<table>(...);
```

A `CREATE TABLE` or `CREATE INDEX` without an existence guard is always a finding at MEDIUM severity.
