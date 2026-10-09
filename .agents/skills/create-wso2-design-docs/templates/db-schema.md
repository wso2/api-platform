# **Proposed DB Schema**

<!--
Every schema change the feature needs, per component and per dialect, plus the upgrade path.

Reminder (see .claude/rules/db-schema-changes.md): these are GA products, so a table that has
already shipped is frozen — only additive changes are permitted (a new nullable-or-defaulted
column, a new index, a new table). A retype, rename, added NOT NULL, added UNIQUE, or changed
foreign key is a migration, not a schema edit. A shipped table that violates the schema rules is
accepted legacy — record it, do not "fix" it here.
-->

# 1. Summary of Changes

| Component | Object | Change | Shipped table? | Upgrade path |
| :---- | :---- | :---- | :---- | :---- |
| \<component\> | \<table / column / index\> | New table / New column / New index | Yes / No | Guarded CREATE / ALTER TABLE / N/A |

# 2. \<Component name\> (\<Control Plane / Data Plane / Portal\>)

\<One or two sentences: what this data represents and why it is stored this way.\>

**New table**

```sql
CREATE TABLE IF NOT EXISTS <table_name> (
    uuid              VARCHAR(40)  NOT NULL,
    organization_uuid VARCHAR(40)  NOT NULL,
    <column>          <type>       <NOT NULL / NULL> <DEFAULT ...>,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (organization_uuid, uuid),
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE
);
```

**New column on an existing table**

```sql
-- In the CREATE TABLE body (reaches fresh installs only):
<column> <type> <DEFAULT ...>,   -- new column

-- And the matching ALTER for already-provisioned databases (one per dialect).
-- Must be nullable or defaulted so it applies while the previous release is still running.
ALTER TABLE <table_name> ADD COLUMN IF NOT EXISTS <column> <type> <DEFAULT ...>;
```

**Indexes**

```sql
CREATE INDEX IF NOT EXISTS idx_<table>_<columns> ON <table_name> (<columns>);
```

\<Say which query each index serves — an index with no named query does not belong here.\>

# 3. Dialect Coverage

<!-- Every dialect the component supports needs its own file updated. List them all. -->

| Dialect | Schema file | Updated |
| :---- | :---- | :---- |
| PostgreSQL | \<path\> | Yes / No |
| SQLite | \<path\> | Yes / No |
| MySQL | \<path\> | Yes / No |
| MSSQL | \<path\> | Yes / No |

<!-- MSSQL has no ADD COLUMN IF NOT EXISTS — guard it: -->

```sql
IF NOT EXISTS (SELECT 1 FROM sys.columns WHERE object_id = OBJECT_ID('<table_name>') AND name = '<column>')
    ALTER TABLE <table_name> ADD <column> <type> <DEFAULT ...>;
```

# 4. Data Model Notes

* **Org scoping**: \<how rows are scoped to an organization, and which index supports it.\>
* **Cardinality / growth**: \<expected row count and growth rate.\>
* **Retention / cleanup**: \<what deletes these rows, and via which cascade.\>
* **Derived values**: \<anything computed on read rather than stored, and why.\>
* **Sensitive columns**: \<anything encrypted or hashed at rest, and with which key.\>

# 5. Upgrade and Migration Impact

* **Fresh install**: \<covered by the guarded CREATE.\>
* **Existing deployment**: \<the ALTER path, or an explicit statement that none is needed.\>
* **Rollback**: \<is the previous release still able to run against the new schema.\>
* **Tested against**: \<dialects and versions the change was actually exercised on.\>

# 6. Accepted Legacy Deviations

<!--
Non-conformances found in already-shipped tables while doing this work. Record and move on —
do not write remediation DDL. Also add the row to the register in
.claude/rules/db-schema-changes.md (Appendix A).
-->

| Table | Rule | Deviation | Recorded |
| :---- | :---- | :---- | :---- |
| \<table\> | \<R#\> | \<what does not conform\> | \<YYYY-MM-DD\> |
