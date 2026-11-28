-- Mutual TLS columns on certificates.
-- usage: upstream trust, downstream client CA, or identity (chain + encrypted key).
-- match_json narrows a role: relay entry; NULL means unnarrowed.
-- private_key_ciphertext and key_algorithm apply to usage: identity only.
ALTER TABLE certificates ADD COLUMN usage TEXT NOT NULL DEFAULT 'upstream';
ALTER TABLE certificates ADD COLUMN role TEXT NOT NULL DEFAULT 'client';
ALTER TABLE certificates ADD COLUMN match_json TEXT;
ALTER TABLE certificates ADD COLUMN private_key_ciphertext TEXT;
ALTER TABLE certificates ADD COLUMN key_algorithm TEXT;
