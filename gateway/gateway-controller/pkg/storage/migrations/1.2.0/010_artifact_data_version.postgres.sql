-- Data shape each stored artifact was written with.
-- Defaulted, not nullable: every pre-existing row was written at '1.0'.
-- Constant default, so Postgres 11+ does not rewrite the table.
ALTER TABLE artifacts ADD COLUMN IF NOT EXISTS data_version TEXT NOT NULL DEFAULT '1.0';
