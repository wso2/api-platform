-- Event Gateway Plugin: SQL Server schema fragment
-- Applied at startup only when the experimental build tag is set.
-- These tables extend the core platform-api schema with WebSub and WebBroker support.

-- WEBSUB APIs table
IF OBJECT_ID(N'dbo.websub_apis', N'U') IS NULL
CREATE TABLE dbo.websub_apis (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(40) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    lifecycle_status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration VARBINARY(MAX) NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    origin VARCHAR(20) NOT NULL DEFAULT 'control_plane',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    -- NO ACTION to avoid SQL Server multiple-cascade-paths restriction (error 1785).
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_websub_apis_project' AND object_id = OBJECT_ID(N'dbo.websub_apis'))
CREATE INDEX idx_websub_apis_project ON dbo.websub_apis(project_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_websub_apis_lifecycle_status' AND object_id = OBJECT_ID(N'dbo.websub_apis'))
CREATE INDEX idx_websub_apis_lifecycle_status ON dbo.websub_apis(lifecycle_status);

-- WebSub API HMAC secrets table (for inbound webhook event verification)
IF OBJECT_ID(N'dbo.websub_api_hmac_secrets', N'U') IS NULL
CREATE TABLE dbo.websub_api_hmac_secrets (
    uuid VARCHAR(40) PRIMARY KEY,
    artifact_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(40) NOT NULL,
    display_name VARCHAR(255),
    encrypted_secret VARBINARY(MAX) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    CONSTRAINT uq_websub_api_hmac_secrets_artifact_handle UNIQUE (artifact_uuid, handle)
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_websub_api_hmac_secrets_artifact' AND object_id = OBJECT_ID(N'dbo.websub_api_hmac_secrets'))
CREATE INDEX idx_websub_api_hmac_secrets_artifact ON dbo.websub_api_hmac_secrets(artifact_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_websub_api_hmac_secrets_status' AND object_id = OBJECT_ID(N'dbo.websub_api_hmac_secrets'))
CREATE INDEX idx_websub_api_hmac_secrets_status ON dbo.websub_api_hmac_secrets(status);

-- WEBBROKER APIs table
IF OBJECT_ID(N'dbo.webbroker_apis', N'U') IS NULL
CREATE TABLE dbo.webbroker_apis (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(40) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    lifecycle_status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration VARBINARY(MAX) NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    origin VARCHAR(20) NOT NULL DEFAULT 'control_plane',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    -- NO ACTION to avoid SQL Server multiple-cascade-paths restriction (error 1785).
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_webbroker_apis_project' AND object_id = OBJECT_ID(N'dbo.webbroker_apis'))
CREATE INDEX idx_webbroker_apis_project ON dbo.webbroker_apis(project_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_webbroker_apis_lifecycle_status' AND object_id = OBJECT_ID(N'dbo.webbroker_apis'))
CREATE INDEX idx_webbroker_apis_lifecycle_status ON dbo.webbroker_apis(lifecycle_status);
