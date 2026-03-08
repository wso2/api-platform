-- PostgreSQL Schema for Gateway-Controller API Configurations
-- Version: 1

-- Base table for all artifact types
CREATE TABLE IF NOT EXISTS artifacts (
    uuid TEXT PRIMARY KEY,
    gateway_id TEXT NOT NULL,
    display_name TEXT NOT NULL,
    version TEXT NOT NULL,
    kind TEXT NOT NULL,
    handle TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending', 'deployed', 'failed', 'undeployed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deployed_at TIMESTAMPTZ,
    UNIQUE(gateway_id, kind, display_name, version),
    UNIQUE(gateway_id, kind, handle)
);

CREATE INDEX IF NOT EXISTS idx_artifacts_status ON artifacts(status);
CREATE INDEX IF NOT EXISTS idx_artifacts_kind ON artifacts(kind);
CREATE INDEX IF NOT EXISTS idx_artifacts_gateway_id ON artifacts(gateway_id);

-- Per-resource-type tables

CREATE TABLE IF NOT EXISTS rest_apis (
    uuid TEXT PRIMARY KEY,
    configuration TEXT NOT NULL,
    FOREIGN KEY(uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS websub_apis (
    uuid TEXT PRIMARY KEY,
    configuration TEXT NOT NULL,
    FOREIGN KEY(uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS llm_providers (
    uuid TEXT PRIMARY KEY,
    configuration TEXT NOT NULL,
    FOREIGN KEY(uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS llm_proxies (
    uuid TEXT PRIMARY KEY,
    configuration TEXT NOT NULL,
    provider_uuid TEXT NOT NULL,
    FOREIGN KEY(uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY(provider_uuid) REFERENCES llm_providers(uuid) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS mcp_proxies (
    uuid TEXT PRIMARY KEY,
    configuration TEXT NOT NULL,
    FOREIGN KEY(uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE
);

-- Table for custom TLS certificates
CREATE TABLE IF NOT EXISTS certificates (
    uuid TEXT PRIMARY KEY,
    gateway_id TEXT NOT NULL,
    name TEXT NOT NULL,
    certificate BYTEA NOT NULL,
    subject TEXT NOT NULL,
    issuer TEXT NOT NULL,
    not_before TIMESTAMPTZ NOT NULL,
    not_after TIMESTAMPTZ NOT NULL,
    cert_count INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(name, gateway_id)
);

CREATE INDEX IF NOT EXISTS idx_cert_name ON certificates(name);
CREATE INDEX IF NOT EXISTS idx_cert_expiry ON certificates(not_after);
CREATE INDEX IF NOT EXISTS idx_certificates_gateway_id ON certificates(gateway_id);

-- LLM Provider Templates table
CREATE TABLE IF NOT EXISTS llm_provider_templates (
    uuid TEXT PRIMARY KEY,
    gateway_id TEXT NOT NULL,
    handle TEXT NOT NULL,
    configuration TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(handle, gateway_id)
);

CREATE INDEX IF NOT EXISTS idx_template_handle ON llm_provider_templates(handle);
CREATE INDEX IF NOT EXISTS idx_llm_provider_templates_gateway_id ON llm_provider_templates(gateway_id);

-- Table for API keys
CREATE TABLE IF NOT EXISTS api_keys (
    uuid TEXT PRIMARY KEY,
    gateway_id TEXT NOT NULL,
    name TEXT NOT NULL,
    api_key TEXT NOT NULL UNIQUE,
    masked_api_key TEXT NOT NULL,
    artifact_uuid TEXT NOT NULL,
    operations TEXT NOT NULL DEFAULT '*',
    status TEXT NOT NULL CHECK(status IN ('active', 'revoked', 'expired')) DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT NOT NULL DEFAULT 'system',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NULL,
    expires_in_unit TEXT NULL,
    expires_in_duration INTEGER NULL,
    source TEXT NOT NULL DEFAULT 'local',
    external_ref_id TEXT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    UNIQUE (artifact_uuid, name, gateway_id)
);

CREATE INDEX IF NOT EXISTS idx_api_key ON api_keys(api_key);
CREATE INDEX IF NOT EXISTS idx_api_key_api ON api_keys(artifact_uuid);
CREATE INDEX IF NOT EXISTS idx_api_key_status ON api_keys(status);
CREATE INDEX IF NOT EXISTS idx_api_key_expiry ON api_keys(expires_at);
CREATE INDEX IF NOT EXISTS idx_created_by ON api_keys(created_by);
CREATE INDEX IF NOT EXISTS idx_api_key_source ON api_keys(source);
CREATE INDEX IF NOT EXISTS idx_api_key_external_ref ON api_keys(external_ref_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_gateway_id ON api_keys(gateway_id);

