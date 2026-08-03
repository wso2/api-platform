import type {
  Api,
  DeployApiInput,
  GatewayDeployment,
} from '../../types/domain';
import { toGatewayDeployment } from '../adapters';
import {
  PLATFORM_API_BASE,
  platformDelete,
  platformGet,
  platformPost,
  usePlatformApi,
} from '../platform/platformClient';
import { delay, useMockApi } from '../shared/apiClientUtils';
import { ApiError } from '../types/errors';

/**
 * Gateway deployments — platform-api's deploy path. Deploying creates an
 * immutable artifact from the API's working copy ("current") or an existing
 * deployment, and pushes it to one gateway. The endpoint family differs by
 * kind (REST API vs MCP proxy) but shares the same request/response schema.
 */
const deploymentsBase = (api: Api) =>
  `${PLATFORM_API_BASE}/rest-apis/${encodeURIComponent(api.handler)}/deployments`;

// Mock mode: in-memory deployments keyed by API handler, so the deploy /
// undeploy / restore flow is fully exercisable without a backend.
const mockDeployments = new Map<string, GatewayDeployment[]>();

const readMock = (api: Api) => mockDeployments.get(api.handler) || [];

// The backend keeps one active deployment per gateway: deploying archives the
// previously DEPLOYED artifact on that gateway. Mirror that in the mock.
const archiveActiveMock = (api: Api, gatewayId: string) => {
  const next = readMock(api).map((deployment) =>
    deployment.gatewayId === gatewayId && deployment.status === 'DEPLOYED'
      ? { ...deployment, status: 'ARCHIVED' as const }
      : deployment
  );
  mockDeployments.set(api.handler, next);
};

const updateMock = (
  api: Api,
  deploymentId: string,
  patch: Partial<GatewayDeployment>
): GatewayDeployment => {
  const existing = readMock(api).find((item) => item.id === deploymentId);
  if (!existing) throw new ApiError('Deployment not found', 'NOT_FOUND', 404);
  const updated = {
    ...existing,
    ...patch,
    updatedAt: new Date().toISOString(),
  };
  mockDeployments.set(
    api.handler,
    readMock(api).map((item) => (item.id === deploymentId ? updated : item))
  );
  return updated;
};

const requirePlatform = () => {
  throw new ApiError('Gateway deployments require the platform API', 'UNKNOWN');
};

export async function listGatewayDeployments(
  orgHandle: string,
  api: Api
): Promise<GatewayDeployment[]> {
  if (useMockApi()) {
    await delay();
    return readMock(api);
  }
  if (usePlatformApi()) {
    const data = await platformGet<{ list?: unknown[] }>(
      deploymentsBase(api),
      orgHandle
    );
    return (data.list || []).map(toGatewayDeployment);
  }
  return [];
}

/** Creates a deployment artifact and pushes it to the target gateway. */
export async function deployApi(
  orgHandle: string,
  api: Api,
  input: DeployApiInput
): Promise<GatewayDeployment> {
  if (useMockApi()) {
    await delay();
    archiveActiveMock(api, input.gatewayId);
    const deployment: GatewayDeployment = {
      id: `gwdep-${Date.now()}`,
      name: input.name,
      gatewayId: input.gatewayId,
      status: 'DEPLOYED',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };
    mockDeployments.set(api.handler, [deployment, ...readMock(api)]);
    return deployment;
  }
  if (usePlatformApi()) {
    return toGatewayDeployment(
      await platformPost<unknown>(deploymentsBase(api), orgHandle, {
        name: input.name,
        base: input.base || 'current',
        gatewayId: input.gatewayId,
      })
    );
  }
  return requirePlatform();
}

/** Suspends an active deployment on its gateway (rollback-able). */
export async function undeployGatewayDeployment(
  orgHandle: string,
  api: Api,
  deployment: GatewayDeployment
): Promise<GatewayDeployment> {
  if (useMockApi()) {
    await delay();
    return updateMock(api, deployment.id, { status: 'UNDEPLOYED' });
  }
  if (usePlatformApi()) {
    const path = `${deploymentsBase(api)}/${encodeURIComponent(
      deployment.id
    )}/undeploy?gatewayId=${encodeURIComponent(deployment.gatewayId)}`;
    return toGatewayDeployment(
      await platformPost<unknown>(path, orgHandle, {})
    );
  }
  return requirePlatform();
}

/** Permanently deletes a non-active deployment artifact. */
export async function deleteGatewayDeployment(
  orgHandle: string,
  api: Api,
  deployment: GatewayDeployment
): Promise<void> {
  if (useMockApi()) {
    await delay();
    mockDeployments.set(
      api.handler,
      readMock(api).filter((item) => item.id !== deployment.id)
    );
    return;
  }
  if (usePlatformApi()) {
    await platformDelete<void>(
      `${deploymentsBase(api)}/${encodeURIComponent(deployment.id)}`,
      orgHandle
    );
    return;
  }
  return requirePlatform();
}

/** Re-activates an UNDEPLOYED/ARCHIVED deployment on its gateway. */
export async function restoreGatewayDeployment(
  orgHandle: string,
  api: Api,
  deployment: GatewayDeployment
): Promise<GatewayDeployment> {
  if (useMockApi()) {
    await delay();
    archiveActiveMock(api, deployment.gatewayId);
    return updateMock(api, deployment.id, { status: 'DEPLOYED' });
  }
  if (usePlatformApi()) {
    const path = `${deploymentsBase(api)}/${encodeURIComponent(
      deployment.id
    )}/restore?gatewayId=${encodeURIComponent(deployment.gatewayId)}`;
    return toGatewayDeployment(
      await platformPost<unknown>(path, orgHandle, {})
    );
  }
  return requirePlatform();
}
