-- A2A Agent artifacts.
IF OBJECT_ID(N'dbo.agents', N'U') IS NULL
CREATE TABLE dbo.agents (
    uuid NVARCHAR(64) NOT NULL,
    gateway_id NVARCHAR(64) NOT NULL,
    configuration NVARCHAR(MAX) NOT NULL,
    signed_public_card NVARCHAR(MAX) NULL,
    signed_protected_card NVARCHAR(MAX) NULL,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES dbo.artifacts(gateway_id, uuid) ON DELETE CASCADE
);
