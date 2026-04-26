-- PostgreSQL Schema for Gateway-Controller API Configurations
-- Version: 2

-- Base table for all artifact types
CREATE TABLE IF NOT EXISTS artifacts (
    uuid TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    display_name TEXT NOT NULL,
    version TEXT NOT NULL,
    kind TEXT NOT NULL,
    handle TEXT NOT NULL,
    desired_state TEXT NOT NULL CHECK(desired_state IN ('deployed', 'undeployed')),
    deployment_id TEXT,
    origin TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deployed_at TIMESTAMPTZ,
    -- NEW COLUMNS: cp_sync_status, cp_artifact_id and cp_sync_info must be added 
    -- to existing deployments via ALTER TABLE migration.
    cp_sync_status  TEXT CHECK(cp_sync_status IN ('pending', 'success', 'failed')),
    cp_sync_info    TEXT,
    cp_artifact_id  TEXT,
    PRIMARY KEY (gateway_id, uuid),
    UNIQUE(gateway_id, kind, display_name, version),
    UNIQUE(gateway_id, kind, handle)
);
CREATE INDEX IF NOT EXISTS idx_artifacts_cp_artifact_id ON artifacts(gateway_id, cp_artifact_id) WHERE cp_artifact_id IS NOT NULL;

-- Per-resource-type tables

CREATE TABLE IF NOT EXISTS rest_apis (
    uuid TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    configuration TEXT NOT NULL,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES artifacts(gateway_id, uuid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS websub_apis (
    uuid TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    configuration TEXT NOT NULL,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES artifacts(gateway_id, uuid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS llm_providers (
    uuid TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    configuration TEXT NOT NULL,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES artifacts(gateway_id, uuid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS llm_proxies (
    uuid TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    configuration TEXT NOT NULL,
    provider_uuid TEXT NOT NULL,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES artifacts(gateway_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY(gateway_id, provider_uuid) REFERENCES llm_providers(gateway_id, uuid) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS mcp_proxies (
    uuid TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    configuration TEXT NOT NULL,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES artifacts(gateway_id, uuid) ON DELETE CASCADE
);

-- Table for custom TLS certificates
CREATE TABLE IF NOT EXISTS certificates (
    uuid TEXT NOT NULL,
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
    PRIMARY KEY (gateway_id, uuid),
    UNIQUE(gateway_id, name)
);

-- LLM Provider Templates table
CREATE TABLE IF NOT EXISTS llm_provider_templates (
    uuid TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    handle TEXT NOT NULL,
    configuration TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (gateway_id, uuid),
    UNIQUE(gateway_id, handle)
);

-- Table for API keys
CREATE TABLE IF NOT EXISTS api_keys (
    uuid TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    name TEXT NOT NULL,
    api_key TEXT NOT NULL,
    masked_api_key TEXT NOT NULL,
    artifact_uuid TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('active', 'revoked', 'expired')) DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by TEXT NOT NULL DEFAULT 'system',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMPTZ NULL,
    source TEXT NOT NULL DEFAULT 'local',
    external_ref_id TEXT NULL,
    issuer TEXT NULL DEFAULT NULL,
    UNIQUE (gateway_id, artifact_uuid, name),
    UNIQUE (gateway_id, uuid),
    PRIMARY KEY (gateway_id, api_key)
);

CREATE INDEX IF NOT EXISTS idx_api_key_status ON api_keys(status);
CREATE INDEX IF NOT EXISTS idx_created_by ON api_keys(created_by);

-- Subscription plans table (organization-scoped rate/billing plans)
CREATE TABLE IF NOT EXISTS subscription_plans (
    uuid TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    plan_name TEXT NOT NULL,
    billing_plan TEXT,
    stop_on_quota_reach BOOLEAN DEFAULT TRUE,
    throttle_limit_count INTEGER,
    throttle_limit_unit TEXT,
    expiry_time TIMESTAMPTZ,
    status TEXT NOT NULL CHECK(status IN ('ACTIVE', 'INACTIVE')) DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (gateway_id, uuid),
    UNIQUE(gateway_id, plan_name)
);

-- Subscriptions table (application-level subscriptions for REST APIs, even before deployment)
-- subscription_token_hash: for xDS validation and request validation (Platform-API stores original token)
CREATE TABLE IF NOT EXISTS subscriptions (
    uuid TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    api_id TEXT NOT NULL,
    application_id TEXT,
    subscription_token_hash TEXT NOT NULL,
    subscription_plan_id TEXT,
    -- NEW COLUMNS: billing_customer_id and billing_subscription_id must be added
    -- to existing deployments via ALTER TABLE migration.
    billing_customer_id TEXT,
    billing_subscription_id TEXT,
    status TEXT NOT NULL CHECK(status IN ('ACTIVE', 'INACTIVE', 'REVOKED')) DEFAULT 'ACTIVE',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY (gateway_id, subscription_plan_id) REFERENCES subscription_plans(gateway_id, uuid),
    UNIQUE(gateway_id, api_id, subscription_token_hash)
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_application_id ON subscriptions(application_id);

-- Table for gateway states (used by eventhub for multi-replica sync)
CREATE TABLE IF NOT EXISTS gateway_states (
    gateway_id TEXT PRIMARY KEY,
    version_id TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Table for events (used by eventhub for multi-replica sync)
CREATE TABLE IF NOT EXISTS events (
    gateway_id TEXT NOT NULL,
    processed_timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    originated_timestamp TIMESTAMPTZ NOT NULL,
    entity_type TEXT NOT NULL,
    action TEXT NOT NULL CHECK(action IN ('CREATE', 'UPDATE', 'DELETE')),
    entity_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    event_data TEXT NOT NULL,
    PRIMARY KEY (gateway_id, event_id),
    FOREIGN KEY (gateway_id) REFERENCES gateway_states(gateway_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_events_gateway_id_processed_timestamp ON events(gateway_id, processed_timestamp);
-- Applications
CREATE TABLE IF NOT EXISTS applications (
    application_uuid TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    application_id TEXT NOT NULL,
    application_name TEXT NOT NULL,
    application_type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (gateway_id, application_uuid)
);
-- Application to API key mappings
CREATE TABLE IF NOT EXISTS application_api_keys (
    application_uuid TEXT NOT NULL,
    api_key_id TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (gateway_id, application_uuid, api_key_id),
    FOREIGN KEY (gateway_id, application_uuid) REFERENCES applications(gateway_id, application_uuid) ON DELETE CASCADE,
    FOREIGN KEY (gateway_id, api_key_id) REFERENCES api_keys(gateway_id, uuid) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_app_api_keys_apikey ON application_api_keys(gateway_id, api_key_id);

-- Table for encrypted secrets (gateway_id + handle form the composite PK)
-- provider and key_version are self-describing inside the ciphertext envelope
CREATE TABLE IF NOT EXISTS secrets (
    gateway_id TEXT NOT NULL,
    handle TEXT NOT NULL,
    display_name TEXT NOT NULL,
    description TEXT,
    ciphertext BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (gateway_id, handle)
);
