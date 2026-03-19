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


-- Base table for all artifact types (REST APIs, WebSub APIs, LLM Providers, LLM Proxies, MCP Proxies)
CREATE TABLE IF NOT EXISTS artifacts (
    uuid TEXT PRIMARY KEY,
    gateway_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    handle TEXT NOT NULL,
    display_name TEXT NOT NULL,
    version TEXT NOT NULL,
    desired_state TEXT NOT NULL CHECK(desired_state IN ('deployed', 'undeployed')),
    deployment_id TEXT,
    origin TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deployed_at TIMESTAMP, -- NULL until first deployment
    UNIQUE(gateway_id, kind, handle),
    UNIQUE(gateway_id, kind, display_name, version)
);

-- No explicit index needed for gateway_id or kind: both UNIQUE(gateway_id, kind, display_name, version)
-- and UNIQUE(gateway_id, kind, handle) create implicit B-tree indexes whose leading columns cover
-- prefix queries on (gateway_id) and (gateway_id, kind).

CREATE INDEX IF NOT EXISTS idx_artifacts_gateway_id_desired_state ON artifacts(gateway_id, desired_state);
CREATE INDEX IF NOT EXISTS idx_artifacts_gateway_id_deployment_id ON artifacts(gateway_id, deployment_id);

-- Per-resource-type tables (each stores source configuration as JSON)

CREATE TABLE IF NOT EXISTS rest_apis (
    uuid TEXT PRIMARY KEY,
    configuration TEXT NOT NULL,
    FOREIGN KEY(uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS websub_apis (
    uuid TEXT PRIMARY KEY,
    configuration TEXT NOT NULL,
    FOREIGN KEY(uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS llm_providers (
    uuid TEXT PRIMARY KEY,
    configuration TEXT NOT NULL,
    FOREIGN KEY(uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS llm_proxies (
    uuid TEXT PRIMARY KEY,
    configuration TEXT NOT NULL,
    provider_uuid TEXT NOT NULL,
    FOREIGN KEY(uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY(provider_uuid) REFERENCES llm_providers(uuid) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS mcp_proxies (
    uuid TEXT PRIMARY KEY,
    configuration TEXT NOT NULL,
    FOREIGN KEY(uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE
);

-- Table for custom TLS certificates
CREATE TABLE IF NOT EXISTS certificates (
    uuid TEXT PRIMARY KEY,
    gateway_id TEXT NOT NULL,
    name TEXT NOT NULL,
    certificate BLOB NOT NULL,
    subject TEXT NOT NULL,
    issuer TEXT NOT NULL,
    not_before TIMESTAMP NOT NULL,
    not_after TIMESTAMP NOT NULL,
    cert_count INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(gateway_id, name)
);

-- No explicit index needed for gateway_id: UNIQUE(gateway_id, name) creates an implicit index
-- whose leading column covers prefix queries on (gateway_id).

-- Index for expiry tracking
CREATE INDEX IF NOT EXISTS idx_certificates_gateway_id_expiry ON certificates(gateway_id, not_after);


-- LLM Provider Templates table
CREATE TABLE IF NOT EXISTS llm_provider_templates (
    uuid TEXT PRIMARY KEY,
    gateway_id TEXT NOT NULL,
    handle TEXT NOT NULL,
    configuration TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(gateway_id, handle)
);

-- No explicit index needed for gateway_id: UNIQUE(gateway_id, handle) creates an implicit index
-- whose leading column covers prefix queries on (gateway_id).

-- Table for API keys
CREATE TABLE IF NOT EXISTS api_keys (
    uuid TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    name TEXT NOT NULL, -- Human-readable name for the API key
    artifact_uuid TEXT NOT NULL,
    api_key TEXT NOT NULL, -- The generated API key (hashed)
    masked_api_key TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('active', 'revoked', 'expired')) DEFAULT 'active',
    created_by TEXT NOT NULL DEFAULT 'system',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NULL, -- NULL means no expiration
    source TEXT NOT NULL DEFAULT 'local',  -- 'local' or 'external'
    external_ref_id TEXT NULL,
    issuer TEXT NULL DEFAULT NULL, -- developer portal identifier; NULL means not specified

    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    UNIQUE (gateway_id, artifact_uuid, name),
    UNIQUE (gateway_id, artifact_uuid, api_key),
    PRIMARY KEY (uuid)
);

-- Indexes for API key lookups
-- No explicit index needed for artifact_uuid or api_key: UNIQUE(gateway_id, artifact_uuid, name) and
-- UNIQUE(gateway_id, artifact_uuid, api_key) create implicit indexes whose leading columns cover
-- prefix queries on (gateway_id) and (gateway_id, artifact_uuid).
CREATE INDEX IF NOT EXISTS idx_api_keys_gateway_id_status ON api_keys(gateway_id, status);
CREATE INDEX IF NOT EXISTS idx_api_keys_gateway_id_created_by ON api_keys(gateway_id, created_by);

-- Subscription plans table (organization-scoped rate/billing plans)
CREATE TABLE IF NOT EXISTS subscription_plans (
    uuid TEXT PRIMARY KEY,
    gateway_id TEXT NOT NULL,
    plan_name TEXT NOT NULL,
    billing_plan TEXT,
    stop_on_quota_reach INTEGER DEFAULT 1,
    throttle_limit_count INTEGER,
    throttle_limit_unit TEXT,
    expiry_time TIMESTAMP,
    status TEXT NOT NULL CHECK(status IN ('ACTIVE', 'INACTIVE')) DEFAULT 'ACTIVE',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(gateway_id, plan_name)
);

-- Subscriptions table (application-level subscriptions for REST APIs)
-- subscription_token_hash: for xDS validation and request validation (Platform-API stores original token)
CREATE TABLE IF NOT EXISTS subscriptions (
    uuid TEXT PRIMARY KEY,
    gateway_id TEXT NOT NULL,
    api_id TEXT NOT NULL,
    application_id TEXT,
    subscription_token_hash TEXT NOT NULL,
    subscription_plan_id TEXT,
    status TEXT NOT NULL CHECK(status IN ('ACTIVE', 'INACTIVE', 'REVOKED')) DEFAULT 'ACTIVE',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (api_id) REFERENCES rest_apis(uuid) ON DELETE CASCADE,
    FOREIGN KEY (subscription_plan_id) REFERENCES subscription_plans(uuid) ON DELETE SET NULL,
    UNIQUE(api_id, subscription_token_hash, gateway_id)
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_api_id ON subscriptions(api_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_application_id ON subscriptions(application_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_gateway_id ON subscriptions(gateway_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_token ON subscriptions(subscription_token_hash);

-- Table for gateway states (used by eventhub for multi-replica sync)
CREATE TABLE IF NOT EXISTS gateway_states (
    gateway_id TEXT PRIMARY KEY,
    version_id TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Table for events (used by eventhub for multi-replica sync)
CREATE TABLE IF NOT EXISTS events (
    gateway_id TEXT NOT NULL,
    processed_timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    originated_timestamp TIMESTAMP NOT NULL,
    entity_type TEXT NOT NULL,
    action TEXT NOT NULL CHECK(action IN ('CREATE', 'UPDATE', 'DELETE')),
    entity_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    event_data TEXT NOT NULL,
    PRIMARY KEY (event_id),
    FOREIGN KEY (gateway_id) REFERENCES gateway_states(gateway_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_events_gateway_id_processed_timestamp ON events(gateway_id, processed_timestamp);
-- Applications
CREATE TABLE IF NOT EXISTS applications (
    application_uuid TEXT PRIMARY KEY,
    application_id TEXT NOT NULL,
    application_name TEXT NOT NULL,
    application_type TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_applications_application_id ON applications(application_id);

-- Application to API key mappings
CREATE TABLE IF NOT EXISTS application_api_keys (
    application_uuid TEXT NOT NULL,
    api_key_id TEXT NOT NULL,
    gateway_id TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (application_uuid, api_key_id, gateway_id),
    FOREIGN KEY (application_uuid) REFERENCES applications(application_uuid) ON DELETE CASCADE,
    FOREIGN KEY (api_key_id, gateway_id) REFERENCES api_keys(uuid, gateway_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_app_api_keys_application_uuid ON application_api_keys(application_uuid, gateway_id);
CREATE INDEX IF NOT EXISTS idx_app_api_keys_apikey ON application_api_keys(api_key_id, gateway_id);

PRAGMA user_version = 1;
