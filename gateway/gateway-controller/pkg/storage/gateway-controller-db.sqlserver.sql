-- SQL Server Schema for Gateway-Controller API Configurations
-- Version: 2
--
-- Portable counterpart of gateway-controller-db.postgres.sql. Type mapping:
--   TEXT (keyed)      -> NVARCHAR(64)/NVARCHAR(255)  (NVARCHAR(MAX) cannot be indexed;
--                        composite key columns are kept small to stay under SQL Server's
--                        900-byte clustered / 1700-byte nonclustered index-key limits)
--   TEXT (free/JSON)  -> NVARCHAR(MAX)
--   BYTEA             -> VARBINARY(MAX)
--   TIMESTAMPTZ       -> DATETIME2(7) DEFAULT SYSUTCDATETIME()
--   BOOLEAN           -> BIT
--   INTEGER           -> INT
-- Every object is guarded by IF NOT EXISTS so the batch is idempotent.

-- Base table for all artifact types
IF OBJECT_ID(N'dbo.artifacts', N'U') IS NULL
CREATE TABLE dbo.artifacts (
    uuid NVARCHAR(64) NOT NULL,
    gateway_id NVARCHAR(64) NOT NULL,
    display_name NVARCHAR(255) NOT NULL,
    version NVARCHAR(64) NOT NULL,
    kind NVARCHAR(64) NOT NULL,
    handle NVARCHAR(255) NOT NULL,
    desired_state NVARCHAR(20) NOT NULL CHECK(desired_state IN ('deployed', 'undeployed')),
    deployment_id NVARCHAR(255),
    origin NVARCHAR(255) NOT NULL,
    created_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    deployed_at DATETIME2(7),
    cp_sync_status  NVARCHAR(20) CHECK(cp_sync_status IN ('pending', 'success', 'failed')),
    cp_sync_info    NVARCHAR(MAX),
    cp_artifact_id  NVARCHAR(255),
    PRIMARY KEY (gateway_id, uuid),
    UNIQUE(gateway_id, kind, display_name, version),
    UNIQUE(gateway_id, kind, handle)
);

IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_artifacts_cp_artifact_id' AND object_id = OBJECT_ID(N'dbo.artifacts'))
CREATE INDEX idx_artifacts_cp_artifact_id ON dbo.artifacts(gateway_id, cp_artifact_id) WHERE cp_artifact_id IS NOT NULL;

-- Per-resource-type tables

IF OBJECT_ID(N'dbo.rest_apis', N'U') IS NULL
CREATE TABLE dbo.rest_apis (
    uuid NVARCHAR(64) NOT NULL,
    gateway_id NVARCHAR(64) NOT NULL,
    configuration NVARCHAR(MAX) NOT NULL,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES dbo.artifacts(gateway_id, uuid) ON DELETE CASCADE
);

IF OBJECT_ID(N'dbo.websub_apis', N'U') IS NULL
CREATE TABLE dbo.websub_apis (
    uuid NVARCHAR(64) NOT NULL,
    gateway_id NVARCHAR(64) NOT NULL,
    configuration NVARCHAR(MAX) NOT NULL,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES dbo.artifacts(gateway_id, uuid) ON DELETE CASCADE
);

IF OBJECT_ID(N'dbo.webbroker_apis', N'U') IS NULL
CREATE TABLE dbo.webbroker_apis (
    uuid NVARCHAR(64) NOT NULL,
    gateway_id NVARCHAR(64) NOT NULL,
    configuration NVARCHAR(MAX) NOT NULL,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES dbo.artifacts(gateway_id, uuid) ON DELETE CASCADE
);

IF OBJECT_ID(N'dbo.llm_providers', N'U') IS NULL
CREATE TABLE dbo.llm_providers (
    uuid NVARCHAR(64) NOT NULL,
    gateway_id NVARCHAR(64) NOT NULL,
    configuration NVARCHAR(MAX) NOT NULL,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES dbo.artifacts(gateway_id, uuid) ON DELETE CASCADE
);

IF OBJECT_ID(N'dbo.llm_proxies', N'U') IS NULL
CREATE TABLE dbo.llm_proxies (
    uuid NVARCHAR(64) NOT NULL,
    gateway_id NVARCHAR(64) NOT NULL,
    configuration NVARCHAR(MAX) NOT NULL,
    provider_uuid NVARCHAR(64) NOT NULL,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES dbo.artifacts(gateway_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY(gateway_id, provider_uuid) REFERENCES dbo.llm_providers(gateway_id, uuid) ON DELETE NO ACTION
);

IF OBJECT_ID(N'dbo.mcp_proxies', N'U') IS NULL
CREATE TABLE dbo.mcp_proxies (
    uuid NVARCHAR(64) NOT NULL,
    gateway_id NVARCHAR(64) NOT NULL,
    configuration NVARCHAR(MAX) NOT NULL,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES dbo.artifacts(gateway_id, uuid) ON DELETE CASCADE
);

-- Table for custom TLS certificates
IF OBJECT_ID(N'dbo.certificates', N'U') IS NULL
CREATE TABLE dbo.certificates (
    uuid NVARCHAR(64) NOT NULL,
    gateway_id NVARCHAR(64) NOT NULL,
    name NVARCHAR(255) NOT NULL,
    certificate VARBINARY(MAX) NOT NULL,
    subject NVARCHAR(MAX) NOT NULL,
    issuer NVARCHAR(MAX) NOT NULL,
    not_before DATETIME2(7) NOT NULL,
    not_after DATETIME2(7) NOT NULL,
    cert_count INT NOT NULL DEFAULT 1,
    created_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (gateway_id, uuid),
    UNIQUE(gateway_id, name)
);

-- LLM Provider Templates table
IF OBJECT_ID(N'dbo.llm_provider_templates', N'U') IS NULL
CREATE TABLE dbo.llm_provider_templates (
    uuid NVARCHAR(64) NOT NULL,
    gateway_id NVARCHAR(64) NOT NULL,
    group_id NVARCHAR(255) NOT NULL,
    handle NVARCHAR(255) NOT NULL,
    version NVARCHAR(64) NOT NULL DEFAULT 'v1.0',
    configuration NVARCHAR(MAX) NOT NULL,
    created_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (gateway_id, uuid),
    UNIQUE(gateway_id, group_id, version)
);

-- Table for API keys
IF OBJECT_ID(N'dbo.api_keys', N'U') IS NULL
CREATE TABLE dbo.api_keys (
    uuid NVARCHAR(64) NOT NULL,
    gateway_id NVARCHAR(64) NOT NULL,
    name NVARCHAR(255) NOT NULL,
    api_key NVARCHAR(255) NOT NULL,
    masked_api_key NVARCHAR(255) NOT NULL,
    artifact_uuid NVARCHAR(64) NOT NULL,
    status NVARCHAR(20) NOT NULL CHECK(status IN ('active', 'revoked', 'expired')) DEFAULT 'active',
    created_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    created_by NVARCHAR(255) NOT NULL DEFAULT 'system',
    updated_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    expires_at DATETIME2(7) NULL,
    source NVARCHAR(64) NOT NULL DEFAULT 'local',
    external_ref_id NVARCHAR(255) NULL,
    issuer NVARCHAR(255) NULL DEFAULT NULL,
    PRIMARY KEY (gateway_id, api_key),
    CONSTRAINT uq_api_keys_artifact_name UNIQUE (gateway_id, artifact_uuid, name),
    CONSTRAINT uq_api_keys_uuid UNIQUE (gateway_id, uuid)
);

IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_key_status' AND object_id = OBJECT_ID(N'dbo.api_keys'))
CREATE INDEX idx_api_key_status ON dbo.api_keys(status);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_created_by' AND object_id = OBJECT_ID(N'dbo.api_keys'))
CREATE INDEX idx_created_by ON dbo.api_keys(created_by);

-- Subscription plans table (organization-scoped rate/billing plans)
IF OBJECT_ID(N'dbo.subscription_plans', N'U') IS NULL
CREATE TABLE dbo.subscription_plans (
    uuid NVARCHAR(64) NOT NULL,
    gateway_id NVARCHAR(64) NOT NULL,
    plan_name NVARCHAR(255) NOT NULL,
    billing_plan NVARCHAR(MAX),
    stop_on_quota_reach BIT DEFAULT 1,
    throttle_limit_count INT,
    throttle_limit_unit NVARCHAR(64),
    expiry_time DATETIME2(7),
    status NVARCHAR(20) NOT NULL CHECK(status IN ('ACTIVE', 'INACTIVE')) DEFAULT 'ACTIVE',
    created_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (gateway_id, uuid),
    UNIQUE(gateway_id, plan_name)
);

-- Subscriptions table (application-level subscriptions for REST APIs, even before deployment)
IF OBJECT_ID(N'dbo.subscriptions', N'U') IS NULL
CREATE TABLE dbo.subscriptions (
    uuid NVARCHAR(64) NOT NULL,
    gateway_id NVARCHAR(64) NOT NULL,
    api_id NVARCHAR(255) NOT NULL,
    application_id NVARCHAR(255),
    subscription_token_hash NVARCHAR(64) NOT NULL,
    subscription_plan_id NVARCHAR(64),
    billing_customer_id NVARCHAR(255),
    billing_subscription_id NVARCHAR(255),
    status NVARCHAR(20) NOT NULL CHECK(status IN ('ACTIVE', 'INACTIVE', 'REVOKED')) DEFAULT 'ACTIVE',
    created_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY (gateway_id, subscription_plan_id) REFERENCES dbo.subscription_plans(gateway_id, uuid),
    UNIQUE(gateway_id, api_id, subscription_token_hash)
);

IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscriptions_application_id' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscriptions_application_id ON dbo.subscriptions(application_id);

-- Table for gateway states (used by eventhub for multi-replica sync)
IF OBJECT_ID(N'dbo.gateway_states', N'U') IS NULL
CREATE TABLE dbo.gateway_states (
    gateway_id NVARCHAR(64) NOT NULL PRIMARY KEY,
    version_id NVARCHAR(255) NOT NULL DEFAULT '',
    updated_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME()
);

-- Table for events (used by eventhub for multi-replica sync)
IF OBJECT_ID(N'dbo.events', N'U') IS NULL
CREATE TABLE dbo.events (
    gateway_id NVARCHAR(64) NOT NULL,
    processed_timestamp DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    originated_timestamp DATETIME2(7) NOT NULL,
    entity_type NVARCHAR(64) NOT NULL,
    action NVARCHAR(20) NOT NULL CHECK(action IN ('CREATE', 'UPDATE', 'DELETE')),
    entity_id NVARCHAR(255) NOT NULL,
    event_id NVARCHAR(64) NOT NULL,
    event_data NVARCHAR(MAX) NOT NULL,
    PRIMARY KEY (gateway_id, event_id),
    FOREIGN KEY (gateway_id) REFERENCES dbo.gateway_states(gateway_id) ON DELETE CASCADE
);

IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_events_gateway_id_processed_timestamp' AND object_id = OBJECT_ID(N'dbo.events'))
CREATE INDEX idx_events_gateway_id_processed_timestamp ON dbo.events(gateway_id, processed_timestamp);

-- Applications
IF OBJECT_ID(N'dbo.applications', N'U') IS NULL
CREATE TABLE dbo.applications (
    application_uuid NVARCHAR(64) NOT NULL,
    gateway_id NVARCHAR(64) NOT NULL,
    application_id NVARCHAR(255) NOT NULL,
    application_name NVARCHAR(255) NOT NULL,
    application_type NVARCHAR(255) NOT NULL,
    created_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (gateway_id, application_uuid)
);

-- Application to API key mappings
IF OBJECT_ID(N'dbo.application_api_keys', N'U') IS NULL
CREATE TABLE dbo.application_api_keys (
    application_uuid NVARCHAR(64) NOT NULL,
    api_key_id NVARCHAR(64) NOT NULL,
    gateway_id NVARCHAR(64) NOT NULL,
    created_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (gateway_id, application_uuid, api_key_id),
    FOREIGN KEY (gateway_id, application_uuid) REFERENCES dbo.applications(gateway_id, application_uuid) ON DELETE CASCADE,
    FOREIGN KEY (gateway_id, api_key_id) REFERENCES dbo.api_keys(gateway_id, uuid) ON DELETE CASCADE
);

IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_app_api_keys_apikey' AND object_id = OBJECT_ID(N'dbo.application_api_keys'))
CREATE INDEX idx_app_api_keys_apikey ON dbo.application_api_keys(gateway_id, api_key_id);

-- Table for encrypted secrets (gateway_id + handle form the composite PK)
IF OBJECT_ID(N'dbo.secrets', N'U') IS NULL
CREATE TABLE dbo.secrets (
    gateway_id NVARCHAR(64) NOT NULL,
    handle NVARCHAR(255) NOT NULL,
    display_name NVARCHAR(255) NOT NULL,
    description NVARCHAR(MAX),
    ciphertext VARBINARY(MAX) NOT NULL,
    created_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (gateway_id, handle)
);

-- Table for encrypted per-artifact webhook secrets (gateway-scoped)
IF OBJECT_ID(N'dbo.webhook_secrets', N'U') IS NULL
CREATE TABLE dbo.webhook_secrets (
    uuid NVARCHAR(64) NOT NULL,
    gateway_id NVARCHAR(64) NOT NULL,
    artifact_uuid NVARCHAR(64) NOT NULL,
    name NVARCHAR(255) NOT NULL,
    display_name NVARCHAR(255) NOT NULL,
    ciphertext VARBINARY(MAX) NOT NULL,
    status NVARCHAR(20) NOT NULL CHECK(status IN ('active', 'revoked')) DEFAULT 'active',
    created_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (gateway_id, uuid),
    CONSTRAINT uq_webhook_secrets_artifact_name UNIQUE (gateway_id, artifact_uuid, name),
    FOREIGN KEY (gateway_id, artifact_uuid) REFERENCES dbo.artifacts(gateway_id, uuid) ON DELETE CASCADE
);

IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_webhook_secrets_artifact' AND object_id = OBJECT_ID(N'dbo.webhook_secrets'))
CREATE INDEX idx_webhook_secrets_artifact ON dbo.webhook_secrets(gateway_id, artifact_uuid);
