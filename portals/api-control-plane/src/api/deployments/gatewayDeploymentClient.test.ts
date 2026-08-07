import { http, HttpResponse } from 'msw';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { server } from '../../test/server';
import type { Api } from '../../types/domain';

vi.mock('@asgardeo/auth-react', () => {
  const instance = {
    getAccessToken: vi.fn().mockResolvedValue('tok'),
    refreshAccessToken: vi.fn(),
  };
  return { AsgardeoSPAClient: { getInstance: () => instance } };
});

const BASE = 'http://platform.test';

async function loadClient() {
  vi.stubEnv('VITE_PLATFORM_API_BASE_URL', BASE);
  vi.resetModules();
  return import('./gatewayDeploymentClient');
}

const ORG = 'acme';

const restApi: Api = {
  id: 'api-1',
  projectId: 'proj-1',
  name: 'orders-api',
  displayName: 'Orders API',
  handler: 'orders-api',
  kind: 'API_PROXY',
  status: 'ACTIVE',
};

describe('gatewayDeploymentClient (platform mode)', () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => vi.unstubAllEnvs());

  it('listGatewayDeployments maps the platform list envelope', async () => {
    const { listGatewayDeployments } = await loadClient();
    server.use(
      http.get(`${BASE}/api/v0.9/rest-apis/orders-api/deployments`, () =>
        HttpResponse.json({
          count: 1,
          list: [
            {
              deploymentId: 'dep-1',
              name: 'v1.0-prod',
              gatewayId: 'gw-1',
              status: 'DEPLOYED',
              createdAt: '2026-07-01T08:00:00Z',
            },
          ],
        })
      )
    );
    const deployments = await listGatewayDeployments(ORG, restApi);
    expect(deployments).toEqual([
      {
        id: 'dep-1',
        name: 'v1.0-prod',
        gatewayId: 'gw-1',
        status: 'DEPLOYED',
        statusReason: undefined,
        baseDeploymentId: undefined,
        createdAt: '2026-07-01T08:00:00Z',
        updatedAt: undefined,
      },
    ]);
  });

  it('deployApi posts name/base/gatewayId to the REST API deployments path', async () => {
    const { deployApi } = await loadClient();
    let body: Record<string, unknown> | undefined;
    server.use(
      http.post(
        `${BASE}/api/v0.9/rest-apis/orders-api/deployments`,
        async ({ request }) => {
          body = (await request.json()) as Record<string, unknown>;
          return HttpResponse.json(
            {
              deploymentId: 'dep-2',
              name: body.name,
              gatewayId: body.gatewayId,
              status: 'DEPLOYING',
            },
            { status: 201 }
          );
        }
      )
    );
    const deployment = await deployApi(ORG, restApi, {
      name: 'v1.0-prod',
      gatewayId: 'gw-1',
    });
    expect(body).toEqual({
      name: 'v1.0-prod',
      base: 'current',
      gatewayId: 'gw-1',
    });
    expect(deployment).toMatchObject({ id: 'dep-2', status: 'DEPLOYING' });
  });

  it('undeployGatewayDeployment posts to the undeploy path with the gatewayId query', async () => {
    const { undeployGatewayDeployment } = await loadClient();
    let gatewayIdParam: string | null = null;
    server.use(
      http.post(
        `${BASE}/api/v0.9/rest-apis/orders-api/deployments/dep-1/undeploy`,
        ({ request }) => {
          gatewayIdParam = new URL(request.url).searchParams.get('gatewayId');
          return HttpResponse.json({
            deploymentId: 'dep-1',
            name: 'v1.0-prod',
            gatewayId: 'gw-1',
            status: 'UNDEPLOYING',
          });
        }
      )
    );
    const deployment = await undeployGatewayDeployment(ORG, restApi, {
      id: 'dep-1',
      name: 'v1.0-prod',
      gatewayId: 'gw-1',
      status: 'DEPLOYED',
    });
    expect(gatewayIdParam).toBe('gw-1');
    expect(deployment.status).toBe('UNDEPLOYING');
  });

  it('deleteGatewayDeployment deletes the deployment artifact', async () => {
    const { deleteGatewayDeployment } = await loadClient();
    let called = false;
    server.use(
      http.delete(
        `${BASE}/api/v0.9/rest-apis/orders-api/deployments/dep-1`,
        () => {
          called = true;
          return new HttpResponse(null, { status: 204 });
        }
      )
    );
    await deleteGatewayDeployment(ORG, restApi, {
      id: 'dep-1',
      name: 'v1.0-prod',
      gatewayId: 'gw-1',
      status: 'UNDEPLOYED',
    });
    expect(called).toBe(true);
  });

  it('restoreGatewayDeployment posts to the restore path', async () => {
    const { restoreGatewayDeployment } = await loadClient();
    server.use(
      http.post(
        `${BASE}/api/v0.9/rest-apis/orders-api/deployments/dep-1/restore`,
        () =>
          HttpResponse.json({
            deploymentId: 'dep-1',
            name: 'v1.0-prod',
            gatewayId: 'gw-1',
            status: 'DEPLOYING',
          })
      )
    );
    await expect(
      restoreGatewayDeployment(ORG, restApi, {
        id: 'dep-1',
        name: 'v1.0-prod',
        gatewayId: 'gw-1',
        status: 'UNDEPLOYED',
      })
    ).resolves.toMatchObject({ status: 'DEPLOYING' });
  });
});
