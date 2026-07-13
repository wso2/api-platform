-- SQL Server Schema for Event-Gateway-Controller-specific tables
-- Applied against the same database gateway-controller (core) opened, after
-- core's own schema (gateway-controller-db.sqlserver.sql) has been applied.
-- Core's schema scripts do not define these tables — only this module does.
-- Every object is guarded by IF OBJECT_ID/IF NOT EXISTS so the batch is
-- idempotent, matching core's own SQL Server schema conventions.

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
