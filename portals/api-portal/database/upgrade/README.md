# API Portal schema upgrade scripts

The portal applies its schema itself only on SQLite (`ensureSchema` in
`src/server.js`). A Postgres or SQL Server database is provisioned out of band,
so a column added to a table that has already shipped reaches an existing
database only through a script here.

Each script is idempotent: run it once per database when upgrading to the
release that introduced it, before starting the new release. Re-running it, or
re-applying the full `schema.<dialect>.sql` (which carries the same guarded
statements), is harmless.

| Script | Change |
|---|---|
| `user-idp-references-display-name.{postgres,sqlserver}.sql` | Adds the nullable `user_idp_references.display_name` column |
