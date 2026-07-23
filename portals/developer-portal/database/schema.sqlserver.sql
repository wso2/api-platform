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
IF OBJECT_ID(N'dbo.dp_organizations', N'U') IS NULL
CREATE TABLE dbo.dp_organizations (
    uuid VARCHAR(40) PRIMARY KEY,
    display_name NVARCHAR(255) NOT NULL UNIQUE,
    business_owner NVARCHAR(255),
    business_owner_contact VARCHAR(255),
    business_owner_email VARCHAR(255),
    handle VARCHAR(255) NOT NULL UNIQUE,
    idp_ref_id VARCHAR(255) NOT NULL,
    cp_ref_id VARCHAR(255),
    configuration NVARCHAR(MAX) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME()
);

-- Views table (organization-scoped grouping of APIs for gateway/portal visibility)
IF OBJECT_ID(N'dbo.dp_views', N'U') IS NULL
CREATE TABLE dbo.dp_views (
    uuid VARCHAR(40) PRIMARY KEY,
    org_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    display_name NVARCHAR(255) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_view_handle_org_uuid' AND object_id = OBJECT_ID(N'dbo.dp_views'))
CREATE UNIQUE INDEX uq_view_handle_org_uuid ON dbo.dp_views(handle, org_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_view_org_uuid' AND object_id = OBJECT_ID(N'dbo.dp_views'))
CREATE INDEX idx_view_org_uuid ON dbo.dp_views(org_uuid);

-- Organization Assets table (per-view branding/content assets, e.g. logos, docs)
IF OBJECT_ID(N'dbo.dp_organization_assets', N'U') IS NULL
CREATE TABLE dbo.dp_organization_assets (
    uuid VARCHAR(40) PRIMARY KEY,
    file_name VARCHAR(255) NOT NULL,
    file_content VARBINARY(MAX) NOT NULL,
    file_type VARCHAR(20) NOT NULL,
    file_path VARCHAR(255) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    view_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION,
    -- CASCADE: an org asset is meaningless once its view is gone.
    FOREIGN KEY (view_uuid) REFERENCES dp_views(uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_organization_asset_type_name_path_org_view' AND object_id = OBJECT_ID(N'dbo.dp_organization_assets'))
CREATE UNIQUE INDEX uq_organization_asset_type_name_path_org_view ON dbo.dp_organization_assets(file_type, file_name, file_path, org_uuid, view_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_organization_asset_org_uuid' AND object_id = OBJECT_ID(N'dbo.dp_organization_assets'))
CREATE INDEX idx_organization_asset_org_uuid ON dbo.dp_organization_assets(org_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_organization_asset_view_uuid' AND object_id = OBJECT_ID(N'dbo.dp_organization_assets'))
CREATE INDEX idx_organization_asset_view_uuid ON dbo.dp_organization_assets(view_uuid);

-- Labels table (organization-scoped labels used for gateway/view assignment)
IF OBJECT_ID(N'dbo.dp_labels', N'U') IS NULL
CREATE TABLE dbo.dp_labels (
    uuid VARCHAR(40) PRIMARY KEY,
    org_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    display_name NVARCHAR(255) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_label_handle_org_uuid' AND object_id = OBJECT_ID(N'dbo.dp_labels'))
CREATE UNIQUE INDEX uq_label_handle_org_uuid ON dbo.dp_labels(handle, org_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_label_org_uuid' AND object_id = OBJECT_ID(N'dbo.dp_labels'))
CREATE INDEX idx_label_org_uuid ON dbo.dp_labels(org_uuid);

-- Tags table (organization-scoped free-form API tags)
IF OBJECT_ID(N'dbo.dp_tags', N'U') IS NULL
CREATE TABLE dbo.dp_tags (
    uuid VARCHAR(40) PRIMARY KEY,
    org_uuid VARCHAR(40) NOT NULL,
    name NVARCHAR(255) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_tag_name_org_uuid' AND object_id = OBJECT_ID(N'dbo.dp_tags'))
CREATE UNIQUE INDEX uq_tag_name_org_uuid ON dbo.dp_tags(name, org_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_tag_org_uuid' AND object_id = OBJECT_ID(N'dbo.dp_tags'))
CREATE INDEX idx_tag_org_uuid ON dbo.dp_tags(org_uuid);

-- View-Label mappings (many-to-many: which labels belong to a view)
IF OBJECT_ID(N'dbo.dp_view_label_mappings', N'U') IS NULL
CREATE TABLE dbo.dp_view_label_mappings (
    uuid VARCHAR(40) PRIMARY KEY,
    view_uuid VARCHAR(40) NOT NULL,
    label_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (view_uuid) REFERENCES dp_views(uuid) ON DELETE CASCADE,
    FOREIGN KEY (label_uuid) REFERENCES dp_labels(uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_view_label_mappings_label_view' AND object_id = OBJECT_ID(N'dbo.dp_view_label_mappings'))
CREATE UNIQUE INDEX uq_view_label_mappings_label_view ON dbo.dp_view_label_mappings(label_uuid, view_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_view_label_mappings_view_uuid' AND object_id = OBJECT_ID(N'dbo.dp_view_label_mappings'))
CREATE INDEX idx_view_label_mappings_view_uuid ON dbo.dp_view_label_mappings(view_uuid);

-- API Metadata table (core record for REST APIs, MCP servers, AI agents, etc.)
IF OBJECT_ID(N'dbo.dp_api_metadata', N'U') IS NULL
CREATE TABLE dbo.dp_api_metadata (
    uuid VARCHAR(40) PRIMARY KEY,
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
    -- Nullable: SET NULL keeps the API record if its owning org reference is cleared.
    org_uuid VARCHAR(40),
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE SET NULL
);
-- org_uuid, ref_id, and handle are all nullable/optional in combination here. SQL Server's
-- plain UNIQUE INDEX treats NULL as equal to NULL (unlike Postgres/SQLite), so a bare
-- composite index would wrongly block a second row once one NULL-org_uuid combination
-- existed. Filtering to org_uuid IS NOT NULL (and ref_id IS NOT NULL where relevant)
-- reproduces the Postgres/SQLite "NULL never collides" semantics.
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_metadata_name_version_org' AND object_id = OBJECT_ID(N'dbo.dp_api_metadata'))
CREATE UNIQUE INDEX uq_api_metadata_name_version_org ON dbo.dp_api_metadata(name, version, org_uuid) WHERE org_uuid IS NOT NULL;
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_metadata_org_ref_id' AND object_id = OBJECT_ID(N'dbo.dp_api_metadata'))
CREATE UNIQUE INDEX uq_api_metadata_org_ref_id ON dbo.dp_api_metadata(org_uuid, ref_id) WHERE org_uuid IS NOT NULL AND ref_id IS NOT NULL;
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_metadata_handle_org' AND object_id = OBJECT_ID(N'dbo.dp_api_metadata'))
CREATE UNIQUE INDEX uq_api_metadata_handle_org ON dbo.dp_api_metadata(handle, org_uuid) WHERE org_uuid IS NOT NULL;
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_metadata_status' AND object_id = OBJECT_ID(N'dbo.dp_api_metadata'))
CREATE INDEX idx_api_metadata_status ON dbo.dp_api_metadata(status);

-- API Contents table (spec files, docs, icons, etc. attached to an API)
IF OBJECT_ID(N'dbo.dp_api_contents', N'U') IS NULL
CREATE TABLE dbo.dp_api_contents (
    uuid VARCHAR(40) PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    file_content VARBINARY(MAX) NOT NULL,
    type VARCHAR(64) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    lookup_key VARCHAR(255),
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (api_uuid) REFERENCES dp_api_metadata(uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_content_api_type_file_name' AND object_id = OBJECT_ID(N'dbo.dp_api_contents'))
CREATE UNIQUE INDEX uq_api_content_api_type_file_name ON dbo.dp_api_contents(api_uuid, type, file_name);
-- lookup_key is nullable -- filtered so multiple NULL-lookup_key rows per (api_uuid, type)
-- are allowed, matching Postgres/SQLite behavior (see the note on dp_api_metadata above).
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_content_api_type_lookup_key' AND object_id = OBJECT_ID(N'dbo.dp_api_contents'))
CREATE UNIQUE INDEX uq_api_content_api_type_lookup_key ON dbo.dp_api_contents(api_uuid, type, lookup_key) WHERE lookup_key IS NOT NULL;

-- API-Label mappings (many-to-many: which labels are attached to an API)
IF OBJECT_ID(N'dbo.dp_api_label_mappings', N'U') IS NULL
CREATE TABLE dbo.dp_api_label_mappings (
    uuid VARCHAR(40) PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    label_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (api_uuid) REFERENCES dp_api_metadata(uuid) ON DELETE CASCADE,
    FOREIGN KEY (label_uuid) REFERENCES dp_labels(uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_label_mappings_label_api' AND object_id = OBJECT_ID(N'dbo.dp_api_label_mappings'))
CREATE UNIQUE INDEX uq_api_label_mappings_label_api ON dbo.dp_api_label_mappings(label_uuid, api_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_label_mappings_api_uuid' AND object_id = OBJECT_ID(N'dbo.dp_api_label_mappings'))
CREATE INDEX idx_api_label_mappings_api_uuid ON dbo.dp_api_label_mappings(api_uuid);

-- API-Tag mappings (many-to-many: which tags are attached to an API)
IF OBJECT_ID(N'dbo.dp_api_tag_mappings', N'U') IS NULL
CREATE TABLE dbo.dp_api_tag_mappings (
    uuid VARCHAR(40) PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    tag_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (api_uuid) REFERENCES dp_api_metadata(uuid) ON DELETE CASCADE,
    FOREIGN KEY (tag_uuid) REFERENCES dp_tags(uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_tag_mappings_tag_api' AND object_id = OBJECT_ID(N'dbo.dp_api_tag_mappings'))
CREATE UNIQUE INDEX uq_api_tag_mappings_tag_api ON dbo.dp_api_tag_mappings(tag_uuid, api_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_tag_mappings_api_uuid' AND object_id = OBJECT_ID(N'dbo.dp_api_tag_mappings'))
CREATE INDEX idx_api_tag_mappings_api_uuid ON dbo.dp_api_tag_mappings(api_uuid);

-- Subscription Plans table (organization-scoped rate/billing plans)
-- Throttling limits live in dp_subscription_plan_limits (one row per limit).
IF OBJECT_ID(N'dbo.dp_subscription_plans', N'U') IS NULL
CREATE TABLE dbo.dp_subscription_plans (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(255) NOT NULL,
    display_name NVARCHAR(255) NOT NULL,
    description NVARCHAR(1023),
    ref_id VARCHAR(255),
    -- Nullable: SET NULL keeps the plan record if its owning org reference is cleared.
    org_uuid VARCHAR(40),
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE SET NULL
);
-- org_uuid is nullable -- filtered for the same NULL-handling reason as dp_api_metadata above.
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_subscription_plan_org_handle' AND object_id = OBJECT_ID(N'dbo.dp_subscription_plans'))
CREATE UNIQUE INDEX uq_subscription_plan_org_handle ON dbo.dp_subscription_plans(org_uuid, handle) WHERE org_uuid IS NOT NULL;

-- Subscription Plan Limits table (throttling limits for a plan)
IF OBJECT_ID(N'dbo.dp_subscription_plan_limits', N'U') IS NULL
CREATE TABLE dbo.dp_subscription_plan_limits (
    uuid VARCHAR(40) PRIMARY KEY,
    plan_uuid VARCHAR(40) NOT NULL,
    limit_type VARCHAR(20) NOT NULL DEFAULT 'REQUEST_COUNT',
    time_unit VARCHAR(20),
    time_amount INT NOT NULL DEFAULT 1,
    limit_count BIGINT NOT NULL,
    FOREIGN KEY (plan_uuid) REFERENCES dp_subscription_plans(uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_dp_subscription_plan_limits_plan' AND object_id = OBJECT_ID(N'dbo.dp_subscription_plan_limits'))
CREATE INDEX idx_dp_subscription_plan_limits_plan ON dbo.dp_subscription_plan_limits(plan_uuid);
-- Split into two filtered unique indexes because time_unit is nullable (see the
-- postgres schema for the full rationale); this is already how the source model
-- declares it (two named partial indexes), so all three dialects agree exactly.
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_dp_subscription_plan_limits' AND object_id = OBJECT_ID(N'dbo.dp_subscription_plan_limits'))
CREATE UNIQUE INDEX uq_dp_subscription_plan_limits ON dbo.dp_subscription_plan_limits(plan_uuid, limit_type, time_amount, time_unit) WHERE time_unit IS NOT NULL;
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_dp_subscription_plan_limits_null_unit' AND object_id = OBJECT_ID(N'dbo.dp_subscription_plan_limits'))
CREATE UNIQUE INDEX uq_dp_subscription_plan_limits_null_unit ON dbo.dp_subscription_plan_limits(plan_uuid, limit_type, time_amount) WHERE time_unit IS NULL;

-- API-Subscription Plan mappings (many-to-many: which plans an API offers)
IF OBJECT_ID(N'dbo.dp_api_subscription_plan_mappings', N'U') IS NULL
CREATE TABLE dbo.dp_api_subscription_plan_mappings (
    uuid VARCHAR(40) PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    plan_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (api_uuid) REFERENCES dp_api_metadata(uuid) ON DELETE CASCADE,
    FOREIGN KEY (plan_uuid) REFERENCES dp_subscription_plans(uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_subscription_plan_mappings_plan_api' AND object_id = OBJECT_ID(N'dbo.dp_api_subscription_plan_mappings'))
CREATE UNIQUE INDEX uq_api_subscription_plan_mappings_plan_api ON dbo.dp_api_subscription_plan_mappings(plan_uuid, api_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_subscription_plan_mappings_api_uuid' AND object_id = OBJECT_ID(N'dbo.dp_api_subscription_plan_mappings'))
CREATE INDEX idx_api_subscription_plan_mappings_api_uuid ON dbo.dp_api_subscription_plan_mappings(api_uuid);

-- Key Managers table (organization-scoped identity providers used to validate app keys)
IF OBJECT_ID(N'dbo.dp_key_managers', N'U') IS NULL
CREATE TABLE dbo.dp_key_managers (
    uuid VARCHAR(40) PRIMARY KEY,
    org_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    display_name NVARCHAR(255) NOT NULL,
    enabled SMALLINT NOT NULL DEFAULT 1,
    token_endpoint VARCHAR(255) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_key_manager_org_handle' AND object_id = OBJECT_ID(N'dbo.dp_key_managers'))
CREATE UNIQUE INDEX uq_key_manager_org_handle ON dbo.dp_key_managers(org_uuid, handle);

-- Applications table (developer-created consumer apps that subscribe to APIs)
IF OBJECT_ID(N'dbo.dp_applications', N'U') IS NULL
CREATE TABLE dbo.dp_applications (
    uuid VARCHAR(40) PRIMARY KEY,
    org_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    display_name NVARCHAR(255) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    description NVARCHAR(1023),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_application_org_created_by' AND object_id = OBJECT_ID(N'dbo.dp_applications'))
CREATE INDEX idx_application_org_created_by ON dbo.dp_applications(org_uuid, created_by);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_application_org_handle' AND object_id = OBJECT_ID(N'dbo.dp_applications'))
CREATE UNIQUE INDEX uq_application_org_handle ON dbo.dp_applications(org_uuid, handle);

-- Application-KeyManager mappings (per-KM OAuth2 client registration for an application)
IF OBJECT_ID(N'dbo.dp_app_key_mappings', N'U') IS NULL
CREATE TABLE dbo.dp_app_key_mappings (
    uuid VARCHAR(40) PRIMARY KEY,
    app_uuid VARCHAR(40) NOT NULL,
    km_uuid VARCHAR(40) NOT NULL,
    as_client_id VARCHAR(255),
    type VARCHAR(20) NOT NULL DEFAULT 'PRODUCTION',
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (app_uuid) REFERENCES dp_applications(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (km_uuid) REFERENCES dp_key_managers(uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_app_key_mappings_app_uuid' AND object_id = OBJECT_ID(N'dbo.dp_app_key_mappings'))
CREATE INDEX idx_app_key_mappings_app_uuid ON dbo.dp_app_key_mappings(app_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_app_key_mappings_km_uuid' AND object_id = OBJECT_ID(N'dbo.dp_app_key_mappings'))
CREATE INDEX idx_app_key_mappings_km_uuid ON dbo.dp_app_key_mappings(km_uuid);

-- Subscriptions table (application-level subscriptions to an API)
IF OBJECT_ID(N'dbo.dp_subscriptions', N'U') IS NULL
CREATE TABLE dbo.dp_subscriptions (
    uuid VARCHAR(40) PRIMARY KEY,
    created_by VARCHAR(255) NOT NULL,
    api_uuid VARCHAR(40) NOT NULL,
    -- Nullable: SET NULL keeps the subscription record if its plan reference is cleared.
    plan_uuid VARCHAR(40),
    org_uuid VARCHAR(40) NOT NULL,
    token VARCHAR(512),
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (api_uuid) REFERENCES dp_api_metadata(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (plan_uuid) REFERENCES dp_subscription_plans(uuid) ON DELETE SET NULL,
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscription_org_created_by' AND object_id = OBJECT_ID(N'dbo.dp_subscriptions'))
CREATE INDEX idx_subscription_org_created_by ON dbo.dp_subscriptions(org_uuid, created_by);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscription_org_api_uuid' AND object_id = OBJECT_ID(N'dbo.dp_subscriptions'))
CREATE INDEX idx_subscription_org_api_uuid ON dbo.dp_subscriptions(org_uuid, api_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscription_plan_uuid' AND object_id = OBJECT_ID(N'dbo.dp_subscriptions'))
CREATE INDEX idx_subscription_plan_uuid ON dbo.dp_subscriptions(plan_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscription_status' AND object_id = OBJECT_ID(N'dbo.dp_subscriptions'))
CREATE INDEX idx_subscription_status ON dbo.dp_subscriptions(status);
-- api_uuid is only ever a trailing column above (org_uuid, api_uuid) -- add a
-- dedicated leading index so single-column api_uuid lookups/joins stay indexed.
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscription_api_uuid' AND object_id = OBJECT_ID(N'dbo.dp_subscriptions'))
CREATE INDEX idx_subscription_api_uuid ON dbo.dp_subscriptions(api_uuid);
-- token is a single nullable column with a uniqueness requirement. A plain UNIQUE
-- constraint would let SQL Server allow only ONE NULL-token row total across the
-- whole table (unlike Postgres/SQLite, which allow unlimited NULLs). Filtering to
-- token IS NOT NULL reproduces the intended "many token-less subscriptions" behavior.
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_subscription_token' AND object_id = OBJECT_ID(N'dbo.dp_subscriptions'))
CREATE UNIQUE INDEX uq_subscription_token ON dbo.dp_subscriptions(token) WHERE token IS NOT NULL;

-- API Keys table (standalone, non-OAuth2 API key credentials for an API)
IF OBJECT_ID(N'dbo.dp_api_keys', N'U') IS NULL
CREATE TABLE dbo.dp_api_keys (
    uuid VARCHAR(40) PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    -- Nullable: SET NULL keeps the key record if its originating subscription is removed.
    subscription_uuid VARCHAR(40),
    org_uuid VARCHAR(40) NOT NULL,
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
    FOREIGN KEY (api_uuid) REFERENCES dp_api_metadata(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (subscription_uuid) REFERENCES dp_subscriptions(uuid) ON DELETE SET NULL,
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION,
    CONSTRAINT chk_api_key_revoked
        CHECK ((revoked_at IS NULL AND status != 'REVOKED') OR (revoked_at IS NOT NULL AND status = 'REVOKED'))
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_key_org_api_uuid' AND object_id = OBJECT_ID(N'dbo.dp_api_keys'))
CREATE INDEX idx_api_key_org_api_uuid ON dbo.dp_api_keys(org_uuid, api_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_key_subscription_uuid' AND object_id = OBJECT_ID(N'dbo.dp_api_keys'))
CREATE INDEX idx_api_key_subscription_uuid ON dbo.dp_api_keys(subscription_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_key_status' AND object_id = OBJECT_ID(N'dbo.dp_api_keys'))
CREATE INDEX idx_api_key_status ON dbo.dp_api_keys(status);
-- api_uuid is only ever a trailing column above (org_uuid, api_uuid) -- add a
-- dedicated leading index so single-column api_uuid lookups/joins stay indexed.
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_key_api_uuid' AND object_id = OBJECT_ID(N'dbo.dp_api_keys'))
CREATE INDEX idx_api_key_api_uuid ON dbo.dp_api_keys(api_uuid);

-- API Key-Application mappings (which application an API key was issued to)
IF OBJECT_ID(N'dbo.dp_api_key_app_mappings', N'U') IS NULL
CREATE TABLE dbo.dp_api_key_app_mappings (
    key_uuid VARCHAR(40) PRIMARY KEY,
    app_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (key_uuid) REFERENCES dp_api_keys(uuid) ON DELETE CASCADE,
    FOREIGN KEY (app_uuid) REFERENCES dp_applications(uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_key_app_mappings_app_uuid' AND object_id = OBJECT_ID(N'dbo.dp_api_key_app_mappings'))
CREATE INDEX idx_api_key_app_mappings_app_uuid ON dbo.dp_api_key_app_mappings(app_uuid);

-- API Workflows table (agent/automation workflows published under a view)
IF OBJECT_ID(N'dbo.dp_api_workflows', N'U') IS NULL
CREATE TABLE dbo.dp_api_workflows (
    uuid VARCHAR(40) PRIMARY KEY,
    org_uuid VARCHAR(40) NOT NULL,
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
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (view_uuid) REFERENCES dp_views(uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_api_workflow_org_view_handle' AND object_id = OBJECT_ID(N'dbo.dp_api_workflows'))
CREATE UNIQUE INDEX uq_api_workflow_org_view_handle ON dbo.dp_api_workflows(org_uuid, view_uuid, handle);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_workflow_view_uuid' AND object_id = OBJECT_ID(N'dbo.dp_api_workflows'))
CREATE INDEX idx_api_workflow_view_uuid ON dbo.dp_api_workflows(view_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_workflow_status' AND object_id = OBJECT_ID(N'dbo.dp_api_workflows'))
CREATE INDEX idx_api_workflow_status ON dbo.dp_api_workflows(status);

-- Audit table (write-only mutation trail; no FK on performed_by so history
-- survives deletion of the referenced dp_user_idp_references row)
IF OBJECT_ID(N'dbo.dp_audit', N'U') IS NULL
CREATE TABLE dbo.dp_audit (
    uuid VARCHAR(40) PRIMARY KEY,
    action VARCHAR(50) NOT NULL,
    resource_uuid VARCHAR(40) NOT NULL,
    resource_type VARCHAR(50),
    org_uuid VARCHAR(40) NOT NULL,
    performed_by VARCHAR(255),
    performed_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_audit_org_uuid' AND object_id = OBJECT_ID(N'dbo.dp_audit'))
CREATE INDEX idx_audit_org_uuid ON dbo.dp_audit(org_uuid);

-- Events table (outbox: one row per domain event; payload never contains plaintext key secrets)
IF OBJECT_ID(N'dbo.dp_events', N'U') IS NULL
CREATE TABLE dbo.dp_events (
    uuid VARCHAR(40) PRIMARY KEY,
    type VARCHAR(128) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_uuid VARCHAR(40) NOT NULL,
    payload NVARCHAR(MAX) NOT NULL DEFAULT '{}',
    occurred_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_event_status_occurred_at' AND object_id = OBJECT_ID(N'dbo.dp_events'))
CREATE INDEX idx_event_status_occurred_at ON dbo.dp_events(status, occurred_at);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_event_org_uuid' AND object_id = OBJECT_ID(N'dbo.dp_events'))
CREATE INDEX idx_event_org_uuid ON dbo.dp_events(org_uuid);

-- Event Deliveries table (one row per event x webhook subscriber; encrypted_fields
-- holds per-subscriber ciphertext so plaintext never lives in dp_events)
IF OBJECT_ID(N'dbo.dp_event_deliveries', N'U') IS NULL
CREATE TABLE dbo.dp_event_deliveries (
    uuid VARCHAR(40) PRIMARY KEY,
    event_uuid VARCHAR(40) NOT NULL,
    subscriber_id VARCHAR(128) NOT NULL,
    target_url VARCHAR(1023) NOT NULL,
    encrypted_fields NVARCHAR(MAX) DEFAULT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    last_http_status INT,
    last_error VARCHAR(255),
    last_attempt_at DATETIME2(7),
    delivered_at DATETIME2(7),
    FOREIGN KEY (event_uuid) REFERENCES dp_events(uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_event_delivery_event_uuid' AND object_id = OBJECT_ID(N'dbo.dp_event_deliveries'))
CREATE INDEX idx_event_delivery_event_uuid ON dbo.dp_event_deliveries(event_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_event_delivery_event_subscriber' AND object_id = OBJECT_ID(N'dbo.dp_event_deliveries'))
CREATE UNIQUE INDEX uq_event_delivery_event_subscriber ON dbo.dp_event_deliveries(event_uuid, subscriber_id);

-- Sessions table, used by connect-mssql-v2 (or equivalent) for server-side Express session storage.
IF OBJECT_ID(N'dbo.sessions', N'U') IS NULL
CREATE TABLE dbo.sessions (
    sid VARCHAR(255) PRIMARY KEY,
    sess NVARCHAR(MAX) NOT NULL,
    expire DATETIME2(7) NOT NULL
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_session_expire' AND object_id = OBJECT_ID(N'dbo.sessions'))
CREATE INDEX idx_session_expire ON dbo.sessions(expire);

-- User IdP References table (one durable record per distinct IdP `sub` claim; referenced
-- by uuid from created_by/updated_by-style columns elsewhere WITHOUT a foreign key, so
-- those columns keep pointing at a uuid after the row here is deleted)
IF OBJECT_ID(N'dbo.dp_user_idp_references', N'U') IS NULL
CREATE TABLE dbo.dp_user_idp_references (
    uuid VARCHAR(40) PRIMARY KEY,
    idp_id VARCHAR(255) NOT NULL UNIQUE,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME()
);

-- User-Organization mappings (live membership record -- both sides cascade on delete,
-- unlike the "hanging creator" created_by/updated_by pattern used elsewhere)
IF OBJECT_ID(N'dbo.dp_user_organization_mappings', N'U') IS NULL
CREATE TABLE dbo.dp_user_organization_mappings (
    user_uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    PRIMARY KEY (user_uuid, org_uuid),
    FOREIGN KEY (user_uuid) REFERENCES dp_user_idp_references(uuid) ON DELETE CASCADE,
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_user_organization_mappings_org_uuid' AND object_id = OBJECT_ID(N'dbo.dp_user_organization_mappings'))
CREATE INDEX idx_user_organization_mappings_org_uuid ON dbo.dp_user_organization_mappings(org_uuid);

-- Webhook Subscribers table (organization-scoped outbound event subscribers)
IF OBJECT_ID(N'dbo.dp_webhook_subscribers', N'U') IS NULL
CREATE TABLE dbo.dp_webhook_subscribers (
    uuid VARCHAR(40) PRIMARY KEY,
    org_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    display_name NVARCHAR(255) NOT NULL,
    target_url VARCHAR(1023) NOT NULL,
    secret_enc VARBINARY(MAX),
    public_key VARBINARY(MAX),
    event_patterns NVARCHAR(MAX) DEFAULT '[]',
    enabled SMALLINT NOT NULL DEFAULT 1,
    timeout_ms INT NOT NULL DEFAULT 5000,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_webhook_subscriber_org_handle' AND object_id = OBJECT_ID(N'dbo.dp_webhook_subscribers'))
CREATE UNIQUE INDEX uq_webhook_subscriber_org_handle ON dbo.dp_webhook_subscribers(org_uuid, handle);
