-- SQLite Schema for Gateway-Controller API Configurations
-- Version: 1.0
-- Description: Persistent storage for API configurations with lifecycle metadata

-- Main table for deployments
CREATE TABLE IF NOT EXISTS deployments (
    -- Primary identifier (UUID)
    id TEXT PRIMARY KEY,

    -- Extracted fields for fast querying
    display_name TEXT NOT NULL,
    version TEXT NOT NULL,
    context TEXT NOT NULL,              -- Base path (e.g., "/weather")
    kind TEXT NOT NULL,                 -- Deployment type: "RestApi", "graphql", "grpc", "asyncapi"
    handle TEXT NOT NULL UNIQUE,        -- API handle (e.g., petstore-v1.0)

    -- Deployment status
    status TEXT NOT NULL CHECK(status IN ('pending', 'deployed', 'failed')),

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deployed_at TIMESTAMP,               -- NULL until first deployment

    -- Version tracking for xDS snapshots
    deployed_version INTEGER NOT NULL DEFAULT 0,

    -- Composite unique constraint (API display_name + version must be unique)
    UNIQUE(display_name, version)
);

-- Indexes for fast lookups
-- Composite index for display_name+version lookups (most common query)
CREATE INDEX IF NOT EXISTS idx_name_version ON deployments(display_name, version);

-- Filter by deployment status (translator queries pending configs)
CREATE INDEX IF NOT EXISTS idx_status ON deployments(status);

-- Filter by context path (conflict detection)
CREATE INDEX IF NOT EXISTS idx_context ON deployments(context);

-- Filter by API type (reporting/analytics)
CREATE INDEX IF NOT EXISTS idx_kind ON deployments(kind);

-- Note: Policy definitions are no longer stored in the database.
-- They are loaded from files at controller startup (see policies/ directory).
-- The policy_definitions table has been removed as of schema version 3.

-- Table for custom TLS certificates
CREATE TABLE IF NOT EXISTS certificates (
    -- Primary identifier (UUID)
    id TEXT PRIMARY KEY,
    
    -- Human-readable name for the certificate
    name TEXT NOT NULL UNIQUE,
    
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
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index for fast name lookups
CREATE INDEX IF NOT EXISTS idx_cert_name ON certificates(name);

-- Index for expiry tracking
CREATE INDEX IF NOT EXISTS idx_cert_expiry ON certificates(not_after);


-- Table for deployment-specific configurations
CREATE TABLE IF NOT EXISTS deployment_configs (
    id TEXT PRIMARY KEY,
    configuration TEXT NOT NULL,        -- JSON-serialized APIConfiguration
    source_configuration TEXT,          -- JSON-serialized SourceConfiguration
    FOREIGN KEY(id) REFERENCES deployments(id) ON DELETE CASCADE
);

-- LLM Provider Templates table (added in schema version 4)
CREATE TABLE IF NOT EXISTS llm_provider_templates (
    -- Primary identifier (UUID)
    id TEXT PRIMARY KEY,

    -- Template handle (must be unique)
    handle TEXT NOT NULL UNIQUE,

    -- Full template configuration as JSON
    configuration TEXT NOT NULL,

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index for fast name lookups
CREATE INDEX IF NOT EXISTS idx_template_handle ON llm_provider_templates(handle);

-- Table for API keys
CREATE TABLE IF NOT EXISTS api_keys (
    -- Primary identifier (UUID)
    id TEXT PRIMARY KEY,

    -- Human-readable name for the API key
    name TEXT NOT NULL,

    -- The generated API key (hashed)
    api_key TEXT NOT NULL UNIQUE,

    -- Masked version of the API key for display purposes
    masked_api_key TEXT NOT NULL,

    -- API reference
    apiId TEXT NOT NULL,

    -- Comma-separated list of operations the key will have access to
    operations TEXT NOT NULL DEFAULT '[*]',

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
    external_ref_id TEXT NULL,  -- Cloud APIM key ID or other external reference

    -- O(1) lookup optimization for external keys
    index_key TEXT NULL,  -- Pre-computed SHA-256 hash for fast lookup

    -- Human-readable display name for the API key
    display_name TEXT NOT NULL DEFAULT '',

    -- Foreign key relationship to deployments
    FOREIGN KEY (apiId) REFERENCES deployments(id) ON DELETE CASCADE,

    -- Composite unique constraint (handle + api key name must be unique)
    UNIQUE (apiId, name)
);

-- Indexes for API key lookups
CREATE INDEX IF NOT EXISTS idx_api_key ON api_keys(api_key);
CREATE INDEX IF NOT EXISTS idx_api_key_api ON api_keys(apiId);
CREATE INDEX IF NOT EXISTS idx_api_key_status ON api_keys(status);
CREATE INDEX IF NOT EXISTS idx_api_key_expiry ON api_keys(expires_at);
CREATE INDEX IF NOT EXISTS idx_created_by ON api_keys(created_by);
CREATE INDEX IF NOT EXISTS idx_api_key_source ON api_keys(source);
CREATE INDEX IF NOT EXISTS idx_api_key_external_ref ON api_keys(external_ref_id);
CREATE INDEX IF NOT EXISTS idx_api_key_index_key ON api_keys(index_key);

-- Set schema version to 6 (api_keys with external ref, index_key, display_name)
PRAGMA user_version = 6;
