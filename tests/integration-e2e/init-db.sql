-- platform-api and gateway-controller keep separate schemas with overlapping
-- table names (artifacts, gateways, subscriptions, ...), so they must live in
-- different databases on the shared server.
CREATE DATABASE platform_api;
CREATE DATABASE gateway_test;
-- Second gateway-controller store for the multi-gateway scenario.
CREATE DATABASE gateway_test2;
-- Developer portal store for the @devportal scenario. Postgres is used (not the
-- devportal's default SQLite) because the devportal's org-update path relies on
-- UPDATE ... RETURNING rows, which SQLite does not provide. Tables are
-- auto-created by the devportal on startup (sequelize.sync()).
CREATE DATABASE devportal;

-- platform-api only auto-runs schema DDL for SQLite; against an external
-- database it expects the schema to be pre-provisioned by the operator. Apply
-- the platform-api schema to its database here so the file-based org seeding at
-- startup finds its tables. (gateway-controller still auto-migrates its own
-- gateway_test / gateway_test2 schema.)
\connect platform_api
\i /schema/schema.postgres.sql

-- Same for the developer portal database (it does not auto-create its schema on
-- an external postgres — its own postgres compose loads this dump at init too).
\connect devportal
\i /devportal-schema/schema.postgres.sql
