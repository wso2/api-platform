-- PostgreSQL Schema for Gateway-Controller API Configurations
-- Version: 8

-- Main table for deployments
CREATE TABLE IF NOT EXISTS deployments (
    id TEXT PRIMARY KEY,
    gateway_id TEXT NOT NULL DEFAULT 'platform-gateway-id',
    display_name TEXT NOT NULL,
    version TEXT NOT NULL,
    context TEXT NOT NULL,
    kind TEXT NOT NULL,
    handle TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending', 'deployed', 'failed', 'undeployed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deployed_at TIMESTAMPTZ,
    deployed_version BIGINT NOT NULL DEFAULT 0,
    UNIQUE(display_name, version, gateway_id),
    UNIQUE(handle, gateway_id)
);

CREATE INDEX IF NOT EXISTS idx_status ON deployments(status);
CREATE INDEX IF NOT EXISTS idx_context ON deployments(context);
CREATE INDEX IF NOT EXISTS idx_kind ON deployments(kind);
CREATE INDEX IF NOT EXISTS idx_deployments_gateway_id ON deployments(gateway_id);

-- Table for custom TLS certificates
CREATE TABLE IF NOT EXISTS certificates (
    id TEXT PRIMARY KEY,
    gateway_id TEXT NOT NULL DEFAULT 'platform-gateway-id',
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

-- Table for deployment-specific configurations
CREATE TABLE IF NOT EXISTS deployment_configs (
    id TEXT PRIMARY KEY,
    configuration TEXT NOT NULL,
    source_configuration TEXT,
    FOREIGN KEY(id) REFERENCES deployments(id) ON DELETE CASCADE
);

-- LLM Provider Templates table
CREATE TABLE IF NOT EXISTS llm_provider_templates (
    id TEXT PRIMARY KEY,
    gateway_id TEXT NOT NULL DEFAULT 'platform-gateway-id',
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
    id TEXT PRIMARY KEY,
    gateway_id TEXT NOT NULL DEFAULT 'platform-gateway-id',
    name TEXT NOT NULL,
    api_key TEXT NOT NULL UNIQUE,
    masked_api_key TEXT NOT NULL,
    apiId TEXT NOT NULL,
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
    FOREIGN KEY (apiId) REFERENCES deployments(id) ON DELETE CASCADE,
    UNIQUE (apiId, name, gateway_id)
);

CREATE INDEX IF NOT EXISTS idx_api_key ON api_keys(api_key);
CREATE INDEX IF NOT EXISTS idx_api_key_api ON api_keys(apiId);
CREATE INDEX IF NOT EXISTS idx_api_key_status ON api_keys(status);
CREATE INDEX IF NOT EXISTS idx_api_key_expiry ON api_keys(expires_at);
CREATE INDEX IF NOT EXISTS idx_created_by ON api_keys(created_by);
CREATE INDEX IF NOT EXISTS idx_api_key_source ON api_keys(source);
CREATE INDEX IF NOT EXISTS idx_api_key_external_ref ON api_keys(external_ref_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_gateway_id ON api_keys(gateway_id);

-- Subscriptions table (application-level subscriptions for REST APIs)
CREATE TABLE IF NOT EXISTS subscriptions (
    id TEXT PRIMARY KEY,
    gateway_id TEXT NOT NULL DEFAULT 'platform-gateway-id',
    api_id TEXT NOT NULL,
    application_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('ACTIVE', 'INACTIVE', 'REVOKED')) DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (api_id) REFERENCES deployments(id) ON DELETE CASCADE,
    UNIQUE(api_id, application_id, gateway_id)
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_api_id ON subscriptions(api_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_application_id ON subscriptions(application_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_gateway_id ON subscriptions(gateway_id);

-- Migration-safe column additions for existing deployments
ALTER TABLE deployments ADD COLUMN IF NOT EXISTS gateway_id TEXT NOT NULL DEFAULT 'platform-gateway-id';
ALTER TABLE certificates ADD COLUMN IF NOT EXISTS gateway_id TEXT NOT NULL DEFAULT 'platform-gateway-id';
ALTER TABLE llm_provider_templates ADD COLUMN IF NOT EXISTS gateway_id TEXT NOT NULL DEFAULT 'platform-gateway-id';
ALTER TABLE api_keys ADD COLUMN IF NOT EXISTS gateway_id TEXT NOT NULL DEFAULT 'platform-gateway-id';

ALTER TABLE deployments DROP CONSTRAINT IF EXISTS deployments_display_name_version_key;
ALTER TABLE deployments DROP CONSTRAINT IF EXISTS deployments_handle_key;
ALTER TABLE certificates DROP CONSTRAINT IF EXISTS certificates_name_key;
ALTER TABLE llm_provider_templates DROP CONSTRAINT IF EXISTS llm_provider_templates_handle_key;
ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS api_keys_apiid_name_key;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'deployments_display_name_version_gateway_id_key'
    ) THEN
        ALTER TABLE deployments
            ADD CONSTRAINT deployments_display_name_version_gateway_id_key
            UNIQUE (display_name, version, gateway_id);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'deployments_handle_gateway_id_key'
    ) THEN
        ALTER TABLE deployments
            ADD CONSTRAINT deployments_handle_gateway_id_key
            UNIQUE (handle, gateway_id);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'certificates_name_gateway_id_key'
    ) THEN
        ALTER TABLE certificates
            ADD CONSTRAINT certificates_name_gateway_id_key
            UNIQUE (name, gateway_id);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'llm_provider_templates_handle_gateway_id_key'
    ) THEN
        ALTER TABLE llm_provider_templates
            ADD CONSTRAINT llm_provider_templates_handle_gateway_id_key
            UNIQUE (handle, gateway_id);
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'api_keys_apiid_name_gateway_id_key'
    ) THEN
        ALTER TABLE api_keys
            ADD CONSTRAINT api_keys_apiid_name_gateway_id_key
            UNIQUE (apiId, name, gateway_id);
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_deployments_gateway_id ON deployments(gateway_id);
CREATE INDEX IF NOT EXISTS idx_certificates_gateway_id ON certificates(gateway_id);
CREATE INDEX IF NOT EXISTS idx_llm_provider_templates_gateway_id ON llm_provider_templates(gateway_id);
CREATE INDEX IF NOT EXISTS idx_api_keys_gateway_id ON api_keys(gateway_id);

-- Schema migration metadata
CREATE TABLE IF NOT EXISTS schema_migrations (
    id INTEGER PRIMARY KEY,
    version INTEGER NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);
