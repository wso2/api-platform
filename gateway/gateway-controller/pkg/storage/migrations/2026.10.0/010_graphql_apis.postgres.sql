-- GraphQL API artifacts.
CREATE TABLE IF NOT EXISTS graphql_apis (
    uuid TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    configuration TEXT NOT NULL,
    PRIMARY KEY (gateway_id, uuid),
    FOREIGN KEY(gateway_id, uuid) REFERENCES artifacts(gateway_id, uuid) ON DELETE CASCADE
);
-- Postgres: the creating role owns this table. If a DBA applies the migration
-- rather than the controller's own database user, grant it to that user or
-- the gateway cannot read it.
