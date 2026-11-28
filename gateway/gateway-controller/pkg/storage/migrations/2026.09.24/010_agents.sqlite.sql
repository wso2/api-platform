-- A2A Agent artifacts.
-- signed_*_card are signed at deploy time; NULL when signing is off or in passthrough.
CREATE TABLE IF NOT EXISTS agents (
    uuid TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    configuration TEXT NOT NULL,
    signed_public_card TEXT,
    signed_protected_card TEXT,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES artifacts(gateway_id, uuid) ON DELETE CASCADE
);
