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

export type Organization = {
  id: string;
  numericId?: number;
  uuid: string;
  name: string;
  handle: string;
  description?: string;
  status?: string;
};

export type ProjectRepoType = 'MONO_REPO' | 'MULTI_REPO' | undefined;

export type Project = {
  id: string;
  orgId: string;
  name: string;
  handler: string;
  description?: string;
  region?: string;
  version?: string;
  createdDate?: string;
  updatedAt?: string;
  type?: ProjectRepoType;
  gitProvider?: string;
  repository?: string;
};

/**
 * Fields the console collects to create a project. platform-api persists only
 * name + description (the organization is taken from the bearer token); git /
 * region / repo-type are legacy build-plane concepts platform-api does not own.
 */
export type CreateProjectInput = {
  name: string;
  description?: string;
};

export type ApiKind = 'API_PROXY' | 'SERVICE' | 'WEB_APP';

export type ApiStatus = 'ACTIVE' | 'PENDING' | 'FAILED' | 'DRAFT';

export type Api = {
  id: string;
  projectId: string;
  name: string;
  displayName: string;
  handler: string;
  kind: ApiKind;
  status: ApiStatus;
  description?: string;
  version?: string;
  createdAt?: string;
  updatedAt?: string;
  httpBased?: boolean;
  /** Human-readable owner (GraphQL ownerName / platform createdBy when not a raw UUID). */
  owner?: string;
};

// --- Develop section (API detail) ---

export type HttpMethod =
  | 'GET'
  | 'POST'
  | 'PUT'
  | 'DELETE'
  | 'PATCH'
  | 'HEAD'
  | 'OPTIONS';

/** A policy applied at the API or operation level (platform `Policy`). */
export type ApiPolicy = {
  name: string;
  version: string;
  executionCondition?: string;
  params?: Record<string, unknown>;
};

/** An API operation (platform `Operation`/`OperationRequest`). */
export type ApiOperation = {
  name?: string;
  description?: string;
  method: HttpMethod;
  path: string;
  /** Per-operation request-flow policies (platform `request.policies[]`). */
  policies?: ApiPolicy[];
};

export type ApiEndpoints = {
  prodUrl?: string;
  sandboxUrl?: string;
};

/**
 * Full API detail used by the Develop section. Carries the editable
 * fields plus `raw` — the untouched platform object — so updates can
 * GET-merge-PUT without dropping server-managed fields.
 */
export type ApiDetail = Api & {
  context?: string;
  transport?: string[];
  operations: ApiOperation[];
  policies: ApiPolicy[];
  endpoints: ApiEndpoints;
  /** Untouched platform-api object, for merge-on-update. */
  raw?: Record<string, unknown>;
};

export type Environment = {
  id: string;
  name: string;
  type: 'PRODUCTION' | 'DEVELOPMENT';
};

export type Deployment = {
  id: string;
  componentId: string;
  environmentId: string;
  status: 'READY' | 'IN_PROGRESS' | 'FAILED' | 'NOT_DEPLOYED';
  version?: string;
  updatedAt?: string;
};

/**
 * A user-scoped API key summary (platform UserAPIKeyItem). The key value is
 * write-only: platform-api stores a hash and exposes only the masked form.
 */
export type ApiKeySummary = {
  name: string;
  maskedApiKey: string;
  status?: string;
  createdAt?: string;
  expiresAt?: string;
};

/**
 * Adds (injects) an API key for an API. platform-api hashes the supplied
 * value and broadcasts it to every gateway the API is deployed on.
 */
export type CreateApiKeyInput = {
  displayName: string;
  /** The plain-text key value; shown once, stored hashed. */
  apiKey: string;
  /** Optional ISO 8601 expiry. */
  expiresAt?: string;
};

/**
 * Lifecycle state of a deployment artifact on a gateway (platform
 * DeploymentResponse.status). DEPLOYING/UNDEPLOYING are transitional —
 * platform-api waits for the gateway's acknowledgement.
 */
export type GatewayDeploymentStatus =
  | 'DEPLOYED'
  | 'UNDEPLOYED'
  | 'DEPLOYING'
  | 'UNDEPLOYING'
  | 'FAILED'
  | 'ARCHIVED';

/**
 * An immutable deployment artifact of an API on a gateway (platform-api
 * `POST /rest-apis/{id}/deployments` family). Distinct from the legacy
 * environment-keyed `Deployment` used by the GraphQL build plane.
 */
export type GatewayDeployment = {
  id: string;
  name: string;
  gatewayId: string;
  status: GatewayDeploymentStatus;
  /** Error code when status is FAILED (e.g. DEPLOYMENT_TIMEOUT). */
  statusReason?: string;
  baseDeploymentId?: string;
  createdAt?: string;
  updatedAt?: string;
};

export type DeployApiInput = {
  /** Name/label for the deployment (e.g. "v1.0-prod"). */
  name: string;
  /** Target gateway UUID. */
  gatewayId: string;
  /** Definition source: 'current' (working copy, default) or a deploymentId. */
  base?: string;
};

export type ApiProxy = {
  id: string;
  componentId: string;
  context: string;
  version: string;
  visibility: 'PUBLIC' | 'PRIVATE';
};

export type GatewayFunctionalityType = 'regular' | 'ai' | 'event';

/** Whether the gateway runs on the customer's own infra or is WSO2-managed. */
export type GatewayMode = 'self-hosted' | 'managed';

export type Gateway = {
  id: string;
  name: string;
  displayName: string;
  /** Virtual host (domain name) the gateway serves APIs on. */
  vhost: string;
  functionalityType: GatewayFunctionalityType;
  mode: GatewayMode;
  description?: string;
  version?: string;
  isActive?: boolean;
  isCritical?: boolean;
  organizationId?: string;
  createdAt?: string;
  updatedAt?: string;
};

export type CreateGatewayInput = {
  name: string;
  displayName: string;
  vhost: string;
  functionalityType: GatewayFunctionalityType;
  description?: string;
};

export type GatewayToken = {
  id: string;
  token: string;
  createdAt?: string;
  message?: string;
};

/** How the platform authenticates to a Developer Portal instance. */
export type DevPortalAuthType = 'local' | 'idp_client_credentials';

/** Provisioning state of a devportal (platform-api DevPortalResponse.workflowStatus). */
export type DevPortalWorkflowStatus = 'pending' | 'active' | 'failed';

/** Maps 1:1 to platform-api's DevPortalResponse schema. */
export type DevPortal = {
  id: string;
  name: string;
  handle: string;
  description?: string;
  url?: string;
  workflowStatus: DevPortalWorkflowStatus;
  authType: DevPortalAuthType;
  createdAt?: string;
};

export type CreateDevPortalInput = {
  name: string;
  handle: string;
  url: string;
  authType: DevPortalAuthType;
  description?: string;
  /** Required when authType is 'idp_client_credentials'. */
  stsTokenUrl?: string;
  clientId?: string;
  clientSecret?: string;
};

/** `handle` is set at creation and not editable afterwards. */
export type UpdateDevPortalInput = {
  name: string;
  url: string;
  authType: DevPortalAuthType;
  description?: string;
  /** Required when authType is 'idp_client_credentials'. */
  stsTokenUrl?: string;
  clientId?: string;
  clientSecret?: string;
};

/**
 * How the API definition is sourced on create. `scratch` builds an empty proxy;
 * the import variants create from an OpenAPI definition (URL or uploaded file).
 */
export type CreateApiSource =
  | { mode: 'scratch' }
  | { mode: 'import-url'; url: string }
  | { mode: 'import-file'; file: File };

/** Backend (upstream) auth — maps to platform-api UpstreamDefinition.auth. */
export type UpstreamAuth = {
  type: 'basic' | 'bearer' | 'api-key';
  /** Header name (api-key); for basic/bearer the gateway uses Authorization. */
  header?: string;
  /** Secret: API key, bearer token, or Base64 basic credentials. */
  value?: string;
};

export type CreateApiInput = {
  name: string;
  displayName: string;
  description?: string;
  /** Only API proxies are creatable from the console. */
  kind: Extract<ApiKind, 'API_PROXY'>;
  version: string;
  apiContext?: string;
  /** Backend (upstream) production URL. Required for from-scratch creates. */
  prodUrl?: string;
  /** Optional backend (upstream) sandbox URL. */
  sandboxUrl?: string;
  /** Backend auth applied to the main upstream. */
  upstreamAuth?: UpstreamAuth;
  /** Supported transports; defaults to ['http','https']. */
  transport?: string[];
  /** OpenAPI import source; omit/`scratch` to start empty. */
  source?: CreateApiSource;
  /**
   * Operations to create the API with. Populated from a parsed OpenAPI
   * definition on import (platform-api has no server-side import).
   */
  operations?: ApiOperation[];
};
