-- SQLite Schema for Gateway-Controller API Configurations
-- Version: 1

-- Base table for all artifact types (REST APIs, WebSub APIs, LLM Providers, LLM Proxies, MCP Proxies)
CREATE TABLE IF NOT EXISTS artifacts (
    uuid TEXT PRIMARY KEY,
    gateway_id TEXT NOT NULL,
    display_name TEXT NOT NULL,
    version TEXT NOT NULL,
    kind TEXT NOT NULL,
    handle TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending', 'deployed', 'failed', 'undeployed')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deployed_at TIMESTAMP, -- NULL until first deployment
    UNIQUE(gateway_id, kind, display_name, version),
    UNIQUE(gateway_id, kind, handle)
);

CREATE INDEX IF NOT EXISTS idx_artifacts_status ON artifacts(status);
CREATE INDEX IF NOT EXISTS idx_artifacts_kind ON artifacts(kind);
CREATE INDEX IF NOT EXISTS idx_artifacts_gateway_id ON artifacts(gateway_id);

-- Per-resource-type tables (each stores source configuration as JSON)

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

-- Note: Policy definitions are no longer stored in the database.
-- They are loaded from files at controller startup (see policies/ directory).
-- The policy_definitions table has been removed as of schema version 3.

-- Table for custom TLS certificates
CREATE TABLE IF NOT EXISTS certificates (
    -- Primary identifier (UUID)
    uuid TEXT PRIMARY KEY,

    -- Gateway identifier
    gateway_id TEXT NOT NULL,

    -- Human-readable name for the certificate
    name TEXT NOT NULL,

    -- PEM-encoded certificate(s) as BLOB
    certificate BLOB NOT NULL,

    -- Certificate metadata (extracted from first cert in bundle)
    subject TEXT NOT NULL,
    issuer TEXT NOT NULL,
    not_before TIMESTAMP NOT NULL,
    not_after TIMESTAMP NOT NULL,
    cert_count INTEGER NOT NULL DEFAULT 1,

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Certificate names must be unique per gateway
    UNIQUE(name, gateway_id)
);

-- Index for fast name lookups
CREATE INDEX IF NOT EXISTS idx_cert_name ON certificates(name);

-- Index for expiry tracking
CREATE INDEX IF NOT EXISTS idx_cert_expiry ON certificates(not_after);

-- Filter by gateway
CREATE INDEX IF NOT EXISTS idx_certificates_gateway_id ON certificates(gateway_id);

-- LLM Provider Templates table (added in schema version 4)
CREATE TABLE IF NOT EXISTS llm_provider_templates (
    -- Primary identifier (UUID)
    uuid TEXT PRIMARY KEY,

    -- Gateway identifier
    gateway_id TEXT NOT NULL,

    -- Template handle (must be unique within a gateway)
    handle TEXT NOT NULL,

    -- Full template configuration as JSON
    configuration TEXT NOT NULL,

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Template handles must be unique per gateway
    UNIQUE(handle, gateway_id)
);

-- Index for fast name lookups
CREATE INDEX IF NOT EXISTS idx_template_handle ON llm_provider_templates(handle);

-- Filter by gateway
CREATE INDEX IF NOT EXISTS idx_llm_provider_templates_gateway_id ON llm_provider_templates(gateway_id);

-- Table for API keys
CREATE TABLE IF NOT EXISTS api_keys (
    -- Primary identifier (UUID)
    uuid TEXT PRIMARY KEY,

    -- Gateway identifier
    gateway_id TEXT NOT NULL,

    -- Human-readable name for the API key
    name TEXT NOT NULL,

    -- The generated API key (hashed)
    api_key TEXT NOT NULL UNIQUE,

    -- Masked version of the API key for display purposes
    masked_api_key TEXT NOT NULL,

    -- Artifact reference
    artifact_uuid TEXT NOT NULL,

    -- Comma-separated list of operations the key will have access to
    operations TEXT NOT NULL DEFAULT '*',

    -- Key status
    status TEXT NOT NULL CHECK(status IN ('active', 'revoked', 'expired')) DEFAULT 'active',

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- User who generated the API key
    created_by TEXT NOT NULL DEFAULT 'system',

    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NULL,  -- NULL means no expiration

    -- Expiration policy fields
    expires_in_unit TEXT NULL,
    expires_in_duration INTEGER NULL,

    -- External API key support (added in schema version 6)
    source TEXT NOT NULL DEFAULT 'local',  -- 'local' or 'external'
    external_ref_id TEXT NULL,  -- external reference

    -- Human-readable display name for the API key
    display_name TEXT NOT NULL DEFAULT '',

    -- Foreign key relationship to artifacts
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,

    -- Composite unique constraint (artifact + api key name must be unique)
    UNIQUE (artifact_uuid, name, gateway_id)
);

-- Indexes for API key lookups
CREATE INDEX IF NOT EXISTS idx_api_key ON api_keys(api_key);
CREATE INDEX IF NOT EXISTS idx_api_key_api ON api_keys(artifact_uuid);
CREATE INDEX IF NOT EXISTS idx_api_key_status ON api_keys(status);
CREATE INDEX IF NOT EXISTS idx_api_key_expiry ON api_keys(expires_at);
CREATE INDEX IF NOT EXISTS idx_created_by ON api_keys(created_by);
CREATE INDEX IF NOT EXISTS idx_api_key_source ON api_keys(source);
CREATE INDEX IF NOT EXISTS idx_api_key_external_ref ON api_keys(external_ref_id);

-- Subscription plans table (organization-scoped rate/billing plans)
CREATE TABLE IF NOT EXISTS subscription_plans (
    id TEXT PRIMARY KEY,
    gateway_id TEXT NOT NULL,
    plan_name TEXT NOT NULL,
    billing_plan TEXT,
    stop_on_quota_reach INTEGER DEFAULT 1,
    throttle_limit_count INTEGER,
    throttle_limit_unit TEXT,
    expiry_time TIMESTAMP,
    status TEXT NOT NULL CHECK(status IN ('ACTIVE', 'INACTIVE')) DEFAULT 'ACTIVE',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(gateway_id, plan_name)
);

-- Subscriptions table (application-level subscriptions for REST APIs)
-- subscription_token: encrypted (AES-256-GCM) for GET display
-- subscription_token_hash: for xDS validation and uniqueness
CREATE TABLE IF NOT EXISTS subscriptions (
    id TEXT PRIMARY KEY,
    gateway_id TEXT NOT NULL,
    api_id TEXT NOT NULL,
    application_id TEXT,
    subscription_token TEXT NOT NULL,
    subscription_token_hash TEXT NOT NULL,
    subscription_plan_id TEXT,
    status TEXT NOT NULL CHECK(status IN ('ACTIVE', 'INACTIVE', 'REVOKED')) DEFAULT 'ACTIVE',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (api_id) REFERENCES rest_apis(uuid) ON DELETE CASCADE,
    FOREIGN KEY (subscription_plan_id) REFERENCES subscription_plans(id) ON DELETE SET NULL,
    UNIQUE(api_id, subscription_token_hash, gateway_id)
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_api_id ON subscriptions(api_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_application_id ON subscriptions(application_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_gateway_id ON subscriptions(gateway_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_token ON subscriptions(subscription_token_hash);

PRAGMA user_version = 1;
