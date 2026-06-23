---
name: db-schema-review
description: |
  The authoritative skill for ALL schema work on platform-api. Use proactively whenever:
  - Adding a new table to schema.postgres.sql or schema.sql or schema.sqlite.sql
  - Modifying an existing table (new column, type change, constraint, index)
  - Reviewing schema changes before a PR
  - Writing or evaluating a migration plan
  - Asking "is this table well designed?" or "what indexes does this table need?"

  Applies the api-platform house conventions: UUID primary keys, handle/name/version identity triple, org-scoping, JSONB for JSON columns (Postgres), audit columns (created_by/at, updated_by/at), CHECK constraints on status fields, and the standard indexing patterns. Both schema files (schema.postgres.sql and schema.sqlite.sql) must be kept in sync — every change to one must be reflected in the other unless it is an intentional divergence (JSONB/TEXT, BYTEA/BLOB, TIMESTAMPTZ/TIMESTAMP).

  IMPORTANT: This skill must be applied to every schema change, not only when explicitly invoked. If a task involves touching either schema file, run the relevant workflow steps before writing or proposing DDL.
allowed-tools: Bash, Read, Edit, Write, Grep, Glob
---

# Database Schema — Design, Change & Review

Covers the `platform-api` PostgreSQL and SQLite schemas:
- `internal/database/schema.postgres.sql`
- `internal/database/schema.sql`
- `internal/database/schema.sqlite.sql`

This skill governs **all schema work** — designing new tables, modifying existing ones, and reviewing changes for correctness. It is not a post-hoc review tool; it is the process to follow when writing DDL.

## Usage

```
/db-schema-review [new-table-name | existing-table-name | path-to-schema-file]
```

- **No argument** — review both schema files in full.
- **Table name** — apply the relevant workflow for that table (add or modify).
- **Schema file path** — review that file only.

---

## Workflows

There are two entry points. Choose based on whether you are making a change or reviewing existing DDL.

---

### Workflow A — Making a Schema Change

Follow this workflow for every ADD TABLE, ADD COLUMN, ALTER COLUMN, ADD INDEX, or ADD CONSTRAINT.

#### A1 · Read the schemas first

```bash
# Always read both before writing any DDL
cat internal/database/schema.postgres.sql
cat internal/database/schema.sql
cat internal/database/schema.sqlite.sql
```

#### A2 · Draft the DDL against the rules

Work through the relevant rules from the Rules section:
- **New table** → R1 (identity), R2 (org-scoping), R3 (types), R4 (constraints), R5 (audit columns), R6 (indexes)
- **New column** → R3 (type), R4 (constraints), R5 (audit), R6 (index if filterable)
- **Type change** → R3 (correct type for Postgres), R8 (SQLite counterpart)
- **New index** → R6 (correct pattern — FK, status, compound, partial)

Use the quick-reference templates at the bottom of this skill as a starting point.

#### A3 · Apply to BOTH files

Every structural change must be applied to **both** `schema.postgres.sql` and `schema.sqlite.sql` unless it is an intentional divergence (see R8). The only intentional divergences are type-level: JSONB↔TEXT, BYTEA↔BLOB, TIMESTAMPTZ↔TIMESTAMP, SERIAL↔INTEGER, and the three Postgres-only tables.

When applying to `schema.sqlite.sql`:
- Replace `JSONB` with `TEXT`
- Replace `BYTEA` with `BLOB`
- Replace `TIMESTAMPTZ` with `TIMESTAMP`
- Replace `SERIAL` with `INTEGER`
- Replace `'{}'::jsonb` defaults with `'{}'`

#### A4 · Self-review before writing

Before writing the final DDL to disk, check each of the following and confirm they pass:

```
[ ] R1  Every entity table has uuid VARCHAR(40) PRIMARY KEY
[ ] R1  Named resource tables carry handle + name + version (NOT NULL)
[ ] R2  organization_uuid FK present and UNIQUE constraints include it
[ ] R3  JSON columns are JSONB in Postgres, TEXT in SQLite
[ ] R3  VARCHAR widths match the standard table (40/200/255/30/20/1023)
[ ] R4  Every status column has a CHECK constraint
[ ] R4  Every FK has an explicit ON DELETE clause
[ ] R4  Cross-cutting tables FK to artifacts(uuid), not to type-specific tables
[ ] R5  Table is REST API-driven → all four audit columns present (created_by/at, updated_by/at)
[ ] R5  Table is system-managed → created_by and updated_by are ABSENT
[ ] R6  FK columns have indexes
[ ] R6  organization_uuid has an index
[ ] R6  status has an index if the column is used as a filter
[ ] R8  Change applied to both schema files (or divergence is intentional)
```

#### A5 · Write the DDL

Edit both schema files. Keep `CREATE INDEX` statements in the indexes block at the bottom of each file (after all `CREATE TABLE` statements), not inline in the table definition.

---

### Workflow B — Reviewing Existing DDL (PR / Audit)

#### B1 · Locate the schemas

```bash
find . -name "schema.postgres.sql" -o -name "schema.sqlite.sql" | grep internal/database
```

Read both files before making any assessment.

#### B2 · Run all rule groups in order (R1–R8)

For each finding record:
- **Rule** — e.g. `R3-JSONB`
- **Table · column** — exact location
- **Severity** — HIGH (data safety / correctness) · MEDIUM (missing guarantee or index) · LOW (style / inconsistency)
- **Finding** — what is wrong
- **Fix** — the exact DDL needed

#### B3 · Cross-check SQLite ↔ PostgreSQL divergences

After per-table review, verify the two files are structurally in sync (see R8). Intentional divergences do not count as findings.

#### B4 · Report

Produce a findings table sorted by severity. Include a "No issues" row for any rule group that passed cleanly.

---

## Workflow

### Step 0 — Locate the schemas

```bash
find . -name "schema.postgres.sql" -o -name "schema.sqlite.sql" | grep internal/database
```

### Step 1 — Run all rule groups in order

Work through Rules 1–8 below. For each finding, record:

- **Rule** — which rule it violates (e.g. `R3-JSONB`)
- **Table · column** — exact location
- **Severity** — HIGH (data safety / correctness) · MEDIUM (missing guarantee or index) · LOW (style / inconsistency)
- **Finding** — what is wrong
- **Fix** — the exact DDL or change needed

### Step 2 — Cross-check SQLite ↔ PostgreSQL divergences

After per-table review, check that the two files are structurally in sync (see Rule 8). Intentional divergences (JSONB/TEXT, BYTEA/BLOB, TIMESTAMPTZ/TIMESTAMP, SERIAL/INTEGER, Postgres-only tables) do not count as findings. Everything else does.

### Step 3 — Report

Produce a findings table sorted by severity. Include a "No issues" row for any rule group that passed cleanly — reviewers need to know what was checked.

---

## Rules

These rules apply in both Workflow A (making changes) and Workflow B (reviewing). In Workflow A, use them as design constraints before writing DDL. In Workflow B, use them as a checklist to detect violations.

### R1 · Primary Key & Identity

Every table must have a single UUID primary key and, where it is an artifact or named resource, the standard identity triple.

**R1-UUID** — Primary key must be `uuid VARCHAR(40) PRIMARY KEY`. Do not use `SERIAL`, `BIGINT`, or `INTEGER` as a primary key for domain entities. (Junction/mapping tables may use a composite PK.)

**R1-IDENTITY** — Tables representing named resources (APIs, gateways, providers, proxies, applications, subscriptions) must carry the full identity triple directly:

```sql
handle  VARCHAR(255) NOT NULL,   -- url-safe slug, immutable once set
name    VARCHAR(255) NOT NULL,   -- human-readable display name
version VARCHAR(30)  NOT NULL,   -- semver or opaque version string
```

Identity must NOT live in a parent `artifacts` row and be joined in. It must be denormalised onto the type table so queries against a single table are self-contained.

**R1-HANDLE-NAME** — `handle` and `name` are distinct:
- `handle` is the URL-safe slug used in API paths (`/gateways/{handle}`). It must be unique within its org scope and should be treated as immutable after creation.
- `name` is the human-readable display string. It may change.

Conflating them (using `name` as the slug, or allowing `handle` to contain spaces) is a finding.

---

### R2 · Organisation Scoping

**R2-ORG-FK** — Every domain table that is not globally shared must carry `organization_uuid VARCHAR(40) NOT NULL` with `FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE`.

**R2-ORG-UNIQUE** — Uniqueness constraints on named resources must include `organization_uuid`:

```sql
UNIQUE(organization_uuid, handle)   -- not UNIQUE(handle) alone
UNIQUE(organization_uuid, name)
```

A `UNIQUE(name)` without the org scope is a critical data-isolation bug.

**R2-ARTIFACTS-COMPOSITE** — In Postgres only, the `artifacts` table requires `UNIQUE(uuid, organization_uuid)` to support the composite FK from `subscriptions`:

```sql
FOREIGN KEY (artifact_uuid, organization_uuid)
  REFERENCES artifacts(uuid, organization_uuid)
```

This constraint is intentionally absent from the SQLite schema (FK enforcement is opt-in via PRAGMA).

---

### R3 · Column Types

**R3-JSONB** — In PostgreSQL, any column that stores a JSON object or array must use `JSONB`, not `TEXT`. Evidence that a column is JSON-valued:
1. The column has a comment `-- JSON ... as TEXT`
2. The `DEFAULT` is a JSON literal like `'{}'`
3. The same column in a sibling table already uses `JSONB`
4. The column name strongly implies structure: `configuration`, `metadata`, `properties`, `manifest`, `event_data`, `openapi_spec`, `*_hashes`

Known promoted columns (already `JSONB` in `schema.postgres.sql`):

| table | column |
|---|---|
| `rest_apis` | `configuration` |
| `llm_providers` | `configuration`, `openapi_spec` |
| `llm_proxies` | `configuration`, `openapi_spec` |
| `mcp_proxies` | `configuration` |
| `gateways` | `properties`, `manifest` |
| `deployments` | `metadata` |
| `llm_provider_templates` | `configuration` |
| `api_keys` | `api_key_hashes` |
| `events` | `event_data` |

SQLite equivalents are `TEXT` — that is intentional and is not a finding.

**R3-JSONB-SCAN-COMPAT** — Do not promote a column to `JSONB` if the Go repository layer scans it into a plain `string` variable (e.g. `var transportJSON string`) and calls `json.Unmarshal` manually. The `pgx`/`lib/pq` drivers return JSONB as binary, which breaks `string` scan targets at runtime. Only use `JSONB` when the scan target is `pgtype.JSONB`, `json.RawMessage`, or a struct implementing `sql.Scanner`. Example of a column that is intentionally `TEXT` despite holding JSON: `rest_apis.transport` (stores a marshalled `[]string`; Go code scans into `string`).

**R3-BINARY** — Binary data is `BYTEA` (Postgres) / `BLOB` (SQLite). Do not store binary as `TEXT` or `VARCHAR`.

**R3-TIMESTAMPTZ** — Use `TIMESTAMPTZ` (not `TIMESTAMP`) for columns in HA sync / event-log tables (`gateway_states.updated_at`, all timestamps in `events`). Use `TIMESTAMP` for all other audit/lifecycle columns.

**R3-VARCHAR-SIZES** — Standard widths:

| purpose | width |
|---|---|
| UUID / foreign key | `VARCHAR(40)` |
| User identity (email / sub) | `VARCHAR(200)` |
| Handle / name / display | `VARCHAR(255)` |
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

**R4-ARTIFACT-FK** — Any table that references a typed artifact (REST API, LLM proxy, WebSub API, etc.) must FK to `artifacts(uuid)`, not to the type-specific table (e.g. `rest_apis`). The `artifacts` table is the stable identity anchor; type tables are implementation details that may not cover all artifact kinds. Use `artifact_uuid VARCHAR(40) NOT NULL` as the column name and reference `artifacts(uuid) ON DELETE CASCADE`. Never write `REFERENCES rest_apis(uuid)` or `REFERENCES llm_proxies(uuid)` from a cross-cutting table.

**R4-COMPOSITE-FK-PAIR** — When a composite FK is used (e.g. `(artifact_uuid, organization_uuid) → artifacts(uuid, organization_uuid)`), both columns must have individual FK declarations AND the composite FK. The Postgres schema must also carry the `UNIQUE(uuid, organization_uuid)` on the target table to make the composite FK resolvable.

**R4-CONSTRAINT-PAIR** — Paired columns (e.g. throttle_limit_count / throttle_limit_unit) that must both be NULL or both non-NULL must use a named `CONSTRAINT`:

```sql
CONSTRAINT chk_plan_throttle_pair CHECK (
  (throttle_limit_count IS NULL AND throttle_limit_unit IS NULL) OR
  (throttle_limit_count IS NOT NULL AND throttle_limit_unit IS NOT NULL)
)
```

---

### R5 · Audit Columns

**R5-AUDIT-SET** — Only tables that are written as a **direct consequence of a REST API call** carry `created_by` / `updated_by`. Tables written by background sync, gateway callbacks, or internal system processes must NOT include these columns — there is no user principal to record, so the columns would always be null and are misleading.

Rule of thumb: ask "does a human-initiated HTTP request cause this row to be inserted or updated?" If yes → include the full audit set. If no → omit `created_by` and `updated_by`.

Tables in this schema that are **system-managed** (no `created_by`/`updated_by`):
- `gateway_custom_policies` — synced from gateway manifests
- `gateway_custom_policy_usages` — populated by the gateway sync process
- `deployment_status` — written by gateway callbacks reporting deployment state
- `artifacts` — parent record created internally when an API row is inserted
- `gateway_states`, `events` — EventHub HA sync tables

Tables in this schema that are **REST API-driven** (full audit set):
- `organizations`, `projects`, `applications`
- `rest_apis`, `websub_apis`, `webbroker_apis`, `llm_providers`, `llm_proxies`, `mcp_proxies`
- `llm_provider_templates`, `subscription_plans`, `subscriptions`
- `gateways`, `gateway_tokens`, `deployments`
- `api_keys`, `application_api_keys`, `application_artifacts`

For REST API-driven tables, include all four audit columns:

```sql
created_by VARCHAR(200),
created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
updated_by VARCHAR(200),
updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
```

`created_by` / `updated_by` are `VARCHAR(200)` to hold a user's email or subject identifier. They are nullable because seed data and migrations may not have a principal.

**R5-IMMUTABLE-CREATED** — `created_by` and `created_at` must never be updated after insert. Application layer must not include them in `UPDATE` statements. Verify that the SQL query layer (sqlc / raw SQL) does not overwrite them.

**R5-REVOKE-PATTERN** — Tables that have soft-revocation (e.g. `gateway_tokens`) follow a separate pair:

```sql
revoked_by VARCHAR(200),
revoked_at TIMESTAMP,
CHECK (revoked_at IS NULL OR status = 'revoked')
```

The CHECK constraint ties the timestamp to the status value — both must agree.

**R5-IMMUTABLE-JUNCTION** — Pure junction tables (e.g. `application_api_keys`, `application_artifacts`) carry audit columns but are effectively append-only. Rows should be inserted or deleted, never updated in place.

---

### R6 · Indexing

Index every column (or compound) that appears in a `WHERE`, `JOIN`, or `ORDER BY` clause at production query volume. Below are the required coverage patterns.

**R6-FK-INDEX** — Every foreign key column must have an index unless it is already the leftmost column of the PK or a covering UNIQUE constraint.

```sql
-- required for every FK that is not a PK
CREATE INDEX IF NOT EXISTS idx_<table>_<fk_col> ON <table>(<fk_col>);
```

**R6-ORG-INDEX** — Every org-scoped table must have an index on `organization_uuid` alone:

```sql
CREATE INDEX IF NOT EXISTS idx_<table>_org ON <table>(organization_uuid);
```

**R6-STATUS-INDEX** — Tables with a `status` column that is filtered in application list queries need a status index:

```sql
CREATE INDEX IF NOT EXISTS idx_<table>_status ON <table>(status);
```

**R6-COMPOUND-INDEX** — When the common query is `WHERE a = ? AND b = ?`, a single compound index `(a, b)` outperforms two single-column indexes. Column order: most-selective filter first, then the join/filter column second.

```sql
-- org-then-status lookup pattern
CREATE INDEX IF NOT EXISTS idx_subscriptions_org_subscriber
  ON subscriptions(organization_uuid, subscriber_id);
```

**R6-PARTIAL-INDEX** — Use a partial index when the filter discards most rows. Classic example:

```sql
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at
  ON api_keys(expires_at) WHERE expires_at IS NOT NULL;
```

A partial index on a nullable column with `WHERE col IS NOT NULL` is dramatically smaller than a full index and is the correct pattern for expiry-sweep queries.

**R6-UNIQUE-PARTIAL** — For "at most one default per org" patterns, use a partial unique index rather than an application-level guard:

```sql
CREATE UNIQUE INDEX IF NOT EXISTS idx_devportals_default_per_org
  ON devportals(organization_uuid) WHERE is_default = TRUE;
```

**R6-NO-REDUNDANT-INDEX** — Do not create an index that is a prefix of an existing UNIQUE constraint or PK. Flag and remove it.

**R6-GIN-JSONB** — If application code queries inside a JSONB column (e.g. `WHERE event_data @> '{"entity_id": "..."}'`), add a GIN index:

```sql
CREATE INDEX IF NOT EXISTS idx_events_event_data_gin
  ON events USING GIN (event_data);
```

Only add this when there is a concrete query that uses JSONB operators — do not pre-emptively GIN-index every JSONB column.

---

### R7 · Application Logic Safety

These rules govern what the schema design implies for the application layer (Go code, sqlc queries).

**R7-NO-SELECT-STAR** — Schema changes (added columns, type promotions) break `SELECT *` callers. All queries must select named columns. Confirm sqlc or raw SQL queries enumerate columns explicitly.

**R7-JSONB-SCAN** — In Go, a `JSONB` column must be scanned into a type that implements `sql.Scanner` (e.g. `pgtype.JSONB`, `json.RawMessage`, or a custom struct). Scanning into `string` will silently receive the JSON text but bypasses Postgres validation. Verify the corresponding Go struct field type when promoting TEXT → JSONB.

**R7-DEFAULT-CAST** — When a JSONB column has `DEFAULT '{}'`, the application must supply a cast on insert if the driver does not:

```sql
-- insert must use ::jsonb cast or a typed parameter
INSERT INTO api_keys (..., api_key_hashes) VALUES (..., $1::jsonb)
```

Without the cast, some drivers pass the literal string `'{}'` and Postgres rejects it.

**R7-STATUS-ENUM** — Application code must not hard-code status strings without referencing the CHECK constraint values. Define a Go `const` block that mirrors each `CHECK(status IN (...))` — mismatches become compile-time errors rather than runtime panics.

**R7-HANDLE-IMMUTABLE** — `handle` must be set on INSERT and never modified by UPDATE. The application layer must enforce this: do not include `handle` in the `SET` clause of any `UPDATE` statement.

**R7-CREATED-IMMUTABLE** — `created_at` and `created_by` must not appear in `UPDATE` statements. If using sqlc, verify the generated update queries omit them.

**R7-SOFT-DELETE** — This schema does not use soft-delete (`deleted_at`). Status transitions to terminal states (`REVOKED`, `RETIRED`, `ARCHIVED`) serve the same purpose. Do not add `deleted_at` columns unless the design explicitly requires row-level tombstones.

**R7-OPTIMISTIC-LOCK** — For resources that are concurrently edited (e.g. gateway configurations, API configurations), application-level optimistic locking should compare `updated_at` before issuing an UPDATE:

```sql
UPDATE gateways
SET ..., updated_at = NOW(), updated_by = $n
WHERE uuid = $1 AND updated_at = $expected_updated_at
```

If 0 rows are affected, the caller lost the race and must retry. The schema supports this pattern today — no additional columns needed.

---

### R8 · SQLite ↔ PostgreSQL Alignment

The two schema files must remain structurally in sync except for the known intentional divergences listed below. Any structural difference not in this list is a finding.

**Intentional divergences — not findings:**

| Column / feature | SQLite (`schema.sqlite.sql`) | PostgreSQL (`schema.postgres.sql`) |
|---|---|---|
| JSON-valued columns | `TEXT` | `JSONB` |
| Binary columns | `BLOB` | `BYTEA` |
| HA sync timestamps | `TIMESTAMP` | `TIMESTAMPTZ` |
| Other timestamps | `DATETIME` | `TIMESTAMP` |
| Auto-increment keys | `INTEGER` | `SERIAL` |
| Postgres-only tables | absent | `devportals`, `publication_mappings`, `association_mappings` |
| `artifacts` uniqueness | no `UNIQUE(uuid, org)` | has `UNIQUE(uuid, organization_uuid)` |

**R8-SYNC-STRUCTURE** — Table definitions (columns, constraints, CHECK values, FK targets) that are not in the intentional-divergence list must be identical in both files. Check:
- Same columns, same order
- Same `NOT NULL` / nullable on each column
- Same `DEFAULT` values (modulo type syntax: `'{}'` vs `'{}'::jsonb`)
- Same `CHECK` constraint values
- Same index definitions (excluding Postgres-only indexes for Postgres-only tables)

**R8-SQLITE-NO-JSONB** — SQLite does not support `JSONB`. Any `JSONB` that appears in `schema.sqlite.sql` is a bug. All JSON columns in the SQLite schema must be `TEXT`.

---

## Reference: Column Width Quick-Reference

```
VARCHAR(40)   — uuid, all FK columns (organization_uuid, project_uuid, gateway_uuid, …)
VARCHAR(30)   — version
VARCHAR(20)   — status, lifecycle_status, kind, kind_version, throttle_limit_unit
VARCHAR(63)   — short names (api_keys.name)
VARCHAR(64)   — gateway.version (extended), token_hash (short)
VARCHAR(200)  — created_by, updated_by, revoked_by (user email/sub)
VARCHAR(255)  — handle, name, vhost, billing_plan, all display strings
VARCHAR(512)  — subscription_token (encrypted value)
VARCHAR(64)   — subscription_token_hash (SHA-256 hex)
VARCHAR(1023) — description
TEXT          — openapi_spec (SQLite), policy_definition, free-form long text
JSONB / TEXT  — configuration, properties, manifest, metadata, event_data, api_key_hashes
BYTEA / BLOB  — deployments.content
TIMESTAMP     — created_at, updated_at, revoked_at, expires_at, expiry_time
TIMESTAMPTZ   — gateway_states.updated_at, events.processed_timestamp, events.originated_timestamp
BOOLEAN       — is_active, is_critical, is_default, is_enabled, stop_on_quota_reach
INTEGER       — throttle_limit_count
SERIAL        — association_mappings.id (Postgres auto-increment)
```

---

## Reference: Audit Column Template

```sql
-- Include on every table written by application logic
created_by VARCHAR(200),
created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
updated_by VARCHAR(200),
updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
```

For revocable tokens, add:

```sql
revoked_by VARCHAR(200),
revoked_at TIMESTAMP,
CHECK (revoked_at IS NULL OR status = 'revoked')
```

---

## Reference: Standard Index Template

For any new table `<t>` with org-scoping, add at minimum:

```sql
-- FK on org (always)
CREATE INDEX IF NOT EXISTS idx_<t>_org ON <t>(organization_uuid);

-- FK on parent entity (always when present)
CREATE INDEX IF NOT EXISTS idx_<t>_<parent>_id ON <t>(<parent>_uuid);

-- Status filter (when table has status column)
CREATE INDEX IF NOT EXISTS idx_<t>_status ON <t>(status);

-- Common compound filter (add when query pattern is known)
CREATE INDEX IF NOT EXISTS idx_<t>_org_<field> ON <t>(organization_uuid, <filter_field>);
```

Partial index when nullable filter column:

```sql
CREATE INDEX IF NOT EXISTS idx_<t>_<col>
  ON <t>(<col>) WHERE <col> IS NOT NULL;
```
