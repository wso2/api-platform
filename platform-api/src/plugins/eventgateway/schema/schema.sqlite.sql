-- Event Gateway Plugin: SQLite schema fragment
-- Applied at startup only when the experimental build tag is set.
-- These tables extend the core platform-api schema with WebSub and WebBroker support.

-- WEBSUB APIs table
CREATE TABLE IF NOT EXISTS websub_apis (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    lifecycle_status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration BLOB NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    origin VARCHAR(20) NOT NULL DEFAULT 'control_plane',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);
CREATE INDEX IF NOT EXISTS idx_websub_apis_project ON websub_apis(project_uuid);
CREATE INDEX IF NOT EXISTS idx_websub_apis_lifecycle_status ON websub_apis(lifecycle_status);

-- WebSub API HMAC secrets table (for inbound webhook event verification)
CREATE TABLE IF NOT EXISTS websub_api_hmac_secrets (
    uuid VARCHAR(40) PRIMARY KEY,
    artifact_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255),
    encrypted_secret BLOB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    UNIQUE(artifact_uuid, handle)
);
CREATE INDEX IF NOT EXISTS idx_websub_api_hmac_secrets_artifact ON websub_api_hmac_secrets(artifact_uuid);
CREATE INDEX IF NOT EXISTS idx_websub_api_hmac_secrets_status ON websub_api_hmac_secrets(status);

-- WEBBROKER APIs table
CREATE TABLE IF NOT EXISTS webbroker_apis (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    lifecycle_status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration BLOB NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    origin VARCHAR(20) NOT NULL DEFAULT 'control_plane',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);
CREATE INDEX IF NOT EXISTS idx_webbroker_apis_project ON webbroker_apis(project_uuid);
CREATE INDEX IF NOT EXISTS idx_webbroker_apis_lifecycle_status ON webbroker_apis(lifecycle_status);
