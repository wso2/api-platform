# Schema migrations

So a gateway can be upgraded from one release to another without losing its
database. The schema file alone cannot do that - `CREATE TABLE IF NOT EXISTS`
does nothing against a database that already exists - so each release that
changes the schema ships the DDL that brings an older database forward.

## Chain

| release | adds |
|---|---|
| `1.0.0` | baseline, oldest supported source |
| `1.1.0` | `artifacts` cp_sync columns + index, `subscriptions` billing columns |
| `1.2.0` | `artifacts.data_version`. SQL Server support introduced |
| `2026.09.24` | `agents` |
| `2026.10.0` | `graphql_apis`, `certificates` mutual TLS columns |

Start at any release, apply every later one in order, and you end up with what
the current schema file declares. That is the whole contract - with one limit on
SQLite, below.

## Adding a change

Two edits, same PR:

1. **Schema file** - the full current definition a fresh install gets. `CREATE`
   only; a test fails the build on an `ALTER` there.
2. **`next/`** - the step that brings an existing database to the same shape.

Never touch a release directory, and never create one. That happens at release
time, when `next/*` moves into the new tag directory.

```
next/NNN_short_snake_case_name.{sqlite,postgres,sqlserver}.sql
```

All three dialect files exist, even when one is a no-op with a one-line reason.
`NNN` goes up in tens so a change can be inserted between two.

| Change | Step carries |
|---|---|
| New column | per-dialect `ALTER TABLE ... ADD COLUMN`, nullable or defaulted |
| New table | its own `CREATE TABLE`, not a re-run of the schema file |
| New index | its own guarded `CREATE INDEX` |

Guard every statement (`ADD COLUMN IF NOT EXISTS`, `COL_LENGTH`). A step is
several statements: unguarded, a failure partway leaves the first few applied
and the retry dies on statement one, stranding the database half-migrated.
SQLite has no such guard, so keep its steps small and in one transaction.

Keep comments short - only what the SQL does not say.

## Applying them today

There is no runner yet. An operator applies the files by hand, in release order,
for their dialect.

**SQLite source releases are limited to `2026.09.24` and later.** `initSchema`
reads `PRAGMA user_version` and accepts only 0 (fresh), 5 (`2026.09.24`) and
6 (`2026.10.0`); anything else is refused at startup. `1.0.0`, `1.1.0` and
`1.2.0` wrote 1, 2 and 4, and these steps do not set `user_version`, so applying
them to such a database gives it the right shape and it still will not start.
Lifting that needs `initSchema` to accept the older values, or a version marker
the steps maintain - neither is built yet.

Postgres and SQL Server have no such check, so every source release works there.

On Postgres, a table or index belongs to the role that created it. If a DBA
applies the migration rather than the controller's own database user, grant the
new objects to that user afterwards or the gateway cannot read them.

For Postgres and SQL Server this replaces something: the certificate `ALTER`s
used to sit at the bottom of their schema files and ran on every re-apply. They
are now a step under the release that introduced them,
`020_certificate_tls_columns.*`. A fresh install is unaffected - those columns
are in the `CREATE TABLE` body.

## Two differences that are expected

A database that started at `1.0.0` keeps `websub_apis`; a fresh install never
creates it. The table moved to `event-gateway/gateway-controller/pkg/dbschema`.
Never "fix" that by dropping it.

Column order differs on `artifacts`, `certificates` and `subscriptions`.
`ADD COLUMN` appends, a `CREATE TABLE` body does not, and no engine can insert a
column in the middle. Nothing reads columns by position, so it is harmless - but
compare column **sets**, not ordered lists, or a correct chain looks broken.

## Verified

Every path from every release to the latest, on SQLite and PostgreSQL 16,
against that release's own schema file. SQL Server is untested - the image is
amd64 only and needs a CI runner.
