---
name: db-schema-review
description: |
  Design, change, and review relational database schemas. Use proactively whenever:
  - Adding a new table
  - Modifying an existing table (new column, type change, constraint, index)
  - Reviewing schema changes before a PR
  - Writing or evaluating a migration plan
  - Asking "is this table well designed?" or "what indexes does this table need?"

  Applies house conventions: UUID primary keys (VARCHAR(40)), handle/name/version identity triple for named resources, org-scoping, JSONB for JSON columns (Postgres), audit columns (created_by/at, updated_by/at), CHECK constraints on status fields, and standard indexing patterns.

  When the project uses both a Postgres and a SQLite schema, both files must be kept in sync — every change to one must be reflected in the other unless it is an intentional type-level divergence (JSONB/TEXT, BYTEA/BLOB, TIMESTAMPTZ/TIMESTAMP).
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
[ ] R1  Named resource tables carry handle + name + version (NOT NULL)
[ ] R2  organization_uuid FK present and UNIQUE constraints include it (if org-scoped)
[ ] R3  JSON columns use the engine-appropriate type (JSONB / TEXT)
[ ] R3  VARCHAR widths match the standard table (40/200/255/30/20/1023)
[ ] R4  Every status column has a CHECK constraint
[ ] R4  Every FK has an explicit ON DELETE clause
[ ] R5  User-initiated table → all four audit columns present (created_by/at, updated_by/at)
[ ] R5  System-managed table → created_by and updated_by are ABSENT
[ ] R6  FK columns have indexes
[ ] R6  organization_uuid has an index (if org-scoped)
[ ] R6  status has an index if the column is used as a filter
[ ] R8  Change applied to all schema files (or divergence is intentional)
```

#### A5 · Write the DDL

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

**R1-UUID** — Primary key must be `uuid VARCHAR(40) PRIMARY KEY`. Do not use `SERIAL`, `BIGINT`, or `INTEGER` as a primary key for domain entities. Junction/mapping tables may use a composite PK.

**R1-IDENTITY** — Tables representing named resources (APIs, gateways, providers, applications, subscriptions, or any domain entity with a stable slug and a display name) must carry the full identity triple directly:

```sql
handle  VARCHAR(255) NOT NULL,   -- url-safe slug, immutable once set
name    VARCHAR(255) NOT NULL,   -- human-readable display name
version VARCHAR(30)  NOT NULL,   -- semver or opaque version string
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

**R3-JSONB** — In PostgreSQL, any column that stores a JSON object or array must use `JSONB`, not `TEXT`. Evidence that a column is JSON-valued:
1. The `DEFAULT` is a JSON literal like `'{}'`
2. The column name implies structure: `configuration`, `metadata`, `properties`, `manifest`, `settings`, `event_data`
3. The same column in a sibling table already uses `JSONB`

SQLite equivalents should be `TEXT` — that is intentional and is not a finding.

**R3-JSONB-SCAN-COMPAT** — Do not use `JSONB` if the application layer scans it into a plain `string` variable and calls `json.Unmarshal` manually. Postgres drivers return JSONB as binary, which breaks `string` scan targets at runtime. Only use `JSONB` when the scan target is a type implementing `sql.Scanner` (e.g. `pgtype.JSONB`, `json.RawMessage`, or a custom struct).

**R3-BINARY** — Binary data is `BYTEA` (Postgres) / `BLOB` (SQLite). Do not store binary as `TEXT` or `VARCHAR`.

**R3-TIMESTAMPTZ** — Use `TIMESTAMPTZ` for columns in distributed-sync or event-log tables where timezone precision matters. Use `TIMESTAMP` for standard audit/lifecycle columns.

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

Arbitrary widths (`VARCHAR(100)`, `VARCHAR(500)`) should be justified or standardised.

---

### R4 · Constraints

**R4-STATUS-CHECK** — Every `status` column must have a `CHECK` constraint enumerating valid values:

```sql
status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
CHECK (status IN ('ACTIVE', 'INACTIVE'))
```

A `status` column without a `CHECK` is a HIGH severity finding — it lets invalid state enter the database.

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
  ON <table>(organization_uuid) WHERE is_default = TRUE;
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

When the project maintains schema files for multiple database engines (e.g. Postgres + SQLite), the files must remain structurally in sync except for the known intentional type-level divergences listed below.

**Intentional divergences — not findings:**

| Feature | SQLite | PostgreSQL |
|---|---|---|
| JSON-valued columns | `TEXT` | `JSONB` |
| Binary columns | `BLOB` | `BYTEA` |
| Timezone-aware timestamps | `TIMESTAMP` | `TIMESTAMPTZ` |
| Standard timestamps | `DATETIME` | `TIMESTAMP` |
| Auto-increment keys | `INTEGER` | `SERIAL` |
| JSON literal defaults | `'{}'` | `'{}'::jsonb` |

**R8-SYNC-STRUCTURE** — Table definitions (columns, order, constraints, CHECK values, FK targets) that are not in the intentional-divergence list must be identical across all schema files. Check:
- Same columns, same order
- Same `NOT NULL` / nullable on each column
- Same `DEFAULT` values (modulo type syntax)
- Same `CHECK` constraint values
- Same index definitions

**R8-SQLITE-NO-JSONB** — SQLite does not support `JSONB`. Any `JSONB` appearing in a SQLite schema file is a bug — all JSON columns must be `TEXT`.

---

## Reference: Standard VARCHAR Widths

```
VARCHAR(40)   — uuid, all FK columns referencing UUIDs
VARCHAR(30)   — version strings
VARCHAR(20)   — status, lifecycle_status, kind, short enums
VARCHAR(200)  — created_by, updated_by, revoked_by (user email / subject)
VARCHAR(255)  — handle, name, and other display strings
VARCHAR(512)  — tokens (encrypted values)
VARCHAR(64)   — hashes (SHA-256 hex)
VARCHAR(1023) — description
TEXT          — long free-form text, openapi specs, policy definitions
JSONB / TEXT  — configuration, properties, manifest, metadata, event_data
BYTEA / BLOB  — binary content
TIMESTAMP     — created_at, updated_at, revoked_at, expires_at
TIMESTAMPTZ   — sync/event-log timestamps requiring timezone precision
BOOLEAN       — is_active, is_default, is_enabled
INTEGER       — numeric counters and limits
```

---

## Reference: Audit Column Template

```sql
-- Include on every table written by application/user logic
created_by VARCHAR(200),
created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
updated_by VARCHAR(200),
updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
```

For revocable resources, add:

```sql
revoked_by VARCHAR(200),
revoked_at TIMESTAMP,
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
