---
name: designing-db-schemas
description: |
  WSO2 API Platform-specific database schema design, change, and review skill. Use proactively whenever:
  - Adding a new table to platform-api or developer-portal schemas
  - Modifying an existing table (new column, type change, constraint, index)
  - Reviewing schema changes before a PR
  - Writing or evaluating a migration plan
  - Asking "is this table well designed?" or "what indexes does this table need?"

  Scope: applies to every schema.*.sql file in the repository. Gateway controller schemas (gateway/gateway-controller/) are included for structural rules (R1–R2, R4–R10) but R3 type validation is skipped for them — the gateway controller team owns their type choices.
allowed-tools: Bash, Read, Edit, Write, Glob
---

# WSO2 API Platform — Database Schema Design Rules

This skill governs **all schema work** for the WSO2 API Platform: designing new tables, modifying existing ones, and reviewing DDL changes for correctness. It is not a post-hoc review tool — it is the process to follow when writing DDL.

The detailed rules live in **`references/api-platform-db-schema-rules.md`** (next to this skill). This file describes the workflow; the reference file is the source of truth for every rule.

## Usage

```
/api-platform-db-schema-design-rules [table-name | path-to-schema-file]
```

- **No argument** — review all in-scope schema files in the project.
- **Table name** — apply the relevant workflow for that table (add or modify).
- **Schema file path** — review that specific file only.

---

## Schema File Scope

Locate all schema files with:

```bash
find . -name "schema*.sql" | sort
```

All `schema*.sql` files in the repository are in scope. The rules that apply depend on which component owns the file:

| Component | Path pattern | Rules applied |
|---|---|---|
| Platform API | `platform-api/` | R1–R10 (all rules) |
| Developer Portal | `portals/` | R1–R10 (all rules) |
| Gateway Controller | `gateway/gateway-controller/` | R1–R2, R4–R10 — **R3 type rules skipped** |
| Any other component | elsewhere | R1–R10 (all rules) |

**Gateway controller type exemption** — `gateway/gateway-controller/` schemas are owned by a separate team who manage their own type choices. Apply all structural, constraint, audit, index, alignment, and idempotency rules (R1–R2, R4–R10) as normal, but do **not** raise R3 findings (column types, JSONB, BOOLEAN, TIMESTAMPTZ, VARCHAR widths) against those files.

---

## Workflows

Choose a workflow based on whether you are making a change or reviewing existing DDL.

---

### Workflow A — Making a Schema Change

Use this workflow for every ADD TABLE, ADD COLUMN, ALTER COLUMN, ADD INDEX, or ADD CONSTRAINT.

#### Step A1 — Read the schemas first

Locate and read all schema files before writing any DDL:

```bash
find . -name "schema*.sql" | sort
```

Read the full content of each file before drafting anything. Apply R3 type rules to all files **except** those under `gateway/gateway-controller/`.

#### Step A2 — Open the rules reference

Read `references/api-platform-db-schema-rules.md` in full. The rules you need depend on the change:

| Change type | Rules to apply |
|---|---|
| New table | R1 (identity), R2 (org-scoping), R3 (types), R4 (constraints), R5 (audit columns), R6 (indexes), R10 (naming) |
| New column | R3 (type), R4 (constraints), R5 (audit), R6 (index if filterable), R10 (lowercase name) |
| Type change | R3 (correct type for target engine), R8 (counterpart schemas if multi-engine) |
| New index | R6 (correct pattern — FK, status, compound, partial), R10 (lowercase index name) |

Use the quick-reference templates in the rules file as your starting point.

#### Step A3 — Self-review checklist

Before writing DDL to disk, confirm each item passes:

```
[ ] R1  Entity tables: uuid VARCHAR(40) PRIMARY KEY
[ ] R1  Junction/mapping tables: composite PRIMARY KEY — not UNIQUE-only, not surrogate UUID
[ ] R1  Non-leading FK columns of a composite PK have their own indexes
[ ] R1  Named resource tables carry handle + name + version (all NOT NULL)
[ ] R2  organization_uuid FK present; UNIQUE constraints include it (if org-scoped)
[ ] R3  No bare TEXT in Postgres: use VARCHAR(N), BYTEA, or JSONB (query-only) — SQLite TEXT / SQL Server NVARCHAR(MAX) are intentional (R8), not findings
[ ] R3  Large/variable payloads use BYTEA/BLOB — not wide VARCHAR
[ ] R3  Opaque JSON stored as BYTEA/BLOB — JSONB only when queried with JSON operators inside Postgres
[ ] R3  Boolean flags: SMALLINT (Postgres) or INTEGER (SQLite/SQL Server), DEFAULT 1/0 — no BOOLEAN
[ ] R3  VARCHAR widths match the standard table; nothing above VARCHAR(1023) for plain storage
[ ] R3  Indexed/UNIQUE columns ≤ VARCHAR(255) (safe across all engines with utf8mb4)
[ ] R3  All timestamps: TIMESTAMPTZ (Postgres) / DATETIME (SQLite) / DATETIME2 (SQL Server)
[ ] R4  No CHECK constraints for enum/status values — validation in application code only
[ ] R4  Every FK has an explicit ON DELETE clause
[ ] R5  User-initiated table → all four audit columns present (created_by/at, updated_by/at)
[ ] R5  System-managed table → created_by and updated_by are ABSENT
[ ] R5  Every domain entity table has data_version VARCHAR(20) NOT NULL DEFAULT '1.0'
[ ] R6  FK columns have indexes
[ ] R6  organization_uuid has an index (if org-scoped)
[ ] R6  status column has an index if used as a filter
[ ] R8  Change applied to all schema files (or divergence is intentional and documented)
[ ] R9  All DDL is idempotent (IF NOT EXISTS / OBJECT_ID guards)
[ ] R10 All identifiers (table/column/index/constraint) are lowercase snake_case
[ ] R10 Pure junction/mapping tables are named with a _mappings suffix
```

#### Step A4 — Write the DDL

Write idempotent DDL using the engine-specific guards:

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

Keep `CREATE INDEX` statements in a dedicated block after all `CREATE TABLE` statements.

#### Step A5 — Apply to all schema files

Apply the change to every in-scope schema file. For type-level differences between engines, see R8 (intentional divergences). Everything else must be structurally identical.

---

### Workflow B — Reviewing Existing DDL (PR / Audit)

#### Step B1 — Locate and read all schema files

```bash
find . -name "schema*.sql" | sort
```

Read each file in full before assessing anything. Note which files are under `gateway/gateway-controller/` — those skip R3 type validation.

#### Step B2 — Open the rules reference

Read `references/api-platform-db-schema-rules.md`. Evaluate every rule group (R1–R10) in order.

#### Step B3 — Record findings

For each violation:

| Field | Value |
|---|---|
| **Rule** | e.g. `R3-JSONB` |
| **Table · column** | exact location |
| **Severity** | `HIGH` (data safety / correctness) · `MEDIUM` (missing guarantee or index) · `LOW` (style / inconsistency) |
| **Finding** | what is wrong |
| **Fix** | the exact DDL needed |

#### Step B4 — Cross-check multi-engine alignment

After the per-table review, verify all schema files are structurally in sync (see R8). Intentional type-level divergences are not findings.

#### Step B5 — Write findings to JSON

Write a structured findings file so findings can be consumed by other tools or tracked across reviews. The script path below is **relative to this skill's directory** — run it from the skill folder (where this SKILL.md lives), and pass an absolute `--out` so the report lands in the project rather than the skill folder:

```bash
# cwd = this skill's directory
node scripts/generate-schema-report.js \
  --findings '<findings-json-array>' \
  --schema   '<path-to-schema-file>' \
  --out      "$(git rev-parse --show-toplevel)/schema-reports/schema-review.json"
```

See the script's `--help` for all flags. The output shape is:

```json
{
  "meta": { "schema": "<path>", "reviewedAt": "<ISO-8601>", "rules": ["R1","R2","R3","R4","R5","R6","R7","R8","R9","R10"] },
  "findings": [
    { "id": "r3-001", "severity": "HIGH", "rule": "R3-NO-TEXT", "table": "<table>", "column": "<col>", "finding": "...", "fix": "..." }
  ]
}
```

#### Step B6 — Report summary

Produce a findings table sorted by severity. Include a "No issues" row for any rule group that passed cleanly — reviewers need to know what was checked.

---

## Quick-Reference Templates

Copy-paste DDL templates and the standard column-type/width cheat sheet live in **`references/api-platform-db-schema-rules.md`**:

- **New entity table** and **standard column types & widths** — see the *Quick-Reference Templates* section at the end of the rules file.
- **New junction/mapping table** — see rule **R1-COMPOSITE-PK**.

Use these as the starting point when writing DDL (Step A4).
