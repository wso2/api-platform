---
name: designing-db-schemas
description: Design, change, or review a database schema in the WSO2 API Platform. Use when adding or altering a table, column, index, or constraint in any *.sql schema file, reviewing schema changes before a PR, evaluating a migration plan, or asking whether a table is well designed and what indexes it needs.
allowed-tools: Bash, Read, Edit, Write, Glob, Grep
---

# WSO2 API Platform - Database Schema Design

The rules are in `references/api-platform-db-schema-rules.md` (R0-R11). This file is the workflow. Read the rules file before writing or reviewing DDL.

```text
/designing-db-schemas [table-name | path-to-schema-file]
```

No argument reviews everything in scope.

## Scope

Find schema files with:

```bash
find . -name "*.sql" -not -path "*/node_modules/*" -not -path "*/target/*" | sort
```

Not `schema*.sql` - that misses every gateway-controller and event-gateway file.

| Component | Schema files | Migration steps |
|---|---|---|
| Platform API | `platform-api/internal/database/` | `.../migrations/` |
| Eventgateway plugin | `platform-api/plugins/eventgateway/schema/` | `.../migrations/` |
| API Portal | `portals/api-portal/database/` | `.../migrations/` |
| Gateway Controller | `gateway/gateway-controller/pkg/storage/` + `resources/` | `pkg/storage/migrations/` |
| Event Gateway Controller | `event-gateway/gateway-controller/pkg/dbschema/` | `.../migrations/` |
| Fixture | `platform-api/internal/database/init-platform-api-db.sql` | keep in sync |

Two notes:

- **Gateway Controller and Event Gateway Controller skip R3** (column types). Those teams own their type choices. Every other rule applies.
- **Gateway Controller keeps two copies** of its schema, under `pkg/storage/` and `resources/`. Grep before editing: `grep -rln "<table>" --include="*.sql" .`

## Workflow A - making a change

### A0. Is it allowed? (R0)

These are GA products with tables already in customer databases. Allowed on a shipped table: a new nullable-or-defaulted column, a new index, a new table, and widening a column.

Everything else - narrowing, rename, retype, PK/FK change, adding `NOT NULL` or `UNIQUE`, dropping a column - is a migration plan, not a schema edit. Stop and say so. The test is whether the previous release still works against the migrated database; a change that breaks the old binary cannot be one step, however the DDL is written.

A table added earlier in the same unreleased branch is still malleable:

```bash
git log --oneline -1 -S"CREATE TABLE IF NOT EXISTS <table>" -- '*.sql'
```

A shipped table that breaks R1-R10 is `LEGACY-ACCEPTED`, not a bug to fix. Record it in `.claude/rules/db-schema-changes.md` and write no remediation DDL.

When blocked, don't quietly narrow the task. Say which part is blocked, deliver the rest, offer the migration plan as separate work.

### A1. Read

Read every in-scope schema file in full, then the rules file.

| Change | Rules |
|---|---|
| New table | R0-R6, R8-R11 |
| New column | R0, R3-R11 |
| New index | R0, R6, R8-R11 |
| Type change on a shipped column | R0 - blocked |

### A2. Write the DDL

Schema file first: it is the complete current definition of a fresh install - every table, column and index. `CREATE` only, never an `ALTER` (R11-NO-ALTER).

Guards differ per engine (R9):

```sql
-- PostgreSQL / SQLite
CREATE TABLE IF NOT EXISTS <table> (...);
CREATE INDEX IF NOT EXISTS idx_... ON <table>(...);

-- SQL Server
IF OBJECT_ID(N'dbo.<table>', N'U') IS NULL
CREATE TABLE dbo.<table> (...);
```

Apply to every dialect file, same columns in the same order. Keep indexes in a block after the tables. Templates are at the end of the rules file.

### A3. Write the migration step

The schema file reaches fresh installs only, so every change also needs a step. Add it to `migrations/next/` - never to a release directory, never edit a released script, never create a release directory.

```
migrations/next/NNN_short_snake_case_name.{sqlite,postgres,sqlserver}.sql
```

| Change | Step carries |
|---|---|
| New column | per-dialect `ALTER TABLE ... ADD COLUMN`, nullable or defaulted |
| New table | its own `CREATE TABLE`, not a re-run of the schema file |
| New index | its own guarded `CREATE INDEX` |

All three dialect files exist even when one is a no-op. `NNN` goes up in tens. Keep comments short - only what the SQL does not say.

Guard every statement (`ADD COLUMN IF NOT EXISTS`, `COL_LENGTH`) so a re-run is a no-op. A step is several statements: unguarded, a failure partway leaves the first few applied and the retry dies on statement one, stranding the database half-migrated. SQLite has no such guard, so keep its steps small and in one transaction.

Never leave a missing step behind a `-- TODO`. A comment does not create a column on a customer's database.

### A4. Verify

```bash
cd <component> && go build ./... && go test ./internal/repository/... ./internal/database/...
```

Then check the step actually works: build a database from an older release's schema file, apply the steps, and confirm it matches a fresh install of the current schema file. Compare column **sets** - `ADD COLUMN` appends, so order differs and that is fine.

Report which dialects you actually ran and which you only edited.

## Workflow B - reviewing DDL

1. Locate and read every schema file. Note which skip R3.
2. Evaluate R0-R11 in order. For R11 check both directions: an `ALTER` sitting in a schema file, and a DDL change with no step.
3. Record findings: rule id, table/column, severity, what is wrong, exact fix.

Severities: `HIGH` data safety or correctness, `MEDIUM` missing guarantee or index, `LOW` style, `LEGACY-ACCEPTED` shipped table frozen by R0 (no fix - record it in `.claude/rules/db-schema-changes.md`).

When a rule is broken uniformly across many tables, write one finding naming the pattern with a couple of examples, not one per table.

Check dialect alignment (R8):

```bash
for f in platform-api/internal/database/schema*.sql; do
  echo "--- $f"
  awk '/CREATE TABLE.*<table>/,/\);/' "$f" | grep -oE '^\s+[a-z_]+' | tr -d ' '
done
```

Then write the report:

```bash
node scripts/generate-schema-report.js \
  --findings '<json-array>' \
  --schema   '<schema-file>' \
  --out      "$(git rev-parse --show-toplevel)/schema-reports/schema-review.json"
```

Finish with a table sorted by severity, including a clean row for each rule group that passed - reviewers need to know what was checked.
