-- Data shape each stored artifact was written with.
-- Defaulted, not nullable: every pre-existing row was written at '1.0'.
ALTER TABLE artifacts ADD COLUMN data_version TEXT NOT NULL DEFAULT '1.0';
