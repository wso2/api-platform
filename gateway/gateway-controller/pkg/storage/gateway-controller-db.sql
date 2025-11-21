-- SQLite Schema for Gateway-Controller API Configurations
-- Version: 1.0
-- Description: Persistent storage for API configurations with lifecycle metadata

-- Main table for API configurations
CREATE TABLE IF NOT EXISTS api_configs (
    -- Primary identifier (UUID)
    id TEXT PRIMARY KEY,

    -- Extracted fields for fast querying
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    context TEXT NOT NULL,              -- Base path (e.g., "/weather")
    kind TEXT NOT NULL,                  -- API type: "http/rest", "graphql", "grpc", "asyncapi"

    -- Full API configuration as JSON
    configuration TEXT NOT NULL,         -- JSON-serialized APIConfiguration

    -- Deployment status
    status TEXT NOT NULL CHECK(status IN ('pending', 'deployed', 'failed')),

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deployed_at TIMESTAMP,               -- NULL until first deployment

    -- Version tracking for xDS snapshots
    deployed_version INTEGER NOT NULL DEFAULT 0,

    -- Composite unique constraint (API name + version must be unique)
    UNIQUE(name, version)
);

-- Indexes for fast lookups
-- Composite index for name+version lookups (most common query)
CREATE INDEX IF NOT EXISTS idx_name_version ON api_configs(name, version);

-- Filter by deployment status (translator queries pending configs)
CREATE INDEX IF NOT EXISTS idx_status ON api_configs(status);

-- Filter by context path (conflict detection)
CREATE INDEX IF NOT EXISTS idx_context ON api_configs(context);

-- Filter by API type (reporting/analytics)
CREATE INDEX IF NOT EXISTS idx_kind ON api_configs(kind);

-- Policy definitions catalog (added in schema version 2)
CREATE TABLE IF NOT EXISTS policy_definitions (
    name TEXT NOT NULL,
    version TEXT NOT NULL,
    provider TEXT NOT NULL,
    description TEXT,
    flows_request_require_header INTEGER NOT NULL DEFAULT 0, -- boolean: 0/1
    flows_request_require_body INTEGER NOT NULL DEFAULT 0,   -- boolean: 0/1
    flows_response_require_header INTEGER NOT NULL DEFAULT 0, -- boolean: 0/1
    flows_response_require_body INTEGER NOT NULL DEFAULT 0,  -- boolean: 0/1
    parameters_schema TEXT, -- JSON document
    PRIMARY KEY (name, version)
);

CREATE INDEX IF NOT EXISTS idx_policy_provider ON policy_definitions(provider);

-- Set schema version
PRAGMA user_version = 2;
