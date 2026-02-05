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
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Projects table
CREATE TABLE IF NOT EXISTS projects (
    uuid VARCHAR(40) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    description VARCHAR(1023),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(name, organization_uuid)
);

-- APIs table
CREATE TABLE IF NOT EXISTS apis (
    uuid VARCHAR(40) PRIMARY KEY,
    handle VARCHAR(255) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    context VARCHAR(255) NOT NULL,
    version VARCHAR(30) NOT NULL,
    provider VARCHAR(200),
    project_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    lifecycle_status VARCHAR(20) DEFAULT 'CREATED',
    has_thumbnail BOOLEAN DEFAULT FALSE,
    is_default_version BOOLEAN DEFAULT FALSE,
    type VARCHAR(20) DEFAULT 'HTTP',
    transport VARCHAR(255), -- JSON array as TEXT
    policies JSONB DEFAULT '[]'::jsonb, -- JSON array as JSONB
    security_enabled BOOLEAN,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(handle, organization_uuid),
    UNIQUE(name, version, organization_uuid)
);

-- XHub Signature Security Configuration table
CREATE TABLE IF NOT EXISTS xhub_signature_security (
    id SERIAL PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    enabled BOOLEAN,
    header VARCHAR(255),
    algorithm VARCHAR(50),
    secret VARCHAR(255),
    FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE
);

-- API MTLS Configuration table
CREATE TABLE IF NOT EXISTS api_mtls_config (
    id SERIAL PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    enabled BOOLEAN DEFAULT FALSE,
    enforce_if_client_cert_present BOOLEAN,
    verify_client BOOLEAN,
    client_cert BYTEA,
    client_key VARCHAR(512),
    ca_cert BYTEA,
    FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE
);

-- API Key Security Configuration table
CREATE TABLE IF NOT EXISTS api_key_security (
    id SERIAL PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL
    enabled BOOLEAN,
    header VARCHAR(255),
    query VARCHAR(255),
    cookie VARCHAR(255),
    FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE
);

-- OAuth2 Security Configuration table
CREATE TABLE IF NOT EXISTS oauth2_security (
    id SERIAL PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    enabled BOOLEAN,
    authorization_code_enabled BOOLEAN,
    authorization_code_callback_url VARCHAR(255),
    implicit_enabled BOOLEAN,
    implicit_callback_url VARCHAR(255),
    password_enabled BOOLEAN,
    client_credentials_enabled BOOLEAN,
    scopes TEXT, -- JSON array as TEXT
    FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE
);

-- CORS Configuration table
CREATE TABLE IF NOT EXISTS api_cors_config (
    id SERIAL PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    enabled BOOLEAN DEFAULT FALSE,
    allow_origins TEXT,
    allow_methods TEXT,
    allow_headers TEXT,
    expose_headers TEXT,
    max_age INTEGER,
    allow_credentials BOOLEAN,
    FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE
);

-- Backend Services table
CREATE TABLE IF NOT EXISTS backend_services (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    timeout_connect_ms INTEGER,
    timeout_read_ms INTEGER,
    timeout_write_ms INTEGER,
    retries INTEGER,
    loadBalance_algorithm VARCHAR(255) DEFAULT 'ROUND_ROBIN',
    loadBalance_failover BOOLEAN,
    circuit_breaker_enabled BOOLEAN DEFAULT FALSE,
    max_connections INTEGER,
    max_pending_requests INTEGER,
    max_requests INTEGER,
    max_retries INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(name, organization_uuid)
);

-- API Backend Services junction table (many-to-many relationship)
CREATE TABLE IF NOT EXISTS api_backend_services (
    id SERIAL PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    backend_service_uuid VARCHAR(40) NOT NULL,
    is_default BOOLEAN DEFAULT FALSE,
    FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE,
    FOREIGN KEY (backend_service_uuid) REFERENCES backend_services(uuid) ON DELETE CASCADE,
    UNIQUE(api_uuid, backend_service_uuid)
);

-- Backend Endpoints table
CREATE TABLE IF NOT EXISTS backend_endpoints (
    id SERIAL PRIMARY KEY,
    backend_service_uuid VARCHAR(40) NOT NULL,
    url VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    healthcheck_enabled BOOLEAN DEFAULT FALSE,
    healthcheck_interval_seconds INTEGER,
    healthcheck_timeout_seconds INTEGER,
    unhealthy_threshold INTEGER,
    healthy_threshold INTEGER,
    weight INTEGER,
    mtls_enabled BOOLEAN DEFAULT FALSE,
    enforce_if_client_cert_present BOOLEAN,
    verify_client BOOLEAN,
    client_cert BYTEA,
    client_key VARCHAR(512),
    ca_cert BYTEA,
    FOREIGN KEY (backend_service_uuid) REFERENCES backend_services(uuid) ON DELETE CASCADE
);

-- API Rate Limiting Configuration table
CREATE TABLE IF NOT EXISTS api_rate_limiting (
    id SERIAL PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    enabled BOOLEAN DEFAULT FALSE,
    rate_limit_count INTEGER,
    rate_limit_time_unit VARCHAR(10),
    stop_on_quota_reach BOOLEAN DEFAULT TRUE,
    FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE
);

-- API Operations table
CREATE TABLE IF NOT EXISTS api_operations (
    id SERIAL PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    method VARCHAR(10) NOT NULL,
    path VARCHAR(255) NOT NULL,
    authentication_required BOOLEAN,
    scopes TEXT, -- JSON array as TEXT
    policies JSONB DEFAULT '[]'::jsonb, -- JSON array as JSONB
    FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE
);

-- Operation Backend Services (routing) table
CREATE TABLE IF NOT EXISTS operation_backend_services (
    id SERIAL PRIMARY KEY,
    operation_id INTEGER NOT NULL,
    backend_service_uuid VARCHAR(40) NOT NULL,
    weight INTEGER,
    FOREIGN KEY (operation_id) REFERENCES api_operations(id) ON DELETE CASCADE,
    FOREIGN KEY (backend_service_uuid) REFERENCES backend_services(uuid) ON DELETE CASCADE,
    UNIQUE(operation_id, backend_service_uuid)
);

-- Gateways table (scoped to organizations)
-- Must be created before api_deployments which references it
CREATE TABLE IF NOT EXISTS gateways (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    description VARCHAR(1023),
    properties JSONB NOT NULL DEFAULT '{}'::jsonb,
    vhost VARCHAR(255) NOT NULL,
    is_critical BOOLEAN DEFAULT FALSE,
    gateway_functionality_type VARCHAR(20) DEFAULT 'regular' NOT NULL,
    is_active BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
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
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    revoked_at TIMESTAMP,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE,
    CHECK (status IN ('active', 'revoked')),
    CHECK (revoked_at IS NULL OR status = 'revoked')
);

-- API Deployments table (immutable deployment artifacts)
CREATE TABLE IF NOT EXISTS api_deployments (
    deployment_id VARCHAR(40) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    api_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    gateway_uuid VARCHAR(40) NOT NULL,
    base_deployment_id VARCHAR(40),
    content BYTEA NOT NULL,
    metadata TEXT, -- JSON object as TEXT
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE,
    FOREIGN KEY (base_deployment_id) REFERENCES api_deployments(deployment_id) ON DELETE SET NULL
);

-- API Deployment Status table (current deployment state per API+Gateway)
CREATE TABLE IF NOT EXISTS api_deployment_status (
    api_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    gateway_uuid VARCHAR(40) NOT NULL,
    deployment_id VARCHAR(40) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'DEPLOYED',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (api_uuid, organization_uuid, gateway_uuid),
    FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE,
    FOREIGN KEY (deployment_id) REFERENCES api_deployments(deployment_id) ON DELETE CASCADE,
    CHECK (status IN ('DEPLOYED', 'UNDEPLOYED'))
);

-- API Associations table (for both gateways and dev portals)
CREATE TABLE IF NOT EXISTS api_associations (
    id SERIAL PRIMARY KEY,
    api_uuid VARCHAR(40) NOT NULL,
    organization_uuid VARCHAR(40) NOT NULL,
    resource_uuid VARCHAR(40) NOT NULL,
    association_type VARCHAR(20) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(api_uuid, resource_uuid, association_type, organization_uuid),
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

CREATE TABLE IF NOT EXISTS api_publications (
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
    FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE,
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
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_uuid, handle)
);

-- LLM Providers table
CREATE TABLE IF NOT EXISTS llm_providers (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    name VARCHAR(100) NOT NULL,
    description VARCHAR(1023),
    created_by VARCHAR(255),
    version VARCHAR(30) NOT NULL,
    context VARCHAR(200) DEFAULT '/',
    vhost VARCHAR(253),
    template VARCHAR(255) NOT NULL,
    upstream_url TEXT NOT NULL,
    upstream_auth TEXT,
    openapi_spec TEXT,
    model_list TEXT,
    rate_limiting TEXT,
    access_control TEXT,
    policies TEXT,
    security TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid, template) REFERENCES llm_provider_templates(organization_uuid, handle) ON UPDATE CASCADE ON DELETE RESTRICT,
    UNIQUE(organization_uuid, handle)
);

-- LLM Proxies table
CREATE TABLE IF NOT EXISTS llm_proxies (
    uuid VARCHAR(40) PRIMARY KEY,
    organization_uuid VARCHAR(40) NOT NULL,
    project_uuid VARCHAR(40) NOT NULL,
    handle VARCHAR(255) NOT NULL,
    name VARCHAR(253) NOT NULL,
    description VARCHAR(1023),
    created_by VARCHAR(255),
    version VARCHAR(30) NOT NULL,
    context VARCHAR(200) DEFAULT '/',
    vhost VARCHAR(253),
    provider VARCHAR(255) NOT NULL,
    openapi_spec TEXT,
    access_control TEXT,
    policies TEXT,
    security TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'CREATED',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_uuid) REFERENCES organizations(uuid) ON DELETE CASCADE,
    FOREIGN KEY (project_uuid) REFERENCES projects(uuid) ON DELETE CASCADE,
    FOREIGN KEY (organization_uuid, provider) REFERENCES llm_providers(organization_uuid, handle) ON UPDATE CASCADE ON DELETE RESTRICT,
    UNIQUE(organization_uuid, handle)
);

-- Indexes for better performance
CREATE INDEX IF NOT EXISTS idx_projects_organization_id ON projects(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_organizations_handle ON organizations(handle);
CREATE INDEX IF NOT EXISTS idx_apis_project_id ON apis(project_uuid);
CREATE INDEX IF NOT EXISTS idx_apis_name_context_version ON apis(name, context, version);
CREATE INDEX IF NOT EXISTS idx_backend_services_organization_uuid ON backend_services(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_backend_services_name_org ON backend_services(name, organization_uuid);
CREATE INDEX IF NOT EXISTS idx_api_backend_services_api_uuid ON api_backend_services(api_uuid);
CREATE INDEX IF NOT EXISTS idx_api_backend_services_backend_uuid ON api_backend_services(backend_service_uuid);
CREATE INDEX IF NOT EXISTS idx_backend_endpoints_service_id ON backend_endpoints(backend_service_uuid);
CREATE INDEX IF NOT EXISTS idx_api_operations_api_uuid ON api_operations(api_uuid);
CREATE INDEX IF NOT EXISTS idx_operation_backend_services_operation_id ON operation_backend_services(operation_id);
CREATE INDEX IF NOT EXISTS idx_operation_backend_services_backend_uuid ON operation_backend_services(backend_service_uuid);
CREATE INDEX IF NOT EXISTS idx_gateways_org ON gateways(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_gateway_tokens_status ON gateway_tokens(gateway_uuid, status);
CREATE INDEX IF NOT EXISTS idx_api_deployments_api_gateway ON api_deployments(api_uuid, gateway_uuid);
CREATE INDEX IF NOT EXISTS idx_api_deployments_created_at ON api_deployments(api_uuid, gateway_uuid, created_at);
CREATE INDEX IF NOT EXISTS idx_api_deployment_status_deployment ON api_deployment_status(deployment_id);
CREATE INDEX IF NOT EXISTS idx_api_deployment_status_status ON api_deployment_status(status);
CREATE INDEX IF NOT EXISTS idx_api_gw_created ON api_deployments (api_uuid, organization_uuid, gateway_uuid, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_devportals_org ON devportals(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_devportals_active ON devportals(organization_uuid, is_active);
CREATE INDEX IF NOT EXISTS idx_api_publications_api ON api_publications(api_uuid);
CREATE INDEX IF NOT EXISTS idx_api_publications_devportal ON api_publications(devportal_uuid);
CREATE INDEX IF NOT EXISTS idx_api_publications_org ON api_publications(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_api_publications_api_devportal_org ON api_publications(api_uuid, devportal_uuid, organization_uuid);
CREATE UNIQUE INDEX IF NOT EXISTS idx_devportals_default_per_org ON devportals(organization_uuid) WHERE is_default = TRUE;
CREATE INDEX IF NOT EXISTS idx_api_associations_api_resource_type ON api_associations(api_uuid, association_type, organization_uuid);
CREATE INDEX IF NOT EXISTS idx_api_associations_resource ON api_associations(association_type, resource_uuid, organization_uuid);
CREATE INDEX IF NOT EXISTS idx_api_associations_org ON api_associations(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_provider_templates_org ON llm_provider_templates(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_provider_templates_handle ON llm_provider_templates(organization_uuid, handle);
CREATE INDEX IF NOT EXISTS idx_llm_providers_org ON llm_providers(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_providers_handle ON llm_providers(organization_uuid, handle);
CREATE INDEX IF NOT EXISTS idx_llm_providers_template ON llm_providers(organization_uuid, template);
CREATE INDEX IF NOT EXISTS idx_llm_proxies_org ON llm_proxies(organization_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_proxies_project ON llm_proxies(organization_uuid, project_uuid);
CREATE INDEX IF NOT EXISTS idx_llm_proxies_handle ON llm_proxies(organization_uuid, handle);
CREATE INDEX IF NOT EXISTS idx_llm_proxies_provider ON llm_proxies(organization_uuid, provider);
