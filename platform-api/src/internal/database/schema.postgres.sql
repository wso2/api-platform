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

-- Organizations table
CREATE TABLE IF NOT EXISTS organizations (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    region VARCHAR(63) NOT NULL,
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);


-- Projects table
CREATE TABLE IF NOT EXISTS projects (
    uuid VARCHAR(40) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(name, organization_uuid)
);

-- Applications table
CREATE TABLE IF NOT EXISTS applications (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(255) NOT NULL,
    project_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    type VARCHAR(50) NOT NULL,
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(project_uuid, organization_uuid, name),
    UNIQUE(handle, organization_uuid)
);

-- Artifacts table
CREATE TABLE IF NOT EXISTS artifacts (
    uuid VARCHAR(40) PRIMARY KEY,
    kind VARCHAR(20) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE RESTRICT,
    -- Ensure (uuid, organization_uuid) pairs are unique so they can be safely
    -- referenced from subscriptions to enforce API–organization consistency.
    UNIQUE(organization_uuid, uuid)
);

-- REST APIs table
CREATE TABLE IF NOT EXISTS rest_apis (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL,
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    lifecycle_status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration JSONB NOT NULL,
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle),
    CHECK (lifecycle_status IN ('CREATED','STAGED','PUBLISHED','DEPRECATED','RETIRED','BLOCKED'))
);

-- Subscription plans table (organization-scoped rate/billing plans)
CREATE TABLE IF NOT EXISTS subscription_plans (
    uuid VARCHAR(40) PRIMARY KEY,
    plan_name VARCHAR(40) NOT NULL,
    billing_plan VARCHAR(255),
    stop_on_quota_reach BOOLEAN DEFAULT TRUE,
    throttle_limit_count INTEGER,
    throttle_limit_unit VARCHAR(20),
    expiry_time TIMESTAMP,
    organization_uuid VARCHAR(40) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, plan_name),
    UNIQUE(organization_uuid, uuid),
    CHECK (status IN ('ACTIVE', 'INACTIVE')),
    CONSTRAINT chk_plan_throttle_pair CHECK (
      (throttle_limit_count IS NULL AND throttle_limit_unit IS NULL) OR
      (throttle_limit_count IS NOT NULL AND throttle_limit_unit IS NOT NULL)
    )
);

-- Subscriptions table (application-level subscriptions for any artifact type)
-- subscription_token: encrypted value (AES-256-GCM) for retrieval (legacy rows have hash)
-- subscription_token_hash: SHA-256 hash for uniqueness and gateway sync
CREATE TABLE IF NOT EXISTS subscriptions (
    uuid VARCHAR(40) PRIMARY KEY,
    artifact_uuid VARCHAR(40) NOT NULL,
    subscriber_id VARCHAR(255) NOT NULL,
    application_id VARCHAR(255),
    subscription_token VARCHAR(512) NOT NULL,
    subscription_token_hash VARCHAR(64) NOT NULL,
    subscription_plan_uuid VARCHAR(40),
    organization_uuid VARCHAR(40) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (subscription_plan_uuid, organization_uuid)
      REFERENCES subscription_plans(uuid, organization_uuid) ON DELETE RESTRICT,
    FOREIGN KEY (artifact_uuid, organization_uuid)
      REFERENCES artifacts(uuid, organization_uuid) ON DELETE CASCADE,
    UNIQUE(artifact_uuid, subscription_token_hash),
    UNIQUE(artifact_uuid, application_id, organization_uuid),
    CHECK (status IN ('ACTIVE', 'INACTIVE', 'REVOKED'))
);
CREATE INDEX IF NOT EXISTS idx_subscriptions_token ON subscriptions(subscription_token_hash);
-- Supports list/count filters: WHERE organization_uuid = ? AND subscriber_id = ? (no artifact_uuid).
-- The unique constraint on (artifact_uuid, application_id, organization_uuid) is not ordered for this access path.
CREATE INDEX IF NOT EXISTS idx_subscriptions_org_subscriber ON subscriptions(organization_uuid, subscriber_id);

-- Gateways table (scoped to organizations)
-- Must be created before deployments which references it
CREATE TABLE IF NOT EXISTS gateways (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    version VARCHAR(64) NOT NULL DEFAULT '1.0',
    vhost VARCHAR(255) NOT NULL,
    gateway_functionality_type VARCHAR(20) DEFAULT 'regular' NOT NULL,
    properties JSONB NOT NULL DEFAULT '{}'::jsonb,
    manifest JSONB,
    is_active BOOLEAN DEFAULT FALSE,
    is_critical BOOLEAN DEFAULT FALSE,
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle),
    CHECK (gateway_functionality_type IN ('regular', 'ai', 'event'))
);

-- Gateway Association Mappings table (links artifacts to gateways)
CREATE TABLE IF NOT EXISTS gateway_association_mappings (
    artifact_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    gateway_uuid VARCHAR(40) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, artifact_uuid, gateway_uuid)
);

-- Gateway Custom Policies table (org-scoped custom policies synced from gateway manifests)
CREATE TABLE IF NOT EXISTS gateway_custom_policies (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255),
    version VARCHAR(30) NOT NULL,
    description VARCHAR(1023),
    policy_definition TEXT,
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
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
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    revoked_by VARCHAR(200),
    revoked_at TIMESTAMP,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE,
    CHECK (status IN ('active', 'revoked')),
    CHECK (revoked_at IS NULL OR status = 'revoked')
);

-- Artifact Deployments table (immutable deployment artifacts)
CREATE TABLE IF NOT EXISTS deployments (
    deployment_id VARCHAR(40) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    artifact_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    gateway_uuid VARCHAR(40) NOT NULL,
    base_deployment_id VARCHAR(40),
    content BYTEA NOT NULL,
    metadata JSONB,
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE,
    FOREIGN KEY (base_deployment_id) REFERENCES deployments(deployment_id) ON DELETE SET NULL
);

-- Artifact Deployment Status table (current deployment state per artifact+Gateway)
CREATE TABLE IF NOT EXISTS deployment_status (
    artifact_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    gateway_uuid VARCHAR(40) NOT NULL,
    deployment_id VARCHAR(40) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'DEPLOYED',
    status_desired VARCHAR(20),
    performed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    performed_by VARCHAR(200),
    status_reason VARCHAR(50),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (artifact_uuid, organization_uuid, gateway_uuid),
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE,
    FOREIGN KEY (deployment_id) REFERENCES deployments(deployment_id) ON DELETE CASCADE,
    CHECK (status IN ('DEPLOYED', 'DEPLOYING', 'UNDEPLOYED', 'UNDEPLOYING', 'FAILED', 'ARCHIVED')),
    CHECK (status_desired IN ('DEPLOYED', 'UNDEPLOYED'))
);

-- LLM Provider Templates table
CREATE TABLE IF NOT EXISTS llm_provider_templates (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL DEFAULT '1.0',
    description VARCHAR(1023),
    configuration JSONB NOT NULL,
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    organization_uuid VARCHAR(40) NOT NULL,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);

-- LLM Providers table
CREATE TABLE IF NOT EXISTS llm_providers (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL,
    description VARCHAR(1023),
    template_uuid VARCHAR(40) NOT NULL,
    openapi_spec JSONB,
    model_list TEXT,
    configuration JSONB NOT NULL,
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    organization_uuid VARCHAR(40) NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (template_uuid) REFERENCES llm_provider_templates(uuid) ON DELETE RESTRICT,
    UNIQUE(organization_uuid, handle)
);

-- LLM Proxies table
CREATE TABLE IF NOT EXISTS llm_proxies (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL,
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    provider_uuid VARCHAR(40) NOT NULL,
    openapi_spec JSONB,
    configuration JSONB NOT NULL,
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    organization_uuid VARCHAR(40) NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    FOREIGN KEY (provider_uuid) REFERENCES llm_providers(uuid) ON DELETE RESTRICT,
    UNIQUE(organization_uuid, handle)
);

-- MCP Proxies table
CREATE TABLE IF NOT EXISTS mcp_proxies (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL,
    project_uuid VARCHAR(40),
    description VARCHAR(1023),
    configuration JSONB NOT NULL,
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    organization_uuid VARCHAR(40) NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);

-- WEBSUB APIs table
CREATE TABLE IF NOT EXISTS websub_apis (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL,
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    lifecycle_status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration JSONB NOT NULL,
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle),
    CHECK (lifecycle_status IN ('CREATED','STAGED','PUBLISHED','DEPRECATED','RETIRED','BLOCKED'))
);
CREATE INDEX IF NOT EXISTS idx_websub_apis_project ON websub_apis(project_uuid);

-- WEBBROKER APIs table
CREATE TABLE IF NOT EXISTS webbroker_apis (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL,
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    lifecycle_status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration JSONB NOT NULL,
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle),
    CHECK (lifecycle_status IN ('CREATED','STAGED','PUBLISHED','DEPRECATED','RETIRED','BLOCKED'))
);
CREATE INDEX IF NOT EXISTS idx_webbroker_apis_project ON webbroker_apis(project_uuid);

-- API Keys table (stores API keys for artifacts with hashes as JSON string)
CREATE TABLE IF NOT EXISTS api_keys (
    uuid VARCHAR(40) PRIMARY KEY,
    artifact_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(63) NOT NULL,
    masked_api_key VARCHAR(8) NOT NULL,
    api_key_hashes JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_by VARCHAR(200),
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    issuer TEXT NULL DEFAULT NULL,
    allowed_targets TEXT NOT NULL DEFAULT 'ALL',
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    UNIQUE(artifact_uuid, name),
    CHECK (status IN ('active', 'expired', 'revoked'))
);

-- Application API Key mappings table
CREATE TABLE IF NOT EXISTS application_api_keys (
    application_uuid VARCHAR(40) NOT NULL,
    api_key_id VARCHAR(40) NOT NULL,
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (application_uuid, api_key_id),
    FOREIGN KEY (application_uuid) REFERENCES applications(uuid) ON DELETE CASCADE,
    FOREIGN KEY (api_key_id) REFERENCES api_keys(uuid) ON DELETE CASCADE
);

-- Application to artifacts mapping table
CREATE TABLE IF NOT EXISTS application_artifacts (
    application_uuid VARCHAR(40) NOT NULL,
    artifact_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(200),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
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
CREATE INDEX IF NOT EXISTS idx_gateways_org ON gateways(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_gateway_tokens_status ON gateway_tokens(gateway_uuid, status);
CREATE INDEX IF NOT EXISTS idx_artifact_deployments_created_at ON deployments(artifact_uuid, gateway_uuid, created_at);
CREATE INDEX IF NOT EXISTS idx_artifact_gw_created ON deployments(artifact_uuid, organization_uuid, gateway_uuid, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_deployments_base_id ON deployments(base_deployment_id) WHERE base_deployment_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_deployment_status_deployment ON deployment_status(deployment_id);
CREATE INDEX IF NOT EXISTS idx_deployment_status_status ON deployment_status(status);
CREATE INDEX IF NOT EXISTS idx_artifacts_org ON artifacts(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_artifacts_org_uuid ON artifacts(organization_uuid, uuid);
CREATE INDEX IF NOT EXISTS idx_gateway_tokens_hash ON gateway_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_gateway_custom_policies_org ON gateway_custom_policies(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_gateway_custom_policy_usages_artifact ON gateway_custom_policy_usages(artifact_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_provider_templates_org ON llm_provider_templates(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_providers_template ON llm_providers(template_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_providers_org ON llm_providers(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_proxies_project ON llm_proxies(project_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_proxies_provider_uuid ON llm_proxies(provider_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_proxies_org ON llm_proxies(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_mcp_proxies_project ON mcp_proxies(project_uuid);
CREATE INDEX IF NOT EXISTS idx_mcp_proxies_org ON mcp_proxies(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_api_keys_artifact ON api_keys(artifact_uuid);
CREATE INDEX IF NOT EXISTS idx_applications_project_id ON applications(project_uuid, organization_uuid);
CREATE INDEX IF NOT EXISTS idx_applications_name_project ON applications(name, project_uuid, organization_uuid);
CREATE INDEX IF NOT EXISTS idx_applications_handle_org ON applications(handle, organization_uuid);
CREATE INDEX IF NOT EXISTS idx_application_api_keys_app_id ON application_api_keys(application_uuid);
CREATE INDEX IF NOT EXISTS idx_application_api_keys_key_id ON application_api_keys(api_key_id);
CREATE INDEX IF NOT EXISTS idx_application_artifacts_app_id ON application_artifacts(application_uuid);
CREATE INDEX IF NOT EXISTS idx_application_artifacts_artifact_id ON application_artifacts(artifact_uuid);
CREATE INDEX IF NOT EXISTS idx_api_keys_status ON api_keys(status);
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON api_keys(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_rest_apis_lifecycle_status ON rest_apis(lifecycle_status);
CREATE INDEX IF NOT EXISTS idx_websub_apis_lifecycle_status ON websub_apis(lifecycle_status);
CREATE INDEX IF NOT EXISTS idx_webbroker_apis_lifecycle_status ON webbroker_apis(lifecycle_status);
CREATE INDEX IF NOT EXISTS idx_subscription_plans_status ON subscription_plans(status);

-- EventHub tables for multi-replica HA sync
CREATE TABLE IF NOT EXISTS gateway_states (
    gateway_id TEXT PRIMARY KEY,
    version_id TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (gateway_id) REFERENCES gateways(uuid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS events (
    gateway_id TEXT NOT NULL,
    processed_timestamp TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    originated_timestamp TIMESTAMPTZ NOT NULL,
    entity_type TEXT NOT NULL,
    action TEXT NOT NULL CHECK(action IN ('CREATE', 'UPDATE', 'DELETE')),
    entity_id TEXT NOT NULL,
    event_id TEXT NOT NULL,
    event_data JSONB NOT NULL,
    PRIMARY KEY (gateway_id, event_id),
    FOREIGN KEY (gateway_id) REFERENCES gateway_states(gateway_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_events_gateway_id_processed_timestamp ON events(gateway_id, processed_timestamp);
CREATE INDEX IF NOT EXISTS idx_events_entity ON events(entity_type, entity_id);

CREATE TABLE IF NOT EXISTS audit (
   uuid VARCHAR(40) PRIMARY KEY,
   action VARCHAR(50) NOT NULL,
   resource_uuid VARCHAR(40) NOT NULL,
   resource_type VARCHAR(50),
   organization_uuid VARCHAR(40) NOT NULL,
   performed_by VARCHAR(200),
   performed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
   FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE
);
