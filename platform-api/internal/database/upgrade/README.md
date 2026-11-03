# Platform API schema upgrade scripts

Platform API applies its schema itself only on SQLite. A Postgres or SQL Server
database is provisioned by the operator, so a column added to a table that has
already shipped reaches an existing database only through a script here.

Each script is idempotent: run it once per database when upgrading to the
release that introduced it, before starting the new release. Re-running it, or
re-applying the full `schema.<dialect>.sql` (which carries the same guarded
statements), is harmless.

| Script | Change |
|---|---|
| `user-idp-references-display-name.{postgres,sqlserver}.sql` | Adds the nullable `user_idp_references.display_name` column |
