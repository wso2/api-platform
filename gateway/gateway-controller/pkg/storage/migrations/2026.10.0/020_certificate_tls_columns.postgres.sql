-- Mutual TLS columns on certificates. See the sqlite file for column meanings.
-- Replaces the ALTERs appended to gateway-controller-db.postgres.sql (R11-NO-ALTER);
-- delete those once a runner applies this step.
ALTER TABLE certificates ADD COLUMN IF NOT EXISTS usage TEXT NOT NULL DEFAULT 'upstream';
ALTER TABLE certificates ADD COLUMN IF NOT EXISTS role TEXT NOT NULL DEFAULT 'client';
ALTER TABLE certificates ADD COLUMN IF NOT EXISTS match_json TEXT;
ALTER TABLE certificates ADD COLUMN IF NOT EXISTS private_key_ciphertext TEXT;
ALTER TABLE certificates ADD COLUMN IF NOT EXISTS key_algorithm TEXT;
