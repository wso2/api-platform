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

-- Schema for the PostgreSQL dialect -- kept in sync manually
-- with the other schema.*.sql files. See src/db/driver.js for the query
-- layer that targets this schema.

-- Organizations table
CREATE TABLE IF NOT EXISTS organizations (
    uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    display_name VARCHAR(255) NOT NULL,
    business_owner VARCHAR(255),
    business_owner_contact VARCHAR(255),
    business_owner_email VARCHAR(255),
    handle VARCHAR(255) NOT NULL,
    idp_ref_id VARCHAR(255) NOT NULL,
    cp_ref_id VARCHAR(255),
    configuration JSONB NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    UNIQUE(portal_id, handle),
    UNIQUE(portal_id, display_name)
);

-- Views table (portal-scoped grouping of APIs for gateway/portal visibility)
CREATE TABLE IF NOT EXISTS views (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    handle VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_view_handle_org_uuid ON views(handle, org_uuid, portal_id);
CREATE INDEX IF NOT EXISTS idx_view_org_uuid ON views(org_uuid, portal_id);

-- Organization Assets table (per-view branding/content assets, e.g. logos, docs)
CREATE TABLE IF NOT EXISTS organization_assets (
    uuid VARCHAR(40) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    file_content BYTEA NOT NULL,
    file_type VARCHAR(20) NOT NULL,
    file_path VARCHAR(255) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    view_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION,
    -- CASCADE: an org asset is meaningless once its view is gone.
    FOREIGN KEY (portal_id, view_uuid) REFERENCES views(portal_id, uuid) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_organization_asset_type_name_path_org_view
    ON organization_assets(file_type, file_name, file_path, org_uuid, view_uuid, portal_id);
CREATE INDEX IF NOT EXISTS idx_organization_asset_org_uuid ON organization_assets(org_uuid);
CREATE INDEX IF NOT EXISTS idx_organization_asset_view_uuid ON organization_assets(view_uuid);

-- Labels table (portal-scoped labels used for gateway/view assignment)
CREATE TABLE IF NOT EXISTS labels (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    handle VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_label_handle_org_uuid ON labels(handle, org_uuid, portal_id);
CREATE INDEX IF NOT EXISTS idx_label_org_uuid ON labels(org_uuid, portal_id);

-- Tags table (portal-scoped free-form API tags)
CREATE TABLE IF NOT EXISTS tags (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    name VARCHAR(255) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_tag_name_org_uuid ON tags(name, org_uuid, portal_id);
CREATE INDEX IF NOT EXISTS idx_tag_org_uuid ON tags(org_uuid, portal_id);

-- View-Label mappings (many-to-many: which labels belong to a view)
CREATE TABLE IF NOT EXISTS view_label_mappings (
    uuid VARCHAR(40) NOT NULL,
    view_uuid VARCHAR(40) NOT NULL,
    label_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, view_uuid) REFERENCES views(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, label_uuid) REFERENCES labels(portal_id, uuid) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_view_label_mappings_label_view ON view_label_mappings(portal_id, label_uuid, view_uuid);
CREATE INDEX IF NOT EXISTS idx_view_label_mappings_view_uuid ON view_label_mappings(view_uuid);

-- API Metadata table (core record for REST APIs, MCP servers, AI agents, etc.)
-- API is a portal-managed entity: portal_id identifies which portal owns it.
CREATE TABLE IF NOT EXISTS api_metadata (
    uuid VARCHAR(40) NOT NULL,
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
    metadata_search JSONB,
    handle VARCHAR(255) NOT NULL,
    -- Nullable: preserved to keep API records alive when an org is removed.
    -- Nullification is handled by the application layer; ON DELETE NO ACTION is
    -- used because a composite FK cannot partially SET NULL while portal_id is NOT NULL.
    org_uuid VARCHAR(40),
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
-- org_uuid is nullable — partial indexes prevent NULL-org rows from colliding
-- with each other while still enforcing uniqueness among non-NULL org rows.
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_metadata_name_version_org ON api_metadata(name, version, org_uuid, portal_id) WHERE org_uuid IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_metadata_org_ref_id ON api_metadata(org_uuid, ref_id, portal_id) WHERE org_uuid IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_metadata_handle_org ON api_metadata(handle, org_uuid, portal_id) WHERE org_uuid IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_api_metadata_status ON api_metadata(status);

-- API Contents table (spec files, docs, icons, etc. attached to an API)
CREATE TABLE IF NOT EXISTS api_contents (
    uuid VARCHAR(40) NOT NULL,
    api_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    file_content BYTEA NOT NULL,
    type VARCHAR(64) NOT NULL,
    file_name VARCHAR(255) NOT NULL,
    lookup_key VARCHAR(255),
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, api_uuid) REFERENCES api_metadata(portal_id, uuid) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_content_api_type_file_name ON api_contents(api_uuid, type, file_name, portal_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_content_api_type_lookup_key ON api_contents(api_uuid, type, lookup_key, portal_id);

-- API-Label mappings (many-to-many: which labels are attached to an API)
CREATE TABLE IF NOT EXISTS api_label_mappings (
    uuid VARCHAR(40) NOT NULL,
    api_uuid VARCHAR(40) NOT NULL,
    label_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, api_uuid) REFERENCES api_metadata(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, label_uuid) REFERENCES labels(portal_id, uuid) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_label_mappings_label_api ON api_label_mappings(portal_id, label_uuid, api_uuid);
CREATE INDEX IF NOT EXISTS idx_api_label_mappings_api_uuid ON api_label_mappings(api_uuid);

-- API-Tag mappings (many-to-many: which tags are attached to an API)
CREATE TABLE IF NOT EXISTS api_tag_mappings (
    uuid VARCHAR(40) NOT NULL,
    api_uuid VARCHAR(40) NOT NULL,
    tag_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, api_uuid) REFERENCES api_metadata(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, tag_uuid) REFERENCES tags(portal_id, uuid) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_tag_mappings_tag_api ON api_tag_mappings(portal_id, tag_uuid, api_uuid);
CREATE INDEX IF NOT EXISTS idx_api_tag_mappings_api_uuid ON api_tag_mappings(api_uuid);

-- Subscription Plans table (portal-scoped rate/billing plans)
-- Throttling limits live in subscription_plan_limits (one row per limit).
CREATE TABLE IF NOT EXISTS subscription_plans (
    uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    ref_id VARCHAR(255),
    -- Nullable: same ON DELETE NO ACTION rationale as api_metadata.org_uuid above.
    org_uuid VARCHAR(40),
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_subscription_plan_org_handle ON subscription_plans(org_uuid, handle, portal_id);

-- Subscription Plan Limits table (throttling limits for a plan)
CREATE TABLE IF NOT EXISTS subscription_plan_limits (
    uuid VARCHAR(40) NOT NULL,
    plan_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    limit_type VARCHAR(20) NOT NULL DEFAULT 'REQUEST_COUNT',
    time_unit VARCHAR(20),
    time_amount INTEGER NOT NULL DEFAULT 1,
    limit_count BIGINT NOT NULL,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, plan_uuid) REFERENCES subscription_plans(portal_id, uuid) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_subscription_plan_limits_plan ON subscription_plan_limits(plan_uuid);
-- Split into two filtered unique indexes because time_unit is nullable: a plain composite
-- unique index would let Postgres treat every NULL time_unit row as distinct (never colliding),
-- silently allowing duplicate NULL-time_unit limits. These two indexes make both branches explicit.
CREATE UNIQUE INDEX IF NOT EXISTS uq_subscription_plan_limits
    ON subscription_plan_limits(plan_uuid, limit_type, time_amount, time_unit, portal_id) WHERE time_unit IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_subscription_plan_limits_null_unit
    ON subscription_plan_limits(plan_uuid, limit_type, time_amount, portal_id) WHERE time_unit IS NULL;

-- API-Subscription Plan mappings (many-to-many: which plans an API offers)
CREATE TABLE IF NOT EXISTS api_subscription_plan_mappings (
    uuid VARCHAR(40) NOT NULL,
    api_uuid VARCHAR(40) NOT NULL,
    plan_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, api_uuid) REFERENCES api_metadata(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, plan_uuid) REFERENCES subscription_plans(portal_id, uuid) ON DELETE CASCADE
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_subscription_plan_mappings_plan_api
    ON api_subscription_plan_mappings(portal_id, plan_uuid, api_uuid);
CREATE INDEX IF NOT EXISTS idx_api_subscription_plan_mappings_api_uuid ON api_subscription_plan_mappings(api_uuid);

-- Key Managers table (portal-scoped identity providers used to validate app keys)
CREATE TABLE IF NOT EXISTS key_managers (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    handle VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    enabled SMALLINT NOT NULL DEFAULT 1,
    token_endpoint VARCHAR(255) NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_key_manager_org_handle ON key_managers(org_uuid, handle, portal_id);

-- Applications table (portal-scoped developer-created consumer apps)
CREATE TABLE IF NOT EXISTS applications (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_by VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
CREATE INDEX IF NOT EXISTS idx_application_org_created_by ON applications(org_uuid, portal_id, created_by);
CREATE UNIQUE INDEX IF NOT EXISTS uq_application_org_handle ON applications(org_uuid, handle, portal_id);

-- Application-KeyManager mappings (per-KM OAuth2 client registration for an application)
CREATE TABLE IF NOT EXISTS app_key_mappings (
    uuid VARCHAR(40) NOT NULL,
    app_uuid VARCHAR(40) NOT NULL,
    km_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    as_client_id VARCHAR(255),
    type VARCHAR(20) NOT NULL DEFAULT 'PRODUCTION',
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, app_uuid) REFERENCES applications(portal_id, uuid) ON DELETE NO ACTION,
    FOREIGN KEY (portal_id, km_uuid) REFERENCES key_managers(portal_id, uuid) ON DELETE NO ACTION
);
CREATE INDEX IF NOT EXISTS idx_app_key_mappings_app_uuid ON app_key_mappings(app_uuid);
CREATE INDEX IF NOT EXISTS idx_app_key_mappings_km_uuid ON app_key_mappings(km_uuid);

-- Subscriptions table (portal-scoped application-level subscriptions to an API)
CREATE TABLE IF NOT EXISTS subscriptions (
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
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, api_uuid) REFERENCES api_metadata(portal_id, uuid) ON DELETE NO ACTION,
    FOREIGN KEY (portal_id, plan_uuid) REFERENCES subscription_plans(portal_id, uuid) ON DELETE NO ACTION,
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION,
    UNIQUE(token)
);
CREATE INDEX IF NOT EXISTS idx_subscription_org_created_by ON subscriptions(org_uuid, portal_id, created_by);
CREATE INDEX IF NOT EXISTS idx_subscription_org_api_uuid ON subscriptions(org_uuid, portal_id, api_uuid);
CREATE INDEX IF NOT EXISTS idx_subscription_plan_uuid ON subscriptions(plan_uuid);
CREATE INDEX IF NOT EXISTS idx_subscription_status ON subscriptions(status);
-- api_uuid is only ever a trailing column above (org_uuid, api_uuid) — add a
-- dedicated leading index so single-column api_uuid lookups/joins stay indexed.
CREATE INDEX IF NOT EXISTS idx_subscription_api_uuid ON subscriptions(api_uuid);

-- API Keys table (portal-scoped standalone, non-OAuth2 API key credentials for an API)
CREATE TABLE IF NOT EXISTS api_keys (
    uuid VARCHAR(40) NOT NULL,
    api_uuid VARCHAR(40) NOT NULL,
    -- Nullable: same ON DELETE NO ACTION rationale as api_metadata.org_uuid above.
    subscription_uuid VARCHAR(40),
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    handle VARCHAR(128) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    expires_at TIMESTAMPTZ,
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255) NOT NULL,
    revoked_at TIMESTAMPTZ,
    revoked_by VARCHAR(200),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, api_uuid) REFERENCES api_metadata(portal_id, uuid) ON DELETE NO ACTION,
    FOREIGN KEY (portal_id, subscription_uuid) REFERENCES subscriptions(portal_id, uuid) ON DELETE NO ACTION,
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION,
    CONSTRAINT chk_api_key_revoked
        CHECK ((revoked_at IS NULL AND status != 'REVOKED') OR (revoked_at IS NOT NULL AND status = 'REVOKED'))
);
CREATE INDEX IF NOT EXISTS idx_api_key_org_api_uuid ON api_keys(org_uuid, api_uuid);
CREATE INDEX IF NOT EXISTS idx_api_key_subscription_uuid ON api_keys(subscription_uuid);
CREATE INDEX IF NOT EXISTS idx_api_key_status ON api_keys(status);
-- api_uuid is only ever a trailing column above (org_uuid, api_uuid) —- add a
-- dedicated leading index so single-column api_uuid lookups/joins stay indexed.
CREATE INDEX IF NOT EXISTS idx_api_key_api_uuid ON api_keys(api_uuid);
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_key_org_api_handle ON api_keys(org_uuid, api_uuid, handle, portal_id);

-- API Key-Application mappings (which application an API key was issued to)
CREATE TABLE IF NOT EXISTS api_key_app_mappings (
    key_uuid VARCHAR(40) NOT NULL,
    app_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, key_uuid),
    FOREIGN KEY (portal_id, key_uuid) REFERENCES api_keys(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, app_uuid) REFERENCES applications(portal_id, uuid) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_api_key_app_mappings_app_uuid ON api_key_app_mappings(app_uuid);

-- API Workflows table (portal-scoped agent/automation workflows published under a view)
CREATE TABLE IF NOT EXISTS api_workflows (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    view_uuid VARCHAR(40) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    description VARCHAR(1023) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    agent_prompt BYTEA NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PUBLISHED',
    file_content BYTEA,
    content_type VARCHAR(255),
    agent_visibility VARCHAR(255) NOT NULL DEFAULT 'VISIBLE',
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION,
    FOREIGN KEY (portal_id, view_uuid) REFERENCES views(portal_id, uuid) ON DELETE NO ACTION
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_workflow_org_view_handle ON api_workflows(org_uuid, view_uuid, handle, portal_id);
CREATE INDEX IF NOT EXISTS idx_api_workflow_view_uuid ON api_workflows(view_uuid);
CREATE INDEX IF NOT EXISTS idx_api_workflow_status ON api_workflows(status);

-- Audit table (write-only mutation trail; no FK on performed_by so history
-- survives deletion of the referenced user_idp_references row)
CREATE TABLE IF NOT EXISTS audit (
    uuid VARCHAR(40) NOT NULL,
    action VARCHAR(50) NOT NULL,
    resource_uuid VARCHAR(40) NOT NULL,
    resource_type VARCHAR(50),
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    performed_by VARCHAR(255),
    performed_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_audit_org_uuid ON audit(org_uuid, portal_id);

-- Events table (outbox: one row per domain event; payload never contains plaintext key secrets)
CREATE TABLE IF NOT EXISTS events (
    uuid VARCHAR(40) NOT NULL,
    type VARCHAR(128) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    aggregate_type VARCHAR(64) NOT NULL,
    aggregate_uuid VARCHAR(40) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}',
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
CREATE INDEX IF NOT EXISTS idx_event_status_occurred_at ON events(status, occurred_at);
CREATE INDEX IF NOT EXISTS idx_event_org_uuid ON events(org_uuid, portal_id);

-- Event Deliveries table (one row per event x webhook subscriber; encrypted_fields
-- holds per-subscriber ciphertext so plaintext never lives in events)
CREATE TABLE IF NOT EXISTS event_deliveries (
    uuid VARCHAR(40) NOT NULL,
    event_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    subscriber_id VARCHAR(128) NOT NULL,
    target_url VARCHAR(1023) NOT NULL,
    encrypted_fields JSON DEFAULT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
    last_http_status INTEGER,
    last_error VARCHAR(255),
    last_attempt_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, event_uuid) REFERENCES events(portal_id, uuid) ON DELETE NO ACTION
);
CREATE INDEX IF NOT EXISTS idx_event_delivery_event_uuid ON event_deliveries(event_uuid);
CREATE UNIQUE INDEX IF NOT EXISTS uq_event_delivery_event_subscriber ON event_deliveries(portal_id, event_uuid, subscriber_id);

-- Sessions table, used by connect-pg-simple for server-side Express session storage.
CREATE TABLE IF NOT EXISTS sessions (
    sid VARCHAR(255) PRIMARY KEY,
    sess JSON NOT NULL,
    expire TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_session_expire ON sessions(expire);

-- User IdP References table (one durable record per IdP `sub` claim scoped to a portal;
-- referenced by uuid from created_by/updated_by-style columns elsewhere WITHOUT a foreign
-- key, so those columns keep pointing at a uuid after the row here is deleted)
CREATE TABLE IF NOT EXISTS user_idp_references (
    uuid VARCHAR(40) NOT NULL,
    idp_id VARCHAR(255) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid)
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_user_idp_references_idpid_portal ON user_idp_references(idp_id, portal_id);

-- User-Organization mappings (live membership record —- both sides cascade on delete,
-- unlike the "hanging creator" created_by/updated_by pattern used elsewhere)
CREATE TABLE IF NOT EXISTS user_organization_mappings (
    user_uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    PRIMARY KEY (portal_id, user_uuid, org_uuid),
    FOREIGN KEY (portal_id, user_uuid) REFERENCES user_idp_references(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_user_organization_mappings_org_uuid ON user_organization_mappings(org_uuid);

-- Webhook Subscribers table (portal-scoped outbound event subscribers)
CREATE TABLE IF NOT EXISTS webhook_subscribers (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    handle VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    target_url VARCHAR(1023) NOT NULL,
    secret_enc BYTEA,
    event_patterns JSONB DEFAULT '[]',
    enabled SMALLINT NOT NULL DEFAULT 1,
    timeout_ms INTEGER NOT NULL DEFAULT 5000,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_webhook_subscriber_org_handle ON webhook_subscribers(org_uuid, handle, portal_id);
