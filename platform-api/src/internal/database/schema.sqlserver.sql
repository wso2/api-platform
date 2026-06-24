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

-- SQL Server schema derived from schema.postgres.sql
-- Organizations table
IF OBJECT_ID(N'dbo.organizations', N'U') IS NULL
CREATE TABLE dbo.organizations (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    region VARCHAR(63) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME()
);

-- Projects table
IF OBJECT_ID(N'dbo.projects', N'U') IS NULL
CREATE TABLE dbo.projects (
    uuid VARCHAR(40) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(name, organization_uuid)
);

-- Applications table
IF OBJECT_ID(N'dbo.applications', N'U') IS NULL
CREATE TABLE dbo.applications (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(255) NOT NULL,
    project_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(255),
    name VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    type VARCHAR(50) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    -- NO ACTION (not CASCADE) to avoid the SQL Server multiple-cascade-paths
    -- restriction (error 1785). Deleting an organization still removes its
    -- applications via organizations -> projects -> applications, so no
    -- cleanup behavior is lost relative to the Postgres schema.
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION,
    UNIQUE(project_uuid, organization_uuid, name),
    UNIQUE(handle, organization_uuid)
);

-- Artifacts table
IF OBJECT_ID(N'dbo.artifacts', N'U') IS NULL
CREATE TABLE dbo.artifacts (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL,
    kind VARCHAR(20) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION,
    UNIQUE(handle, organization_uuid),
    UNIQUE(name, version, organization_uuid),
    -- Ensure (uuid, organization_uuid) pairs are unique so they can be safely
    -- referenced from subscriptions to enforce API–organization consistency.
    UNIQUE(uuid, organization_uuid)
);

-- REST APIs table
IF OBJECT_ID(N'dbo.rest_apis', N'U') IS NULL
CREATE TABLE dbo.rest_apis (
    uuid VARCHAR(40) PRIMARY KEY,
    description VARCHAR(1023),
    created_by VARCHAR(200),
    project_uuid VARCHAR(40) NOT NULL,
    lifecycle_status VARCHAR(20) DEFAULT 'CREATED',
    transport VARCHAR(255), -- JSON array as NVARCHAR(MAX)
    configuration NVARCHAR(MAX) NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE
);

-- Subscription plans table (organization-scoped rate/billing plans)
IF OBJECT_ID(N'dbo.subscription_plans', N'U') IS NULL
CREATE TABLE dbo.subscription_plans (
    uuid VARCHAR(40) PRIMARY KEY,
    plan_name VARCHAR(40) NOT NULL,
    billing_plan VARCHAR(255),
    stop_on_quota_reach BIT DEFAULT 1,
    throttle_limit_count INT,
    throttle_limit_unit VARCHAR(20),
    expiry_time DATETIME2(7),
    organization_uuid VARCHAR(40) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
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
-- subscription_token: encrypted value (AES-256-GCM) for retrieval (legacy rows have hash)
-- subscription_token_hash: SHA-256 hash for uniqueness and gateway sync
IF OBJECT_ID(N'dbo.subscriptions', N'U') IS NULL
CREATE TABLE dbo.subscriptions (
    uuid VARCHAR(40) PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    subscriber_id VARCHAR(255) NOT NULL,
    application_id VARCHAR(255),
    subscription_token VARCHAR(512) NOT NULL,
    subscription_token_hash VARCHAR(64) NOT NULL,
    subscription_plan_uuid VARCHAR(40),
    organization_uuid VARCHAR(40) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (api_uuid) REFERENCES rest_apis(uuid) ON DELETE CASCADE,
    -- NO ACTION on the organization and artifacts edges to avoid the SQL Server
    -- multiple-cascade-paths restriction (error 1785). Subscriptions are still
    -- removed via the api_uuid -> rest_apis CASCADE edge (which itself cascades
    -- from artifacts/projects/organizations), so cleanup behavior is preserved.
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (subscription_plan_uuid, organization_uuid)
    REFERENCES subscription_plans(uuid, organization_uuid) ON DELETE NO ACTION,
    FOREIGN KEY (api_uuid, organization_uuid)
      REFERENCES artifacts(uuid, organization_uuid) ON DELETE NO ACTION,
    UNIQUE(api_uuid, subscription_token_hash),
    UNIQUE(api_uuid, subscriber_id, organization_uuid),
    CHECK (status IN ('ACTIVE', 'INACTIVE', 'REVOKED'))
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscriptions_token' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscriptions_token ON dbo.subscriptions(subscription_token_hash);
-- Supports list/count filters: WHERE organization_uuid = ? AND subscriber_id = ? (no api_uuid).
-- The unique constraint on (api_uuid, subscriber_id, organization_uuid) is not ordered for this access path.
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscriptions_org_subscriber' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscriptions_org_subscriber ON dbo.subscriptions(organization_uuid, subscriber_id);

-- Gateways table (scoped to organizations)
-- Must be created before deployments which references it
IF OBJECT_ID(N'dbo.gateways', N'U') IS NULL
CREATE TABLE dbo.gateways (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(64) NOT NULL DEFAULT '1.0',
    display_name VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    properties NVARCHAR(MAX) NOT NULL DEFAULT N'{}',
    vhost VARCHAR(255) NOT NULL,
    is_critical BIT DEFAULT 0,
    gateway_functionality_type VARCHAR(20) DEFAULT 'regular' NOT NULL,
    is_active BIT DEFAULT 0,
    manifest NVARCHAR(MAX),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, name),
    CHECK (gateway_functionality_type IN ('regular', 'ai', 'event'))
);

-- Gateway Custom Policies table (org-scoped custom policies synced from gateway manifests)
IF OBJECT_ID(N'dbo.gateway_custom_policies', N'U') IS NULL
CREATE TABLE dbo.gateway_custom_policies (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255),
    version VARCHAR(15) NOT NULL,
    description NVARCHAR(MAX),
    policy_definition NVARCHAR(MAX),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, name, version)
);

-- Gateway Custom Policy Usages table (tracks which APIs use each custom policy)
IF OBJECT_ID(N'dbo.gateway_custom_policy_usages', N'U') IS NULL
CREATE TABLE dbo.gateway_custom_policy_usages (
    policy_uuid VARCHAR(40) NOT NULL,
    api_uuid VARCHAR(40) NOT NULL,
    PRIMARY KEY (policy_uuid, api_uuid),
    FOREIGN KEY (policy_uuid) REFERENCES gateway_custom_policies(uuid) ON DELETE CASCADE,
    FOREIGN KEY (api_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE
);

-- Gateway Tokens table
IF OBJECT_ID(N'dbo.gateway_tokens', N'U') IS NULL
CREATE TABLE dbo.gateway_tokens (
    uuid VARCHAR(40) PRIMARY KEY,
    gateway_uuid VARCHAR(40) NOT NULL,
    token_hash VARCHAR(255) NOT NULL,
    salt VARCHAR(255) NOT NULL,
    status VARCHAR(10) NOT NULL DEFAULT 'active',
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    revoked_at DATETIME2(7),
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE,
    CHECK (status IN ('active', 'revoked')),
    CHECK (revoked_at IS NULL OR status = 'revoked')
);

-- Artifact Deployments table (immutable deployment artifacts)
IF OBJECT_ID(N'dbo.deployments', N'U') IS NULL
CREATE TABLE dbo.deployments (
    deployment_id VARCHAR(40) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    artifact_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    gateway_uuid VARCHAR(40) NOT NULL,
    base_deployment_id VARCHAR(40),
    content VARBINARY(MAX) NOT NULL,
    metadata NVARCHAR(MAX), -- JSON object as NVARCHAR(MAX)
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    -- NO ACTION to avoid the SQL Server multiple-cascade-paths restriction
    -- (error 1785). Organization deletes still reach deployments through
    -- organizations -> gateways -> deployments.
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE,
    -- NO ACTION (not SET NULL): SQL Server forbids cascade actions on a
    -- self-referencing FK (error 1785, "may cause cycles"). Deployments for an
    -- artifact/gateway are deleted together in a single statement (or via the
    -- artifact/gateway CASCADE), so the referenced base row is removed in the
    -- same operation and no dangling reference remains.
    FOREIGN KEY (base_deployment_id) REFERENCES deployments(deployment_id) ON DELETE NO ACTION
);

-- Artifact Deployment Status table (current deployment state per artifact+Gateway)
IF OBJECT_ID(N'dbo.deployment_status', N'U') IS NULL
CREATE TABLE dbo.deployment_status (
    artifact_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    gateway_uuid VARCHAR(40) NOT NULL,
    deployment_id VARCHAR(40) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'DEPLOYED',
    status_desired VARCHAR(20),
    performed_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    status_reason VARCHAR(50),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (artifact_uuid, organization_uuid, gateway_uuid),
    -- Only the deployment_id edge cascades. The artifact/organization/gateway
    -- edges are NO ACTION to avoid the SQL Server multiple-cascade-paths
    -- restriction (error 1785). A status row is always removed when its
    -- referenced deployment is deleted, and deletes of an artifact, gateway or
    -- organization funnel through deployments
    -- (artifact/gateway -> deployments -> deployment_status), so no cleanup is lost.
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (deployment_id) REFERENCES deployments(deployment_id) ON DELETE CASCADE
);

-- Artifact Associations table (for both gateways and dev portals)
IF OBJECT_ID(N'dbo.association_mappings', N'U') IS NULL
CREATE TABLE dbo.association_mappings (
    id INT IDENTITY(1,1) PRIMARY KEY,
    artifact_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    resource_uuid VARCHAR(40) NOT NULL,
    association_type VARCHAR(20) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(artifact_uuid, resource_uuid, association_type, organization_uuid),
    CHECK (association_type IN ('gateway', 'dev_portal'))
);

-- DevPortals table
IF OBJECT_ID(N'dbo.devportals', N'U') IS NULL
CREATE TABLE dbo.devportals (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(100) NOT NULL,
    identifier VARCHAR(100) NOT NULL,
    api_url VARCHAR(255) NOT NULL,
    hostname VARCHAR(255) NOT NULL,
    api_key VARCHAR(255) NOT NULL,
    header_key_name VARCHAR(100) DEFAULT 'x-wso2-api-key',
    is_active BIT DEFAULT 0,
    is_enabled BIT DEFAULT 0,
    is_default BIT DEFAULT 0,
    visibility VARCHAR(20) NOT NULL DEFAULT 'private',
    description VARCHAR(500),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, api_url),
    UNIQUE(organization_uuid, hostname)
);

-- API-DevPortal Publication Tracking Table
-- This table tracks which APIs are published to which DevPortals

IF OBJECT_ID(N'dbo.publication_mappings', N'U') IS NULL
CREATE TABLE dbo.publication_mappings (
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
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),

    -- Foreign key constraints
    PRIMARY KEY (api_uuid, devportal_uuid, organization_uuid),
    -- Only the devportal edge cascades. The api and organization edges are
    -- NO ACTION to avoid the SQL Server multiple-cascade-paths restriction
    -- (error 1785). API deletion removes publication rows explicitly in
    -- application code (APIRepo.DeleteAPI), and organization deletes reach them
    -- through organizations -> devportals -> publication_mappings.
    FOREIGN KEY (api_uuid) REFERENCES rest_apis(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (devportal_uuid) REFERENCES devportals(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION,
    UNIQUE (api_uuid, devportal_uuid, organization_uuid)
);

-- LLM Provider Templates table
IF OBJECT_ID(N'dbo.llm_provider_templates', N'U') IS NULL
CREATE TABLE dbo.llm_provider_templates (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    group_version_id VARCHAR(255) NOT NULL,
    name VARCHAR(253) NOT NULL,
    managed_by VARCHAR(255) NOT NULL DEFAULT 'customer',
    description VARCHAR(1023),
    created_by VARCHAR(255),
    configuration NVARCHAR(MAX) NOT NULL,
    openapi_spec NVARCHAR(MAX),
    version VARCHAR(40) NOT NULL DEFAULT 'v1.0',
    is_latest BIT NOT NULL DEFAULT 1,
    enabled BIT NOT NULL DEFAULT 1,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, group_version_id, version)
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_llm_provider_templates_single_latest' AND object_id = OBJECT_ID(N'dbo.llm_provider_templates'))
CREATE UNIQUE INDEX idx_llm_provider_templates_single_latest ON dbo.llm_provider_templates(organization_uuid, group_version_id) WHERE is_latest = 1;

-- LLM Providers table
IF OBJECT_ID(N'dbo.llm_providers', N'U') IS NULL
CREATE TABLE dbo.llm_providers (
    uuid VARCHAR(40) PRIMARY KEY,
    description VARCHAR(1023),
    created_by VARCHAR(255),
    template_uuid VARCHAR(40) NOT NULL,
    openapi_spec NVARCHAR(MAX),
    model_list NVARCHAR(MAX),
    status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration NVARCHAR(MAX) NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (template_uuid) REFERENCES llm_provider_templates(uuid) ON DELETE NO ACTION
);

-- LLM Proxies table
IF OBJECT_ID(N'dbo.llm_proxies', N'U') IS NULL
CREATE TABLE dbo.llm_proxies (
    uuid VARCHAR(40) PRIMARY KEY,
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    created_by VARCHAR(255),
    provider_uuid VARCHAR(40) NOT NULL,
    openapi_spec NVARCHAR(MAX),
    status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration NVARCHAR(MAX) NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    FOREIGN KEY (provider_uuid) REFERENCES llm_providers(uuid) ON DELETE NO ACTION
);

-- MCP Proxies table
IF OBJECT_ID(N'dbo.mcp_proxies', N'U') IS NULL
CREATE TABLE dbo.mcp_proxies (
    uuid VARCHAR(40) PRIMARY KEY,
    project_uuid VARCHAR(40),
    description VARCHAR(1023),
    created_by VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration NVARCHAR(MAX) NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE
);

-- WEBSUB APIs table
IF OBJECT_ID(N'dbo.websub_apis', N'U') IS NULL
CREATE TABLE dbo.websub_apis (
    uuid VARCHAR(40) PRIMARY KEY,
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    created_by VARCHAR(255),
    lifecycle_status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    transport VARCHAR(255),
    configuration NVARCHAR(MAX) NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_websub_apis_project' AND object_id = OBJECT_ID(N'dbo.websub_apis'))
CREATE INDEX idx_websub_apis_project ON dbo.websub_apis(project_uuid);

-- WebSub API HMAC secrets table (for inbound webhook event verification)
IF OBJECT_ID(N'dbo.websub_api_hmac_secrets', N'U') IS NULL
CREATE TABLE dbo.websub_api_hmac_secrets (
    uuid VARCHAR(40) PRIMARY KEY,
    artifact_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(63) NOT NULL,
    display_name VARCHAR(255),
    encrypted_secret NVARCHAR(MAX) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    CONSTRAINT uq_websub_api_hmac_secrets_artifact_name UNIQUE (artifact_uuid, name)
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_websub_api_hmac_secrets_artifact' AND object_id = OBJECT_ID(N'dbo.websub_api_hmac_secrets'))
CREATE INDEX idx_websub_api_hmac_secrets_artifact ON dbo.websub_api_hmac_secrets(artifact_uuid);

-- WEBBROKER APIs table
IF OBJECT_ID(N'dbo.webbroker_apis', N'U') IS NULL
CREATE TABLE dbo.webbroker_apis (
    uuid VARCHAR(40) PRIMARY KEY,
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    created_by VARCHAR(255),
    lifecycle_status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    transport VARCHAR(255),
    configuration NVARCHAR(MAX) NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_webbroker_apis_project' AND object_id = OBJECT_ID(N'dbo.webbroker_apis'))
CREATE INDEX idx_webbroker_apis_project ON dbo.webbroker_apis(project_uuid);

-- API Keys table (stores API keys for artifacts with hashes as JSON string)
IF OBJECT_ID(N'dbo.api_keys', N'U') IS NULL
CREATE TABLE dbo.api_keys (
    uuid VARCHAR(40) PRIMARY KEY,
    artifact_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(63) NOT NULL,
    masked_api_key VARCHAR(8) NOT NULL,
    api_key_hashes NVARCHAR(MAX) NOT NULL DEFAULT '{}',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    created_by VARCHAR(255),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    expires_at DATETIME2(7),
    issuer NVARCHAR(MAX) NULL DEFAULT NULL,
    allowed_targets NVARCHAR(MAX) NOT NULL DEFAULT 'ALL',
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    UNIQUE(artifact_uuid, name)
);

-- Application API Key mappings table
IF OBJECT_ID(N'dbo.application_api_keys', N'U') IS NULL
CREATE TABLE dbo.application_api_keys (
    application_uuid VARCHAR(40) NOT NULL,
    api_key_id VARCHAR(40) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (application_uuid, api_key_id),
    FOREIGN KEY (application_uuid) REFERENCES applications(uuid) ON DELETE CASCADE,
    FOREIGN KEY (api_key_id) REFERENCES api_keys(uuid) ON DELETE CASCADE
);

-- Application to artifacts mapping table
IF OBJECT_ID(N'dbo.application_artifacts', N'U') IS NULL
CREATE TABLE dbo.application_artifacts (
    application_uuid VARCHAR(40) NOT NULL,
    artifact_uuid VARCHAR(40) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (application_uuid, artifact_uuid),
    FOREIGN KEY (application_uuid) REFERENCES applications(uuid) ON DELETE CASCADE,
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE
);

-- Indexes for better performance
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_projects_organization_id' AND object_id = OBJECT_ID(N'dbo.projects'))
CREATE INDEX idx_projects_organization_id ON dbo.projects(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_rest_apis_project_id' AND object_id = OBJECT_ID(N'dbo.rest_apis'))
CREATE INDEX idx_rest_apis_project_id ON dbo.rest_apis(project_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscriptions_api_uuid' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscriptions_api_uuid ON dbo.subscriptions(api_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscriptions_application_id' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscriptions_application_id ON dbo.subscriptions(application_id);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscriptions_organization_uuid' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscriptions_organization_uuid ON dbo.subscriptions(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscriptions_status' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscriptions_status ON dbo.subscriptions(status);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_gateways_org' AND object_id = OBJECT_ID(N'dbo.gateways'))
CREATE INDEX idx_gateways_org ON dbo.gateways(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_gateway_tokens_status' AND object_id = OBJECT_ID(N'dbo.gateway_tokens'))
CREATE INDEX idx_gateway_tokens_status ON dbo.gateway_tokens(gateway_uuid, status);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_artifact_deployments_artifact_gateway' AND object_id = OBJECT_ID(N'dbo.deployments'))
CREATE INDEX idx_artifact_deployments_artifact_gateway ON dbo.deployments(artifact_uuid, gateway_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_artifact_deployments_created_at' AND object_id = OBJECT_ID(N'dbo.deployments'))
CREATE INDEX idx_artifact_deployments_created_at ON dbo.deployments(artifact_uuid, gateway_uuid, created_at);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_artifact_gw_created' AND object_id = OBJECT_ID(N'dbo.deployments'))
CREATE INDEX idx_artifact_gw_created ON dbo.deployments(artifact_uuid, organization_uuid, gateway_uuid, created_at DESC);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_deployment_status_deployment' AND object_id = OBJECT_ID(N'dbo.deployment_status'))
CREATE INDEX idx_deployment_status_deployment ON dbo.deployment_status(deployment_id);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_deployment_status_status' AND object_id = OBJECT_ID(N'dbo.deployment_status'))
CREATE INDEX idx_deployment_status_status ON dbo.deployment_status(status);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_devportals_org' AND object_id = OBJECT_ID(N'dbo.devportals'))
CREATE INDEX idx_devportals_org ON dbo.devportals(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_devportals_active' AND object_id = OBJECT_ID(N'dbo.devportals'))
CREATE INDEX idx_devportals_active ON dbo.devportals(organization_uuid, is_active);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_publication_mappings_api' AND object_id = OBJECT_ID(N'dbo.publication_mappings'))
CREATE INDEX idx_publication_mappings_api ON dbo.publication_mappings(api_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_publication_mappings_devportal' AND object_id = OBJECT_ID(N'dbo.publication_mappings'))
CREATE INDEX idx_publication_mappings_devportal ON dbo.publication_mappings(devportal_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_publication_mappings_org' AND object_id = OBJECT_ID(N'dbo.publication_mappings'))
CREATE INDEX idx_publication_mappings_org ON dbo.publication_mappings(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_devportals_default_per_org' AND object_id = OBJECT_ID(N'dbo.devportals'))
CREATE UNIQUE INDEX idx_devportals_default_per_org ON dbo.devportals(organization_uuid) WHERE is_default = 1;
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_artifact_associations_artifact_resource_type' AND object_id = OBJECT_ID(N'dbo.association_mappings'))
CREATE INDEX idx_artifact_associations_artifact_resource_type ON dbo.association_mappings(artifact_uuid, association_type, organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_association_mappings_resource' AND object_id = OBJECT_ID(N'dbo.association_mappings'))
CREATE INDEX idx_association_mappings_resource ON dbo.association_mappings(association_type, resource_uuid, organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_association_mappings_org' AND object_id = OBJECT_ID(N'dbo.association_mappings'))
CREATE INDEX idx_association_mappings_org ON dbo.association_mappings(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_artifacts_org' AND object_id = OBJECT_ID(N'dbo.artifacts'))
CREATE INDEX idx_artifacts_org ON dbo.artifacts(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_llm_provider_templates_org' AND object_id = OBJECT_ID(N'dbo.llm_provider_templates'))
CREATE INDEX idx_llm_provider_templates_org ON dbo.llm_provider_templates(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_llm_providers_template' AND object_id = OBJECT_ID(N'dbo.llm_providers'))
CREATE INDEX idx_llm_providers_template ON dbo.llm_providers(template_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_llm_proxies_project' AND object_id = OBJECT_ID(N'dbo.llm_proxies'))
CREATE INDEX idx_llm_proxies_project ON dbo.llm_proxies(project_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_llm_proxies_provider_uuid' AND object_id = OBJECT_ID(N'dbo.llm_proxies'))
CREATE INDEX idx_llm_proxies_provider_uuid ON dbo.llm_proxies(provider_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_keys_artifact' AND object_id = OBJECT_ID(N'dbo.api_keys'))
CREATE INDEX idx_api_keys_artifact ON dbo.api_keys(artifact_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_applications_project_id' AND object_id = OBJECT_ID(N'dbo.applications'))
CREATE INDEX idx_applications_project_id ON dbo.applications(project_uuid, organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_applications_name_project' AND object_id = OBJECT_ID(N'dbo.applications'))
CREATE INDEX idx_applications_name_project ON dbo.applications(name, project_uuid, organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_applications_handle_org' AND object_id = OBJECT_ID(N'dbo.applications'))
CREATE INDEX idx_applications_handle_org ON dbo.applications(handle, organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_application_api_keys_app_id' AND object_id = OBJECT_ID(N'dbo.application_api_keys'))
CREATE INDEX idx_application_api_keys_app_id ON dbo.application_api_keys(application_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_application_api_keys_key_id' AND object_id = OBJECT_ID(N'dbo.application_api_keys'))
CREATE INDEX idx_application_api_keys_key_id ON dbo.application_api_keys(api_key_id);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_application_artifacts_app_id' AND object_id = OBJECT_ID(N'dbo.application_artifacts'))
CREATE INDEX idx_application_artifacts_app_id ON dbo.application_artifacts(application_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_application_artifacts_artifact_id' AND object_id = OBJECT_ID(N'dbo.application_artifacts'))
CREATE INDEX idx_application_artifacts_artifact_id ON dbo.application_artifacts(artifact_uuid);

-- EventHub tables for multi-replica HA sync and gateway event propagation.
-- Counterpart of the gateway_states / events tables in schema.postgres.sql.
-- Keyed columns are bounded NVARCHAR to stay within SQL Server index-key limits.
IF OBJECT_ID(N'dbo.gateway_states', N'U') IS NULL
CREATE TABLE dbo.gateway_states (
    gateway_id NVARCHAR(64) PRIMARY KEY,
    version_id NVARCHAR(255) NOT NULL DEFAULT '',
    updated_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME()
);

IF OBJECT_ID(N'dbo.events', N'U') IS NULL
CREATE TABLE dbo.events (
    gateway_id NVARCHAR(64) NOT NULL,
    processed_timestamp DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    originated_timestamp DATETIME2(7) NOT NULL,
    entity_type NVARCHAR(255) NOT NULL,
    action NVARCHAR(20) NOT NULL CHECK(action IN ('CREATE', 'UPDATE', 'DELETE')),
    entity_id NVARCHAR(255) NOT NULL,
    event_id NVARCHAR(64) NOT NULL,
    event_data NVARCHAR(MAX) NOT NULL,
    PRIMARY KEY (gateway_id, event_id),
    FOREIGN KEY (gateway_id) REFERENCES dbo.gateway_states(gateway_id) ON DELETE CASCADE
);

IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_events_gateway_id_processed_timestamp' AND object_id = OBJECT_ID(N'dbo.events'))
CREATE INDEX idx_events_gateway_id_processed_timestamp ON dbo.events(gateway_id, processed_timestamp);
