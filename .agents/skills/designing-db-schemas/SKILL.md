---
name: designing-db-schemas
description: Design, change, or review a database schema in the WSO2 API Platform. Use when adding or altering a table, column, index, or constraint in any *.sql schema file, reviewing schema changes before a PR, evaluating a migration plan, or asking whether a table is well designed and what indexes it needs.
allowed-tools: Bash, Read, Edit, Write, Glob, Grep
---

# WSO2 API Platform — Database Schema Design

This skill governs **all schema work** for the WSO2 API Platform: designing new tables, adding to existing ones, and reviewing DDL changes. It is not a post-hoc review tool — it is the process to follow when writing DDL.

The rules live in **`references/api-platform-db-schema-rules.md`** (next to this skill). That file is the source of truth for every rule; this file is the workflow.

## Usage

```text
/designing-db-schemas [table-name | path-to-schema-file]
```

- **No argument** — review all in-scope schema files.
- **Table name** — apply the relevant workflow for that table.
- **Schema file path** — review that file only.

---

## Schema file scope

Discover schema files with:

```bash
find . -name "*.sql" -not -path "*/node_modules/*" -not -path "*/target/*" | sort
```

Do **not** use `find . -name "schema*.sql"` — it matches 10 of the 19 schema files and misses every gateway-controller and event-gateway file.

| GA product | Path | Rules |
|---|---|---|
| Platform API | `platform-api/internal/database/` | R0–R10 |
| Platform API — eventgateway plugin | `platform-api/plugins/eventgateway/schema/` | R0–R10 |
| API Portal | `portals/api-portal/database/` | R0–R10 |
| AI Workspace | packages Platform API's schemas — no source of its own | — |
| Gateway Controller | `gateway/gateway-controller/pkg/storage/` **and** `gateway/gateway-controller/resources/` | R0–R2, R4–R10 (**R3 skipped**) |
| Event Gateway Controller | `event-gateway/gateway-controller/pkg/dbschema/` | R0–R2, R4–R10 (**R3 skipped**) |
| Fixtures | `platform-api/internal/database/init-platform-api-db.sql`, `tests/integration-e2e/init-db.sql` | keep in sync |

**Type exemption** — gateway-controller and event-gateway schemas are owned by separate teams who manage their own type choices. Apply all structural, constraint, audit, index, alignment, and idempotency rules (R0–R2, R4–R10) as normal, but do **not** raise R3 findings (column types, JSONB, BOOLEAN, TIMESTAMPTZ, VARCHAR widths) against those files.

**Two copies** — `gateway/gateway-controller` keeps the same schema under both `pkg/storage/` and `resources/`. Grep repo-wide before editing: `grep -rln "<table_name>" --include="*.sql" .`

---

## Workflows

### Workflow A — Making a schema change

#### Step A0 — Admissibility gate (R0)

These are GA products. Decide whether the change is allowed **before** touching a file.

| Request | Verdict |
|---|---|
| New table | Allowed — full R1–R10 apply |
| New nullable/defaulted column on a shipped table | Allowed |
| New index on a shipped table | Allowed |
| Change a shipped column's type or width | **Blocked** — migration |
| Rename a shipped column or table | **Blocked** — migration |
| Add/drop a PK, or change an FK target or `ON DELETE` | **Blocked** — migration |
| Add `NOT NULL` or `UNIQUE` to an existing column | **Blocked** — revalidates customer data |
| Drop a column | **Blocked** as one step — two-release sequence, needs approval |
| "Fix" a shipped table that violates R1–R10 | **Blocked** — record as `LEGACY-ACCEPTED` (R0-LEGACY-ACCEPTED) |

Confirm the table has actually shipped before applying the freeze — one added earlier in the same unreleased branch is still malleable:

```bash
git log --oneline -1 -S"CREATE TABLE IF NOT EXISTS <table>" -- '*.sql'
```

When blocked, don't silently narrow the task. Say which part is blocked and why, deliver the allowed remainder, and offer the migration-plan route as separate, approved work.

#### Step A1 — Read the schemas first

Locate every schema file with the glob above and read each in full before drafting anything. Apply R3 type rules to all files except gateway-controller and event-gateway.

#### Step A2 — Open the rules reference

Read `references/api-platform-db-schema-rules.md`. The rules you need depend on the change:

| Change type | Rules to apply |
|---|---|
| New table | R0 (admissibility), R1 (identity), R2 (org-scoping), R3 (types), R4 (constraints), R5 (audit), R6 (indexes), R8 (all dialects), R9 (idempotent DDL), R10 (naming) |
| New column | R0, R3 (type), R4 (constraints), R5 (audit), R6 (index if filterable), R7 (Go layer sync), R8, R9, R10 |
| New index | R0, R6 (correct pattern — FK, status, compound, partial), R8, R9, R10 |
| Type change on a shipped column | **R0 — blocked.** Stop; this is a migration, not a schema edit |

#### Step A3 — Self-review checklist

```text
[ ] R0  Change is additive — no retype/rename/PK-FK change/NOT NULL/UNIQUE on a shipped table
[ ] R0  Column added to an existing table: per-dialect ALTER TABLE written for already-provisioned
        databases (new tables/indexes need none — their guarded CREATE covers both cases)
[ ] R1  Entity tables: uuid VARCHAR(40) PRIMARY KEY
[ ] R1  Junction/mapping tables: composite PRIMARY KEY — not UNIQUE-only, not surrogate UUID
[ ] R1  Non-leading FK columns of a composite PK have their own indexes
[ ] R1  Named resource tables carry handle + name + version (all NOT NULL)
[ ] R1  handle VARCHAR(40) slug ≠ name VARCHAR(255) display string; no UNIQUE on name
[ ] R2  organization_uuid FK present; UNIQUE constraints include it (if org-scoped)
[ ] R3  No bare TEXT in Postgres — SQLite TEXT / SQL Server NVARCHAR(MAX) are intentional (R8)
[ ] R3  Large/variable payloads use BYTEA/BLOB/VARBINARY(MAX) — not wide VARCHAR
[ ] R3  JSONB only when a repository query reads inside it with JSON operators; scan target checked
        against the driver in use (pgx v5 also takes *string/*[]byte — see R3-JSONB-SCAN-COMPAT)
[ ] R3  Boolean flags: SMALLINT (Postgres/SQL Server) or INTEGER (SQLite), 0/1 — no BOOLEAN
[ ] R3  VARCHAR widths match R3-VARCHAR-SIZES; nothing above VARCHAR(1023) for plain storage
[ ] R3  Indexed/UNIQUE columns ≤ VARCHAR(255); hashes are VARCHAR(255)
[ ] R3  Timestamps: TIMESTAMPTZ (Postgres) / DATETIME (SQLite) / DATETIME2(7) (SQL Server), UTC
[ ] R4  No CHECK constraints for enum/status values — validated in the service layer instead
[ ] R4  Required columns NOT NULL, with a DEFAULT where one is sensible
[ ] R4  No plaintext credential/token column; hashed or vault-referenced, named accordingly
[ ] R4  Every FK has an explicit ON DELETE clause
[ ] R5  User-initiated table → all four audit columns; system-managed → created_by/updated_by ABSENT
[ ] R5  Every domain entity table has data_version VARCHAR(20) NOT NULL DEFAULT '1.0'
[ ] R6  FK columns, organization_uuid, and filtered status columns have indexes — except where the
        column is already the leftmost part of the PK or a covering UNIQUE constraint
[ ] R7  Go model/repository/DTO updated in the same commit; named columns, no SELECT *
[ ] R8  Change applied to every dialect file (or divergence is intentional and documented)
[ ] R9  All DDL is idempotent (IF NOT EXISTS / OBJECT_ID / sys.indexes guards)
[ ] R10 All identifiers lowercase snake_case; pure mapping tables have a _mappings suffix
```

#### Step A4 — Write the DDL

Use the engine-specific guards (R9) — `CREATE TABLE IF NOT EXISTS` is not valid T-SQL:

```sql
-- PostgreSQL / SQLite
CREATE TABLE IF NOT EXISTS <table> (...);
CREATE INDEX IF NOT EXISTS idx_... ON <table>(...);

-- SQL Server
IF OBJECT_ID(N'dbo.<table>', N'U') IS NULL
CREATE TABLE dbo.<table> (...);

IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_...' AND object_id = OBJECT_ID(N'dbo.<table>'))
CREATE INDEX idx_... ON dbo.<table>(...);
```

Keep `CREATE INDEX` statements in a dedicated block after all `CREATE TABLE` statements. Start from the quick-reference templates at the end of the rules file.

#### Step A5 — Apply to all schema files, then ship the upgrade path

Apply to every in-scope dialect file, same column order, same position. Only the R8 divergences may differ.

Then, **if the change adds a column to an existing table**, write the per-dialect `ALTER TABLE` (R0-UPGRADE-PATH) — `CREATE TABLE IF NOT EXISTS` does nothing to a database that already exists, so a column added to a `CREATE TABLE` body reaches fresh installs only. A new table or a new index needs no `ALTER`: its guarded `CREATE ... IF NOT EXISTS` / `OBJECT_ID` / `sys.indexes` form (R9, Step A4) already applies to fresh and already-provisioned databases alike.

#### Step A6 — Verify on more than one dialect

```bash
cd <component> && go build ./... && go test ./internal/repository/... ./internal/database/...
```

Then: start from an empty **SQLite** database; start from an empty **server dialect** (Postgres or SQL Server); and apply the `ALTER` to a **pre-existing** database from the previous release. Report which dialects were actually exercised and which were only edited.

---

### Workflow B — Reviewing existing DDL (PR / audit)

#### Step B1 — Locate and read all schema files

Use the glob above. Note which files are gateway-controller or event-gateway — those skip R3.

#### Step B2 — Open the rules reference

Read `references/api-platform-db-schema-rules.md`. Evaluate every rule group (R0–R10) in order.

#### Step B3 — Record findings

| Field | Value |
|---|---|
| **Rule** | e.g. `R3-NO-TEXT` |
| **Table · column** | exact location |
| **Severity** | `HIGH` (data safety / correctness) · `MEDIUM` (missing guarantee or index) · `LOW` (style) · `LEGACY-ACCEPTED` (shipped table, frozen by R0) |
| **Finding** | what is wrong |
| **Fix** | the exact DDL needed — **omit for `LEGACY-ACCEPTED`** |

Two adjustments this repo requires:

- **A shipped table's violation is `LEGACY-ACCEPTED`, not `HIGH`.** Record it in the deviations table of `.claude/rules/db-schema-changes.md` so later reviews stop re-reporting it. Do not propose remediation DDL.
- **Blanket-missing findings collapse.** When a rule is violated uniformly across many tables, record one representative finding naming the pattern with a couple of examples — not one per table.

Findings on new tables/columns in the diff are live and use the normal severities.

#### Step B4 — Cross-check multi-engine alignment

Verify all dialect files are structurally in sync (R8). Intentional type-level divergences are not findings. To compare one table across dialects:

```bash
for f in platform-api/internal/database/schema*.sql; do
  echo "--- $f"
  awk '/CREATE TABLE.*<table>/,/\);/' "$f" | grep -oE '^\s+[a-z_]+' | tr -d ' '
done
```

#### Step B5 — Write findings to JSON

Run from this skill's directory, with an absolute `--out` so the report lands in the project:

```bash
node scripts/generate-schema-report.js \
  --findings '<findings-json-array>' \
  --schema   '<path-to-schema-file>' \
  --out      "$(git rev-parse --show-toplevel)/schema-reports/schema-review.json"
```

Output shape:

```json
{
  "meta": { "schema": "<path>", "reviewedAt": "<ISO-8601>", "rules": ["R0","R1","R2","R3","R4","R5","R6","R7","R8","R9","R10"] },
  "summary": { "HIGH": 0, "MEDIUM": 0, "LOW": 0, "LEGACY-ACCEPTED": 0 },
  "findings": [
    { "id": "r3-001", "severity": "HIGH", "rule": "R3-NO-TEXT", "table": "<table>", "column": "<col>", "finding": "...", "fix": "..." }
  ]
}
```

#### Step B6 — Report summary

Produce a findings table sorted by severity. Include a "No issues" row for any rule group that passed cleanly — reviewers need to know what was checked.

---

## Quick-reference templates

New entity table (Postgres and SQL Server), the standard column-type/width cheat sheet, and the junction/mapping table pattern (under **R1-COMPOSITE-PK**) all live in **`references/api-platform-db-schema-rules.md`**. Use them as the starting point in Step A4.
