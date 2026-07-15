/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// ============================================================================
// HTTP Client Types
// ============================================================================

/**
 * HTTP Methods
 */
export type HttpMethod = 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH';

/**
 * Request configuration for API calls
 */
export interface ApiRequestConfig {
  path: string;
  method: HttpMethod;
  data?: unknown;
  params?: Record<string, unknown>;
  headers?: Record<string, string>;
  baseUrl?: string;
}

/**
 * Generic API response type for GraphQL
 */
export interface GQLResponse<T> {
  data: T;
  errors?: Array<{
    message: string;
    locations?: Array<{ line: number; column: number }>;
    path?: string[];
  }>;
}

// ============================================================================
// Policy Hub Types
// ============================================================================

/**
 * Policy type from Policy Hub API
 */
export interface PolicyHubPolicy {
  name: string;
  version: string;
  displayName: string;
  description?: string;
  provider?: string;
  categories?: string[];
  isLatest?: boolean;
  [key: string]: unknown;
}

/**
 * Response type for guardrails endpoint
 */
export interface GuardrailsResponse {
  count: number;
  data: PolicyHubPolicy[];
  pagination?: {
    offset: number;
    limit: number;
    total: number;
  };
  meta?: {
    trace_id: string;
    timestamp: string;
    request_id: string;
  };
}

// ============================================================================
// Organization Types
// ============================================================================

/**
 * User organization owner
 */
export interface OrganizationOwner {
  id: number;
  idpId: string;
  createdAt?: string;
}

/**
 * Organization information.
 * `id` is a UUID string when coming from the Platform API.
 * `uuid` is kept for backward compatibility — set to the same value as `id`.
 */
export interface Organization {
  handle: string;
  /** UUID string (Platform API) */
  id: string;
  name: string;
  /** Alias for id — kept for backward compat with existing contexts */
  uuid: string;
  region?: string;
  owner?: OrganizationOwner;
}

/**
 * User validation response
 */
export interface ValidateUserResponse {
  organizations: Organization[];
  idpId: string;
  isNewUserSignup?: boolean;
}

// ============================================================================
// Project Types
// ============================================================================

/**
 * Permission entry for a project
 */
export interface ProjectPermission {
  key: string;
  value: string[];
}

/**
 * Project type enum
 */
export enum ProjectTypeResponse {
  MONO_REPO = 'MONO_REPO',
  MULTI_REPO = 'MULTI_REPO',
}

/**
 * Base project information
 * Matches platform-api's `Project` schema (openapi.yaml).
 */
export interface ProjectBase {
  id: string;
  displayName: string;
  description?: string;
  organizationId: string;
  createdAt?: string;
  updatedAt?: string;
}

// ============================================================================
// LLM Provider Types
// ============================================================================

/**
 * LLM Provider type
 */
export type LLMProviderType =
  | 'openai'
  | 'azure'
  | 'anthropic'
  | 'bedrock'
  | string;

/**
 * Model definition within a model provider
 */
export interface LLMModel {
  id: string;
  displayName: string;
  description?: string;
}

/**
 * Model provider
 */
export interface ModelProvider {
  id: string;
  displayName: string;
  models: LLMModel[];
}

/**
 * Authentication configuration for upstream
 */
export interface UpstreamAuth {
  type: 'api-key' | 'oauth2' | 'basic' | string;
  header?: string;
  valuePrefix?: string;
  value?: string;
}

/**
 * Upstream configuration for the LLM provider
 */
export interface UpstreamEndpoint {
  url: string;
  ref?: string;
  auth?: UpstreamAuth;
}

export interface Upstream {
  main: UpstreamEndpoint;
}

/**
 * Access control exception
 */
export interface AccessControlException {
  path: string;
  methods: string[];
}

/**
 * Access control configuration
 */
export interface AccessControl {
  mode: 'deny_all' | 'allow_all' | string;
  exceptions?: AccessControlException[];
}

/**
 * Rate limit configuration
 */
export interface RateLimitConfig {
  requestCount?: number;
  tokenCount?: number;
  requestResetDuration?: number;
  requestResetUnit?:
    | 'second'
    | 'minute'
    | 'hour'
    | 'day'
    | 'week'
    | 'month'
    | string;
  tokenResetDuration?: number;
  tokenResetUnit?:
    | 'second'
    | 'minute'
    | 'hour'
    | 'day'
    | 'week'
    | 'month'
    | string;
  [key: string]: unknown;
}

/**
 * Resource-wise rate limiting
 */
export interface ResourceWiseRateLimiting {
  default?: RateLimitConfig;
  resources?: Array<{
    resource: string;
    limit: RateLimitConfig;
  }>;
}

/**
 * Provider level rate limiting
 */
export interface ProviderLevelRateLimiting {
  global?: RateLimitConfig;
  resourceWise?: ResourceWiseRateLimiting;
}

/**
 * Rate limiting configuration
 */
export interface RateLimiting {
  providerLevel?: ProviderLevelRateLimiting;
  consumerLevel?: ProviderLevelRateLimiting;
}

/**
 * Policy path configuration
 */
export interface PolicyPath {
  path: string;
  methods: string[];
  params?: Record<string, unknown>;
}

/**
 * Policy configuration
 */
export interface Policy {
  name: string;
  version: string;
  paths?: PolicyPath[];
}

/**
 * Global (api-level) policy — no path binding
 */
export interface GlobalPolicy {
  name: string;
  version: string;
  executionCondition?: string;
  params?: Record<string, unknown>;
}

/**
 * Operation policy — path-bound
 */
export interface OperationPolicy {
  name: string;
  version: string;
  executionCondition?: string;
  paths: PolicyPath[];
}

/**
 * API Key security configuration
 */
export interface ApiKeySecurity {
  enabled: boolean;
  key?: string;
  in?: 'header' | 'query';
  valuePrefix?: string;
}

/**
 * OAuth2 grant type configuration
 */
export interface OAuth2GrantType {
  enabled: boolean;
  callbackUrl?: string;
}

/**
 * OAuth2 grant types configuration
 */
export interface OAuth2GrantTypes {
  authorizationCode?: OAuth2GrantType;
  implicit?: OAuth2GrantType;
  password?: OAuth2GrantType;
  clientCredentials?: OAuth2GrantType;
}

/**
 * OAuth2 security configuration
 */
export interface OAuth2Security {
  grantTypes?: OAuth2GrantTypes;
  scopes?: string[];
}

/**
 * Security configuration for LLM provider
 */
export interface SecurityConfig {
  enabled: boolean;
  apiKey?: ApiKeySecurity;
  oauth2?: OAuth2Security;
}

/**
 * LLM Provider
 */
export interface LLMProvider {
  id: string;
  displayName: string;
  description?: string;
  version?: string;
  context?: string;
  vhost?: string;
  template?: string;
  openapi?: string;
  modelProviders?: ModelProvider[];
  upstream?: Upstream;
  accessControl?: AccessControl;
  rateLimiting?: RateLimiting;
  globalPolicies?: GlobalPolicy[];
  operationPolicies?: OperationPolicy[];
  policies?: Policy[];
  security?: SecurityConfig;
  status?: 'Active' | 'Degraded' | 'Paused' | string;
  readOnly?: boolean;
  createdAt?: string;
  createdBy?: string;
  updatedAt?: string;
  lastUpdated?: string;
}

/**
 * Create LLM Provider Request - only required fields for creation
 */
export interface CreateLLMProviderRequest {
  id: string;
  displayName: string;
  description: string;
  version: string;
  context?: string;
  vhost?: string;
  template: string;
  modelProviders?: ModelProvider[];
  upstream: Upstream;
  accessControl: AccessControl;
  globalPolicies?: GlobalPolicy[];
  operationPolicies?: OperationPolicy[];
  policies?: Policy[];
  openapi?: string;
  security?: SecurityConfig;
}

/** Read-only fields from API (excluded from update requests) */
type LLMProviderReadOnlyFields =
  | 'status'
  | 'createdAt'
  | 'createdBy'
  | 'updatedAt'
  | 'lastUpdated';

/**
 * Update LLM Provider request - all fields optional, read-only excluded
 */
export type UpdateLLMProviderRequest = Partial<
  Omit<LLMProvider, LLMProviderReadOnlyFields>
>;

// ============================================================================
// Provider Template Types
// ============================================================================

/**
 * Token location configuration
 */
export interface TokenLocation {
  location: 'payload' | 'header' | 'queryParam' | 'pathParam' | string;
  identifier: string;
}

/**
 * Template metadata auth configuration
 */
export interface TemplateMetadataAuth {
  type: 'apiKey' | 'oauth2' | 'basic' | string;
  header?: string;
  valuePrefix?: string;
}

/**
 * Template metadata configuration
 * Contains default upstream configuration values
 */
export interface TemplateMetadata {
  endpointUrl?: string;
  auth?: TemplateMetadataAuth;
  logoUrl?: string;
  openapiSpecUrl?: string;
}

/**
 * Provider Template - used for response, context storage, and updates
 * Read-only fields (createdAt, updatedAt) excluded
 */
export interface ProviderTemplate {
  id: string;
  displayName: string;
  provider?: string;
  managedBy?: string;
  groupId?: string;
  description?: string;
  version: string;
  isLatest?: boolean;
  enabled?: boolean;
  /**
   * True when the template was pushed up from a data-plane gateway (origin
   * "gateway_api"). Its gateway-consumed configuration — the Connection
   * (endpoint/auth) and Token Mapping (extraction identifiers + resource
   * mappings) sections — is owned by the gateway and read-only in the control
   * plane; name/description/OpenAPI/logo remain editable.
   */
  readOnly?: boolean;
  promptTokens?: TokenLocation;
  completionTokens?: TokenLocation;
  totalTokens?: TokenLocation;
  remainingTokens?: TokenLocation;
  requestModel?: TokenLocation;
  responseModel?: TokenLocation;
  metadata?: TemplateMetadata;
  resourceMappings?: ResourceMappings;
  openapi?: string;
  createdBy?: string;
  createdAt?: string;
  updatedAt?: string;
  /** Flat logo URL, present on list responses (LLMProviderTemplateListItem). */
  logoUrl?: string;
}

/**
 * Per-resource token & model extraction overrides. Each mapping targets a
 * resource path pattern (e.g. "/responses" or "/chat/*") and may override any
 * of the six extraction identifiers for requests matching that path.
 */
export interface ResourceMapping {
  resource: string;
  promptTokens?: TokenLocation;
  completionTokens?: TokenLocation;
  totalTokens?: TokenLocation;
  remainingTokens?: TokenLocation;
  requestModel?: TokenLocation;
  responseModel?: TokenLocation;
}

export interface ResourceMappings {
  resources?: ResourceMapping[];
}

/**
 * Create Provider Template request. Mirrors `ProviderTemplate`; `id` is
 * required (the backend expects an explicit handle, e.g. "kimi-full").
 */
export type CreateProviderTemplateRequest = ProviderTemplate;

/**
 * Update Provider Template request - all fields optional
 */
export type UpdateProviderTemplateRequest = Partial<ProviderTemplate>;

// ============================================================================
// API Response Wrapper Types
// ============================================================================

/**
 * Pagination information from API responses
 */
export interface Pagination {
  total: number;
  offset: number;
  limit: number;
}

/**
 * Generic API list response wrapper
 * Used for endpoints that return paginated lists
 */
export interface ApiListResponse<T> {
  count: number;
  list: T[];
  pagination: Pagination;
}

/**
 * LLM Providers list API response
 */
export type LLMProvidersResponse = ApiListResponse<LLMProvider>;

/**
 * Provider Templates list API response
 */
export type ProviderTemplatesResponse = ApiListResponse<ProviderTemplate>;

// ============================================================================
// LLM Proxy Types
// ============================================================================

/**
 * API Key security configuration for LLM Proxy
 */
export interface ProxyApiKeySecurity {
  enabled: boolean;
  key: string;
  in: 'header' | 'query';
  valuePrefix?: string;
}

/**
 * Security configuration for LLM Proxy
 */
export interface ProxySecurityConfig {
  enabled: boolean;
  apiKey?: ProxyApiKeySecurity;
}

/**
 * Selected provider configuration for LLM Proxy
 */
export interface ProxyProviderConfig {
  id: string;
  auth?: UpstreamAuth;
}

/**
 * LLM Proxy
 */
export interface Proxy {
  id: string;
  displayName: string;
  description?: string;
  version?: string;
  projectId?: string;
  context?: string;
  vhost?: string;
  provider?: string | ProxyProviderConfig;
  openapi?: string;
  globalPolicies?: GlobalPolicy[];
  operationPolicies?: OperationPolicy[];
  policies?: Policy[];
  security?: ProxySecurityConfig;
  readOnly?: boolean;
  createdAt?: string;
  updatedAt?: string;
}
// LLM Provider Deployment Types
// ============================================================================

/**
 * Deployment status enum
 */
export type DeploymentStatus = 'DEPLOYED' | 'UNDEPLOYED' | 'ARCHIVED' | 'DEPLOYING' | 'UNDEPLOYING' | 'FAILED';

/**
 * Deployment response
 */
export interface DeploymentResponse {
  deploymentId: string;
  name: string;
  gatewayId: string;
  status: DeploymentStatus;
  baseDeploymentId?: string;
  metadata?: Record<string, unknown>;
  statusReason?: string | null;
  createdAt: string;
  updatedAt?: string;
}

/**
 * Create Proxy Request
 */
export interface CreateProxyRequest {
  id: string;
  displayName: string;
  description: string;
  version: string;
  projectId: string;
  context: string;
  vhost?: string;
  provider: ProxyProviderConfig;
  openapi: string;
  globalPolicies?: GlobalPolicy[];
  operationPolicies?: OperationPolicy[];
  policies: Policy[];
  security: ProxySecurityConfig;
}

/** Read-only fields from API (excluded from update requests) */
type ProxyReadOnlyFields = 'createdAt' | 'updatedAt';

/**
 * Update Proxy request - all fields optional, read-only excluded
 */
export type UpdateProxyRequest = Partial<Omit<Proxy, ProxyReadOnlyFields>>;

/**
 * Proxies list API response
 */
export type ProxiesResponse = ApiListResponse<Proxy>;

export interface DeploymentListResponse {
  count: number;
  list: DeploymentResponse[];
}

/**
 * Deploy request
 */
export interface DeployRequest {
  name: string;
  gatewayId: string;
  base: string;
  metadata?: Record<string, unknown>;
}

// ============================================================================
// LLM Provider API Key Types
// ============================================================================

/**
 * Create API Key request
 */
export interface CreateLLMProviderAPIKeyRequest {
  id?: string;
  displayName: string;
  expiresAt?: string;
  issuer?: string;
  allowedTargets?: string[];
}

/**
 * Create API Key response
 */
export interface CreateLLMProviderAPIKeyResponse {
  status: string;
  message: string;
  id: string;
  apiKey: string;
}

/**
 * Create API Key request for LLM proxies
 */
export interface CreateLLMProxyAPIKeyRequest {
  id?: string;
  displayName: string;
  expiresAt?: string;
  issuer?: string;
  allowedTargets?: string[];
}

/**
 * Create API Key response for LLM proxies
 */
export interface CreateLLMProxyAPIKeyResponse {
  status: string;
  message: string;
  id: string;
  apiKey: string;
}

/**
 * API Key list response for LLM providers
 */
export type APIKeyListResponse = ApiListResponse<UserAPIKey>;

// ============================================================================
// MCP Server Types
// ============================================================================

/**
 * MCP Server upstream auth configuration
 */
export interface MCPServerUpstreamAuth {
  type: string;
  header: string;
  value: string;
}

/**
 * MCP Server upstream endpoint configuration
 */
export interface MCPServerUpstreamEndpoint {
  url: string;
  ref?: string;
  hostRewrite?: string;
  auth?: MCPServerUpstreamAuth;
}

/**
 * MCP Server upstream configuration
 */
export interface MCPServerUpstream {
  main: MCPServerUpstreamEndpoint;
}

/**
 * MCP Server capabilities
 */
export interface MCPServerCapabilities {
  tools: MCPServerTool[];
  resources: MCPServerResource[];
  prompts: MCPServerPrompt[];
}

/**
 * MCP Server
 */
export interface MCPServer {
  id: string;
  displayName: string;
  description?: string;
  version?: string;
  projectId?: string;
  context?: string;
  vhost?: string;
  upstream?: MCPServerUpstream;
  kind?: string;
  policies?: unknown[];
  capabilities?: MCPServerCapabilities;
  readOnly?: boolean;
  createdAt?: string;
  updatedAt?: string;
}

/**
 * Create MCP Server Request
 */
export interface CreateMCPServerRequest {
  id: string;
  displayName: string;
  description?: string;
  version?: string;
  projectId?: string;
  context?: string;
  vhost?: string;
  upstream?: MCPServerUpstream;
  mcpSpecVersion?: string;
  kind?: string;
  policies?: unknown[];
  capabilities?: MCPServerCapabilities;
}

/** Read-only fields from API (excluded from update requests) */
type MCPServerReadOnlyFields = 'id' | 'createdAt' | 'updatedAt';

/**
 * Update MCP Server request - all fields optional, read-only excluded
 */
export type UpdateMCPServerRequest = Partial<Omit<MCPServer, MCPServerReadOnlyFields>>;

/**
 * MCP Servers list API response
 */
export type MCPServerListResponse = ApiListResponse<MCPServer>;

// ============================================================================
// MCP Server Validation Types
// ============================================================================

/**
 * MCP Server Info Fetch Request
 */
export interface MCPServerInfoFetchRequest {
  url: string;
  proxyId?: string;
  transportType?: string;
  headers?: Record<string, string>;
  auth?: {
    type: string;
    header: string;
    value: string;
  };
}

/**
 * MCP Server Info Fetch Response
 */
export interface MCPServerInfoFetchResponse {
  serverInfo: {
    name: string;
    version: string;
  };
  tools?: MCPServerTool[];
  resources?: MCPServerResource[];
  prompts?: MCPServerPrompt[];
}

export interface MCPServerTool {
  name: string;
  description?: string;
  inputSchema?: Record<string, unknown>;
}

export interface MCPServerResource {
  name: string;
  uri: string;
  description?: string;
  mimeType?: string;
}

export interface MCPServerPrompt {
  name: string;
  description?: string;
  arguments?: MCPServerPromptArgument[];
}

export interface MCPServerPromptArgument {
  name: string;
  description?: string;
  required?: boolean;
}

// ============================================================================
// Application Types
// ============================================================================

/**
 * Application
 */
export interface Application {
  id: string;
  displayName: string;
  type?: string;
  description?: string;
  owner?: string;
  projectId?: string;
  createdAt?: string;
  updatedAt?: string;
  lastUpdated?: string;
}

/**
 * Create Application Request
 */
export interface CreateApplicationRequest {
  id: string;
  displayName: string;
  type: 'genai';
  description?: string;
  owner?: string;
  projectId?: string;
}

/**
 * Update Application Request
 */
export interface UpdateApplicationRequest {
  displayName?: string;
  description?: string;
}

/**
 * Applications list API response
 */
export type ApplicationListResponse = ApiListResponse<Application>;

/**
 * Applications list query params
 */
export interface ApplicationListQueryParams {
  projectId?: string;
  limit?: number;
  offset?: number;
}

/**
 * Mapped API Key
 */
export interface MappedAPIKey {
  keyId: string;
  associatedEntity?: {
    id?: string;
    kind?: string;
  };
  status?: string;
  userId?: string;
  apiId?: string;
  apiName?: string;
  createdAt?: string;
  updatedAt?: string;
  expiresAt?: string;
}

/**
 * Mapped API Key list response
 */
export interface MappedAPIKeyListResponse {
  count: number;
  list: MappedAPIKey[];
  pagination?: {
    total: number;
    offset: number;
    limit: number;
  };
}

/**
 * Application API key mapping list query params
 */
export interface APIKeyMappingListQueryParams {
  limit?: number;
  offset?: number;
}

/**
 * Add Application API Keys Request
 */
export interface AddApplicationAPIKeysRequest {
  apiKeys: {
    keyId: string;
    associatedEntity: { id: string };
  }[];
}

/**
 * Remove application API key mapping options
 */
export interface RemoveApplicationAPIKeyOptions {
  entityID: string;
}

/**
 * Application Association target
 */
export interface ApplicationAssociation {
  id: string;
  kind?: string;
  displayName: string;
  version: string;
  [key: string]: unknown;
}

/**
 * Application Association list response
 */
export interface ApplicationAssociationListResponse {
  count: number;
  list: ApplicationAssociation[];
  pagination?: {
    total: number;
    offset: number;
    limit: number;
  };
}

/**
 * Add Application Associations Request
 */
export interface AddApplicationAssociationsRequest {
  associations: {
    id: string;
    kind?: string;
  }[];
}

/**
 * Association list query params
 */
export interface AssociationListQueryParams {
  limit?: number;
  offset?: number;
}

// ============================================================================
// Key Management Types
// ============================================================================

/**
 * API Key Revoke Request
 */
export interface APIKeyRevokeRequest {
  apiKey: string;
}

/**
 * User API Key
 */
export interface UserAPIKey {
  id?: string;
  displayName?: string;
  artifactId?: string;
  artifactType?: string;
  maskedApiKey?: string;
  status?: string;
  allowedTargets?: string;
  createdAt?: string;
  createdBy?: string;
  updatedAt?: string;
  expiresAt?: string;
}

/**
 * User API Key list response
 */
export type UserAPIKeyListResponse = ApiListResponse<UserAPIKey>;
