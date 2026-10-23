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
    -- In a multi-portal scenario the same logical org (same handle) has a distinct UUID per
    -- portal; this uuid identifies the org within this portal only, not across portals.
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
CREATE INDEX IF NOT EXISTS idx_org_idp_ref_id ON organizations(idp_ref_id, portal_id);

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
CREATE INDEX IF NOT EXISTS idx_organization_asset_org_uuid ON organization_assets(org_uuid, portal_id);
CREATE INDEX IF NOT EXISTS idx_organization_asset_view_uuid ON organization_assets(view_uuid, portal_id);

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
CREATE UNIQUE INDEX IF NOT EXISTS uq_view_label_mappings_label_view ON view_label_mappings(label_uuid, view_uuid, portal_id);
CREATE INDEX IF NOT EXISTS idx_view_label_mappings_view_uuid ON view_label_mappings(view_uuid, portal_id);

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
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_metadata_name_version_org ON api_metadata(name, version, org_uuid, portal_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_metadata_org_ref_id ON api_metadata(org_uuid, ref_id, portal_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_metadata_handle_org ON api_metadata(handle, org_uuid, portal_id);
CREATE INDEX IF NOT EXISTS idx_api_metadata_status ON api_metadata(status);
CREATE INDEX IF NOT EXISTS idx_api_metadata_org_uuid ON api_metadata(org_uuid, portal_id);

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
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_label_mappings_label_api ON api_label_mappings(label_uuid, api_uuid, portal_id);
CREATE INDEX IF NOT EXISTS idx_api_label_mappings_api_uuid ON api_label_mappings(api_uuid, portal_id);

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
CREATE UNIQUE INDEX IF NOT EXISTS uq_api_tag_mappings_tag_api ON api_tag_mappings(tag_uuid, api_uuid, portal_id);
CREATE INDEX IF NOT EXISTS idx_api_tag_mappings_api_uuid ON api_tag_mappings(api_uuid, portal_id);

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
    ON api_subscription_plan_mappings(plan_uuid, api_uuid, portal_id);
CREATE INDEX IF NOT EXISTS idx_api_subscription_plan_mappings_api_uuid ON api_subscription_plan_mappings(api_uuid, portal_id);

-- Key Managers table (portal-scoped identity providers used to validate app keys)
CREATE TABLE IF NOT EXISTS key_managers (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    handle VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    enabled SMALLINT NOT NULL DEFAULT 1,
    token_endpoint VARCHAR(255) NOT NULL,
    -- PRODUCTION | SANDBOX. Which environment the keys this key manager issues
    -- belong to. A deployment runs one key manager per environment, so this is a
    -- property of the key manager rather than a per-key choice; every key it
    -- creates inherits it (oauth2_consumer_keys.key_type).
    --
    -- Fixed at creation and refused on update, for the same reason the driver type
    -- is: keys already issued were created as one kind, and relabelling the key
    -- manager cannot change what they are.
    --
    -- Not a CHECK constraint (R4-NO-ENUM-CHECK) — validated in the service layer
    -- against constants.KEY_TYPE.
    key_type VARCHAR(20) NOT NULL DEFAULT 'PRODUCTION',
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
CREATE INDEX IF NOT EXISTS idx_application_org_created_by ON applications(org_uuid, created_by, portal_id);
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
CREATE INDEX IF NOT EXISTS idx_app_key_mappings_app_uuid ON app_key_mappings(app_uuid, portal_id);
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
CREATE INDEX IF NOT EXISTS idx_subscription_org_created_by ON subscriptions(org_uuid, created_by, portal_id);
CREATE INDEX IF NOT EXISTS idx_subscription_org_api_uuid ON subscriptions(org_uuid, api_uuid, portal_id);
CREATE INDEX IF NOT EXISTS idx_subscription_plan_uuid ON subscriptions(plan_uuid);
CREATE INDEX IF NOT EXISTS idx_subscription_status ON subscriptions(status);
-- api_uuid is only ever a trailing column above (org_uuid, api_uuid) -- add a
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
CREATE INDEX IF NOT EXISTS idx_api_key_org_api_uuid ON api_keys(org_uuid, api_uuid, portal_id);
CREATE INDEX IF NOT EXISTS idx_api_key_subscription_uuid ON api_keys(subscription_uuid);
CREATE INDEX IF NOT EXISTS idx_api_key_status ON api_keys(status);
-- api_uuid is only ever a trailing column above (org_uuid, api_uuid) —- add a
-- dedicated leading index so single-column api_uuid lookups/joins stay indexed.
CREATE INDEX IF NOT EXISTS idx_api_key_api_uuid ON api_keys(api_uuid);
-- Handle is the caller-facing id used to address a key within an API, so it must be
-- unique per (org, portal, api). Enforced here for a race-free guarantee.
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
-- ---------------------------------------------------------------------------
-- OAuth2 consumer keys — one row per OAuth application this portal registered
-- on a key manager via Dynamic Client Registration (RFC 7591).
--
-- Only the identity of the registered client, plus whatever RFC 7592 gave the
-- portal to manage it with. Deliberately absent:
--   * the client secret — the key manager returns it once and never again;
--   * the client metadata (redirect_uris, grant_types, …) — it lives at the
--     key manager and is re-read from there, so there is one copy.
--
-- The RFC 7592 registration access token and client configuration URI ARE kept,
-- in registration_access_token_enc (encrypted at rest) and
-- registration_client_uri — see the notes on those columns. Both are NULL for a
-- key manager that issues no such token, and a key in that state is managed with
-- the portal's provisioning credential against a constructed
-- <registration_endpoint>/<consumer_key>, which is what every key did before
-- these columns existed.
--
-- key_manager_id is deliberately NOT a foreign key: key managers are declared
-- in configuration ([[api_portal.key_manager]]), not rows in key_managers, so
-- there is nothing to reference.
--
-- The key↔application association lives in oauth2_consumer_key_app_mappings, not
-- as a column here.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS oauth2_consumer_keys (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    -- The `id` of an [[api_portal.key_manager]] config entry. Not an FK; see above.
    key_manager_id VARCHAR(255) NOT NULL,
    -- The OAuth2 client_id the key manager issued.
    consumer_key VARCHAR(255) NOT NULL,
    -- What the developer called this when they created it. The consumer key
    -- alone is not something a person can recognise in a list. Nullable: a
    -- driver need not collect a name, and a key imported by an older client
    -- may not carry one -- the UI falls back to the consumer key.
    name VARCHAR(255),
    -- RFC 7592 client-configuration credentials, when the key manager issued them.
    --
    -- A conforming server returns a registration access token alongside the new
    -- client, and read/update/delete of THAT client authenticate with it rather
    -- than the portal's own provisioning credential. It may also rotate: Keycloak
    -- returns a new token from every update and kills the old one immediately, so
    -- these columns are rewritten from each response, not just at creation.
    --
    -- registration_client_uri is stored rather than rebuilt because RFC 7592 S3
    -- makes it REQUIRED in the response precisely so the configuration endpoint
    -- can sit on a different host or path than the registration endpoint --
    -- Keycloak's does. NULL in both means this key is managed with the
    -- provisioning credential against a constructed URL, which is how every key
    -- on a key manager that issues no token behaves.
    -- PRODUCTION | SANDBOX, copied from the key manager when the key was created.
    --
    -- Stored rather than derived from the key manager for two reasons: a
    -- config-declared key manager has no row to join to, so a query for sandbox
    -- keys would have nowhere to look; and it is a historical fact about what this
    -- credential was issued as, which should not change if the key manager's own
    -- flag ever did.
    --
    -- Resolved from the key manager in exactly one place today. When a per-key
    -- choice is added later, that one line becomes `body.keyType ?? km.keyType`
    -- and this column needs no change.
    key_type VARCHAR(20) NOT NULL DEFAULT 'PRODUCTION',
    registration_access_token_enc BYTEA,
    registration_client_uri VARCHAR(1023),
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- (portal_id, uuid) matches every sibling table, and is required for the
    -- composite FK below: organizations' own key is (portal_id, uuid), so a
    -- single-column reference to organizations(uuid) cannot be created.
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);

-- The list query filters org_uuid + created_by, so that compound leads; it is
-- also a covering prefix for org-only lookups, hence no separate org index.
CREATE INDEX IF NOT EXISTS idx_oauth2_consumer_key_org_created_by ON oauth2_consumer_keys(org_uuid, created_by, portal_id);
CREATE INDEX IF NOT EXISTS idx_oauth2_consumer_key_km_id ON oauth2_consumer_keys(key_manager_id, portal_id);
CREATE INDEX IF NOT EXISTS idx_oauth2_consumer_key_status ON oauth2_consumer_keys(status);

-- Which application an OAuth2 key belongs to.
--
-- Its own table rather than a column on oauth2_consumer_keys: the association is
-- optional, is set and cleared independently of the key, and is written by a
-- different operation. A nullable column would also have made "no application"
-- and "application deleted" indistinguishable.
--
-- Shaped to match api_key_app_mappings, which solves the identical problem for
-- API keys. key_uuid is the whole key: a key belongs to at most one application,
-- so re-associating upserts this row rather than inserting a second one, and
-- created_by records who last did it. An application holds any number of keys,
-- which is why app_uuid is not part of the key and carries its own index.
--
-- Both foreign keys are composite. oauth2_consumer_keys and applications are each
-- keyed on (portal_id, uuid), so a single-column reference to either `uuid` has no
-- unique index to point at and cannot be created.
--
-- CASCADE on both sides: this row is meaningless without either end. Deleting the
-- key at the key manager already deletes its row here; deleting the application
-- drops the association without touching the key, which stays usable and simply
-- becomes unassociated.
CREATE TABLE IF NOT EXISTS oauth2_consumer_key_app_mappings (
    key_uuid VARCHAR(40) NOT NULL,
    app_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, key_uuid),
    FOREIGN KEY (portal_id, key_uuid) REFERENCES oauth2_consumer_keys(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, app_uuid) REFERENCES applications(portal_id, uuid) ON DELETE CASCADE
);

-- app_uuid is not the leading column of the primary key, so the per-application
-- listing (GET /applications/{id}/oauth2-keys) needs its own index.
CREATE INDEX IF NOT EXISTS idx_oauth2_key_app_mapping_app ON oauth2_consumer_key_app_mappings(app_uuid);


-- The driver configuration that makes a `key_managers` row usable for Dynamic
-- Client Registration. Separate from `key_managers` because that table has
-- shipped: it holds a handle, a display name and a token endpoint, and adding
-- half a dozen columns to it — one of them a credential — is not an additive
-- change anyone wants to make to a table already in customer databases.
--
-- A config-declared key manager ([[api_portal.key_manager]]) has no row here at
-- all; its equivalent lives in the deployed TOML. A row exists only for a key
-- manager created through the REST API, and only once someone supplies enough
-- detail for it to register clients. A `key_managers` row with no row here is
-- still valid — it just cannot generate keys.
--
-- Deliberately absent: the TLS and address-range escapes
-- (insecure_skip_verify, allow_private_endpoints, allow_http_endpoints). Those
-- relax the SSRF guard, and an endpoint set through an authenticated API is
-- exactly the case that guard exists for. They stay operator-only, settable in
-- configuration and nowhere else, so an API-created key manager always gets the
-- safe defaults.
CREATE TABLE IF NOT EXISTS key_manager_configurations (
    -- The key manager this configures. The FK pair IS the identity, so it is the
    -- primary key: no surrogate uuid, and the 1:0..1 invariant is enforced by the
    -- key itself rather than a separate UNIQUE constraint.
    key_manager_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    org_uuid VARCHAR(40) NOT NULL,
    -- Which built-in driver handles it: thunderid | wso2is | asgardeo | keycloak | custom | provision. Resolved
    -- against the driver registry at use time. Not a CHECK constraint
    -- (R4-NO-ENUM-CHECK): the valid set is the registered drivers, which ship in
    -- the image and change with it.
    driver_type VARCHAR(40) NOT NULL,
    registration_endpoint VARCHAR(255) NOT NULL,
    -- Nullable: a key manager issuing only client_credentials tokens has no
    -- interactive authorization endpoint.
    authorize_endpoint VARCHAR(255),
    description VARCHAR(1023),
    -- How the portal authenticates ITSELF to this key manager to register
    -- clients: client_credentials | basic | mtls.
    auth_method VARCHAR(20) NOT NULL,
    -- The method-specific remainder — client_id, scopes, resource,
    -- send_credentials_in_body, username, cert paths. JSON precisely so that a
    -- new credential field is a code change rather than three dialect ALTERs
    -- plus a migration this component cannot express. NEVER the secret.
    auth_config JSONB,
    -- Operator-chosen options for this key manager, as a JSON object. A bag rather
    -- than a column each, because these accumulate: a new option is then a code
    -- change in the DAO instead of three dialect ALTERs on a shipped table.
    --
    -- It is NOT an open bag. The DAO parses a closed set of known keys and drops
    -- anything else, so this is a typed structure that happens to be stored as
    -- JSON — the same treatment the (since reverted) connection_policy column had.
    -- Unparseable JSON falls back to "no options set", never to a partial read.
    --
    -- Known keys:
    --   supportedGrantTypes — string[]. The subset of the driver's declared grant
    --     types a portal user may pick. Absent/empty means "whatever the driver
    --     offers", which is the backwards-compatible default. An admin restriction,
    --     not a capability statement: the key manager may well support more.
    --
    -- VARCHAR(1023), not TEXT (R3-NO-TEXT) and not JSONB (R3-JSONB) — no repository
    -- query reads inside it; it is fetched whole with the row and parsed in JS.
    settings VARCHAR(1023),
    -- The client secret / basic password, encrypted with security.encryption_key
    -- (AES-256-GCM), never plaintext, and decrypted only when a driver is built.
    -- Same treatment as webhook_subscribers.secret_enc.
    auth_secret_enc BYTEA,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, key_manager_uuid),
    -- Owned by the parent: deleting a key manager deletes its configuration.
    FOREIGN KEY (portal_id, key_manager_uuid) REFERENCES key_managers(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);

-- One configuration per key manager, and the lookup every read uses.
CREATE INDEX IF NOT EXISTS idx_key_manager_config_org ON key_manager_configurations(org_uuid, portal_id);



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
CREATE INDEX IF NOT EXISTS idx_api_workflow_view_uuid ON api_workflows(view_uuid, portal_id);
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
CREATE UNIQUE INDEX IF NOT EXISTS uq_event_delivery_event_subscriber ON event_deliveries(event_uuid, subscriber_id, portal_id);

-- Sessions table, used by the app's SqlSessionStore for server-side Express session storage.
-- Portal-scoped: each portal's sessions are isolated at the schema level; the DAO scopes every
-- query by portal_id so one portal's process can never observe another portal's session rows.
CREATE TABLE IF NOT EXISTS sessions (
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    sid VARCHAR(255) NOT NULL,
    sess JSON NOT NULL,
    expire TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (portal_id, sid)
);
CREATE INDEX IF NOT EXISTS idx_session_expire ON sessions(expire);

-- User IdP References table (one durable record per IdP `sub` claim per portal; referenced
-- by uuid from created_by/updated_by-style columns elsewhere WITHOUT a foreign key, so those
-- columns keep pointing at a uuid after the row here is deleted).
--
-- Portal-scoped: each portal maintains its own view of a user identity so cross-portal reads
-- are structurally impossible. If the same IdP `sub` is used across two portals, each portal
-- gets its own row.
CREATE TABLE IF NOT EXISTS user_idp_references (
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    uuid VARCHAR(40) NOT NULL,
    idp_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    UNIQUE (portal_id, idp_id)
);

-- User-Organization mappings (live membership record; both sides cascade on delete,
-- unlike the "hanging creator" created_by/updated_by pattern used elsewhere).
--
-- Portal-scoped: portal_id participates in the primary key and in the composite FKs to
-- user_idp_references and organizations, so memberships never span portals.
CREATE TABLE IF NOT EXISTS user_organization_mappings (
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    user_uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    PRIMARY KEY (portal_id, user_uuid, org_uuid),
    FOREIGN KEY (portal_id, user_uuid) REFERENCES user_idp_references(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_user_organization_mappings_org_uuid ON user_organization_mappings(portal_id, org_uuid);

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

-- Meters (monetization): what a paid plan charges for.
--   - type REQUEST_COUNT counts requests; type TOKENS adds up a number from each request's event,
--     read at type_key (such as metadata.aiTokenUsage.promptTokens; null for REQUEST_COUNT).
--   - filter_fields lists the fields the prices on this meter may filter on, each with the event
--     key it is read from (a JSON array of { "field", "eventKey" }; empty for none).
-- A meter has no filters of its own: each price picks the requests it counts with its own filters
-- (subscription_plan_pricing_usage.filters).
CREATE TABLE IF NOT EXISTS meters (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    handle VARCHAR(40) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    type VARCHAR(20) NOT NULL,
    type_key VARCHAR(255),
    filter_fields BYTEA NOT NULL,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_meter_org_handle ON meters(org_uuid, handle, portal_id);

-- Plan pricing (monetization): one row per paid subscription plan; a plan with no row here is free.
--   - pricing_type FLAT_RATE: flat_amount is charged every billing period.
--   - pricing_type USAGE_BASED: the plan's prices are in subscription_plan_pricing_usage.
-- Amounts are decimal strings, never floats, so a price is stored exactly as the administrator
-- typed it.
CREATE TABLE IF NOT EXISTS subscription_plan_pricing (
    subscription_plan_uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    pricing_type VARCHAR(20) NOT NULL,
    currency VARCHAR(3) NOT NULL,
    billing_period VARCHAR(20) NOT NULL DEFAULT 'MONTH',
    flat_amount VARCHAR(40),
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, subscription_plan_uuid),
    FOREIGN KEY (portal_id, subscription_plan_uuid) REFERENCES subscription_plans(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
CREATE INDEX IF NOT EXISTS idx_subscription_plan_pricing_org ON subscription_plan_pricing(org_uuid, portal_id);

-- Plan prices (monetization): the prices of a usage based plan. Each price charges for what its
-- meter counts (meter_uuid), narrowed by its filters (a JSON array of { field, operator, values };
-- empty for none), in one pricing model:
--   - PER_UNIT: unit_amount per unit_quantity units;
--   - VOLUME or GRADUATED: tiers, a JSON array of price tiers.
-- A meter may be priced more than once in a plan, and a meter that a price uses cannot be deleted.
-- A price has no name of its own: it is shown as its meter's name and its filters, and prices are
-- listed by that name (no order is stored). Prices are set when the plan is created and never
-- change; deleting the plan deletes its prices.
CREATE TABLE IF NOT EXISTS subscription_plan_pricing_usage (
    uuid VARCHAR(40) NOT NULL,
    subscription_plan_uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    meter_uuid VARCHAR(40) NOT NULL,
    filters BYTEA NOT NULL,
    pricing_model VARCHAR(20) NOT NULL,
    unit_amount VARCHAR(40),
    unit_quantity BIGINT,
    tiers BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, subscription_plan_uuid) REFERENCES subscription_plan_pricing(portal_id, subscription_plan_uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, meter_uuid) REFERENCES meters(portal_id, uuid) ON DELETE NO ACTION,
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
CREATE INDEX IF NOT EXISTS idx_subscription_plan_pricing_usage_org ON subscription_plan_pricing_usage(org_uuid, portal_id);
CREATE INDEX IF NOT EXISTS idx_subscription_plan_pricing_usage_plan ON subscription_plan_pricing_usage(subscription_plan_uuid, portal_id);
CREATE INDEX IF NOT EXISTS idx_subscription_plan_pricing_usage_meter ON subscription_plan_pricing_usage(meter_uuid, portal_id);


-- Billing configuration (monetization): how one organization's portal sells paid plans. One per
-- organization.
--   - provider is the billing engine (MOESIF: Moesif meters usage and keeps the billing catalog).
--   - payment_provider is the payment provider the engine bills through (STRIPE: customers,
--     subscriptions, invoices and payments).
--   - The other columns are the settings of those two, and last_event_at is when Stripe last sent
--     the portal an event.
-- The Moesif Collector Application Id, the keys and the webhook signing secret are encrypted with
-- security.encryption_key (AES-256-GCM) and never stored in plain text. The Stripe publishable key
-- is public (the browser starts Stripe's checkout with it), so it is stored as is; Stripe's test or
-- live mode is read from it.
-- Removing billing takes out only what setup created and leaves what was there before, so setup
-- records it: moesif_stripe_connected_by_portal is set when setup connected Moesif to Stripe, and
-- moesif_webhook_endpoint_id only when setup created Moesif's endpoint in Stripe.
CREATE TABLE IF NOT EXISTS billing_configurations (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    provider VARCHAR(20) NOT NULL DEFAULT 'MOESIF',
    payment_provider VARCHAR(20) NOT NULL DEFAULT 'STRIPE',
    moesif_collector_application_id_enc BYTEA NOT NULL,
    moesif_management_key_enc BYTEA NOT NULL,
    stripe_api_key_enc BYTEA NOT NULL,
    stripe_publishable_key VARCHAR(255) NOT NULL,
    stripe_webhook_endpoint_id VARCHAR(255),
    stripe_webhook_secret_enc BYTEA,
    -- The public address the portal's Stripe webhook endpoint is under: server.base_url, or the
    -- one the administrator chose.
    stripe_webhook_base_url VARCHAR(1023),
    moesif_webhook_endpoint_id VARCHAR(255),
    moesif_stripe_connected_by_portal SMALLINT NOT NULL DEFAULT 0,
    last_event_at TIMESTAMPTZ,
    created_by VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(255) NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
CREATE UNIQUE INDEX IF NOT EXISTS uq_billing_configuration_org ON billing_configurations(org_uuid, portal_id);

-- Billing API plans (monetization): one paid plan as sold on one API, under one billing
-- configuration. What the portal created for it with the billing engine is in
-- billing_api_plan_items (with Moesif, which syncs it to the payment provider).
-- A row is written only once everything for it exists with the billing engine, by the request that
-- offers the plan on the API (publishing or updating the API, or setting up billing). If any part
-- fails, what the request created is removed again and the request fails, so no API plan is left
-- half made. Deleting the API or the plan, or removing billing, deletes the matching API plans;
-- what was created in Moesif stays there.
CREATE TABLE IF NOT EXISTS billing_api_plans (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    configuration_uuid VARCHAR(40) NOT NULL,
    api_uuid VARCHAR(40) NOT NULL,
    plan_uuid VARCHAR(40) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, configuration_uuid) REFERENCES billing_configurations(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, api_uuid) REFERENCES api_metadata(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, plan_uuid) REFERENCES subscription_plans(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
CREATE INDEX IF NOT EXISTS idx_billing_api_plan_org ON billing_api_plans(org_uuid, portal_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_billing_api_plan ON billing_api_plans(configuration_uuid, api_uuid, plan_uuid, portal_id);
CREATE INDEX IF NOT EXISTS idx_billing_api_plan_plan ON billing_api_plans(plan_uuid, portal_id);
CREATE INDEX IF NOT EXISTS idx_billing_api_plan_api ON billing_api_plans(api_uuid, portal_id);

-- Billing API plan items (monetization): what the billing engine holds for one billing API plan:
-- one item for a flat rate plan's fee (price_uuid null), or one per usage price. price_ref is the
-- payment provider's price, and each item becomes one item of the subscriber's subscription
-- there. With Moesif, each item is a Moesif plan with one price, and a usage price's item also has
-- a Moesif billing meter, which counts the price's requests and reports them to the payment
-- provider. Written with its API plan, in the same transaction, and never changed afterwards.
CREATE TABLE IF NOT EXISTS billing_api_plan_items (
    uuid VARCHAR(40) NOT NULL,
    api_plan_uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    price_uuid VARCHAR(40),
    price_ref VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, api_plan_uuid) REFERENCES billing_api_plans(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
CREATE INDEX IF NOT EXISTS idx_billing_api_plan_item_org ON billing_api_plan_items(org_uuid, portal_id);
CREATE INDEX IF NOT EXISTS idx_billing_api_plan_item_api_plan ON billing_api_plan_items(api_plan_uuid, portal_id);

-- Billing customers (monetization): the payment provider's customer for a portal user, one per
-- user per billing configuration, created at the user's first paid checkout (customer_ref is the
-- provider's id). With Moesif, customer_ref is also the user's Moesif company id: the gateway sends
-- it with each request event (metadata.billingCustomerId), and Moesif's billing meters count usage
-- by it.
CREATE TABLE IF NOT EXISTS billing_customers (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    configuration_uuid VARCHAR(40) NOT NULL,
    user_uuid VARCHAR(255) NOT NULL,
    customer_ref VARCHAR(255) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, configuration_uuid) REFERENCES billing_configurations(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
CREATE INDEX IF NOT EXISTS idx_billing_customer_org ON billing_customers(org_uuid, portal_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_billing_customer_user ON billing_customers(configuration_uuid, user_uuid, portal_id);

-- Billing checkouts (monetization): one per checkout the portal started with the payment provider.
-- status is OPEN until the provider confirms the payment (or that none was needed); then it is
-- COMPLETED and the subscription is created. It is EXPIRED when the checkout ends unpaid. No
-- subscription exists before it is COMPLETED.
-- api_uuid, plan_uuid and subscription_uuid have no foreign key, so a checkout still names what it
-- was for after that is deleted; invoices of ended subscriptions are named from it.
CREATE TABLE IF NOT EXISTS billing_checkouts (
    uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    user_uuid VARCHAR(255) NOT NULL,
    api_uuid VARCHAR(40) NOT NULL,
    plan_uuid VARCHAR(40) NOT NULL,
    customer_uuid VARCHAR(40) NOT NULL,
    checkout_ref VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'OPEN',
    subscription_uuid VARCHAR(40),
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, uuid),
    FOREIGN KEY (portal_id, customer_uuid) REFERENCES billing_customers(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
CREATE INDEX IF NOT EXISTS idx_billing_checkout_org ON billing_checkouts(org_uuid, portal_id);
CREATE INDEX IF NOT EXISTS idx_billing_checkout_customer ON billing_checkouts(customer_uuid, portal_id);
-- A user has at most one open checkout per API (and one subscription per API): starting another
-- checkout for the API reuses or ends the open one. startCheckout checks this in code, under a lock
-- on the user's billing_customers row, rather than with a partial unique index
-- (WHERE status = 'OPEN'), which not every database supports. This index makes that check fast.
CREATE INDEX IF NOT EXISTS idx_billing_checkout_user_api ON billing_checkouts(user_uuid, api_uuid, portal_id);

-- Subscription billing (monetization): the billing state of each paid subscription: the payment
-- provider's subscription (billing_subscription_id), its status, its current period, and ends_at
-- when it is set to end. Deleting the subscription deletes this row.
-- configuration_uuid, customer_uuid and api_plan_uuid have no foreign key: removing billing deletes
-- the configuration with its customers and API plans, but the row of a cancelled subscription is
-- kept and still names them.
CREATE TABLE IF NOT EXISTS subscription_billing (
    subscription_uuid VARCHAR(40) NOT NULL,
    org_uuid VARCHAR(40) NOT NULL,
    portal_id VARCHAR(255) NOT NULL DEFAULT 'portal_id',
    configuration_uuid VARCHAR(40) NOT NULL,
    customer_uuid VARCHAR(40) NOT NULL,
    api_plan_uuid VARCHAR(40) NOT NULL,
    billing_subscription_id VARCHAR(255) NOT NULL,
    billing_status VARCHAR(20) NOT NULL,
    current_period_start TIMESTAMPTZ,
    current_period_end TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (portal_id, subscription_uuid),
    FOREIGN KEY (portal_id, subscription_uuid) REFERENCES subscriptions(portal_id, uuid) ON DELETE CASCADE,
    FOREIGN KEY (portal_id, org_uuid) REFERENCES organizations(portal_id, uuid) ON DELETE NO ACTION
);
CREATE INDEX IF NOT EXISTS idx_subscription_billing_org ON subscription_billing(org_uuid, portal_id);
CREATE UNIQUE INDEX IF NOT EXISTS uq_subscription_billing_ref ON subscription_billing(portal_id, billing_subscription_id);
