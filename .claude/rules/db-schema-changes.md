# Rule: Database Schema Changes

## Context & Scope

Applies whenever adding or altering a table, column, index, or constraint in any `*.sql` schema file, and whenever changing the Go `model`/`repository` code that reads or writes those columns.

**The rules are R0–R10, defined in `.agents/skills/designing-db-schemas/references/api-platform-db-schema-rules.md`.** That file is the single source of truth — column types and widths, primary-key and foreign-key shape, org-scoping, audit columns, indexing, multi-engine alignment, idempotent DDL, and naming. Invoke the `designing-db-schemas` skill to apply them; this file exists to state the two things that govern *whether* a change is allowed at all, and to hold the deviations register.

## Directives

1. **These are GA products — shipped tables are frozen (R0-FROZEN).** Gateway Controller, Event Gateway Controller, Platform API, API Portal, and AI Workspace (which packages Platform API's schemas) are all in customer hands. On a table that has shipped, the only permitted changes are additive: a new nullable-or-defaulted column, a new index, a new table. Never retype, re-widen, rename, add or drop a primary key, change a foreign key's target or `ON DELETE`, add `NOT NULL` to an existing nullable column, or add a `UNIQUE` constraint — each rewrites or revalidates customer data on upgrade, and this repo has no migration framework to do it safely. Column removal is a two-release sequence. Anything blocked here needs an approved migration plan, not a schema edit.

2. **A shipped table that violates R1–R10 is accepted legacy, not a bug to fix (R0-LEGACY-ACCEPTED).** Record it in Appendix A below at severity `LEGACY-ACCEPTED` and move on. Do not write remediation DDL, and do not let a schema audit turn into unplanned migration work.

3. **Every change ships its upgrade path (R0-UPGRADE-PATH).** What that path is depends on the change: a **new table** or **new index** needs only its guarded `CREATE ... IF NOT EXISTS` (or the `OBJECT_ID`/`sys.indexes` equivalent), which runs against fresh and already-provisioned databases alike. A **column added to an existing table** needs more: `CREATE TABLE IF NOT EXISTS` is a no-op against a database that already exists, so a column added to a `CREATE TABLE` body reaches fresh installs only — ship the matching per-dialect `ALTER TABLE`, nullable or defaulted so it applies while the previous release is still running.

4. **No deferring a violation behind a code comment.** Never resolve a missing dialect file, a missing `ALTER` path, an absent foreign key, or a plaintext-secret column with a `-- TODO`/`FIXME` comment and merge anyway — a comment does not create a column on a customer's database. Fix it, or raise a tracked issue with an owner and a deadline in the PR description. This applies to violations **in the change under review**. A violation that already exists in a shipped table is exempt: it is `LEGACY-ACCEPTED` under Directive 2 — record it in Appendix A, do not write remediation DDL, and leave the fix to an approved migration plan. Recording it there is the documentation, not a deferral comment.

> **Before outputting any schema change:**
> * Does it touch a shipped table in any way other than adding a nullable/defaulted column or an index? (If so, stop — that's a migration.)
> * Is a shipped table's non-conformance being "fixed" instead of recorded in Appendix A?
> * Does a column added to an existing table ship a per-dialect `ALTER TABLE` for already-provisioned databases? (New tables and indexes need none — their guarded `CREATE` covers both cases.)
> * Have R1–R10 been applied to the new table/column via the `designing-db-schemas` skill?

---

## Appendix A — Accepted legacy deviations

Shipped tables that don't conform to R1–R10, frozen under Directive 1. **Do not "fix" these.** Reviews add a row rather than re-reporting; the `designing-db-schemas` Workflow B writes findings here at severity `LEGACY-ACCEPTED`.

| Product | Table | Rule | Deviation | Recorded |
|---|---|---|---|---|
| _(populate from the first full audit)_ | | | | |
