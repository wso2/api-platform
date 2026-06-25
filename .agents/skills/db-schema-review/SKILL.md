---
name: db-schema-review
description: |
  Design, change, and review relational database schemas. Use proactively whenever:
  - Adding a new table
  - Modifying an existing table (new column, type change, constraint, index)
  - Reviewing schema changes before a PR
  - Writing or evaluating a migration plan
  - Asking "is this table well designed?" or "what indexes does this table need?"

  Applies house conventions: UUID primary keys (VARCHAR(40)) for entity tables, composite PRIMARY KEY for junction/mapping tables (never UNIQUE-only — breaks Postgres logical replication), handle/name/version identity triple for named resources, org-scoping, JSONB only when JSON operators are used (Postgres) — otherwise VARCHAR or BLOB/BYTEA, audit columns (created_by/at, updated_by/at), data_version for on-the-fly migrations, and standard indexing patterns. Enum/status validation belongs in application code — do NOT add CHECK constraints for enum values at the DB layer.

  When the project uses Postgres, SQLite, and/or SQL Server schemas, all files must be kept in sync — every change to one must be reflected in the others unless it is an intentional type-level divergence (JSONB/TEXT/NVARCHAR(MAX), BYTEA/BLOB/VARBINARY(MAX), TIMESTAMPTZ/DATETIME/DATETIME2).
allowed-tools: Bash, Read, Edit, Write, Grep, Glob
---

# Database Schema — Design, Change & Review

This skill governs **all schema work** — designing new tables, modifying existing ones, and reviewing changes for correctness. It is not a post-hoc review tool; it is the process to follow when writing DDL.

## Usage

```
/db-schema-review [new-table-name | existing-table-name | path-to-schema-file]
```

- **No argument** — review all schema files in the project.
- **Table name** — apply the relevant workflow for that table (add or modify).
- **Schema file path** — review that file only.

---

## Workflows

There are two entry points. Choose based on whether you are making a change or reviewing existing DDL.

---

### Workflow A — Making a Schema Change

Follow this workflow for every ADD TABLE, ADD COLUMN, ALTER COLUMN, ADD INDEX, or ADD CONSTRAINT.

#### A1 · Read the schemas first

Locate and read all schema files before writing any DDL:

```bash
find . -name "*.sql" | grep -i schema
```

#### A2 · Draft the DDL against the rules

Work through the relevant rules from the Rules section:
- **New table** → R1 (identity), R2 (org-scoping), R3 (types), R4 (constraints), R5 (audit columns), R6 (indexes)
- **New column** → R3 (type), R4 (constraints), R5 (audit), R6 (index if filterable)
- **Type change** → R3 (correct type for target engine), R8 (counterpart schema if dual-engine)
- **New index** → R6 (correct pattern — FK, status, compound, partial)

Use the quick-reference templates at the bottom of this skill as a starting point.

#### A3 · Apply to all schema files

When the project maintains multiple schema files (e.g. Postgres + SQLite), apply every structural change to all of them unless it is an intentional type-level divergence (see R8).

#### A4 · Self-review before writing

Before writing the final DDL to disk, confirm each item passes:

```
[ ] R1  Every entity table has uuid VARCHAR(40) PRIMARY KEY
[ ] R1  Junction/mapping tables use a composite PRIMARY KEY — not a surrogate UUID, not UNIQUE-only
[ ] R1  Non-leading FK columns of a composite PK have their own indexes
[ ] R1  Named resource tables carry handle + name + version (NOT NULL)
[ ] R2  organization_uuid FK present and UNIQUE constraints include it (if org-scoped)
[ ] R3  No TEXT columns — use VARCHAR(N), JSONB (Postgres, query-only), or BYTEA/BLOB
[ ] R3  Any payload that can grow large uses BYTEA/BLOB — not wide VARCHAR (configuration, properties, manifest, policy_definition, metadata, api_key_hashes, openapi_spec, model_list, content)
[ ] R3  Opaque JSON stored as a string uses BYTEA/BLOB (e.g. event_data) — not VARCHAR; JSONB only when queried with JSON operators inside Postgres
[ ] R3  Boolean flags use SMALLINT/INTEGER (not BOOLEAN), with DEFAULT 1/0
[ ] R3  VARCHAR widths match the standard table — no width above VARCHAR(1023) for plain storage, VARCHAR(255) max for any indexed/unique column
[ ] R3  Engine limits checked: Oracle VARCHAR2 ≤ 4,000 bytes; MySQL indexed VARCHAR ≤ 191 chars (utf8mb4 default) or ≤ 768 (large prefix); Postgres UNIQUE index entry ≤ 8,191 bytes (see R3-VARCHAR-ENGINE-LIMITS)
[ ] R3  All timestamp columns use TIMESTAMPTZ (Postgres) / DATETIME (SQLite) — never bare TIMESTAMP in Postgres
[ ] R4  No enum/status CHECK constraints added (validation in application code only)
[ ] R4  Every FK has an explicit ON DELETE clause
[ ] R5  User-initiated table → all four audit columns present (created_by/at, updated_by/at)
[ ] R5  System-managed table → created_by and updated_by are ABSENT
[ ] R5  Every data model table has data_version VARCHAR(20) NOT NULL DEFAULT 'v1.0'
[ ] R6  FK columns have indexes
[ ] R6  organization_uuid has an index (if org-scoped)
[ ] R6  status has an index if the column is used as a filter
[ ] R8  Change applied to all schema files (or divergence is intentional)
[ ] R9  All DDL is idempotent — CREATE TABLE uses IF NOT EXISTS (Postgres/SQLite) or IF OBJECT_ID(...) IS NULL (SQL Server); CREATE INDEX uses IF NOT EXISTS
```

#### A5 · Write the DDL

All DDL must be idempotent — safe to re-run without errors. Use the engine-specific guard for each statement:

```sql
-- PostgreSQL / SQLite
CREATE TABLE IF NOT EXISTS <table> (...);
CREATE INDEX IF NOT EXISTS idx_... ON ...;

-- SQL Server
IF OBJECT_ID(N'dbo.<table>', N'U') IS NULL
CREATE TABLE dbo.<table> (...);

IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_...' AND object_id = OBJECT_ID(N'dbo.<table>'))
CREATE INDEX idx_... ON dbo.<table>(...);
```

Keep `CREATE INDEX` statements in a dedicated indexes block after all `CREATE TABLE` statements, not inline in table definitions.

---

### Workflow B — Reviewing Existing DDL (PR / Audit)

#### B1 · Locate and read all schema files

```bash
find . -name "*.sql" | grep -i schema
```

Read all files before making any assessment.

#### B2 · Run all rule groups in order (R1–R8)

For each finding record:
- **Rule** — e.g. `R3-JSONB`
- **Table · column** — exact location
- **Severity** — HIGH (data safety / correctness) · MEDIUM (missing guarantee or index) · LOW (style / inconsistency)
- **Finding** — what is wrong
- **Fix** — the exact DDL needed

#### B3 · Cross-check multi-engine divergences

After per-table review, verify all schema files are structurally in sync (see R8). Intentional type-level divergences do not count as findings.

#### B4 · Report

Produce a findings table sorted by severity. Include a "No issues" row for any rule group that passed cleanly — reviewers need to know what was checked.

---

## Rules

These rules apply in both Workflow A (making changes) and Workflow B (reviewing). In Workflow A, use them as design constraints before writing DDL. In Workflow B, use them as a checklist to detect violations.

---

### R1 · Primary Key & Identity

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
handle  VARCHAR(255) NOT NULL,                -- url-safe slug, immutable once set
name    VARCHAR(255) NOT NULL,                -- human-readable display name
version VARCHAR(30)  NOT NULL DEFAULT 'v1.0', -- semver or opaque version string
```

Identity must be denormalised onto the table itself so queries against a single table are self-contained. Do not rely on a parent record for identity fields.

**R1-HANDLE-NAME** — `handle` and `name` are distinct:
- `handle` is the URL-safe slug used in API paths (e.g. `/resources/{handle}`). It must be unique within its scope and treated as immutable after creation.
- `name` is the human-readable display string. It may change.

Conflating them (using `name` as the slug, or allowing `handle` to contain spaces) is a finding.

---

### R2 · Organisation Scoping

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

### R3 · Column Types

**R3-NO-TEXT** — Do not use the bare `TEXT` type for any column. Every column must use either a bounded `VARCHAR(N)`, a binary type, or (Postgres only) `JSONB` when content is queried. The two allowed alternatives for structured data:
- `VARCHAR(N)` — for short, bounded strings whose length is genuinely constrained (UUIDs, handles, names, status codes, version strings, descriptions, hashes, tokens). Keep widths within the standard table (see R3-VARCHAR-SIZES). **Do not use wide VARCHAR for large or variable-length payloads** — engine index and row-size limits make this dangerous across MySQL and Oracle (see R3-VARCHAR-ENGINE-LIMITS).
- `BYTEA` (Postgres) / `BLOB` (SQLite/MySQL) / `BLOB` (Oracle) — for any payload that can grow large or unbounded: binary content, serialised structs, large JSON documents, and opaque byte streams (`openapi_spec`, `model_list`, compiled artifacts, `configuration`, `properties`, `manifest`, `policy_definition`, `metadata`, `api_key_hashes`).

A `TEXT` column is always a finding at MEDIUM severity. Determine which of the above fits and apply that type.

**R3-JSONB** — In PostgreSQL, use `JSONB` only when the application queries inside the JSON using Postgres JSON operators. Evidence that a column is JSON-valued and actively queried inside:
1. The `DEFAULT` is a JSON literal like `'{}'`
2. The column name implies structure: `settings`, `event_data`
3. The same column in a sibling table already uses `JSONB`

SQLite equivalents should be `TEXT` — that is intentional and is not a finding.

**R3-JSONB-SCAN-COMPAT** — Do not use `JSONB` if the application layer scans it into a plain `string` variable and calls `json.Unmarshal` manually. Postgres drivers return JSONB as binary, which breaks `string` scan targets at runtime. Only use `JSONB` when the scan target is a type implementing `sql.Scanner` (e.g. `pgtype.JSONB`, `json.RawMessage`, or a custom struct).

**R3-BOOLEAN-AS-INT** — Do not use the `BOOLEAN` type. Represent boolean flags as integers: `SMALLINT` (Postgres) or `INTEGER` (SQLite/generic), with `DEFAULT 1` for true and `DEFAULT 0` for false. This avoids cross-engine inconsistencies and driver coercion surprises. Example:

```sql
-- Postgres
is_active SMALLINT DEFAULT 0,
enabled   SMALLINT NOT NULL DEFAULT 1,

-- SQLite / generic
is_active INTEGER DEFAULT 0,
enabled   INTEGER NOT NULL DEFAULT 1,
```

**R3-BINARY** — Binary data is `BYTEA` (Postgres) / `BLOB` (SQLite). Do not store binary as `TEXT` or `VARCHAR`.

**R3-TIMESTAMPTZ** — Use `TIMESTAMPTZ` for **all** timestamp columns in PostgreSQL. `TIMESTAMPTZ` stores a point in time (UTC internally) and converts to the session timezone on read — it is always correct across timezones, DST shifts, and distributed replicas. `TIMESTAMP` (without timezone) is a bare clock reading with no timezone attached; two sessions in different zones read the same value as different real-world moments. Since both types use 8 bytes, there is no storage reason to choose `TIMESTAMP`. The only exception is a "floating" local-time value that must stay fixed regardless of timezone (e.g. "shop opens at 09:00") — which does not apply to any column in this schema.

**R3-VARCHAR-SIZES** — Standard widths:

| purpose | width |
|---|---|
| UUID / foreign key | `VARCHAR(40)` |
| User identity (email / sub) | `VARCHAR(200)` |
| Handle / name / display string | `VARCHAR(255)` |
| Version string | `VARCHAR(30)` |
| Lifecycle / status enum | `VARCHAR(20)` |
| Description / reason | `VARCHAR(1023)` |
| Token / hash | `VARCHAR(512)` or `VARCHAR(64)` as appropriate |

Arbitrary widths (`VARCHAR(100)`, `VARCHAR(500)`) should be justified or standardised. Any width above `VARCHAR(1023)` is a strong signal that the column should be `BYTEA`/`BLOB` instead.

**R3-VARCHAR-ENGINE-LIMITS** — VARCHAR is not a safe type for large payloads. Each major engine imposes hard limits that are easy to violate with wide VARCHAR columns, especially when those columns appear in indexes or UNIQUE constraints. Understand these limits before assigning any width above `VARCHAR(255)`:

**Oracle (standard `MAX_STRING_SIZE=STANDARD`):**
- `VARCHAR2` max: **4,000 bytes**. A `VARCHAR(8192)` DDL statement is a syntax error on standard Oracle.
- With `MAX_STRING_SIZE=EXTENDED`: up to 32,767 bytes, but this requires a non-default database configuration and stores values > 4,000 bytes out-of-line (similar to a LOB).
- Columns wider than 4,000 bytes must use `CLOB` (character) or `BLOB` (binary) on Oracle.
- `UNIQUE` and index key size: bounded by the DB block size. On a default 8 KB block, the practical single-column index key limit is ~6,400 bytes — a `VARCHAR2(4000)` column with a unique constraint is at the edge and risks `ORA-01450` on wide values.
- Use `VARCHAR2` for Oracle instead of `VARCHAR`; they are equivalent but `VARCHAR` semantics are reserved for future change in the Oracle spec.

**MySQL / MariaDB:**
- `VARCHAR` max per column: **65,535 bytes**, but this is a *shared* per-row budget across all VARCHAR columns in the table. A table with multiple wide VARCHAR columns will hit `Row size too large` errors.
- **Index key length limits (InnoDB):**
  - Default (`innodb_large_prefix=OFF`): **767 bytes** per index column. With `utf8mb4` encoding (4 bytes/char), this caps an indexed VARCHAR at **191 characters** before needing a prefix index.
  - With `innodb_large_prefix=ON` (default since MySQL 5.7.7 / Barracuda row format): **3,072 bytes** per index column → caps indexed VARCHAR at **768 characters** with utf8mb4.
  - A `UNIQUE(handle)` on `VARCHAR(255)` with utf8mb4 uses 1,020 bytes — safe only with large prefix enabled. A `UNIQUE` on `VARCHAR(1023)` uses 4,092 bytes — exceeds even the large-prefix limit and will fail.
- Partial indexes (WHERE clause) are **not supported** in MySQL — those are Postgres-only.
- MySQL does not support `BYTEA`; use `BLOB` (up to 65,535 bytes), `MEDIUMBLOB` (up to 16 MB), or `LONGBLOB` (up to 4 GB).

**PostgreSQL:**
- `VARCHAR(N)` max: up to 1 GB — no practical storage concern.
- `UNIQUE` / B-tree index key size: **8,191 bytes** per entry. A single `VARCHAR(4096)` column in a UNIQUE constraint can hold entries up to 4,096 single-byte characters safely; with multi-byte UTF-8 this can exceed 8,191 bytes at runtime, causing `index row size exceeds maximum`.
- Even in Postgres, columns wider than `VARCHAR(1023)` that appear in any UNIQUE constraint or index are a risk. Use `BYTEA` and index a hash if uniqueness on a large blob is needed.

**Practical rules derived from these limits:**

| Usage | Safe max width |
|---|---|
| Plain storage, no index | `VARCHAR(1023)` (above this, use BYTEA/BLOB) |
| Appears in a UNIQUE constraint or any index | `VARCHAR(255)` (safe across all engines with utf8mb4) |
| Oracle target (any index or non-extended) | `VARCHAR(255)` |
| MySQL target (utf8mb4, default prefix limit) | `VARCHAR(191)` |

When a column must be unique but its value can be large (e.g. a URL, a JSON key, a content hash), store the value in `BYTEA`/`BLOB` and put a SHA-256 hash in a separate `VARCHAR(64)` column — index and unique-constrain the hash, not the value.

---

### R4 · Constraints

**R4-NO-ENUM-CHECK** — **Do NOT add `CHECK (col IN (...))` constraints for enum or status columns at the database layer.** Enum validation belongs exclusively in application code (Go constants, service-layer validation). DB-layer enum checks create schema coupling: adding a new valid value requires a DDL migration even when no structural change is needed. Define application-level constants that mirror the valid values instead.

The only `CHECK` constraints that belong in the schema are structural/cross-column invariants — for example:
```sql
-- Cross-column consistency: both NULL or both non-NULL
CONSTRAINT chk_throttle_pair CHECK (
  (throttle_limit_count IS NULL AND throttle_limit_unit IS NULL) OR
  (throttle_limit_count IS NOT NULL AND throttle_limit_unit IS NOT NULL)
)
-- Temporal consistency: revoked_at only set when status is revoked
CHECK (revoked_at IS NULL OR status = 'revoked')
```

**R4-NOT-NULL** — Columns that are always required must be `NOT NULL`. Review nullable columns and ask: "can a row legitimately exist without this value?" If no, add `NOT NULL`. Common offenders: `organization_uuid`, `name`, `handle`, `version`, `status`.

**R4-FK-BEHAVIOR** — Every foreign key must declare an explicit `ON DELETE` action. Choose deliberately:

| relationship | rule |
|---|---|
| Child owned by parent (cascade delete is safe) | `ON DELETE CASCADE` |
| Reference that must not dangle but parent cannot be deleted | `ON DELETE RESTRICT` |
| Optional reference — row survives if target is deleted | `ON DELETE SET NULL` |

Omitting `ON DELETE` means the DB default (`RESTRICT` in most engines) applies silently — flag it.

**R4-CONSTRAINT-PAIR** — Paired columns that must both be NULL or both non-NULL must use a named `CONSTRAINT`:

```sql
CONSTRAINT chk_throttle_pair CHECK (
  (throttle_limit_count IS NULL AND throttle_limit_unit IS NULL) OR
  (throttle_limit_count IS NOT NULL AND throttle_limit_unit IS NOT NULL)
)
```

---

### R5 · Audit Columns

**R5-AUDIT-SET** — Only tables that are written as a **direct consequence of a user-initiated action** (e.g. a REST API call) carry `created_by` / `updated_by`. Tables written by background sync, callbacks, or internal system processes must NOT include these columns — there is no user principal to record, so the columns would always be null and are misleading.

Rule of thumb: ask "does a human-initiated request cause this row to be inserted or updated?" If yes → include the full audit set. If no → omit `created_by` and `updated_by`.

For user-initiated tables, include all four audit columns:

```sql
created_by VARCHAR(200),
created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
updated_by VARCHAR(200),
updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
```

`created_by` / `updated_by` are `VARCHAR(200)` to hold a user's email or subject identifier. They are nullable because seed data and migrations may not have a principal.

**R5-IMMUTABLE-CREATED** — `created_by` and `created_at` must never be updated after insert. The application layer must not include them in `UPDATE` statements.

**R5-REVOKE-PATTERN** — Tables that have soft-revocation follow a separate pair:

```sql
revoked_by VARCHAR(200),
revoked_at TIMESTAMP,
CHECK (revoked_at IS NULL OR status = 'revoked')
```

The CHECK constraint ties the timestamp to the status value — both must agree.

**R5-IMMUTABLE-JUNCTION** — Pure junction tables carry audit columns but are effectively append-only. Rows should be inserted or deleted, never updated in place.

**R5-DATA-VERSION** — Every table that represents a data model (i.e. carries audit columns or is a domain entity) must include:

```sql
data_version VARCHAR(20) NOT NULL DEFAULT 'v1.0',
```

Place it immediately before `created_by` (or before `created_at` on system-managed tables that have no `created_by`). The format is dot-versioned with a `v` prefix: `v1.0`, `v1.1`, `v2.0`. This column enables on-the-fly data migrations — application code can inspect `data_version` to determine the shape of serialised data in `configuration`, `properties`, or other payload columns and upgrade rows lazily on read. Junction/mapping tables and pure system/event tables (e.g. `events`, `gateway_states`, `audit`, `deployment_status`, `gateway_custom_policy_usages`, `application_api_keys`, `application_artifacts`, `gateway_association_mappings`) are excluded.

---

### R6 · Indexing

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

**R6-COMPOUND-INDEX** — When the common query is `WHERE a = ? AND b = ?`, a single compound index `(a, b)` outperforms two single-column indexes. Column order: most-selective filter first.

```sql
CREATE INDEX IF NOT EXISTS idx_<table>_org_<field> ON <table>(organization_uuid, <filter_field>);
```

**R6-PARTIAL-INDEX** — Use a partial index when the filter discards most rows:

```sql
CREATE INDEX IF NOT EXISTS idx_<table>_expires_at
  ON <table>(expires_at) WHERE expires_at IS NOT NULL;
```

**R6-UNIQUE-PARTIAL** — For "at most one default per org" patterns, use a partial unique index:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_<table>_default_per_org
  ON <table>(organization_uuid) WHERE is_default = 1;
```

**R6-NO-REDUNDANT-INDEX** — Do not create an index that is a prefix of an existing UNIQUE constraint or PK. Flag and remove it.

**R6-GIN-JSONB** — If application code queries inside a JSONB column using JSONB operators, add a GIN index:

```sql
CREATE INDEX IF NOT EXISTS idx_<table>_<col>_gin
  ON <table> USING GIN (<col>);
```

Only add this when there is a concrete query that uses JSONB operators — do not pre-emptively GIN-index every JSONB column.

---

### R7 · Application Logic Safety

**R7-NO-SELECT-STAR** — Schema changes (added columns, type promotions) break `SELECT *` callers. All queries must select named columns.

**R7-JSONB-SCAN** — A `JSONB` column must be scanned into a type that implements `sql.Scanner`. Scanning into `string` bypasses Postgres validation. Verify the Go struct field type when promoting TEXT → JSONB.

**R7-DEFAULT-CAST** — When a JSONB column has `DEFAULT '{}'`, supply a cast on insert if the driver does not:

```sql
INSERT INTO <table> (..., config) VALUES (..., $1::jsonb)
```

**R7-STATUS-ENUM** — Application code must not hard-code status strings without referencing the CHECK constraint values. Define constants that mirror each `CHECK(status IN (...))`.

**R7-HANDLE-IMMUTABLE** — `handle` must be set on INSERT and never modified by UPDATE. Do not include `handle` in the `SET` clause of any `UPDATE` statement.

**R7-CREATED-IMMUTABLE** — `created_at` and `created_by` must not appear in `UPDATE` statements.

**R7-SOFT-DELETE** — Prefer status transitions to terminal states (`REVOKED`, `ARCHIVED`, `RETIRED`) over soft-delete (`deleted_at`) columns. Only add `deleted_at` when the design explicitly requires row-level tombstones.

**R7-OPTIMISTIC-LOCK** — For resources that are concurrently edited, application-level optimistic locking should compare `updated_at` before issuing an UPDATE:

```sql
UPDATE <table>
SET ..., updated_at = NOW(), updated_by = $n
WHERE uuid = $1 AND updated_at = $expected_updated_at
```

If 0 rows are affected, the caller lost the race and must retry.

---

### R8 · Multi-Engine Schema Alignment

When the project maintains schema files for multiple database engines (e.g. Postgres + SQLite + SQL Server), the files must remain structurally in sync except for the known intentional type-level divergences listed below.

**Intentional divergences — not findings:**

| Feature | SQLite | PostgreSQL | SQL Server |
|---|---|---|---|
| JSON-valued columns (queried with JSON operators) | `TEXT` | `JSONB` | `NVARCHAR(MAX)` |
| JSON-valued columns (opaque storage only) | `TEXT` | `VARCHAR(N)` or `BYTEA` | `VARCHAR(N)` or `NVARCHAR(MAX)` |
| Binary columns | `BLOB` | `BYTEA` | `VARBINARY(MAX)` |
| Large text payloads | `TEXT` | `BYTEA` | `NVARCHAR(MAX)` |
| All timestamps | `DATETIME` | `TIMESTAMPTZ` | `DATETIME2(7) DEFAULT SYSUTCDATETIME()` |
| Auto-increment keys | `INTEGER` | `SERIAL` | `IDENTITY(1,1)` |
| Boolean flags | `INTEGER` (0/1) | `SMALLINT` (0/1) | `SMALLINT` (0/1) |
| JSON literal defaults | `'{}'` | `'{}'::jsonb` | `'{}'` |

**R8-SYNC-STRUCTURE** — Table definitions (columns, order, constraints, CHECK values, FK targets) that are not in the intentional-divergence list must be identical across all schema files. Check:
- Same columns, same order
- Same `NOT NULL` / nullable on each column
- Same `DEFAULT` values (modulo type syntax)
- Same `CHECK` constraint values
- Same index definitions

**R8-SQLITE-NO-JSONB** — SQLite does not support `JSONB`. Any `JSONB` appearing in a SQLite schema file is a bug — all JSON columns must be `TEXT`.

---

### R9 · Idempotent DDL

Every `CREATE TABLE` and `CREATE INDEX` statement must be safe to re-run. This allows schema files to be applied repeatedly (CI, fresh environments, disaster recovery) without errors.

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

---

## Reference: Standard VARCHAR Widths

```
VARCHAR(20)   — status, lifecycle_status, kind, short enums
VARCHAR(30)   — version strings
VARCHAR(40)   — uuid, all FK columns referencing UUIDs
VARCHAR(64)   — hashes (SHA-256 hex)
VARCHAR(200)  — created_by, updated_by, revoked_by (user email / subject)
VARCHAR(255)  — handle, name, display strings
              — SAFE upper bound for indexed/unique columns across all engines (utf8mb4-aware)
VARCHAR(512)  — tokens (encrypted values)
VARCHAR(1023) — description, reason
              — UPPER BOUND for plain-storage VARCHAR; anything wider belongs in BYTEA/BLOB

-- TEXT is BANNED: use VARCHAR(N), JSONB, or BYTEA/BLOB instead.

JSONB         — (Postgres only) JSON columns actively queried with JSON operators; plain opaque JSON storage uses VARCHAR(N) or BYTEA instead
BYTEA / BLOB  — all large or variable-length payloads: openapi_spec, model_list, content, configuration,
                properties, manifest, policy_definition, metadata, api_key_hashes, and any future column
                whose value can exceed a few hundred bytes.

TIMESTAMPTZ   — all timestamp columns in PostgreSQL (created_at, updated_at, revoked_at, expires_at, performed_at, expiry_time)
              — DATETIME on SQLite, TIMESTAMP WITH TIME ZONE on Oracle, DATETIME on MySQL
SMALLINT      — boolean flags (Postgres): is_active, is_default, is_enabled — use 0/1, never BOOLEAN
INTEGER       — boolean flags (SQLite/MySQL/generic), numeric counters and limits

-- Engine VARCHAR / index limits (see R3-VARCHAR-ENGINE-LIMITS for full detail):
--   Oracle standard:  VARCHAR2 max 4,000 bytes — DDL error above this without EXTENDED mode
--   MySQL utf8mb4:    indexed VARCHAR ≤ 191 chars (default prefix) / 768 chars (large_prefix ON)
--   PostgreSQL:       UNIQUE/B-tree index entry ≤ 8,191 bytes — runtime error on wide values
```

---

## Reference: Audit Column Template

```sql
-- Include on every data model table (before the audit columns)
data_version VARCHAR(20) NOT NULL DEFAULT 'v1.0',

-- Include on every table written by application/user logic
-- PostgreSQL: use TIMESTAMPTZ. SQLite/MySQL: use DATETIME/TIMESTAMP.
created_by VARCHAR(200),
created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,   -- DATETIME on SQLite
updated_by VARCHAR(200),
updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,   -- DATETIME on SQLite
```

For revocable resources, add:

```sql
revoked_by VARCHAR(200),
revoked_at TIMESTAMPTZ,   -- DATETIME on SQLite
CHECK (revoked_at IS NULL OR status = 'revoked')
```

---

## Reference: Standard Index Template

For any new org-scoped table `<t>`:

```sql
-- FK on org (always)
CREATE INDEX IF NOT EXISTS idx_<t>_org ON <t>(organization_uuid);

-- FK on parent entity (always when present)
CREATE INDEX IF NOT EXISTS idx_<t>_<parent>_uuid ON <t>(<parent>_uuid);

-- Status filter (when table has status column)
CREATE INDEX IF NOT EXISTS idx_<t>_status ON <t>(status);

-- Common compound filter (when query pattern is known)
CREATE INDEX IF NOT EXISTS idx_<t>_org_<field> ON <t>(organization_uuid, <filter_field>);
```

Partial index for nullable filter columns:

```sql
CREATE INDEX IF NOT EXISTS idx_<t>_<col>
  ON <t>(<col>) WHERE <col> IS NOT NULL;
```
