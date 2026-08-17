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

import type {
  ApiProxy,
  Api,
  ApiKind,
  ApiStatus,
  Deployment,
  ApiPortal,
  ApiPortalAuthConfig,
  ApiPortalAuthType,
  ApiPortalMetadata,
  ApiPortalWorkflowStatus,
  Environment,
  Gateway,
  GatewayDeployment,
  GatewayDeploymentStatus,
  GatewayFunctionalityType,
  Organization,
  Project,
} from '../types/domain';

type AnyRecord = Record<string, unknown>;

const asRecord = (value: unknown): AnyRecord =>
  value && typeof value === 'object' ? (value as AnyRecord) : {};

const asString = (value: unknown, fallback = '') => {
  if (typeof value === 'string' && value.length > 0) return value;
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value);
  }
  return fallback;
};

const asOptionalString = (value: unknown) => asString(value) || undefined;

const asOptionalNumber = (value: unknown) => {
  if (typeof value === 'number' && Number.isFinite(value)) return value;
  if (typeof value === 'string' && value.trim().length > 0) {
    const numberValue = Number(value);
    if (Number.isFinite(numberValue)) return numberValue;
  }
  return undefined;
};

const asKind = (value: unknown): ApiKind => {
  const normalized = asString(value);
  if (
    normalized === 'API_PROXY' ||
    normalized === 'SERVICE' ||
    normalized === 'WEB_APP'
  ) {
    return normalized;
  }
  if (
    normalized === 'proxy' ||
    normalized === 'gitProxy' ||
    normalized === 'restAPI' ||
    normalized === 'graphql' ||
    normalized === 'thirdPartyAPI'
  ) {
    return 'API_PROXY';
  }
  if (
    normalized === 'byocWebApp' ||
    normalized === 'byocWebAppsDockerfileLess' ||
    normalized === 'byoiWebApp' ||
    normalized === 'buildpackWebApp'
  ) {
    return 'WEB_APP';
  }
  if (
    normalized === 'ballerinaService' ||
    normalized === 'byocService' ||
    normalized === 'byoiService' ||
    normalized === 'buildpackService' ||
    normalized === 'miApiService' ||
    normalized === 'prismMockService'
  ) {
    return 'SERVICE';
  }
  return 'API_PROXY';
};

const asStatus = (value: unknown): ApiStatus => {
  const normalized = asString(value).toUpperCase();
  if (normalized === 'ACTIVE' || normalized === 'RUNNING') return 'ACTIVE';
  if (normalized === 'PENDING' || normalized === 'IN_PROGRESS')
    return 'PENDING';
  if (normalized === 'FAILED' || normalized === 'ERROR') return 'FAILED';
  if (normalized === 'DRAFT') return 'DRAFT';
  return 'DRAFT';
};

export const toOrganization = (value: unknown): Organization => {
  const source = asRecord(value);
  const organization = asRecord(source.organization);
  const sourceWithOrganization = {
    ...organization,
    ...source,
  };
  const numericId = asOptionalNumber(
    sourceWithOrganization.id ??
      sourceWithOrganization.orgId ??
      sourceWithOrganization.organizationId
  );
  const id = asString(
    sourceWithOrganization.id,
    asString(
      sourceWithOrganization.orgId,
      asString(
        sourceWithOrganization.organizationId,
        asString(sourceWithOrganization.uuid, 'unknown-org')
      )
    )
  );
  const handle = asString(
    sourceWithOrganization.handle,
    asString(
      sourceWithOrganization.orgHandle,
      asString(
        sourceWithOrganization.organizationHandle,
        asString(sourceWithOrganization.organizationHandler, id)
      )
    )
  );
  return {
    id,
    numericId,
    uuid: asString(sourceWithOrganization.uuid, id),
    name: asString(
      sourceWithOrganization.name,
      asString(
        sourceWithOrganization.displayName,
        asString(sourceWithOrganization.orgName, 'Unnamed organization')
      )
    ),
    handle,
    description: asOptionalString(sourceWithOrganization.description),
    status: asOptionalString(sourceWithOrganization.status),
  };
};

const asProjectRepoType = (value: unknown): Project['type'] => {
  const normalized = asString(value).toUpperCase();
  if (normalized === 'MONO_REPO' || normalized === 'MULTI_REPO') {
    return normalized;
  }
  return undefined;
};

export const toProject = (value: unknown): Project => {
  const source = asRecord(value);
  const id = asString(source.id, 'unknown-project');
  return {
    id,
    orgId: asString(
      source.orgId,
      asString(source.organizationId, asString(source.orgUuid))
    ),
    name: asString(
      source.displayName,
      asString(source.name, 'Unnamed project')
    ),
    handler: asString(
      source.handler,
      asString(
        source.handle,
        asString(source.projectHandler, asString(source.projectHandle, id))
      )
    ),
    description: asOptionalString(source.description),
    region: asOptionalString(source.region),
    version: asOptionalString(source.version),
    createdDate: asOptionalString(source.createdDate ?? source.createdAt),
    updatedAt: asOptionalString(source.updatedAt ?? source.updatedDate),
    type: asProjectRepoType(source.type),
    gitProvider: asOptionalString(source.gitProvider),
    repository: asOptionalString(source.repository),
  };
};

// platform-api's createdBy is an internal user UUID — only surface owners that
// read as names (GraphQL ownerName, or future human-readable createdBy).
const UUID_PATTERN =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

const asOwnerName = (value: unknown): string | undefined => {
  const owner = asOptionalString(value);
  if (!owner || UUID_PATTERN.test(owner)) return undefined;
  return owner;
};

export const toApi = (value: unknown): Api => {
  const source = asRecord(value);
  const id = asString(source.id, 'unknown-component');
  const name = asString(source.name, id);
  return {
    id,
    projectId: asString(source.projectId),
    name,
    displayName: asString(source.displayName, name),
    handler: asString(source.handler, name),
    kind: asKind(source.kind ?? source.displayType ?? source.componentSubType),
    status: asStatus(source.status ?? source.initStatus),
    description: asOptionalString(source.description),
    version: asOptionalString(source.version),
    createdAt: asOptionalString(source.createdAt),
    updatedAt: asOptionalString(source.updatedAt),
    httpBased:
      typeof source.httpBased === 'boolean' ? source.httpBased : undefined,
    owner: asOwnerName(source.ownerName ?? source.createdBy),
  };
};

export const toEnvironment = (value: unknown): Environment => {
  const source = asRecord(value);
  return {
    id: asString(source.id, 'unknown-environment'),
    name: asString(source.name, 'Unnamed environment'),
    type: source.type === 'PRODUCTION' ? 'PRODUCTION' : 'DEVELOPMENT',
  };
};

export const toDeployment = (value: unknown): Deployment => {
  const source = asRecord(value);
  const status =
    source.status === 'READY' ||
    source.status === 'IN_PROGRESS' ||
    source.status === 'FAILED' ||
    source.status === 'NOT_DEPLOYED'
      ? source.status
      : 'NOT_DEPLOYED';
  return {
    id: asString(source.id, 'unknown-deployment'),
    componentId: asString(source.componentId),
    environmentId: asString(source.environmentId),
    status,
    version: asOptionalString(source.version),
    updatedAt: asOptionalString(source.updatedAt),
  };
};

const asBoolean = (value: unknown): boolean | undefined =>
  typeof value === 'boolean' ? value : undefined;

const asFunctionalityType = (value: unknown): GatewayFunctionalityType => {
  const normalized = asString(value).toLowerCase();
  if (normalized === 'ai' || normalized === 'event') return normalized;
  return 'regular';
};

// Self-hosted vs WSO2-managed is tagged in the gateway's free-form `properties`
// (gatewayMode); absent/anything-else is treated as WSO2-managed.
const asGatewayMode = (source: AnyRecord): Gateway['mode'] => {
  const props = asRecord(source.properties);
  const mode = asString(source.mode) || asString(props.gatewayMode);
  return mode === 'self-hosted' ? 'self-hosted' : 'managed';
};

export const toGateway = (value: unknown): Gateway => {
  const source = asRecord(value);
  const name = asString(
    source.displayName,
    asString(source.name, 'unknown-gateway')
  );
  return {
    id: asString(source.id, name),
    name,
    displayName: asString(source.displayName, name),
    // platform-api 0.12 returns `endpoints[]`; older releases sent a single
    // `vhost`. Prefer the first endpoint, falling back to vhost.
    vhost: asString(
      Array.isArray(source.endpoints) ? source.endpoints[0] : undefined,
      asString(source.vhost)
    ),
    functionalityType: asFunctionalityType(source.functionalityType),
    mode: asGatewayMode(source),
    description: asOptionalString(source.description),
    version: asOptionalString(source.version),
    isActive: asBoolean(source.isActive),
    isCritical: asBoolean(source.isCritical),
    organizationId: asOptionalString(source.organizationId),
    createdAt: asOptionalString(source.createdAt),
    updatedAt: asOptionalString(source.updatedAt),
  };
};

const API_PORTAL_AUTH_TYPES: ApiPortalAuthType[] = ['local', 'oauth2'];

const asApiPortalAuthType = (value: unknown): ApiPortalAuthType => {
  const normalized = asString(value).toLowerCase();
  return API_PORTAL_AUTH_TYPES.includes(normalized as ApiPortalAuthType)
    ? (normalized as ApiPortalAuthType)
    : API_PORTAL_AUTH_TYPES[0];
};

const API_PORTAL_WORKFLOW_STATUSES: ApiPortalWorkflowStatus[] = [
  'pending',
  'active',
  'failed',
];

const asApiPortalWorkflowStatus = (value: unknown): ApiPortalWorkflowStatus => {
  const normalized = asString(value).toLowerCase();
  return API_PORTAL_WORKFLOW_STATUSES.includes(
    normalized as ApiPortalWorkflowStatus
  )
    ? (normalized as ApiPortalWorkflowStatus)
    : API_PORTAL_WORKFLOW_STATUSES[0];
};

const toApiPortalAuthConfig = (
  value: unknown
): ApiPortalAuthConfig | undefined => {
  const source = asRecord(value);
  const stsTokenUrl = asOptionalString(source.stsTokenUrl);
  const clientId = asOptionalString(source.clientId);
  if (!stsTokenUrl && !clientId) return undefined;
  return { stsTokenUrl, clientId };
};

const toApiPortalMetadata = (
  value: unknown
): ApiPortalMetadata | undefined => {
  if (!value || typeof value !== 'object') return undefined;
  const source = value as ApiPortalMetadata;
  return Object.keys(source).length > 0 ? source : undefined;
};

export const toApiPortal = (value: unknown): ApiPortal => {
  const source = asRecord(value);
  const name = asString(source.name, 'unknown-api-portal');
  return {
    id: asString(source.id, name),
    name,
    handle: asString(source.handle, name),
    description: asOptionalString(source.description),
    url: asOptionalString(source.url),
    workflowStatus: asApiPortalWorkflowStatus(source.workflowStatus),
    authType: asApiPortalAuthType(source.authType),
    authConfig: toApiPortalAuthConfig(source.authConfig),
    metadata: toApiPortalMetadata(source.metadata),
    createdAt: asOptionalString(source.createdAt),
    updatedAt: asOptionalString(source.updatedAt),
    organizationId: asOptionalString(source.organizationId),
  };
};

const GATEWAY_DEPLOYMENT_STATUSES: GatewayDeploymentStatus[] = [
  'DEPLOYED',
  'UNDEPLOYED',
  'DEPLOYING',
  'UNDEPLOYING',
  'FAILED',
  'ARCHIVED',
];

const asGatewayDeploymentStatus = (value: unknown): GatewayDeploymentStatus => {
  const normalized = asString(value).toUpperCase() as GatewayDeploymentStatus;
  return GATEWAY_DEPLOYMENT_STATUSES.includes(normalized)
    ? normalized
    : 'FAILED';
};

/** Maps a platform-api DeploymentResponse to a GatewayDeployment. */
export const toGatewayDeployment = (value: unknown): GatewayDeployment => {
  const source = asRecord(value);
  const id = asString(source.deploymentId ?? source.id, 'unknown-deployment');
  return {
    id,
    name: asString(source.name, id),
    gatewayId: asString(source.gatewayId),
    status: asGatewayDeploymentStatus(source.status),
    statusReason: asOptionalString(source.statusReason),
    baseDeploymentId: asOptionalString(source.baseDeploymentId),
    createdAt: asOptionalString(source.createdAt),
    updatedAt: asOptionalString(source.updatedAt),
  };
};

export const toApiProxy = (value: unknown): ApiProxy => {
  const source = asRecord(value);
  return {
    id: asString(source.id, 'unknown-api-proxy'),
    componentId: asString(source.componentId),
    context: asString(source.context, '/'),
    version: asString(source.version, '1.0.0'),
    visibility: source.visibility === 'PRIVATE' ? 'PRIVATE' : 'PUBLIC',
  };
};
