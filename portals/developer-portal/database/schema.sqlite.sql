-- Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
--
-- WSO2 LLC. licenses this file to you under the Apache License,
-- Version 2.0 (the "License"); you may not use this file except
-- in compliance with the License.
-- You may obtain a copy of the License at
--
-- http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing,
-- software distributed under the License is distributed on an
-- "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
-- KIND, either express or implied.  See the License for the
-- specific language governing permissions and limitations
-- under the License.

-- Schema for the SQLite dialect -- kept in sync manually
-- with the other schema.*.sql files. See src/db/driver.js for the query
-- layer that targets this schema.

-- Organizations table
CREATE TABLE IF NOT EXISTS dp_organizations (
    uuid VARCHAR(40) PRIMARY KEY,
    display_name VARCHAR(255) NOT NULL UNIQUE,
    business_owner VARCHAR(255),
    business_owner_contact VARCHAR(255),
    business_owner_email VARCHAR(255),
    handle VARCHAR(255) NOT NULL UNIQUE,
    idp_ref_id VARCHAR(255) NOT NULL,
    cp_ref_id VARCHAR(255),
    configuration TEXT NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Views table (organization-scoped grouping of APIs for gateway/portal visibility)
CREATE TABLE IF NOT EXISTS dp_views (
    uuid VARCHAR(40) PRIMARY KEY,
    org_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_view_handle_org_uuid ON dp_views(handle, org_uuid);
CREATE INDEX IF NOT EXISTS idx_view_org_uuid ON dp_views(org_uuid);

-- Organization Assets table (per-view branding/content assets, e.g. logos, docs)
CREATE TABLE IF NOT EXISTS dp_organization_assets (
    uuid VARCHAR(40) PRIMARY KEY,
    file_name VARCHAR(255) NOT NULL,
    file_content BLOB NOT NULL,
    file_type VARCHAR(20) NOT NULL,
    file_path VARCHAR(255) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    view_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION,
    -- CASCADE: an org asset is meaningless once its view is gone.
    FOREIGN KEY (view_uuid) REFERENCES dp_views(uuid) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_organization_asset_type_name_path_org_view
    ON dp_organization_assets(file_type, file_name, file_path, org_uuid, view_uuid);
CREATE INDEX IF NOT EXISTS idx_organization_asset_org_uuid ON dp_organization_assets(org_uuid);
CREATE INDEX IF NOT EXISTS idx_organization_asset_view_uuid ON dp_organization_assets(view_uuid);

-- Labels table (organization-scoped labels used for gateway/view assignment)
CREATE TABLE IF NOT EXISTS dp_labels (
    uuid VARCHAR(40) PRIMARY KEY,
    org_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_label_handle_org_uuid ON dp_labels(handle, org_uuid);
CREATE INDEX IF NOT EXISTS idx_label_org_uuid ON dp_labels(org_uuid);

-- Tags table (organization-scoped free-form API tags)
CREATE TABLE IF NOT EXISTS dp_tags (
    uuid VARCHAR(40) PRIMARY KEY,
    org_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_tag_name_org_uuid ON dp_tags(name, org_uuid);
CREATE INDEX IF NOT EXISTS idx_tag_org_uuid ON dp_tags(org_uuid);

-- View-Label mappings (many-to-many: which labels belong to a view)
CREATE TABLE IF NOT EXISTS dp_view_label_mappings (
    uuid VARCHAR(40) PRIMARY KEY,
    view_uuid VARCHAR(40) NOT NULL,
    label_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (view_uuid) REFERENCES dp_views(uuid) ON DELETE CASCADE,
    FOREIGN KEY (label_uuid) REFERENCES dp_labels(uuid) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_view_label_mappings_label_view ON dp_view_label_mappings(label_uuid, view_uuid);
CREATE INDEX IF NOT EXISTS idx_view_label_mappings_view_uuid ON dp_view_label_mappings(view_uuid);

-- API Metadata table (core record for REST APIs, MCP servers, AI agents, etc.)
CREATE TABLE IF NOT EXISTS dp_api_metadata (
    uuid VARCHAR(40) PRIMARY KEY,
    ref_id VARCHAR(255),
    name VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL,
    description VARCHAR(1023),
    version VARCHAR(30) NOT NULL,
    type VARCHAR(20) NOT NULL,
    agent_visibility VARCHAR(255) NOT NULL DEFAULT 'VISIBLE',
    technical_owner VARCHAR(255),
    technical_owner_email VARCHAR(255),
    business_owner VARCHAR(255),
    business_owner_email VARCHAR(255),
    sandbox_url VARCHAR(255),
    production_url VARCHAR(255),
    metadata_search TEXT,
    handle VARCHAR(255) NOT NULL,
    -- Nullable: SET NULL keeps the API record if its owning org reference is cleared.
    org_uuid VARCHAR(40),
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE SET NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_metadata_name_version_org ON dp_api_metadata(name, version, org_uuid);
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_metadata_org_ref_id ON dp_api_metadata(org_uuid, ref_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_metadata_handle_org ON dp_api_metadata(handle, org_uuid);
CREATE INDEX IF NOT EXISTS idx_api_metadata_status ON dp_api_metadata(status);

-- API Contents table (spec files, docs, icons, etc. attached to an API)
CREATE TABLE IF NOT EXISTS dp_api_contents (
    uuid VARCHAR(40) PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    file_content BLOB NOT NULL,
    type VARCHAR(64) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    lookup_key VARCHAR(255),
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (api_uuid) REFERENCES dp_api_metadata(uuid) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_content_api_type_file_name ON dp_api_contents(api_uuid, type, file_name);
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_content_api_type_lookup_key ON dp_api_contents(api_uuid, type, lookup_key);

-- API-Label mappings (many-to-many: which labels are attached to an API)
CREATE TABLE IF NOT EXISTS dp_api_label_mappings (
    uuid VARCHAR(40) PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    label_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (api_uuid) REFERENCES dp_api_metadata(uuid) ON DELETE CASCADE,
    FOREIGN KEY (label_uuid) REFERENCES dp_labels(uuid) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_label_mappings_label_api ON dp_api_label_mappings(label_uuid, api_uuid);
CREATE INDEX IF NOT EXISTS idx_api_label_mappings_api_uuid ON dp_api_label_mappings(api_uuid);

-- API-Tag mappings (many-to-many: which tags are attached to an API)
CREATE TABLE IF NOT EXISTS dp_api_tag_mappings (
    uuid VARCHAR(40) PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    tag_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (api_uuid) REFERENCES dp_api_metadata(uuid) ON DELETE CASCADE,
    FOREIGN KEY (tag_uuid) REFERENCES dp_tags(uuid) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_tag_mappings_tag_api ON dp_api_tag_mappings(tag_uuid, api_uuid);
CREATE INDEX IF NOT EXISTS idx_api_tag_mappings_api_uuid ON dp_api_tag_mappings(api_uuid);

-- Subscription Plans table (organization-scoped rate/billing plans)
-- Throttling limits live in dp_subscription_plan_limits (one row per limit).
CREATE TABLE IF NOT EXISTS dp_subscription_plans (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    ref_id VARCHAR(255),
    -- Nullable: SET NULL keeps the plan record if its owning org reference is cleared.
    org_uuid VARCHAR(40),
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE SET NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_subscription_plan_org_handle ON dp_subscription_plans(org_uuid, handle);

-- Subscription Plan Limits table (throttling limits for a plan)
CREATE TABLE IF NOT EXISTS dp_subscription_plan_limits (
    uuid VARCHAR(40) PRIMARY KEY,
    plan_uuid VARCHAR(40) NOT NULL,
    limit_type VARCHAR(20) NOT NULL DEFAULT 'REQUEST_COUNT',
    time_unit VARCHAR(20),
    time_amount INTEGER NOT NULL DEFAULT 1,
    limit_count BIGINT NOT NULL,
    FOREIGN KEY (plan_uuid) REFERENCES dp_subscription_plans(uuid) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_dp_subscription_plan_limits_plan ON dp_subscription_plan_limits(plan_uuid);
-- Split into two filtered unique indexes because time_unit is nullable (see the
-- postgres schema for the full rationale); SQLite supports partial indexes since 3.8.0.
CREATE UNIQUE INDEX IF NOT EXISTS uq_dp_subscription_plan_limits
    ON dp_subscription_plan_limits(plan_uuid, limit_type, time_amount, time_unit) WHERE time_unit IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_dp_subscription_plan_limits_null_unit
    ON dp_subscription_plan_limits(plan_uuid, limit_type, time_amount) WHERE time_unit IS NULL;

-- API-Subscription Plan mappings (many-to-many: which plans an API offers)
CREATE TABLE IF NOT EXISTS dp_api_subscription_plan_mappings (
    uuid VARCHAR(40) PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    plan_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (api_uuid) REFERENCES dp_api_metadata(uuid) ON DELETE CASCADE,
    FOREIGN KEY (plan_uuid) REFERENCES dp_subscription_plans(uuid) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_subscription_plan_mappings_plan_api
    ON dp_api_subscription_plan_mappings(plan_uuid, api_uuid);
CREATE INDEX IF NOT EXISTS idx_api_subscription_plan_mappings_api_uuid ON dp_api_subscription_plan_mappings(api_uuid);

-- Key Managers table (organization-scoped identity providers used to validate app keys)
CREATE TABLE IF NOT EXISTS dp_key_managers (
    uuid VARCHAR(40) PRIMARY KEY,
    org_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    enabled INTEGER NOT NULL DEFAULT 1,
    token_endpoint VARCHAR(255) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_key_manager_org_handle ON dp_key_managers(org_uuid, handle);

-- Applications table (developer-created consumer apps that subscribe to APIs)
CREATE TABLE IF NOT EXISTS dp_applications (
    uuid VARCHAR(40) PRIMARY KEY,
    org_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION
);
CREATE INDEX IF NOT EXISTS idx_application_org_created_by ON dp_applications(org_uuid, created_by);
CREATE UNIQUE INDEX IF NOT EXISTS uq_application_org_handle ON dp_applications(org_uuid, handle);

-- Application-KeyManager mappings (per-KM OAuth2 client registration for an application)
CREATE TABLE IF NOT EXISTS dp_app_key_mappings (
    uuid VARCHAR(40) PRIMARY KEY,
    app_uuid VARCHAR(40) NOT NULL,
    km_uuid VARCHAR(40) NOT NULL,
    as_client_id VARCHAR(255),
    type VARCHAR(20) NOT NULL DEFAULT 'PRODUCTION',
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (app_uuid) REFERENCES dp_applications(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (km_uuid) REFERENCES dp_key_managers(uuid) ON DELETE NO ACTION
);
CREATE INDEX IF NOT EXISTS idx_app_key_mappings_app_uuid ON dp_app_key_mappings(app_uuid);
CREATE INDEX IF NOT EXISTS idx_app_key_mappings_km_uuid ON dp_app_key_mappings(km_uuid);

-- Subscriptions table (application-level subscriptions to an API)
CREATE TABLE IF NOT EXISTS dp_subscriptions (
    uuid VARCHAR(40) PRIMARY KEY,
    created_by VARCHAR(255) NOT NULL,
    api_uuid VARCHAR(40) NOT NULL,
    -- Nullable: SET NULL keeps the subscription record if its plan reference is cleared.
    plan_uuid VARCHAR(40),
    org_uuid VARCHAR(40) NOT NULL,
    token VARCHAR(512),
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (api_uuid) REFERENCES dp_api_metadata(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (plan_uuid) REFERENCES dp_subscription_plans(uuid) ON DELETE SET NULL,
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION,
    UNIQUE(token)
);
CREATE INDEX IF NOT EXISTS idx_subscription_org_created_by ON dp_subscriptions(org_uuid, created_by);
CREATE INDEX IF NOT EXISTS idx_subscription_org_api_uuid ON dp_subscriptions(org_uuid, api_uuid);
CREATE INDEX IF NOT EXISTS idx_subscription_plan_uuid ON dp_subscriptions(plan_uuid);
CREATE INDEX IF NOT EXISTS idx_subscription_status ON dp_subscriptions(status);
-- api_uuid is only ever a trailing column above (org_uuid, api_uuid) -- add a
-- dedicated leading index so single-column api_uuid lookups/joins stay indexed.
CREATE INDEX IF NOT EXISTS idx_subscription_api_uuid ON dp_subscriptions(api_uuid);

-- API Keys table (standalone, non-OAuth2 API key credentials for an API)
CREATE TABLE IF NOT EXISTS dp_api_keys (
    uuid VARCHAR(40) PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    -- Nullable: SET NULL keeps the key record if its originating subscription is removed.
    subscription_uuid VARCHAR(40),
    org_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(128) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    expires_at DATETIME,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    revoked_at DATETIME,
    revoked_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (api_uuid) REFERENCES dp_api_metadata(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (subscription_uuid) REFERENCES dp_subscriptions(uuid) ON DELETE SET NULL,
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION,
    CONSTRAINT chk_api_key_revoked
        CHECK ((revoked_at IS NULL AND status != 'REVOKED') OR (revoked_at IS NOT NULL AND status = 'REVOKED'))
);
CREATE INDEX IF NOT EXISTS idx_api_key_org_api_uuid ON dp_api_keys(org_uuid, api_uuid);
CREATE INDEX IF NOT EXISTS idx_api_key_subscription_uuid ON dp_api_keys(subscription_uuid);
CREATE INDEX IF NOT EXISTS idx_api_key_status ON dp_api_keys(status);
-- api_uuid is only ever a trailing column above (org_uuid, api_uuid) -- add a
-- dedicated leading index so single-column api_uuid lookups/joins stay indexed.
CREATE INDEX IF NOT EXISTS idx_api_key_api_uuid ON dp_api_keys(api_uuid);

-- API Key-Application mappings (which application an API key was issued to)
CREATE TABLE IF NOT EXISTS dp_api_key_app_mappings (
    key_uuid VARCHAR(40) PRIMARY KEY,
    app_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (key_uuid) REFERENCES dp_api_keys(uuid) ON DELETE CASCADE,
    FOREIGN KEY (app_uuid) REFERENCES dp_applications(uuid) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_api_key_app_mappings_app_uuid ON dp_api_key_app_mappings(app_uuid);

-- API Workflows table (agent/automation workflows published under a view)
CREATE TABLE IF NOT EXISTS dp_api_workflows (
    uuid VARCHAR(40) PRIMARY KEY,
    org_uuid VARCHAR(40) NOT NULL,
    view_uuid VARCHAR(40) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    description VARCHAR(1023) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    agent_prompt BLOB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PUBLISHED',
    file_content BLOB,
    content_type VARCHAR(255),
    agent_visibility VARCHAR(255) NOT NULL DEFAULT 'VISIBLE',
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (view_uuid) REFERENCES dp_views(uuid) ON DELETE NO ACTION
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_workflow_org_view_handle ON dp_api_workflows(org_uuid, view_uuid, handle);
CREATE INDEX IF NOT EXISTS idx_api_workflow_view_uuid ON dp_api_workflows(view_uuid);
CREATE INDEX IF NOT EXISTS idx_api_workflow_status ON dp_api_workflows(status);

-- Audit table (write-only mutation trail; no FK on performed_by so history
-- survives deletion of the referenced dp_user_idp_references row)
CREATE TABLE IF NOT EXISTS dp_audit (
    uuid VARCHAR(40) PRIMARY KEY,
    action VARCHAR(50) NOT NULL,
    resource_uuid VARCHAR(40) NOT NULL,
    resource_type VARCHAR(50),
    org_uuid VARCHAR(40) NOT NULL,
    performed_by VARCHAR(255),
    performed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_audit_org_uuid ON dp_audit(org_uuid);

-- Events table (outbox: one row per domain event; payload never contains plaintext key secrets)
CREATE TABLE IF NOT EXISTS dp_events (
    uuid VARCHAR(40) PRIMARY KEY,
    type VARCHAR(128) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_uuid VARCHAR(40) NOT NULL,
    payload TEXT NOT NULL DEFAULT '{}',
    occurred_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION
);
CREATE INDEX IF NOT EXISTS idx_event_status_occurred_at ON dp_events(status, occurred_at);
CREATE INDEX IF NOT EXISTS idx_event_org_uuid ON dp_events(org_uuid);

-- Event Deliveries table (one row per event x webhook subscriber; encrypted_fields
-- holds per-subscriber ciphertext so plaintext never lives in dp_events)
CREATE TABLE IF NOT EXISTS dp_event_deliveries (
    uuid VARCHAR(40) PRIMARY KEY,
    event_uuid VARCHAR(40) NOT NULL,
    subscriber_id VARCHAR(128) NOT NULL,
    target_url VARCHAR(1023) NOT NULL,
    encrypted_fields TEXT DEFAULT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    last_http_status INTEGER,
    last_error VARCHAR(255),
    last_attempt_at DATETIME,
    delivered_at DATETIME,
    FOREIGN KEY (event_uuid) REFERENCES dp_events(uuid) ON DELETE NO ACTION
);
CREATE INDEX IF NOT EXISTS idx_event_delivery_event_uuid ON dp_event_deliveries(event_uuid);
CREATE UNIQUE INDEX IF NOT EXISTS uq_event_delivery_event_subscriber ON dp_event_deliveries(event_uuid, subscriber_id);

-- Sessions table, used by connect-session-sequelize for server-side Express session storage.
CREATE TABLE IF NOT EXISTS sessions (
    sid VARCHAR(255) PRIMARY KEY,
    sess TEXT NOT NULL,
    expire DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_session_expire ON sessions(expire);

-- User IdP References table (one durable record per distinct IdP `sub` claim; referenced
-- by uuid from created_by/updated_by-style columns elsewhere WITHOUT a foreign key, so
-- those columns keep pointing at a uuid after the row here is deleted)
CREATE TABLE IF NOT EXISTS dp_user_idp_references (
    uuid VARCHAR(40) PRIMARY KEY,
    idp_id VARCHAR(255) NOT NULL UNIQUE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- User-Organization mappings (live membership record -- both sides cascade on delete,
-- unlike the "hanging creator" created_by/updated_by pattern used elsewhere)
CREATE TABLE IF NOT EXISTS dp_user_organization_mappings (
    user_uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    PRIMARY KEY (user_uuid, org_uuid),
    FOREIGN KEY (user_uuid) REFERENCES dp_user_idp_references(uuid) ON DELETE CASCADE,
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_user_organization_mappings_org_uuid ON dp_user_organization_mappings(org_uuid);

-- Webhook Subscribers table (organization-scoped outbound event subscribers)
CREATE TABLE IF NOT EXISTS dp_webhook_subscribers (
    uuid VARCHAR(40) PRIMARY KEY,
    org_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    target_url VARCHAR(1023) NOT NULL,
    secret_enc BLOB,
    public_key BLOB,
    event_patterns TEXT DEFAULT '[]',
    enabled INTEGER NOT NULL DEFAULT 1,
    timeout_ms INTEGER NOT NULL DEFAULT 5000,
    created_by VARCHAR(255) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (org_uuid) REFERENCES dp_organizations(uuid) ON DELETE NO ACTION
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_webhook_subscriber_org_handle ON dp_webhook_subscribers(org_uuid, handle);
