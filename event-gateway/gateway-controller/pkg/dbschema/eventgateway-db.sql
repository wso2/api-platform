-- SQLite Schema for Event-Gateway-Controller-specific tables
-- Applied against the same database gateway-controller (core) opened, after
-- core's own schema (gateway-controller-db.sql) has been applied. Core's
-- schema scripts do not define these tables — only this module does.

CREATE TABLE IF NOT EXISTS websub_apis (
    uuid TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    configuration TEXT NOT NULL,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES artifacts(gateway_id, uuid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS webbroker_apis (
    uuid TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    configuration TEXT NOT NULL,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES artifacts(gateway_id, uuid) ON DELETE CASCADE
);

-- Per-API HMAC secrets for the websub-hmac-auth policy.
-- Ciphertext is AES-256-GCM encrypted; plaintext is never stored.
CREATE TABLE IF NOT EXISTS webhook_secrets (
    uuid          TEXT NOT NULL,
    gateway_id    TEXT NOT NULL,
    artifact_uuid TEXT NOT NULL,
    name          TEXT NOT NULL,
    display_name  TEXT NOT NULL,
    ciphertext    BLOB NOT NULL,
    status        TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active', 'revoked')),
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (gateway_id, uuid),
    UNIQUE (gateway_id, artifact_uuid, name),
    FOREIGN KEY (gateway_id, artifact_uuid) REFERENCES artifacts(gateway_id, uuid) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_webhook_secrets_artifact ON webhook_secrets(gateway_id, artifact_uuid);
