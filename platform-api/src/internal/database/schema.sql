/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

-- Organizations table
CREATE TABLE IF NOT EXISTS organizations (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(40) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    region VARCHAR(63) NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);


-- Projects table
CREATE TABLE IF NOT EXISTS projects (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);

-- Applications table
CREATE TABLE IF NOT EXISTS applications (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(40) NOT NULL,
    project_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    type VARCHAR(50) NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);

-- Artifacts table
CREATE TABLE IF NOT EXISTS artifacts (
    uuid VARCHAR(40) PRIMARY KEY,
    type VARCHAR(20) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    -- Ensure (uuid, organization_uuid) pairs are unique so they can be safely
    -- referenced from subscriptions to enforce API–organization consistency.
    UNIQUE(organization_uuid, uuid)
);


-- REST APIs table
CREATE TABLE IF NOT EXISTS rest_apis (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    lifecycle_status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration BLOB NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    origin VARCHAR(20) NOT NULL DEFAULT 'control_plane',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);

-- Subscription plans table (organization-scoped rate/billing plans)
-- Throttling limits now live in subscription_plan_limits (one row per limit).
CREATE TABLE IF NOT EXISTS subscription_plans (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    billing_plan VARCHAR(255),
    expiry_time DATETIME,
    organization_uuid VARCHAR(40) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);

-- Subscription plan limits table (throttling limits for a plan).
CREATE TABLE IF NOT EXISTS subscription_plan_limits (
    uuid VARCHAR(40) PRIMARY KEY,
    subscription_plan_uuid VARCHAR(40) NOT NULL,
    limit_type VARCHAR(20) NOT NULL DEFAULT 'REQUEST_COUNT',
    time_unit VARCHAR(20) NOT NULL,
    time_amount INTEGER NOT NULL DEFAULT 1,
    limit_count BIGINT NOT NULL,
    -- limit_count_unit (KB/MB/GB) for the quota; only set when limit_type is BANDWIDTH.
    limit_count_unit VARCHAR(10),
    stop_on_quota_reach SMALLINT NOT NULL DEFAULT 1,
    FOREIGN KEY (subscription_plan_uuid) REFERENCES subscription_plans(uuid) ON DELETE CASCADE,
    UNIQUE(subscription_plan_uuid, limit_type, time_amount, time_unit)
);

-- Subscriptions table (application-level subscriptions for any artifact type)
-- subscription_token: encrypted (AES-256-GCM) for retrieval; subscription_token_hash for uniqueness and gateway sync
-- application_id references applications in DevPortal/STS (no FK in platform), optional for token-based subscriptions
CREATE TABLE IF NOT EXISTS subscriptions (
    uuid VARCHAR(40) PRIMARY KEY,
    artifact_uuid VARCHAR(40) NOT NULL,
    subscriber_id VARCHAR(255) NOT NULL,
    application_id VARCHAR(255),
    subscription_token VARCHAR(512) NOT NULL,
    subscription_token_hash VARCHAR(255) NOT NULL,
    subscription_plan_uuid VARCHAR(40),
    organization_uuid VARCHAR(40) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (subscription_plan_uuid) REFERENCES subscription_plans(uuid),
    FOREIGN KEY (artifact_uuid, organization_uuid)
      REFERENCES artifacts(uuid, organization_uuid) ON DELETE CASCADE,
    UNIQUE(artifact_uuid, subscription_token_hash)
);

-- Gateways table (scoped to organizations)
-- Must be created before deployments which references it
CREATE TABLE IF NOT EXISTS gateways (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    version VARCHAR(30) NOT NULL DEFAULT '1.0',
    vhost VARCHAR(255) NOT NULL,
    gateway_functionality_type VARCHAR(20) NOT NULL DEFAULT 'regular',
    properties BLOB NOT NULL,
    manifest BLOB,
    is_active INTEGER DEFAULT 0,
    is_critical INTEGER DEFAULT 0,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);

-- Artifact Gateway Mapping table (links artifacts to gateways)
CREATE TABLE IF NOT EXISTS artifact_gateway_mappings (
    artifact_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    gateway_uuid VARCHAR(40) NOT NULL,
    metadata BLOB,
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (organization_uuid, artifact_uuid, gateway_uuid),
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE
);

-- Gateway Custom Policies table (org-scoped custom policies synced from gateway manifests)
CREATE TABLE IF NOT EXISTS gateway_custom_policies (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255),
    version VARCHAR(30) NOT NULL,
    description VARCHAR(1023),
    policy_definition BLOB,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, name, version)
);

-- Gateway Custom Policy Usages table (tracks which APIs use each custom policy)
CREATE TABLE IF NOT EXISTS gateway_custom_policy_usages (
    policy_uuid VARCHAR(40) NOT NULL,
    artifact_uuid VARCHAR(40) NOT NULL,
    PRIMARY KEY (policy_uuid, artifact_uuid),
    FOREIGN KEY (policy_uuid) REFERENCES gateway_custom_policies(uuid) ON DELETE CASCADE,
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE
);

-- Gateway Tokens table
CREATE TABLE IF NOT EXISTS gateway_tokens (
    uuid VARCHAR(40) PRIMARY KEY,
    gateway_uuid VARCHAR(40) NOT NULL,
    token_hash VARCHAR(255) NOT NULL,
    salt VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    revoked_by VARCHAR(200),
    revoked_at DATETIME,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE
);

-- Artifact Deployments table (immutable deployment artifacts)
CREATE TABLE IF NOT EXISTS deployments (
    uuid VARCHAR(40) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    artifact_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    gateway_uuid VARCHAR(40) NOT NULL,
    base_deployment_uuid VARCHAR(40),
    content BLOB NOT NULL,
    metadata BLOB,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE,
    FOREIGN KEY (base_deployment_uuid) REFERENCES deployments(uuid) ON DELETE SET NULL
);

-- Artifact Deployment Status table (current deployment state per artifact+Gateway)
CREATE TABLE IF NOT EXISTS deployment_status (
    artifact_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    gateway_uuid VARCHAR(40) NOT NULL,
    deployment_uuid VARCHAR(40) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'DEPLOYED',
    status_desired VARCHAR(20),
    performed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    performed_by VARCHAR(200),
    status_reason VARCHAR(50),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (organization_uuid, artifact_uuid, gateway_uuid),
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE,
    FOREIGN KEY (deployment_uuid) REFERENCES deployments(uuid) ON DELETE CASCADE
);

-- LLM Provider Templates table
CREATE TABLE IF NOT EXISTS llm_provider_templates (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(40) NOT NULL,
    group_id VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    managed_by VARCHAR(255) NOT NULL DEFAULT 'customer',
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    description VARCHAR(1023),
    configuration BLOB NOT NULL,
    openapi_spec BLOB,
    is_latest INTEGER NOT NULL DEFAULT 1,
    enabled INTEGER NOT NULL DEFAULT 1,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    origin VARCHAR(20) NOT NULL DEFAULT 'control_plane',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, group_id, version),
    UNIQUE(organization_uuid, handle)
);

-- LLM Providers table
CREATE TABLE IF NOT EXISTS llm_providers (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    description VARCHAR(1023),
    template_uuid VARCHAR(40) NOT NULL,
    openapi_spec BLOB,
    model_list BLOB,
    configuration BLOB NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    origin VARCHAR(20) NOT NULL DEFAULT 'control_plane',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    organization_uuid VARCHAR(40) NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (template_uuid) REFERENCES llm_provider_templates(uuid),
    UNIQUE(organization_uuid, handle)
);

-- LLM Proxies table
CREATE TABLE IF NOT EXISTS llm_proxies (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    provider_uuid VARCHAR(40) NOT NULL,
    openapi_spec BLOB,
    configuration BLOB NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    origin VARCHAR(20) NOT NULL DEFAULT 'control_plane',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    organization_uuid VARCHAR(40) NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    FOREIGN KEY (provider_uuid) REFERENCES llm_providers(uuid),
    UNIQUE(organization_uuid, handle)
);

-- MCP Proxies table
CREATE TABLE IF NOT EXISTS mcp_proxies (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    project_uuid VARCHAR(40),
    description VARCHAR(1023),
    configuration BLOB NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    origin VARCHAR(20) NOT NULL DEFAULT 'control_plane',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    organization_uuid VARCHAR(40) NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);

-- WEBSUB APIs table
CREATE TABLE IF NOT EXISTS websub_apis (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    lifecycle_status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration BLOB NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    origin VARCHAR(20) NOT NULL DEFAULT 'control_plane',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);
CREATE INDEX IF NOT EXISTS idx_websub_apis_project ON websub_apis(project_uuid);

-- WebSub API HMAC secrets table (for inbound webhook event verification)
CREATE TABLE IF NOT EXISTS websub_api_hmac_secrets (
    uuid VARCHAR(40) PRIMARY KEY,
    artifact_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255),
    encrypted_secret BLOB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    UNIQUE(artifact_uuid, handle)
);
CREATE INDEX IF NOT EXISTS idx_websub_api_hmac_secrets_artifact ON websub_api_hmac_secrets(artifact_uuid);

-- WEBBROKER APIs table
CREATE TABLE IF NOT EXISTS webbroker_apis (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    lifecycle_status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration BLOB NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    origin VARCHAR(20) NOT NULL DEFAULT 'control_plane',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);
CREATE INDEX IF NOT EXISTS idx_webbroker_apis_project ON webbroker_apis(project_uuid);

-- API Keys table (stores API keys for artifacts with hashes as JSON string)
CREATE TABLE IF NOT EXISTS api_keys (
    uuid VARCHAR(40) PRIMARY KEY,
    artifact_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(63) NOT NULL,
    masked_api_key VARCHAR(8) NOT NULL,
    api_key_hashes BLOB NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME,
    issuer VARCHAR(255) NULL DEFAULT NULL,
    allowed_targets VARCHAR(255) NOT NULL DEFAULT 'ALL',
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    UNIQUE(artifact_uuid, name)
);

-- Application API Key mappings table
CREATE TABLE IF NOT EXISTS application_api_key_mappings (
    application_uuid VARCHAR(40) NOT NULL,
    api_key_id VARCHAR(40) NOT NULL,
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (application_uuid, api_key_id),
    FOREIGN KEY (application_uuid) REFERENCES applications(uuid) ON DELETE CASCADE,
    FOREIGN KEY (api_key_id) REFERENCES api_keys(uuid) ON DELETE CASCADE
);

-- Application to artifacts mapping table
CREATE TABLE IF NOT EXISTS application_artifact_mappings (
    application_uuid VARCHAR(40) NOT NULL,
    artifact_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(200),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (application_uuid, artifact_uuid),
    FOREIGN KEY (application_uuid) REFERENCES applications(uuid) ON DELETE CASCADE,
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE
);

-- Indexes for better performance
CREATE INDEX IF NOT EXISTS idx_projects_organization_id ON projects(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_rest_apis_project_id ON rest_apis(project_uuid);
CREATE INDEX IF NOT EXISTS idx_subscriptions_artifact_uuid ON subscriptions(artifact_uuid);
CREATE INDEX IF NOT EXISTS idx_subscriptions_application_id ON subscriptions(application_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_organization_uuid ON subscriptions(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_subscriptions_status ON subscriptions(status);
CREATE INDEX IF NOT EXISTS idx_subscriptions_plan ON subscriptions(subscription_plan_uuid);
CREATE INDEX IF NOT EXISTS idx_subscriptions_token ON subscriptions(subscription_token_hash);
CREATE INDEX IF NOT EXISTS idx_subscriptions_org_subscriber ON subscriptions(organization_uuid, subscriber_id);
-- Enforce one subscription per application per artifact per org. MySQL allows multiple NULLs in a unique
-- index, so this correctly permits multiple token-based (NULL application_id) subscriptions on the same artifact.
CREATE UNIQUE INDEX IF NOT EXISTS uq_subscriptions_org_artifact_app ON subscriptions(organization_uuid, artifact_uuid, application_id);
CREATE INDEX IF NOT EXISTS idx_gateways_org ON gateways(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_gateway_tokens_status ON gateway_tokens(gateway_uuid, status);
CREATE INDEX IF NOT EXISTS idx_gateway_tokens_hash ON gateway_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_artifact_deployments_created_at ON deployments(artifact_uuid, gateway_uuid, created_at);
CREATE INDEX IF NOT EXISTS idx_artifact_gw_created ON deployments(organization_uuid, artifact_uuid, gateway_uuid, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_deployments_base_id ON deployments(base_deployment_uuid) WHERE base_deployment_uuid IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_deployment_status_deployment ON deployment_status(deployment_uuid);
CREATE INDEX IF NOT EXISTS idx_deployment_status_status ON deployment_status(status);
CREATE INDEX IF NOT EXISTS idx_artifacts_org ON artifacts(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_artifacts_org_uuid ON artifacts(organization_uuid, uuid);
CREATE INDEX IF NOT EXISTS idx_gateway_custom_policies_org ON gateway_custom_policies(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_gateway_custom_policy_usages_artifact ON gateway_custom_policy_usages(artifact_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_provider_templates_org ON llm_provider_templates(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_provider_templates_group ON llm_provider_templates(organization_uuid, group_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_llm_provider_templates_latest ON llm_provider_templates(organization_uuid, group_id) WHERE is_latest = 1;
CREATE INDEX IF NOT EXISTS idx_llm_providers_template ON llm_providers(template_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_providers_org ON llm_providers(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_proxies_project ON llm_proxies(project_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_proxies_provider_uuid ON llm_proxies(provider_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_proxies_org ON llm_proxies(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_mcp_proxies_project ON mcp_proxies(project_uuid);
CREATE INDEX IF NOT EXISTS idx_mcp_proxies_org ON mcp_proxies(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_api_keys_artifact ON api_keys(artifact_uuid);
CREATE INDEX IF NOT EXISTS idx_rest_apis_org ON rest_apis(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_applications_org ON applications(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_applications_project_id ON applications(organization_uuid, project_uuid);
CREATE INDEX IF NOT EXISTS idx_application_api_key_mappings_app_id ON application_api_key_mappings(application_uuid);
CREATE INDEX IF NOT EXISTS idx_application_api_key_mappings_key_id ON application_api_key_mappings(api_key_id);
CREATE INDEX IF NOT EXISTS idx_application_artifact_mappings_app_id ON application_artifact_mappings(application_uuid);
CREATE INDEX IF NOT EXISTS idx_application_artifact_mappings_artifact_id ON application_artifact_mappings(artifact_uuid);
CREATE INDEX IF NOT EXISTS idx_api_keys_status ON api_keys(status);
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON api_keys(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_rest_apis_lifecycle_status ON rest_apis(lifecycle_status);
CREATE INDEX IF NOT EXISTS idx_websub_apis_lifecycle_status ON websub_apis(lifecycle_status);
CREATE INDEX IF NOT EXISTS idx_webbroker_apis_lifecycle_status ON webbroker_apis(lifecycle_status);
CREATE INDEX IF NOT EXISTS idx_subscription_plans_org    ON subscription_plans(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_subscription_plans_status ON subscription_plans(status);
CREATE INDEX IF NOT EXISTS idx_subscription_plan_limits_plan ON subscription_plan_limits(subscription_plan_uuid);

-- EventHub tables for multi-replica HA sync
CREATE TABLE IF NOT EXISTS gateway_states (
    gateway_id VARCHAR(40) PRIMARY KEY,
    version_id VARCHAR(255) NOT NULL DEFAULT '',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (gateway_id) REFERENCES gateways(uuid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS events (
    gateway_id VARCHAR(40) NOT NULL,
    processed_timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    originated_timestamp DATETIME NOT NULL,
    entity_type VARCHAR(255) NOT NULL,
    action VARCHAR(20) NOT NULL,
    entity_id VARCHAR(255) NOT NULL,
    event_id VARCHAR(64) NOT NULL,
    event_data TEXT NOT NULL,
    PRIMARY KEY (gateway_id, event_id),
    FOREIGN KEY (gateway_id) REFERENCES gateway_states(gateway_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_events_gateway_id_processed_timestamp ON events(gateway_id, processed_timestamp);
CREATE INDEX IF NOT EXISTS idx_events_entity ON events(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_artifact_gateway_mappings_artifact_uuid ON artifact_gateway_mappings(artifact_uuid);
CREATE INDEX IF NOT EXISTS idx_artifact_gateway_mappings_gateway_uuid ON artifact_gateway_mappings(gateway_uuid);

CREATE TABLE IF NOT EXISTS audit (
   uuid VARCHAR(40) PRIMARY KEY,
   action VARCHAR(50) NOT NULL,
   resource_uuid VARCHAR(40) NOT NULL,
   resource_type VARCHAR(50),
   organization_uuid VARCHAR(40) NOT NULL,
   performed_by VARCHAR(200),
   performed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
   FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_audit_org ON audit(organization_uuid);

-- Secrets table for encrypted secret management
CREATE TABLE IF NOT EXISTS secrets (
    uuid              VARCHAR(40)   PRIMARY KEY,
    organization_uuid VARCHAR(40)   NOT NULL,
    handle            VARCHAR(40)   NOT NULL,
    name              VARCHAR(255)  NOT NULL,
    description       VARCHAR(1023),
    ciphertext        BLOB          NOT NULL,
    hash              VARCHAR(255)  NOT NULL,
    data_version      VARCHAR(20)   NOT NULL DEFAULT '1.0',
    type              VARCHAR(20)   NOT NULL DEFAULT 'GENERIC',
    provider          VARCHAR(20)   NOT NULL DEFAULT 'IN_BUILT',
    status            VARCHAR(20)   NOT NULL DEFAULT 'ACTIVE',
    created_at        DATETIME      NOT NULL,
    created_by        VARCHAR(255),
    updated_at        DATETIME      NOT NULL,
    updated_by        VARCHAR(255),
    UNIQUE (organization_uuid, handle),
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_secrets_updated_at ON secrets(updated_at);

CREATE TABLE IF NOT EXISTS secret_scopes (
    secret_uuid VARCHAR(40)  NOT NULL,
    scope       VARCHAR(30)  NOT NULL,
    scope_value VARCHAR(40)  NOT NULL,
    PRIMARY KEY (secret_uuid, scope, scope_value),
    FOREIGN KEY (secret_uuid) REFERENCES secrets(uuid) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_secret_scopes_scope ON secret_scopes(scope, scope_value);

-- Pre-computed secret handle references per deployed artifact per gateway.
CREATE TABLE IF NOT EXISTS artifact_secret_refs (
    organization_uuid VARCHAR(40)  NOT NULL,
    artifact_uuid     VARCHAR(40)  NOT NULL,
    secret_handle     VARCHAR(40)  NOT NULL,
    gateway_id        VARCHAR(40)  NOT NULL DEFAULT '',
    created_at        DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (organization_uuid, artifact_uuid, secret_handle, gateway_id),
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (artifact_uuid)     REFERENCES artifacts(uuid)     ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_asr_org_handle
    ON artifact_secret_refs(organization_uuid, secret_handle);

CREATE INDEX IF NOT EXISTS idx_asr_org_gateway
    ON artifact_secret_refs(organization_uuid, gateway_id);
