-- Control-plane sync tracking on artifacts.
-- SQLite has no ADD COLUMN IF NOT EXISTS; the version marker guards the re-run.
ALTER TABLE artifacts ADD COLUMN cp_sync_status TEXT CHECK(cp_sync_status IN ('pending', 'success', 'failed'));
ALTER TABLE artifacts ADD COLUMN cp_sync_info TEXT;
ALTER TABLE artifacts ADD COLUMN cp_artifact_id TEXT;

CREATE INDEX IF NOT EXISTS idx_artifacts_cp_artifact_id
    ON artifacts(gateway_id, cp_artifact_id) WHERE cp_artifact_id IS NOT NULL;
