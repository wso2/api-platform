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
    uuid TEXT PRIMARY KEY,
    handle TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Projects table
CREATE TABLE IF NOT EXISTS projects (
    uuid TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    organization_id TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_id) REFERENCES organizations(uuid) ON DELETE CASCADE
);

-- APIs table
CREATE TABLE IF NOT EXISTS apis (
    uuid TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    display_name TEXT,
    description TEXT,
    context TEXT NOT NULL,
    version TEXT NOT NULL,
    provider TEXT,
    project_id TEXT NOT NULL,
    lifecycle_status TEXT DEFAULT 'CREATED',
    has_thumbnail BOOLEAN DEFAULT FALSE,
    is_default_version BOOLEAN DEFAULT FALSE,
    is_revision BOOLEAN DEFAULT FALSE,
    revisioned_api_id TEXT,
    revision_id INTEGER DEFAULT 0,
    type TEXT DEFAULT 'HTTP',
    transport TEXT, -- JSON array as TEXT
    security_enabled BOOLEAN,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(uuid) ON DELETE CASCADE,
    UNIQUE(name, context, version, project_id)
);

-- API MTLS Configuration table
CREATE TABLE IF NOT EXISTS api_mtls_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    api_uuid TEXT NOT NULL,
    enabled BOOLEAN DEFAULT FALSE,
    enforce_if_client_cert_present BOOLEAN,
    verify_client BOOLEAN,
    client_cert TEXT,
    client_key TEXT,
    ca_cert TEXT,
    FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE
);

-- API Key Security Configuration table
CREATE TABLE IF NOT EXISTS api_key_security (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    api_uuid TEXT NOT NULL,
    enabled BOOLEAN,
    header TEXT,
    query TEXT,
    cookie TEXT,
    FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE
);

-- OAuth2 Security Configuration table
CREATE TABLE IF NOT EXISTS oauth2_security (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    api_uuid TEXT NOT NULL,
    enabled BOOLEAN,
    authorization_code_enabled BOOLEAN,
    authorization_code_callback_url TEXT,
    implicit_enabled BOOLEAN,
    implicit_callback_url TEXT,
    password_enabled BOOLEAN,
    client_credentials_enabled BOOLEAN,
    scopes TEXT, -- JSON array as TEXT
    FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE
);

-- CORS Configuration table
CREATE TABLE IF NOT EXISTS api_cors_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    api_uuid TEXT NOT NULL,
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
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    api_uuid TEXT NOT NULL,
    name TEXT UNIQUE NOT NULL,
    is_default BOOLEAN DEFAULT FALSE,
    timeout_connect_ms INTEGER,
    timeout_read_ms INTEGER,
    timeout_write_ms INTEGER,
    retries INTEGER,
    loadBalanace_algorithm TEXT DEFAULT 'ROUND_ROBIN',
    loadBalanace_failover BOOLEAN,
    circuit_breaker_enabled BOOLEAN DEFAULT FALSE,
    max_connections INTEGER,
    max_pending_requests INTEGER,
    max_requests INTEGER,
    max_retries INTEGER,
    FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE
);

-- Backend Endpoints table
CREATE TABLE IF NOT EXISTS backend_endpoints (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    backend_service_id INTEGER NOT NULL,
    url TEXT NOT NULL,
    description TEXT,
    healthcheck_enabled BOOLEAN DEFAULT FALSE,
    healthcheck_interval_seconds INTEGER,
    healthcheck_timeout_seconds INTEGER,
    unhealthy_threshold INTEGER,
    healthy_threshold INTEGER,
    weight INTEGER,
    mtls_enabled BOOLEAN DEFAULT FALSE,
    enforce_if_client_cert_present BOOLEAN,
    verify_client BOOLEAN,
    client_cert TEXT,
    client_key TEXT,
    ca_cert TEXT,
    FOREIGN KEY (backend_service_id) REFERENCES backend_services(id) ON DELETE CASCADE
);

-- API Rate Limiting Configuration table
CREATE TABLE IF NOT EXISTS api_rate_limiting (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    api_uuid TEXT NOT NULL,
    enabled BOOLEAN DEFAULT FALSE,
    rate_limit_count INTEGER,
    rate_limit_time_unit TEXT,
    stop_on_quota_reach BOOLEAN DEFAULT TRUE,
    FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE
);

-- API Operations table
CREATE TABLE IF NOT EXISTS api_operations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    api_uuid TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    authentication_required BOOLEAN,
    scopes TEXT, -- JSON array as TEXT
    FOREIGN KEY (api_uuid) REFERENCES apis(uuid) ON DELETE CASCADE
);

-- Operation Backend Services (routing) table
CREATE TABLE IF NOT EXISTS operation_backend_services (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    operation_id INTEGER NOT NULL,
    backend_service_name TEXT NOT NULL,
    weight INTEGER,
    FOREIGN KEY (operation_id) REFERENCES api_operations(id) ON DELETE CASCADE
);

-- Request Policies table
CREATE TABLE IF NOT EXISTS policies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    operation_id INTEGER NOT NULL,
    flow_direction TEXT NOT NULL, -- 'REQUEST' or 'RESPONSE'
    name TEXT NOT NULL,
    params TEXT, -- JSON object as TEXT
    FOREIGN KEY (operation_id) REFERENCES api_operations(id) ON DELETE CASCADE
);


-- Gateways table (scoped to organizations)
CREATE TABLE IF NOT EXISTS gateways (
    uuid TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL,
    name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (organization_id) REFERENCES organizations(uuid) ON DELETE CASCADE,
    UNIQUE(organization_id, name)
);

-- Gateway Tokens table
CREATE TABLE IF NOT EXISTS gateway_tokens (
    uuid TEXT PRIMARY KEY,
    gateway_uuid TEXT NOT NULL,
    token_hash TEXT NOT NULL,
    salt TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    revoked_at DATETIME,
    FOREIGN KEY (gateway_uuid) REFERENCES gateways(uuid) ON DELETE CASCADE,
    CHECK (status IN ('active', 'revoked')),
    CHECK (revoked_at IS NULL OR status = 'revoked')
);

-- Indexes for better performance
CREATE INDEX IF NOT EXISTS idx_projects_organization_id ON projects(organization_id);
CREATE INDEX IF NOT EXISTS idx_organizations_handle ON organizations(handle);
CREATE INDEX IF NOT EXISTS idx_apis_project_id ON apis(project_id);
CREATE INDEX IF NOT EXISTS idx_apis_name_context_version ON apis(name, context, version);
CREATE INDEX IF NOT EXISTS idx_backend_services_api_uuid ON backend_services(api_uuid);
CREATE INDEX IF NOT EXISTS idx_backend_endpoints_service_id ON backend_endpoints(backend_service_id);
CREATE INDEX IF NOT EXISTS idx_api_operations_api_uuid ON api_operations(api_uuid);
CREATE INDEX IF NOT EXISTS idx_operation_backend_services_operation_id ON operation_backend_services(operation_id);
CREATE INDEX IF NOT EXISTS idx_gateways_org ON gateways(organization_id);
CREATE INDEX IF NOT EXISTS idx_gateway_tokens_status ON gateway_tokens(gateway_uuid, status);
