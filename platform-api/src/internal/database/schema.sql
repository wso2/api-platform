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
    handle VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    region VARCHAR(63) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Projects table
CREATE TABLE IF NOT EXISTS projects (
    uuid VARCHAR(40) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(name, organization_uuid)
);

-- Artifacts table
CREATE TABLE IF NOT EXISTS artifacts (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL,
    kind VARCHAR(20) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE RESTRICT,
    UNIQUE(handle, organization_uuid),
    UNIQUE(name, version, organization_uuid),
    -- Ensure (uuid, organization_uuid) pairs are unique so they can be safely
    -- referenced from subscriptions to enforce API–organization consistency.
    UNIQUE(uuid, organization_uuid)
);

-- REST APIs table
CREATE TABLE IF NOT EXISTS rest_apis (
    uuid VARCHAR(40) PRIMARY KEY,
    description VARCHAR(1023),
    created_by VARCHAR(200),
    project_uuid VARCHAR(40) NOT NULL,
    lifecycle_status VARCHAR(20) DEFAULT 'CREATED',
    transport VARCHAR(255), -- JSON array as TEXT
    configuration TEXT NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE
);

-- Subscription plans table (organization-scoped rate/billing plans)
CREATE TABLE IF NOT EXISTS subscription_plans (
    uuid VARCHAR(40) PRIMARY KEY,
    plan_name VARCHAR(40) NOT NULL,
    billing_plan VARCHAR(255),
    stop_on_quota_reach BOOLEAN DEFAULT 1,
    throttle_limit_count INTEGER,
    throttle_limit_unit VARCHAR(20),
    expiry_time DATETIME,
    organization_uuid VARCHAR(40) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, plan_name),
    UNIQUE(uuid, organization_uuid),
    CHECK (status IN ('ACTIVE', 'INACTIVE')),
    CONSTRAINT chk_plan_throttle_pair CHECK (
      (throttle_limit_count IS NULL AND throttle_limit_unit IS NULL) OR
      (throttle_limit_count IS NOT NULL AND throttle_limit_unit IS NOT NULL)
    )
);

-- Subscriptions table (application-level subscriptions for REST APIs)
-- application_id references applications in DevPortal/STS (no FK in platform), optional for token-based subscriptions
-- subscription_token: encrypted (AES-256-GCM) for retrieval; subscription_token_hash for uniqueness and gateway sync
CREATE TABLE IF NOT EXISTS subscriptions (
    uuid VARCHAR(40) PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    application_id VARCHAR(255),
    subscription_token VARCHAR(512) NOT NULL,
    subscription_token_hash VARCHAR(64) NOT NULL,
    subscription_plan_uuid VARCHAR(40),
    organization_uuid VARCHAR(40) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (api_uuid) REFERENCES rest_apis(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (subscription_plan_uuid, organization_uuid)
      REFERENCES subscription_plans(uuid, organization_uuid) ON DELETE RESTRICT,
    FOREIGN KEY (api_uuid, organization_uuid)
      REFERENCES artifacts(uuid, organization_uuid) ON DELETE CASCADE,
    UNIQUE(api_uuid, subscription_token_hash),
    UNIQUE(api_uuid, application_id, organization_uuid),
    CHECK (status IN ('ACTIVE', 'INACTIVE', 'REVOKED'))
);

-- Gateways table (scoped to organizations)
-- Must be created before deployments which references it
CREATE TABLE IF NOT EXISTS gateways (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    properties TEXT NOT NULL DEFAULT '{}',
    vhost VARCHAR(255) NOT NULL,
    is_critical BOOLEAN DEFAULT FALSE,
    gateway_functionality_type VARCHAR(20) DEFAULT 'regular' NOT NULL,
    is_active BOOLEAN DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, name),
    CHECK (gateway_functionality_type IN ('regular', 'ai', 'event'))
);

-- Gateway Tokens table
CREATE TABLE IF NOT EXISTS gateway_tokens (
    uuid VARCHAR(40) PRIMARY KEY,
    gateway_uuid VARCHAR(40) NOT NULL,
    token_hash VARCHAR(255) NOT NULL,
    salt VARCHAR(255) NOT NULL,
    status VARCHAR(10) NOT NULL DEFAULT 'active',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    revoked_at DATETIME,
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
    base_deployment_id VARCHAR(40), -- Reference to the deployment used as base, NULL if based on "current"
    content BLOB NOT NULL, -- Immutable deployment artifact (YAML string)
    metadata TEXT, -- JSON object for flexible key-value metadata
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
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
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (artifact_uuid, organization_uuid, gateway_uuid),
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE,
    FOREIGN KEY (deployment_id) REFERENCES deployments(deployment_id) ON DELETE CASCADE,
    CHECK (status IN ('DEPLOYED', 'UNDEPLOYED'))
);

-- Artifact Associations table (for both gateways and dev portals)
CREATE TABLE IF NOT EXISTS association_mappings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    artifact_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    resource_uuid VARCHAR(40) NOT NULL,
    association_type VARCHAR(20) NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(artifact_uuid, resource_uuid, association_type, organization_uuid),
    CHECK (association_type IN ('gateway', 'dev_portal'))
);

-- DevPortals table
CREATE TABLE IF NOT EXISTS devportals (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(100) NOT NULL,
    identifier VARCHAR(100) NOT NULL,
    api_url VARCHAR(255) NOT NULL,
    hostname VARCHAR(255) NOT NULL,
    api_key VARCHAR(255) NOT NULL,
    header_key_name VARCHAR(100) DEFAULT 'x-wso2-api-key',
    is_active BOOLEAN DEFAULT FALSE,
    is_enabled BOOLEAN DEFAULT FALSE,
    is_default BOOLEAN DEFAULT FALSE,
    visibility VARCHAR(20) NOT NULL DEFAULT 'private',
    description VARCHAR(500),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, api_url),
    UNIQUE(organization_uuid, hostname)
);

-- API-DevPortal Publication Tracking Table
-- This table tracks which APIs are published to which DevPortals
CREATE TABLE IF NOT EXISTS publication_mappings (
    api_uuid VARCHAR(40) NOT NULL,
    devportal_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('published', 'failed', 'publishing')),
    api_version VARCHAR(50),
    devportal_ref_id VARCHAR(100),

    -- Gateway endpoints for sandbox and production
    sandbox_endpoint_url VARCHAR(500) NOT NULL,
    production_endpoint_url VARCHAR(500) NOT NULL,

    -- Timestamps
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,

    -- Foreign key constraints
    PRIMARY KEY (api_uuid, devportal_uuid, organization_uuid),
    FOREIGN KEY (api_uuid) REFERENCES rest_apis(uuid) ON DELETE CASCADE,
    FOREIGN KEY (devportal_uuid) REFERENCES devportals(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE (api_uuid, devportal_uuid, organization_uuid)
);

-- LLM Provider Templates table
CREATE TABLE IF NOT EXISTS llm_provider_templates (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    name VARCHAR(253) NOT NULL,
    description VARCHAR(1023),
    created_by VARCHAR(255),
    configuration TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);

-- LLM Providers table
CREATE TABLE IF NOT EXISTS llm_providers (
    uuid VARCHAR(40) PRIMARY KEY,
    description VARCHAR(1023),
    created_by VARCHAR(255),
    template_uuid VARCHAR(40) NOT NULL,
    openapi_spec TEXT,
    model_list TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration TEXT NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (template_uuid) REFERENCES llm_provider_templates(uuid) ON DELETE RESTRICT
);

-- LLM Proxies table
CREATE TABLE IF NOT EXISTS llm_proxies (
    uuid VARCHAR(40) PRIMARY KEY,
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    created_by VARCHAR(255),
    provider_uuid VARCHAR(40) NOT NULL,
    openapi_spec TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration TEXT NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    FOREIGN KEY (provider_uuid) REFERENCES llm_providers(uuid) ON DELETE RESTRICT
);

-- MCP Proxies table
CREATE TABLE IF NOT EXISTS mcp_proxies (
    uuid VARCHAR(40) PRIMARY KEY,
    project_uuid VARCHAR(40),
    description VARCHAR(1023),
    created_by VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration TEXT NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE
);

-- API Keys table (stores API keys for artifacts with hashes as JSON string)
CREATE TABLE IF NOT EXISTS api_keys (
    uuid VARCHAR(40) PRIMARY KEY,
    artifact_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(63) NOT NULL,
    masked_api_key VARCHAR(8) NOT NULL,
    api_key_hashes TEXT NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    created_by VARCHAR(255),
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME,
    provisioned_by TEXT NULL DEFAULT NULL,
    allowed_targets TEXT NOT NULL DEFAULT 'ALL',
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    UNIQUE(artifact_uuid, name)
);

-- Indexes for better performance
CREATE INDEX IF NOT EXISTS idx_projects_organization_id ON projects(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_rest_apis_project_id ON rest_apis(project_uuid);
CREATE INDEX IF NOT EXISTS idx_subscriptions_api_uuid ON subscriptions(api_uuid);
CREATE INDEX IF NOT EXISTS idx_subscriptions_application_id ON subscriptions(application_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_organization_uuid ON subscriptions(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_subscriptions_status ON subscriptions(status);
CREATE INDEX IF NOT EXISTS idx_subscriptions_token ON subscriptions(subscription_token_hash);
CREATE INDEX IF NOT EXISTS idx_gateways_org ON gateways(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_gateway_tokens_status ON gateway_tokens(gateway_uuid, status);
CREATE INDEX IF NOT EXISTS idx_gateway_tokens_hash ON gateway_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_artifact_deployments_artifact_gateway ON deployments(artifact_uuid, gateway_uuid);
CREATE INDEX IF NOT EXISTS idx_artifact_deployments_created_at ON deployments(artifact_uuid, gateway_uuid, created_at);
CREATE INDEX IF NOT EXISTS idx_artifact_gw_created ON deployments(artifact_uuid, organization_uuid, gateway_uuid, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_deployment_status_deployment ON deployment_status(deployment_id);
CREATE INDEX IF NOT EXISTS idx_deployment_status_status ON deployment_status(status);
CREATE INDEX IF NOT EXISTS idx_devportals_org ON devportals(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_devportals_active ON devportals(organization_uuid, is_active);
CREATE INDEX IF NOT EXISTS idx_publication_mappings_api ON publication_mappings(api_uuid);
CREATE INDEX IF NOT EXISTS idx_publication_mappings_devportal ON publication_mappings(devportal_uuid);
CREATE INDEX IF NOT EXISTS idx_publication_mappings_org ON publication_mappings(organization_uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_devportals_default_per_org ON devportals(organization_uuid) WHERE is_default = TRUE;
CREATE INDEX IF NOT EXISTS idx_artifact_associations_artifact_resource_type ON association_mappings(artifact_uuid, association_type, organization_uuid);
CREATE INDEX IF NOT EXISTS idx_association_mappings_resource ON association_mappings(association_type, resource_uuid, organization_uuid);
CREATE INDEX IF NOT EXISTS idx_association_mappings_org ON association_mappings(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_artifacts_org ON artifacts(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_provider_templates_org ON llm_provider_templates(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_providers_template ON llm_providers(template_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_proxies_project ON llm_proxies(project_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_proxies_provider_uuid ON llm_proxies(provider_uuid);
CREATE INDEX IF NOT EXISTS idx_api_keys_artifact ON api_keys(artifact_uuid);
