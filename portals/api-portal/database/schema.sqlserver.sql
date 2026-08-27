/*
 *  Copyright (c) 2026, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

-- Schema for the SQL Server dialect -- kept in sync manually
-- with the other schema.*.sql files. See src/db/driver.js for the query
-- layer that targets this schema.

-- Organizations table
IF OBJECT_ID(N'dbo.organizations', N'U') IS NULL
CREATE TABLE dbo.organizations (
    uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    display_name NVARCHAR(255) NOT NULL,
    business_owner NVARCHAR(255),
    business_owner_contact VARCHAR(255),
    business_owner_email VARCHAR(255),
    handle VARCHAR(255) NOT NULL,
    idp_ref_id VARCHAR(255) NOT NULL,
    cp_ref_id VARCHAR(255),
    configuration NVARCHAR(MAX) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    UNIQUE(portal_id, handle),
    UNIQUE(portal_id, display_name)
);

-- Views table (portal-scoped grouping of APIs for gateway/portal visibility)
IF OBJECT_ID(N'dbo.views', N'U') IS NULL
CREATE TABLE dbo.views (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    handle VARCHAR(255) NOT NULL,
    display_name NVARCHAR(255) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_view_handle_org_uuid' AND object_id = OBJECT_ID(N'dbo.views'))
CREATE UNIQUE INDEX uq_view_handle_org_uuid ON dbo.views(handle, org_uuid, portal_id);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_view_org_uuid' AND object_id = OBJECT_ID(N'dbo.views'))
CREATE INDEX idx_view_org_uuid ON dbo.views(org_uuid, portal_id);

-- Organization Assets table (per-view branding/content assets, e.g. logos, docs)
IF OBJECT_ID(N'dbo.organization_assets', N'U') IS NULL
CREATE TABLE dbo.organization_assets (
    uuid VARCHAR(40) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    file_content VARBINARY(MAX) NOT NULL,
    file_type VARCHAR(20) NOT NULL,
    file_path VARCHAR(255) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    view_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION,
    -- CASCADE: an org asset is meaningless once its view is gone.
    FOREIGN KEY (portal_id, view_uuid) REFERENCES views(portal_id, uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_organization_asset_type_name_path_org_view' AND object_id = OBJECT_ID(N'dbo.organization_assets'))
CREATE UNIQUE INDEX uq_organization_asset_type_name_path_org_view ON dbo.organization_assets(file_type, file_name, file_path, org_uuid, view_uuid, portal_id);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_organization_asset_org_uuid' AND object_id = OBJECT_ID(N'dbo.organization_assets'))
CREATE INDEX idx_organization_asset_org_uuid ON dbo.organization_assets(org_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_organization_asset_view_uuid' AND object_id = OBJECT_ID(N'dbo.organization_assets'))
CREATE INDEX idx_organization_asset_view_uuid ON dbo.organization_assets(view_uuid);

-- Labels table (portal-scoped labels used for gateway/view assignment)
IF OBJECT_ID(N'dbo.labels', N'U') IS NULL
CREATE TABLE dbo.labels (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    handle VARCHAR(255) NOT NULL,
    display_name NVARCHAR(255) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_label_handle_org_uuid' AND object_id = OBJECT_ID(N'dbo.labels'))
CREATE UNIQUE INDEX uq_label_handle_org_uuid ON dbo.labels(handle, org_uuid, portal_id);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_label_org_uuid' AND object_id = OBJECT_ID(N'dbo.labels'))
CREATE INDEX idx_label_org_uuid ON dbo.labels(org_uuid, portal_id);

-- Tags table (portal-scoped free-form API tags)
IF OBJECT_ID(N'dbo.tags', N'U') IS NULL
CREATE TABLE dbo.tags (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    name NVARCHAR(255) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_tag_name_org_uuid' AND object_id = OBJECT_ID(N'dbo.tags'))
CREATE UNIQUE INDEX uq_tag_name_org_uuid ON dbo.tags(name, org_uuid, portal_id);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_tag_org_uuid' AND object_id = OBJECT_ID(N'dbo.tags'))
CREATE INDEX idx_tag_org_uuid ON dbo.tags(org_uuid, portal_id);

-- View-Label mappings (many-to-many: which labels belong to a view)
IF OBJECT_ID(N'dbo.view_label_mappings', N'U') IS NULL
CREATE TABLE dbo.view_label_mappings (
    uuid VARCHAR(40) NOT NULL,
    view_uuid VARCHAR(40) NOT NULL,
    label_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, view_uuid) REFERENCES views(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, label_uuid) REFERENCES labels(portal_id, uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_view_label_mappings_label_view' AND object_id = OBJECT_ID(N'dbo.view_label_mappings'))
CREATE UNIQUE INDEX uq_view_label_mappings_label_view ON dbo.view_label_mappings(portal_id, label_uuid, view_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_view_label_mappings_view_uuid' AND object_id = OBJECT_ID(N'dbo.view_label_mappings'))
CREATE INDEX idx_view_label_mappings_view_uuid ON dbo.view_label_mappings(view_uuid);

-- API Metadata table (core record for REST APIs, MCP servers, AI agents, etc.)
-- API is a portal-managed entity: portal_id identifies which portal owns it.
IF OBJECT_ID(N'dbo.api_metadata', N'U') IS NULL
CREATE TABLE dbo.api_metadata (
    uuid VARCHAR(40) NOT NULL,
    ref_id VARCHAR(255),
    name NVARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL,
    description NVARCHAR(1023),
    version VARCHAR(30) NOT NULL,
    type VARCHAR(20) NOT NULL,
    agent_visibility VARCHAR(255) NOT NULL DEFAULT 'VISIBLE',
    technical_owner NVARCHAR(255),
    technical_owner_email VARCHAR(255),
    business_owner NVARCHAR(255),
    business_owner_email VARCHAR(255),
    sandbox_url VARCHAR(255),
    production_url VARCHAR(255),
    metadata_search NVARCHAR(MAX),
    handle VARCHAR(255) NOT NULL,
    -- Nullable: preserved to keep API records alive when an org is removed.
    -- Nullification is handled by the application layer; ON DELETE NO ACTION is
    -- used because a composite FK cannot partially SET NULL while portal_id is NOT NULL.
    org_uuid VARCHAR(40),
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
-- org_uuid, ref_id, and handle are all nullable/optional in combination here. SQL Server's
-- plain UNIQUE INDEX treats NULL as equal to NULL (unlike Postgres/SQLite), so a bare
-- composite index would wrongly block a second row once one NULL-org_uuid combination
-- existed. Filtering to org_uuid IS NOT NULL (and ref_id IS NOT NULL where relevant)
-- reproduces the Postgres/SQLite "NULL never collides" semantics.
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_metadata_name_version_org' AND object_id = OBJECT_ID(N'dbo.api_metadata'))
CREATE UNIQUE INDEX uq_api_metadata_name_version_org ON dbo.api_metadata(name, version, org_uuid, portal_id) WHERE org_uuid IS NOT NULL;
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_metadata_org_ref_id' AND object_id = OBJECT_ID(N'dbo.api_metadata'))
CREATE UNIQUE INDEX uq_api_metadata_org_ref_id ON dbo.api_metadata(org_uuid, ref_id, portal_id) WHERE org_uuid IS NOT NULL AND ref_id IS NOT NULL;
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_metadata_handle_org' AND object_id = OBJECT_ID(N'dbo.api_metadata'))
CREATE UNIQUE INDEX uq_api_metadata_handle_org ON dbo.api_metadata(handle, org_uuid, portal_id) WHERE org_uuid IS NOT NULL;
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_metadata_status' AND object_id = OBJECT_ID(N'dbo.api_metadata'))
CREATE INDEX idx_api_metadata_status ON dbo.api_metadata(status);

-- API Contents table (spec files, docs, icons, etc. attached to an API)
IF OBJECT_ID(N'dbo.api_contents', N'U') IS NULL
CREATE TABLE dbo.api_contents (
    uuid VARCHAR(40) NOT NULL,
    api_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    file_content VARBINARY(MAX) NOT NULL,
    type VARCHAR(64) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    lookup_key VARCHAR(255),
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, api_uuid) REFERENCES api_metadata(portal_id, uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_content_api_type_file_name' AND object_id = OBJECT_ID(N'dbo.api_contents'))
CREATE UNIQUE INDEX uq_api_content_api_type_file_name ON dbo.api_contents(api_uuid, type, file_name, portal_id);
-- lookup_key is nullable -- filtered so multiple NULL-lookup_key rows per (api_uuid, type)
-- are allowed, matching Postgres/SQLite behavior (see the note on api_metadata above).
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_content_api_type_lookup_key' AND object_id = OBJECT_ID(N'dbo.api_contents'))
CREATE UNIQUE INDEX uq_api_content_api_type_lookup_key ON dbo.api_contents(api_uuid, type, lookup_key, portal_id) WHERE lookup_key IS NOT NULL;

-- API-Label mappings (many-to-many: which labels are attached to an API)
IF OBJECT_ID(N'dbo.api_label_mappings', N'U') IS NULL
CREATE TABLE dbo.api_label_mappings (
    uuid VARCHAR(40) NOT NULL,
    api_uuid VARCHAR(40) NOT NULL,
    label_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, api_uuid) REFERENCES api_metadata(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, label_uuid) REFERENCES labels(portal_id, uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_label_mappings_label_api' AND object_id = OBJECT_ID(N'dbo.api_label_mappings'))
CREATE UNIQUE INDEX uq_api_label_mappings_label_api ON dbo.api_label_mappings(portal_id, label_uuid, api_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_label_mappings_api_uuid' AND object_id = OBJECT_ID(N'dbo.api_label_mappings'))
CREATE INDEX idx_api_label_mappings_api_uuid ON dbo.api_label_mappings(api_uuid);

-- API-Tag mappings (many-to-many: which tags are attached to an API)
IF OBJECT_ID(N'dbo.api_tag_mappings', N'U') IS NULL
CREATE TABLE dbo.api_tag_mappings (
    uuid VARCHAR(40) NOT NULL,
    api_uuid VARCHAR(40) NOT NULL,
    tag_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, api_uuid) REFERENCES api_metadata(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, tag_uuid) REFERENCES tags(portal_id, uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_tag_mappings_tag_api' AND object_id = OBJECT_ID(N'dbo.api_tag_mappings'))
CREATE UNIQUE INDEX uq_api_tag_mappings_tag_api ON dbo.api_tag_mappings(portal_id, tag_uuid, api_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_tag_mappings_api_uuid' AND object_id = OBJECT_ID(N'dbo.api_tag_mappings'))
CREATE INDEX idx_api_tag_mappings_api_uuid ON dbo.api_tag_mappings(api_uuid);

-- Subscription Plans table (portal-scoped rate/billing plans)
-- Throttling limits live in subscription_plan_limits (one row per limit).
IF OBJECT_ID(N'dbo.subscription_plans', N'U') IS NULL
CREATE TABLE dbo.subscription_plans (
    uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    display_name NVARCHAR(255) NOT NULL,
    description NVARCHAR(1023),
    ref_id VARCHAR(255),
    -- Nullable: same ON DELETE NO ACTION rationale as api_metadata.org_uuid above.
    org_uuid VARCHAR(40),
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
-- org_uuid is nullable -- filtered for the same NULL-handling reason as api_metadata above.
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_subscription_plan_org_handle' AND object_id = OBJECT_ID(N'dbo.subscription_plans'))
CREATE UNIQUE INDEX uq_subscription_plan_org_handle ON dbo.subscription_plans(org_uuid, handle, portal_id) WHERE org_uuid IS NOT NULL;

-- Subscription Plan Limits table (throttling limits for a plan)
IF OBJECT_ID(N'dbo.subscription_plan_limits', N'U') IS NULL
CREATE TABLE dbo.subscription_plan_limits (
    uuid VARCHAR(40) NOT NULL,
    plan_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    limit_type VARCHAR(20) NOT NULL DEFAULT 'REQUEST_COUNT',
    time_unit VARCHAR(20),
    time_amount INT NOT NULL DEFAULT 1,
    limit_count BIGINT NOT NULL,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, plan_uuid) REFERENCES subscription_plans(portal_id, uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscription_plan_limits_plan' AND object_id = OBJECT_ID(N'dbo.subscription_plan_limits'))
CREATE INDEX idx_subscription_plan_limits_plan ON dbo.subscription_plan_limits(plan_uuid);
-- Split into two filtered unique indexes because time_unit is nullable (see the
-- postgres schema for the full rationale); this is already how the source model
-- declares it (two named partial indexes), so all three dialects agree exactly.
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_subscription_plan_limits' AND object_id = OBJECT_ID(N'dbo.subscription_plan_limits'))
CREATE UNIQUE INDEX uq_subscription_plan_limits ON dbo.subscription_plan_limits(plan_uuid, limit_type, time_amount, time_unit, portal_id) WHERE time_unit IS NOT NULL;
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_subscription_plan_limits_null_unit' AND object_id = OBJECT_ID(N'dbo.subscription_plan_limits'))
CREATE UNIQUE INDEX uq_subscription_plan_limits_null_unit ON dbo.subscription_plan_limits(plan_uuid, limit_type, time_amount, portal_id) WHERE time_unit IS NULL;

-- API-Subscription Plan mappings (many-to-many: which plans an API offers)
IF OBJECT_ID(N'dbo.api_subscription_plan_mappings', N'U') IS NULL
CREATE TABLE dbo.api_subscription_plan_mappings (
    uuid VARCHAR(40) NOT NULL,
    api_uuid VARCHAR(40) NOT NULL,
    plan_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, api_uuid) REFERENCES api_metadata(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, plan_uuid) REFERENCES subscription_plans(portal_id, uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_subscription_plan_mappings_plan_api' AND object_id = OBJECT_ID(N'dbo.api_subscription_plan_mappings'))
CREATE UNIQUE INDEX uq_api_subscription_plan_mappings_plan_api ON dbo.api_subscription_plan_mappings(portal_id, plan_uuid, api_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_subscription_plan_mappings_api_uuid' AND object_id = OBJECT_ID(N'dbo.api_subscription_plan_mappings'))
CREATE INDEX idx_api_subscription_plan_mappings_api_uuid ON dbo.api_subscription_plan_mappings(api_uuid);

-- Key Managers table (portal-scoped identity providers used to validate app keys)
IF OBJECT_ID(N'dbo.key_managers', N'U') IS NULL
CREATE TABLE dbo.key_managers (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    handle VARCHAR(255) NOT NULL,
    display_name NVARCHAR(255) NOT NULL,
    enabled SMALLINT NOT NULL DEFAULT 1,
    token_endpoint VARCHAR(255) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_key_manager_org_handle' AND object_id = OBJECT_ID(N'dbo.key_managers'))
CREATE UNIQUE INDEX uq_key_manager_org_handle ON dbo.key_managers(org_uuid, handle, portal_id);

-- Applications table (portal-scoped developer-created consumer apps)
IF OBJECT_ID(N'dbo.applications', N'U') IS NULL
CREATE TABLE dbo.applications (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_by VARCHAR(255) NOT NULL,
    display_name NVARCHAR(255) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    description NVARCHAR(1023),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_application_org_created_by' AND object_id = OBJECT_ID(N'dbo.applications'))
CREATE INDEX idx_application_org_created_by ON dbo.applications(org_uuid, portal_id, created_by);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_application_org_handle' AND object_id = OBJECT_ID(N'dbo.applications'))
CREATE UNIQUE INDEX uq_application_org_handle ON dbo.applications(org_uuid, handle, portal_id);

-- Application-KeyManager mappings (per-KM OAuth2 client registration for an application)
IF OBJECT_ID(N'dbo.app_key_mappings', N'U') IS NULL
CREATE TABLE dbo.app_key_mappings (
    uuid VARCHAR(40) NOT NULL,
    app_uuid VARCHAR(40) NOT NULL,
    km_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    as_client_id VARCHAR(255),
    type VARCHAR(20) NOT NULL DEFAULT 'PRODUCTION',
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, app_uuid) REFERENCES applications(portal_id, uuid) ON DELETE NO ACTION,
    FOREIGN KEY (portal_id, km_uuid) REFERENCES key_managers(portal_id, uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_app_key_mappings_app_uuid' AND object_id = OBJECT_ID(N'dbo.app_key_mappings'))
CREATE INDEX idx_app_key_mappings_app_uuid ON dbo.app_key_mappings(app_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_app_key_mappings_km_uuid' AND object_id = OBJECT_ID(N'dbo.app_key_mappings'))
CREATE INDEX idx_app_key_mappings_km_uuid ON dbo.app_key_mappings(km_uuid);

-- Subscriptions table (portal-scoped application-level subscriptions to an API)
IF OBJECT_ID(N'dbo.subscriptions', N'U') IS NULL
CREATE TABLE dbo.subscriptions (
    uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    api_uuid VARCHAR(40) NOT NULL,
    -- Nullable: same ON DELETE NO ACTION rationale as api_metadata.org_uuid above.
    plan_uuid VARCHAR(40),
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    -- token is globally unique across the entire database (not just per-portal) so
    -- a subscription token cannot accidentally be reused by another portal on the same DB.
    token VARCHAR(512),
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, api_uuid) REFERENCES api_metadata(portal_id, uuid) ON DELETE NO ACTION,
    FOREIGN KEY (portal_id, plan_uuid) REFERENCES subscription_plans(portal_id, uuid) ON DELETE NO ACTION,
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscription_org_created_by' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscription_org_created_by ON dbo.subscriptions(org_uuid, portal_id, created_by);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscription_org_api_uuid' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscription_org_api_uuid ON dbo.subscriptions(org_uuid, portal_id, api_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscription_plan_uuid' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscription_plan_uuid ON dbo.subscriptions(plan_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscription_status' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscription_status ON dbo.subscriptions(status);
-- api_uuid is only ever a trailing column above (org_uuid, api_uuid) -- add a
-- dedicated leading index so single-column api_uuid lookups/joins stay indexed.
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscription_api_uuid' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscription_api_uuid ON dbo.subscriptions(api_uuid);
-- token is a single nullable column with a uniqueness requirement. A plain UNIQUE
-- constraint would let SQL Server allow only ONE NULL-token row total across the
-- whole table (unlike Postgres/SQLite, which allow unlimited NULLs). Filtering to
-- token IS NOT NULL reproduces the intended "many token-less subscriptions" behavior.
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_subscription_token' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE UNIQUE INDEX uq_subscription_token ON dbo.subscriptions(token) WHERE token IS NOT NULL;

-- API Keys table (portal-scoped standalone, non-OAuth2 API key credentials for an API)
IF OBJECT_ID(N'dbo.api_keys', N'U') IS NULL
CREATE TABLE dbo.api_keys (
    uuid VARCHAR(40) NOT NULL,
    api_uuid VARCHAR(40) NOT NULL,
    -- Nullable: same ON DELETE NO ACTION rationale as api_metadata.org_uuid above.
    subscription_uuid VARCHAR(40),
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    handle VARCHAR(128) NOT NULL,
    display_name NVARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    expires_at DATETIME2(7),
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    revoked_at DATETIME2(7),
    revoked_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, api_uuid) REFERENCES api_metadata(portal_id, uuid) ON DELETE NO ACTION,
    FOREIGN KEY (portal_id, subscription_uuid) REFERENCES subscriptions(portal_id, uuid) ON DELETE NO ACTION,
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION,
    CONSTRAINT chk_api_key_revoked
        CHECK ((revoked_at IS NULL AND status != 'REVOKED') OR (revoked_at IS NOT NULL AND status = 'REVOKED'))
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_key_org_api_uuid' AND object_id = OBJECT_ID(N'dbo.api_keys'))
CREATE INDEX idx_api_key_org_api_uuid ON dbo.api_keys(org_uuid, api_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_key_subscription_uuid' AND object_id = OBJECT_ID(N'dbo.api_keys'))
CREATE INDEX idx_api_key_subscription_uuid ON dbo.api_keys(subscription_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_key_status' AND object_id = OBJECT_ID(N'dbo.api_keys'))
CREATE INDEX idx_api_key_status ON dbo.api_keys(status);
-- api_uuid is only ever a trailing column above (org_uuid, api_uuid) -- add a
-- dedicated leading index so single-column api_uuid lookups/joins stay indexed.
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_key_api_uuid' AND object_id = OBJECT_ID(N'dbo.api_keys'))
CREATE INDEX idx_api_key_api_uuid ON dbo.api_keys(api_uuid);
-- Handle is the caller-facing id used to address a key within an API, so it must be
-- unique per (org, portal, api). Enforced here for a race-free guarantee.
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_key_org_api_handle' AND object_id = OBJECT_ID(N'dbo.api_keys'))
CREATE UNIQUE INDEX uq_api_key_org_api_handle ON dbo.api_keys(org_uuid, api_uuid, handle, portal_id);

-- API Key-Application mappings (which application an API key was issued to)
-- key_uuid IS the api_keys.uuid — no separate surrogate key on this table.
IF OBJECT_ID(N'dbo.api_key_app_mappings', N'U') IS NULL
CREATE TABLE dbo.api_key_app_mappings (
    key_uuid VARCHAR(40) NOT NULL,
    app_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, key_uuid),
    FOREIGN KEY (portal_id, key_uuid) REFERENCES api_keys(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, app_uuid) REFERENCES applications(portal_id, uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_key_app_mappings_app_uuid' AND object_id = OBJECT_ID(N'dbo.api_key_app_mappings'))
CREATE INDEX idx_api_key_app_mappings_app_uuid ON dbo.api_key_app_mappings(app_uuid);

-- API Workflows table (portal-scoped agent/automation workflows published under a view)
IF OBJECT_ID(N'dbo.api_workflows', N'U') IS NULL
CREATE TABLE dbo.api_workflows (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    view_uuid VARCHAR(40) NOT NULL,
    display_name NVARCHAR(255) NOT NULL,
    description NVARCHAR(1023) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    agent_prompt VARBINARY(MAX) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PUBLISHED',
    file_content VARBINARY(MAX),
    content_type VARCHAR(255),
    agent_visibility VARCHAR(255) NOT NULL DEFAULT 'VISIBLE',
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION,
    FOREIGN KEY (portal_id, view_uuid) REFERENCES views(portal_id, uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_workflow_org_view_handle' AND object_id = OBJECT_ID(N'dbo.api_workflows'))
CREATE UNIQUE INDEX uq_api_workflow_org_view_handle ON dbo.api_workflows(org_uuid, view_uuid, handle, portal_id);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_workflow_view_uuid' AND object_id = OBJECT_ID(N'dbo.api_workflows'))
CREATE INDEX idx_api_workflow_view_uuid ON dbo.api_workflows(view_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_workflow_status' AND object_id = OBJECT_ID(N'dbo.api_workflows'))
CREATE INDEX idx_api_workflow_status ON dbo.api_workflows(status);

-- Audit table (write-only mutation trail; no FK on performed_by so history
-- survives deletion of the referenced user_idp_references row)
IF OBJECT_ID(N'dbo.audit', N'U') IS NULL
CREATE TABLE dbo.audit (
    uuid VARCHAR(40) NOT NULL,
    action VARCHAR(50) NOT NULL,
    resource_uuid VARCHAR(40) NOT NULL,
    resource_type VARCHAR(50),
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    performed_by VARCHAR(255),
    performed_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_audit_org_uuid' AND object_id = OBJECT_ID(N'dbo.audit'))
CREATE INDEX idx_audit_org_uuid ON dbo.audit(org_uuid, portal_id);

-- Events table (outbox: one row per domain event; payload never contains plaintext key secrets)
IF OBJECT_ID(N'dbo.events', N'U') IS NULL
CREATE TABLE dbo.events (
    uuid VARCHAR(40) NOT NULL,
    type VARCHAR(128) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_uuid VARCHAR(40) NOT NULL,
    payload NVARCHAR(MAX) NOT NULL DEFAULT '{}',
    occurred_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_event_status_occurred_at' AND object_id = OBJECT_ID(N'dbo.events'))
CREATE INDEX idx_event_status_occurred_at ON dbo.events(status, occurred_at);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_event_org_uuid' AND object_id = OBJECT_ID(N'dbo.events'))
CREATE INDEX idx_event_org_uuid ON dbo.events(org_uuid, portal_id);

-- Event Deliveries table (one row per event x webhook subscriber; encrypted_fields
-- holds per-subscriber ciphertext so plaintext never lives in events)
IF OBJECT_ID(N'dbo.event_deliveries', N'U') IS NULL
CREATE TABLE dbo.event_deliveries (
    uuid VARCHAR(40) NOT NULL,
    event_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    subscriber_id VARCHAR(128) NOT NULL,
    target_url VARCHAR(1023) NOT NULL,
    encrypted_fields NVARCHAR(MAX) DEFAULT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    last_http_status INT,
    last_error VARCHAR(255),
    last_attempt_at DATETIME2(7),
    delivered_at DATETIME2(7),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, event_uuid) REFERENCES events(portal_id, uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_event_delivery_event_uuid' AND object_id = OBJECT_ID(N'dbo.event_deliveries'))
CREATE INDEX idx_event_delivery_event_uuid ON dbo.event_deliveries(event_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_event_delivery_event_subscriber' AND object_id = OBJECT_ID(N'dbo.event_deliveries'))
CREATE UNIQUE INDEX uq_event_delivery_event_subscriber ON dbo.event_deliveries(portal_id, event_uuid, subscriber_id);

-- Sessions table, used by connect-mssql-v2 (or equivalent) for server-side Express session storage.
-- Intentionally excluded from the portal_id composite-PK pattern: portal_id is
-- stored inside the sess JSON payload instead of as a schema column.
IF OBJECT_ID(N'dbo.sessions', N'U') IS NULL
CREATE TABLE dbo.sessions (
    sid VARCHAR(255) PRIMARY KEY,
    sess NVARCHAR(MAX) NOT NULL,
    expire DATETIME2(7) NOT NULL
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_session_expire' AND object_id = OBJECT_ID(N'dbo.sessions'))
CREATE INDEX idx_session_expire ON dbo.sessions(expire);

-- User IdP References table (one durable record per IdP `sub` claim scoped to a portal;
-- referenced by uuid from created_by/updated_by-style columns elsewhere WITHOUT a foreign
-- key, so those columns keep pointing at a uuid after the row here is deleted)
IF OBJECT_ID(N'dbo.user_idp_references', N'U') IS NULL
CREATE TABLE dbo.user_idp_references (
    uuid VARCHAR(40) NOT NULL,
    idp_id VARCHAR(255) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid)
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_user_idp_references_idpid_portal' AND object_id = OBJECT_ID(N'dbo.user_idp_references'))
CREATE UNIQUE INDEX uq_user_idp_references_idpid_portal ON dbo.user_idp_references(idp_id, portal_id);

-- User-Organization mappings (live membership record -- both sides cascade on delete,
-- unlike the "hanging creator" created_by/updated_by pattern used elsewhere)
IF OBJECT_ID(N'dbo.user_organization_mappings', N'U') IS NULL
CREATE TABLE dbo.user_organization_mappings (
    user_uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    PRIMARY KEY (portal_id, user_uuid, org_uuid),
    FOREIGN KEY (portal_id, user_uuid) REFERENCES user_idp_references(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_user_organization_mappings_org_uuid' AND object_id = OBJECT_ID(N'dbo.user_organization_mappings'))
CREATE INDEX idx_user_organization_mappings_org_uuid ON dbo.user_organization_mappings(org_uuid);

-- Webhook Subscribers table (portal-scoped outbound event subscribers)
IF OBJECT_ID(N'dbo.webhook_subscribers', N'U') IS NULL
CREATE TABLE dbo.webhook_subscribers (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    handle VARCHAR(255) NOT NULL,
    display_name NVARCHAR(255) NOT NULL,
    target_url VARCHAR(1023) NOT NULL,
    secret_enc VARBINARY(MAX),
    event_patterns NVARCHAR(MAX) DEFAULT '[]',
    enabled SMALLINT NOT NULL DEFAULT 1,
    timeout_ms INT NOT NULL DEFAULT 5000,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_webhook_subscriber_org_handle' AND object_id = OBJECT_ID(N'dbo.webhook_subscribers'))
CREATE UNIQUE INDEX uq_webhook_subscriber_org_handle ON dbo.webhook_subscribers(org_uuid, handle, portal_id);
