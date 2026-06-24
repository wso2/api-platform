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
IF OBJECT_ID(N'dbo.organizations', N'U') IS NULL
CREATE TABLE dbo.organizations (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(40) UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    region VARCHAR(63) NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME()
);

-- Projects table
IF OBJECT_ID(N'dbo.projects', N'U') IS NULL
CREATE TABLE dbo.projects (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);

-- Applications table
IF OBJECT_ID(N'dbo.applications', N'U') IS NULL
CREATE TABLE dbo.applications (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(40) NOT NULL,
    project_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    type VARCHAR(50) NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    -- NO ACTION (not CASCADE) to avoid the SQL Server multiple-cascade-paths
    -- restriction (error 1785). Deleting an organization still removes its
    -- applications via organizations -> projects -> applications, so no
    -- cleanup behavior is lost relative to the Postgres schema.
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION,
    UNIQUE(organization_uuid, handle)
);

-- Artifacts table
IF OBJECT_ID(N'dbo.artifacts', N'U') IS NULL
CREATE TABLE dbo.artifacts (
    uuid VARCHAR(40) PRIMARY KEY,
    type VARCHAR(20) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION,
    -- Ensure (uuid, organization_uuid) pairs are unique so they can be safely
    -- referenced from subscriptions to enforce API-organization consistency.
    UNIQUE(uuid, organization_uuid)
);

-- REST APIs table
IF OBJECT_ID(N'dbo.rest_apis', N'U') IS NULL
CREATE TABLE dbo.rest_apis (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    lifecycle_status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration VARBINARY(MAX) NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    -- NO ACTION to avoid SQL Server multiple-cascade-paths restriction (error 1785).
    -- Rows are removed via the artifact CASCADE edge.
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);

-- Subscription plans table (organization-scoped rate/billing plans)
IF OBJECT_ID(N'dbo.subscription_plans', N'U') IS NULL
CREATE TABLE dbo.subscription_plans (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    billing_plan VARCHAR(255),
    stop_on_quota_reach SMALLINT DEFAULT 1,
    throttle_limit_count INT,
    throttle_limit_unit VARCHAR(20),
    expiry_time DATETIME2(7),
    organization_uuid VARCHAR(40) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle),
    UNIQUE(uuid, organization_uuid)
);

-- Subscriptions table (application-level subscriptions for any artifact type)
-- subscription_token: encrypted value (AES-256-GCM) for retrieval (legacy rows have hash)
-- subscription_token_hash: SHA-256 hash for uniqueness and gateway sync
IF OBJECT_ID(N'dbo.subscriptions', N'U') IS NULL
CREATE TABLE dbo.subscriptions (
    uuid VARCHAR(40) PRIMARY KEY,
    artifact_uuid VARCHAR(40) NOT NULL,
    subscriber_id VARCHAR(255) NOT NULL,
    application_id VARCHAR(255),
    subscription_token VARCHAR(512) NOT NULL,
    subscription_token_hash VARCHAR(64) NOT NULL,
    subscription_plan_uuid VARCHAR(40),
    organization_uuid VARCHAR(40) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    -- NO ACTION on the organization and artifact+org edges to avoid the SQL Server
    -- multiple-cascade-paths restriction (error 1785). Subscriptions are still
    -- removed via the artifact_uuid -> artifacts CASCADE edge (which itself cascades
    -- from projects/organizations), so cleanup behavior is preserved.
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (subscription_plan_uuid, organization_uuid)
        REFERENCES subscription_plans(uuid, organization_uuid) ON DELETE NO ACTION,
    FOREIGN KEY (artifact_uuid, organization_uuid)
        REFERENCES artifacts(uuid, organization_uuid) ON DELETE NO ACTION,
    UNIQUE(artifact_uuid, subscription_token_hash)
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscriptions_token' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscriptions_token ON dbo.subscriptions(subscription_token_hash);
-- Supports list/count filters: WHERE organization_uuid = ? AND subscriber_id = ? (no artifact_uuid).
-- The unique constraint on (organization_uuid, artifact_uuid, application_id) is not ordered for this access path.
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscriptions_org_subscriber' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscriptions_org_subscriber ON dbo.subscriptions(organization_uuid, subscriber_id);
-- Enforce one subscription per application per artifact per org. Filtered to exclude NULL application_id
-- (token-based subscriptions) so all backends behave identically — SQL Server treats NULLs as equal
-- in a plain UNIQUE constraint, which would block multiple token-based subscriptions on the same artifact.
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'uq_subscriptions_org_artifact_app' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE UNIQUE INDEX uq_subscriptions_org_artifact_app ON dbo.subscriptions(organization_uuid, artifact_uuid, application_id) WHERE application_id IS NOT NULL;

-- Gateways table (scoped to organizations)
-- Must be created before deployments which references it
IF OBJECT_ID(N'dbo.gateways', N'U') IS NULL
CREATE TABLE dbo.gateways (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    vhost VARCHAR(255) NOT NULL,
    gateway_functionality_type VARCHAR(20) DEFAULT 'regular' NOT NULL,
    properties VARBINARY(MAX) NOT NULL,
    manifest VARBINARY(MAX),
    is_active SMALLINT DEFAULT 0,
    is_critical SMALLINT DEFAULT 0,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);

-- Gateway Association Mappings table (links artifacts to gateways)
IF OBJECT_ID(N'dbo.gateway_association_mappings', N'U') IS NULL
CREATE TABLE dbo.gateway_association_mappings (
    artifact_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    gateway_uuid VARCHAR(40) NOT NULL,
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (organization_uuid, artifact_uuid, gateway_uuid),
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    -- NO ACTION to avoid SQL Server multiple-cascade-paths restriction (error 1785).
    -- Rows are cleaned up via the artifact CASCADE edge.
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE NO ACTION
);

-- Gateway Custom Policies table (org-scoped custom policies synced from gateway manifests)
IF OBJECT_ID(N'dbo.gateway_custom_policies', N'U') IS NULL
CREATE TABLE dbo.gateway_custom_policies (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255),
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    description VARCHAR(1023),
    policy_definition VARBINARY(MAX),
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, name, version)
);

-- Gateway Custom Policy Usages table (tracks which APIs use each custom policy)
IF OBJECT_ID(N'dbo.gateway_custom_policy_usages', N'U') IS NULL
CREATE TABLE dbo.gateway_custom_policy_usages (
    policy_uuid VARCHAR(40) NOT NULL,
    artifact_uuid VARCHAR(40) NOT NULL,
    PRIMARY KEY (policy_uuid, artifact_uuid),
    FOREIGN KEY (policy_uuid) REFERENCES gateway_custom_policies(uuid) ON DELETE CASCADE,
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE
);

-- Gateway Tokens table
IF OBJECT_ID(N'dbo.gateway_tokens', N'U') IS NULL
CREATE TABLE dbo.gateway_tokens (
    uuid VARCHAR(40) PRIMARY KEY,
    gateway_uuid VARCHAR(40) NOT NULL,
    token_hash VARCHAR(255) NOT NULL,
    salt VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    revoked_by VARCHAR(200),
    revoked_at DATETIME2(7),
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE
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
    metadata VARBINARY(MAX),
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
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
    performed_by VARCHAR(200),
    status_reason VARCHAR(50),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (organization_uuid, artifact_uuid, gateway_uuid),
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

-- Artifact Associations table (for gateways)
IF OBJECT_ID(N'dbo.gateway_associations', N'U') IS NULL
CREATE TABLE dbo.gateway_associations (
    id INT IDENTITY(1,1) PRIMARY KEY,
    artifact_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    gateway_uuid VARCHAR(40) NOT NULL,
    metadata NVARCHAR(MAX),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE,
    UNIQUE(artifact_uuid, gateway_uuid, organization_uuid)
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
    handle VARCHAR(40) NOT NULL,
    group_id VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    managed_by VARCHAR(255) NOT NULL DEFAULT 'customer',
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    description VARCHAR(1023),
    configuration VARBINARY(MAX) NOT NULL,
    openapi_spec VARBINARY(MAX),
    is_latest SMALLINT NOT NULL DEFAULT 1,
    enabled SMALLINT NOT NULL DEFAULT 1,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, group_id, version),
    UNIQUE(organization_uuid, handle)
);

-- LLM Providers table
IF OBJECT_ID(N'dbo.llm_providers', N'U') IS NULL
CREATE TABLE dbo.llm_providers (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    description VARCHAR(1023),
    template_uuid VARCHAR(40) NOT NULL,
    openapi_spec VARBINARY(MAX),
    model_list VARBINARY(MAX),
    configuration VARBINARY(MAX) NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    organization_uuid VARCHAR(40) NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    -- NO ACTION to avoid SQL Server multiple-cascade-paths restriction (error 1785).
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (template_uuid) REFERENCES llm_provider_templates(uuid) ON DELETE NO ACTION,
    UNIQUE(organization_uuid, handle)
);

-- LLM Proxies table
IF OBJECT_ID(N'dbo.llm_proxies', N'U') IS NULL
CREATE TABLE dbo.llm_proxies (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    provider_uuid VARCHAR(40) NOT NULL,
    openapi_spec VARBINARY(MAX),
    configuration VARBINARY(MAX) NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    organization_uuid VARCHAR(40) NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    -- NO ACTION to avoid SQL Server multiple-cascade-paths restriction (error 1785).
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    FOREIGN KEY (provider_uuid) REFERENCES llm_providers(uuid) ON DELETE NO ACTION,
    UNIQUE(organization_uuid, handle)
);

-- MCP Proxies table
IF OBJECT_ID(N'dbo.mcp_proxies', N'U') IS NULL
CREATE TABLE dbo.mcp_proxies (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    project_uuid VARCHAR(40),
    description VARCHAR(1023),
    configuration VARBINARY(MAX) NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    organization_uuid VARCHAR(40) NOT NULL,
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    -- NO ACTION to avoid SQL Server multiple-cascade-paths restriction (error 1785).
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);

-- WEBSUB APIs table
IF OBJECT_ID(N'dbo.websub_apis', N'U') IS NULL
CREATE TABLE dbo.websub_apis (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    lifecycle_status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration VARBINARY(MAX) NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    -- NO ACTION to avoid SQL Server multiple-cascade-paths restriction (error 1785).
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_websub_apis_project' AND object_id = OBJECT_ID(N'dbo.websub_apis'))
CREATE INDEX idx_websub_apis_project ON dbo.websub_apis(project_uuid);

-- WebSub API HMAC secrets table (for inbound webhook event verification)
IF OBJECT_ID(N'dbo.websub_api_hmac_secrets', N'U') IS NULL
CREATE TABLE dbo.websub_api_hmac_secrets (
    uuid VARCHAR(40) PRIMARY KEY,
    artifact_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255),
    encrypted_secret VARBINARY(MAX) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    CONSTRAINT uq_websub_api_hmac_secrets_artifact_handle UNIQUE (artifact_uuid, handle)
);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_websub_api_hmac_secrets_artifact' AND object_id = OBJECT_ID(N'dbo.websub_api_hmac_secrets'))
CREATE INDEX idx_websub_api_hmac_secrets_artifact ON dbo.websub_api_hmac_secrets(artifact_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_websub_api_hmac_secrets_status' AND object_id = OBJECT_ID(N'dbo.websub_api_hmac_secrets'))
CREATE INDEX idx_websub_api_hmac_secrets_status ON dbo.websub_api_hmac_secrets(status);

-- WEBBROKER APIs table
IF OBJECT_ID(N'dbo.webbroker_apis', N'U') IS NULL
CREATE TABLE dbo.webbroker_apis (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL DEFAULT 'v1.0',
    project_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    lifecycle_status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    configuration VARBINARY(MAX) NOT NULL,
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    -- NO ACTION to avoid SQL Server multiple-cascade-paths restriction (error 1785).
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE NO ACTION,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
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
    api_key_hashes VARBINARY(MAX) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    data_version VARCHAR(20) NOT NULL DEFAULT '1.0',
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    updated_by VARCHAR(200),
    updated_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    expires_at DATETIME2(7),
    issuer VARCHAR(255) NULL DEFAULT NULL,
    allowed_targets VARCHAR(255) NOT NULL DEFAULT 'ALL',
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE,
    UNIQUE(artifact_uuid, name)
);

-- Application API Key mappings table
IF OBJECT_ID(N'dbo.application_api_keys', N'U') IS NULL
CREATE TABLE dbo.application_api_keys (
    application_uuid VARCHAR(40) NOT NULL,
    api_key_id VARCHAR(40) NOT NULL,
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (application_uuid, api_key_id),
    FOREIGN KEY (application_uuid) REFERENCES applications(uuid) ON DELETE CASCADE,
    FOREIGN KEY (api_key_id) REFERENCES api_keys(uuid) ON DELETE CASCADE
);

-- Application to artifacts mapping table
IF OBJECT_ID(N'dbo.application_artifacts', N'U') IS NULL
CREATE TABLE dbo.application_artifacts (
    application_uuid VARCHAR(40) NOT NULL,
    artifact_uuid VARCHAR(40) NOT NULL,
    created_by VARCHAR(200),
    created_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    PRIMARY KEY (application_uuid, artifact_uuid),
    FOREIGN KEY (application_uuid) REFERENCES applications(uuid) ON DELETE CASCADE,
    FOREIGN KEY (artifact_uuid) REFERENCES artifacts(uuid) ON DELETE CASCADE
);

-- Indexes for better performance
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_projects_organization_id' AND object_id = OBJECT_ID(N'dbo.projects'))
CREATE INDEX idx_projects_organization_id ON dbo.projects(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_rest_apis_project_id' AND object_id = OBJECT_ID(N'dbo.rest_apis'))
CREATE INDEX idx_rest_apis_project_id ON dbo.rest_apis(project_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscriptions_artifact_uuid' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscriptions_artifact_uuid ON dbo.subscriptions(artifact_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscriptions_application_id' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscriptions_application_id ON dbo.subscriptions(application_id);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscriptions_organization_uuid' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscriptions_organization_uuid ON dbo.subscriptions(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscriptions_status' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscriptions_status ON dbo.subscriptions(status);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscriptions_plan' AND object_id = OBJECT_ID(N'dbo.subscriptions'))
CREATE INDEX idx_subscriptions_plan ON dbo.subscriptions(subscription_plan_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_gateways_org' AND object_id = OBJECT_ID(N'dbo.gateways'))
CREATE INDEX idx_gateways_org ON dbo.gateways(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_gateway_tokens_status' AND object_id = OBJECT_ID(N'dbo.gateway_tokens'))
CREATE INDEX idx_gateway_tokens_status ON dbo.gateway_tokens(gateway_uuid, status);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_gateway_tokens_hash' AND object_id = OBJECT_ID(N'dbo.gateway_tokens'))
CREATE INDEX idx_gateway_tokens_hash ON dbo.gateway_tokens(token_hash);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_artifact_deployments_created_at' AND object_id = OBJECT_ID(N'dbo.deployments'))
CREATE INDEX idx_artifact_deployments_created_at ON dbo.deployments(artifact_uuid, gateway_uuid, created_at);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_artifact_gw_created' AND object_id = OBJECT_ID(N'dbo.deployments'))
CREATE INDEX idx_artifact_gw_created ON dbo.deployments(organization_uuid, artifact_uuid, gateway_uuid, created_at DESC);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_deployments_base_id' AND object_id = OBJECT_ID(N'dbo.deployments'))
CREATE INDEX idx_deployments_base_id ON dbo.deployments(base_deployment_id) WHERE base_deployment_id IS NOT NULL;
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_deployment_status_deployment' AND object_id = OBJECT_ID(N'dbo.deployment_status'))
CREATE INDEX idx_deployment_status_deployment ON dbo.deployment_status(deployment_id);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_deployment_status_status' AND object_id = OBJECT_ID(N'dbo.deployment_status'))
CREATE INDEX idx_deployment_status_status ON dbo.deployment_status(status);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_artifacts_org' AND object_id = OBJECT_ID(N'dbo.artifacts'))
CREATE INDEX idx_artifacts_org ON dbo.artifacts(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_artifacts_org_uuid' AND object_id = OBJECT_ID(N'dbo.artifacts'))
CREATE INDEX idx_artifacts_org_uuid ON dbo.artifacts(organization_uuid, uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_gateway_custom_policies_org' AND object_id = OBJECT_ID(N'dbo.gateway_custom_policies'))
CREATE INDEX idx_gateway_custom_policies_org ON dbo.gateway_custom_policies(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_gateway_custom_policy_usages_artifact' AND object_id = OBJECT_ID(N'dbo.gateway_custom_policy_usages'))
CREATE INDEX idx_gateway_custom_policy_usages_artifact ON dbo.gateway_custom_policy_usages(artifact_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_llm_provider_templates_org' AND object_id = OBJECT_ID(N'dbo.llm_provider_templates'))
CREATE INDEX idx_llm_provider_templates_org ON dbo.llm_provider_templates(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_llm_provider_templates_group' AND object_id = OBJECT_ID(N'dbo.llm_provider_templates'))
CREATE INDEX idx_llm_provider_templates_group ON dbo.llm_provider_templates(organization_uuid, group_id);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_llm_providers_template' AND object_id = OBJECT_ID(N'dbo.llm_providers'))
CREATE INDEX idx_llm_providers_template ON dbo.llm_providers(template_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_llm_providers_org' AND object_id = OBJECT_ID(N'dbo.llm_providers'))
CREATE INDEX idx_llm_providers_org ON dbo.llm_providers(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_llm_proxies_project' AND object_id = OBJECT_ID(N'dbo.llm_proxies'))
CREATE INDEX idx_llm_proxies_project ON dbo.llm_proxies(project_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_llm_proxies_provider_uuid' AND object_id = OBJECT_ID(N'dbo.llm_proxies'))
CREATE INDEX idx_llm_proxies_provider_uuid ON dbo.llm_proxies(provider_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_llm_proxies_org' AND object_id = OBJECT_ID(N'dbo.llm_proxies'))
CREATE INDEX idx_llm_proxies_org ON dbo.llm_proxies(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_mcp_proxies_project' AND object_id = OBJECT_ID(N'dbo.mcp_proxies'))
CREATE INDEX idx_mcp_proxies_project ON dbo.mcp_proxies(project_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_mcp_proxies_org' AND object_id = OBJECT_ID(N'dbo.mcp_proxies'))
CREATE INDEX idx_mcp_proxies_org ON dbo.mcp_proxies(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_keys_artifact' AND object_id = OBJECT_ID(N'dbo.api_keys'))
CREATE INDEX idx_api_keys_artifact ON dbo.api_keys(artifact_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_keys_status' AND object_id = OBJECT_ID(N'dbo.api_keys'))
CREATE INDEX idx_api_keys_status ON dbo.api_keys(status);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_api_keys_expires_at' AND object_id = OBJECT_ID(N'dbo.api_keys'))
CREATE INDEX idx_api_keys_expires_at ON dbo.api_keys(expires_at) WHERE expires_at IS NOT NULL;
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_rest_apis_org' AND object_id = OBJECT_ID(N'dbo.rest_apis'))
CREATE INDEX idx_rest_apis_org ON dbo.rest_apis(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_applications_org' AND object_id = OBJECT_ID(N'dbo.applications'))
CREATE INDEX idx_applications_org ON dbo.applications(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_applications_project_id' AND object_id = OBJECT_ID(N'dbo.applications'))
CREATE INDEX idx_applications_project_id ON dbo.applications(organization_uuid, project_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_application_api_keys_app_id' AND object_id = OBJECT_ID(N'dbo.application_api_keys'))
CREATE INDEX idx_application_api_keys_app_id ON dbo.application_api_keys(application_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_application_api_keys_key_id' AND object_id = OBJECT_ID(N'dbo.application_api_keys'))
CREATE INDEX idx_application_api_keys_key_id ON dbo.application_api_keys(api_key_id);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_application_artifacts_app_id' AND object_id = OBJECT_ID(N'dbo.application_artifacts'))
CREATE INDEX idx_application_artifacts_app_id ON dbo.application_artifacts(application_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_application_artifacts_artifact_id' AND object_id = OBJECT_ID(N'dbo.application_artifacts'))
CREATE INDEX idx_application_artifacts_artifact_id ON dbo.application_artifacts(artifact_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_rest_apis_lifecycle_status' AND object_id = OBJECT_ID(N'dbo.rest_apis'))
CREATE INDEX idx_rest_apis_lifecycle_status ON dbo.rest_apis(lifecycle_status);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_websub_apis_lifecycle_status' AND object_id = OBJECT_ID(N'dbo.websub_apis'))
CREATE INDEX idx_websub_apis_lifecycle_status ON dbo.websub_apis(lifecycle_status);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_webbroker_apis_lifecycle_status' AND object_id = OBJECT_ID(N'dbo.webbroker_apis'))
CREATE INDEX idx_webbroker_apis_lifecycle_status ON dbo.webbroker_apis(lifecycle_status);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscription_plans_org' AND object_id = OBJECT_ID(N'dbo.subscription_plans'))
CREATE INDEX idx_subscription_plans_org    ON dbo.subscription_plans(organization_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_subscription_plans_status' AND object_id = OBJECT_ID(N'dbo.subscription_plans'))
CREATE INDEX idx_subscription_plans_status ON dbo.subscription_plans(status);

-- EventHub tables for multi-replica HA sync and gateway event propagation.
-- Keyed columns are bounded NVARCHAR to stay within SQL Server index-key limits.
IF OBJECT_ID(N'dbo.gateway_states', N'U') IS NULL
CREATE TABLE dbo.gateway_states (
    gateway_id VARCHAR(40) PRIMARY KEY,
    version_id NVARCHAR(255) NOT NULL DEFAULT '',
    updated_at DATETIME2(7) NOT NULL DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (gateway_id) REFERENCES dbo.gateways(uuid) ON DELETE CASCADE
);

IF OBJECT_ID(N'dbo.events', N'U') IS NULL
CREATE TABLE dbo.events (
    gateway_id VARCHAR(40) NOT NULL,
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
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_events_entity' AND object_id = OBJECT_ID(N'dbo.events'))
CREATE INDEX idx_events_entity ON dbo.events(entity_type, entity_id);

-- Audit table
IF OBJECT_ID(N'dbo.audit', N'U') IS NULL
CREATE TABLE dbo.audit (
    uuid VARCHAR(40) PRIMARY KEY,
    action VARCHAR(50) NOT NULL,
    resource_uuid VARCHAR(40) NOT NULL,
    resource_type VARCHAR(50),
    organization_uuid VARCHAR(40) NOT NULL,
    performed_by VARCHAR(200),
    performed_at DATETIME2(7) DEFAULT SYSUTCDATETIME(),
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE
);

IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_gateway_association_mappings_artifact' AND object_id = OBJECT_ID(N'dbo.gateway_association_mappings'))
CREATE INDEX idx_gateway_association_mappings_artifact ON dbo.gateway_association_mappings(artifact_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_gateway_association_mappings_gateway' AND object_id = OBJECT_ID(N'dbo.gateway_association_mappings'))
CREATE INDEX idx_gateway_association_mappings_gateway ON dbo.gateway_association_mappings(gateway_uuid);
IF NOT EXISTS (SELECT 1 FROM sys.indexes WHERE name = N'idx_audit_org' AND object_id = OBJECT_ID(N'dbo.audit'))
CREATE INDEX idx_audit_org ON dbo.audit(organization_uuid);
