-- GraphQL API artifacts.
IF OBJECT_ID(N'dbo.graphql_apis', N'U') IS NULL
CREATE TABLE dbo.graphql_apis (
    uuid NVARCHAR(64) NOT NULL,
    gateway_id NVARCHAR(64) NOT NULL,
    configuration NVARCHAR(MAX) NOT NULL,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES dbo.artifacts(gateway_id, uuid) ON DELETE CASCADE
);
