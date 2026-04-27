-- SQLite Schema for Gateway-Controller API Configurations
-- Version: 2

-- Base table for all artifact types (REST APIs, WebSub APIs, LLM Providers, LLM Proxies, MCP Proxies)
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
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deployed_at TIMESTAMP, -- NULL until first deployment
    -- NEW COLUMNS: cp_sync_status, cp_artifact_id and cp_sync_info must be added 
    -- to existing deployments via ALTER TABLE migration.
    cp_sync_status TEXT CHECK(cp_sync_status IN ('pending', 'success', 'failed')),
    cp_sync_info TEXT,
    cp_artifact_id TEXT,
    PRIMARY KEY (gateway_id, uuid),
    UNIQUE(gateway_id, kind, display_name, version),
    UNIQUE(gateway_id, kind, handle)
);
CREATE INDEX IF NOT EXISTS idx_artifacts_cp_artifact_id ON artifacts(gateway_id, cp_artifact_id) WHERE cp_artifact_id IS NOT NULL;

-- Per-resource-type tables (each stores source configuration as JSON)

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

-- Note: Policy definitions are no longer stored in the database.
-- They are loaded from files at controller startup (see policies/ directory).
-- The policy_definitions table has been removed as of schema version 3.

-- Table for custom TLS certificates
CREATE TABLE IF NOT EXISTS certificates (
    -- Primary identifier (UUID)
    uuid TEXT NOT NULL,

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

    PRIMARY KEY (gateway_id, uuid),

    -- Certificate names must be unique per gateway
    UNIQUE(gateway_id, name)
);

-- LLM Provider Templates table (added in schema version 4)
CREATE TABLE IF NOT EXISTS llm_provider_templates (
    -- Primary identifier (UUID)
    uuid TEXT NOT NULL,

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
    PRIMARY KEY (gateway_id, uuid),
    UNIQUE(gateway_id, handle)
);

-- Table for API keys
CREATE TABLE IF NOT EXISTS api_keys (
    -- UUID v7 from platform API, or locally generated if not provided
    uuid TEXT NOT NULL,

    -- Gateway identifier
    gateway_id TEXT NOT NULL,

    -- Human-readable name for the API key
    name TEXT NOT NULL,

    -- The generated API key (hashed)
    api_key TEXT NOT NULL,

    -- Masked version of the API key for display purposes
    masked_api_key TEXT NOT NULL,

    -- Artifact association (the API may not be deployed locally yet)
    artifact_uuid TEXT NOT NULL,

    -- Key status
    status TEXT NOT NULL CHECK(status IN ('active', 'revoked', 'expired')) DEFAULT 'active',

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- User who generated the API key
    created_by TEXT NOT NULL DEFAULT 'system',

    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NULL,  -- NULL means no expiration

    -- External API key support (added in schema version 6)
    source TEXT NOT NULL DEFAULT 'local',  -- 'local' or 'external'
    external_ref_id TEXT NULL,  -- external reference

    -- Portal and target tracking
    issuer TEXT NULL DEFAULT NULL,               -- developer portal identifier; NULL means not specified

    -- Composite unique constraint (artifact + api key name must be unique)
    UNIQUE (gateway_id, artifact_uuid, name),

    -- API key UUID must be unique within a gateway for cross-table references
    UNIQUE (gateway_id, uuid),

    -- Composite primary key
    PRIMARY KEY (gateway_id, api_key)
);

-- Indexes for API key lookups
CREATE INDEX IF NOT EXISTS idx_api_key_status ON api_keys(status);
CREATE INDEX IF NOT EXISTS idx_created_by ON api_keys(created_by);

-- Subscription plans table (organization-scoped rate/billing plans)
CREATE TABLE IF NOT EXISTS subscription_plans (
    uuid TEXT NOT NULL,
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
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY (gateway_id, subscription_plan_id) REFERENCES subscription_plans(gateway_id, uuid),
    UNIQUE(gateway_id, api_id, subscription_token_hash)
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_application_id ON subscriptions(application_id);

-- Table for gateway states (used by eventhub for multi-replica sync)
CREATE TABLE IF NOT EXISTS gateway_states (
    gateway_id TEXT PRIMARY KEY,
    version_id TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Table for events (used by eventhub for multi-replica sync)
CREATE TABLE IF NOT EXISTS events (
    gateway_id TEXT NOT NULL,
    processed_timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    originated_timestamp TIMESTAMP NOT NULL,
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
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (gateway_id, application_uuid)
);
-- Application to API key mappings
CREATE TABLE IF NOT EXISTS application_api_keys (
    application_uuid TEXT NOT NULL,
    api_key_id TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (gateway_id, application_uuid, api_key_id),
    FOREIGN KEY (gateway_id, application_uuid) REFERENCES applications(gateway_id, application_uuid) ON DELETE CASCADE,
    FOREIGN KEY (gateway_id, api_key_id) REFERENCES api_keys(gateway_id, uuid) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_app_api_keys_apikey ON application_api_keys(gateway_id, api_key_id);

-- Table for encrypted secrets
CREATE TABLE IF NOT EXISTS secrets (
    gateway_id TEXT NOT NULL,           -- gateway identifier
    handle TEXT NOT NULL,               -- secret identifier (e.g., wso2-openai-api-key)
    display_name TEXT NOT NULL,         -- human-readable name for list views
    description TEXT,                   -- optional human-readable description
    ciphertext BLOB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (gateway_id, handle)
);

PRAGMA user_version = 2;
