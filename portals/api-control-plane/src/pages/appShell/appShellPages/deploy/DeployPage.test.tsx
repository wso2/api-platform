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

import type { UseQueryResult } from '@tanstack/react-query';
import { Route, Routes } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import {
  useApi,
  useDeleteGatewayDeployment,
  useDeployApi,
  useGatewayDeployments,
  useGateways,
  useRestoreGatewayDeployment,
  useUndeployGatewayDeployment,
} from '../../../../api/hooks/useMvpQueries';
import { renderWithProviders, screen } from '../../../../test/utils';
import type { Api, Gateway, GatewayDeployment } from '../../../../types/domain';
import { DeployPage } from './DeployPage';

vi.mock('../../api/hooks/useMvpQueries', async (importActual) => ({
  ...(await importActual<typeof import('../../../../api/hooks/useMvpQueries')>()),
  useApi: vi.fn(),
  useGateways: vi.fn(),
  useGatewayDeployments: vi.fn(),
  useDeployApi: vi.fn(),
  useUndeployGatewayDeployment: vi.fn(),
  useRestoreGatewayDeployment: vi.fn(),
  useDeleteGatewayDeployment: vi.fn(),
}));

const ROUTE = '/organizations/acme/projects/retail/apis/orders-api/deploy';

const api: Api = {
  id: 'api-1',
  projectId: 'proj-1',
  name: 'orders-api',
  displayName: 'Orders API',
  handler: 'orders-api',
  kind: 'API_PROXY',
  status: 'ACTIVE',
  version: '1.0.0',
};

const gateway: Gateway = {
  id: 'gw-1',
  name: 'prod-gw',
  displayName: 'Production Gateway',
  vhost: 'mg.acme.dev',
  functionalityType: 'regular',
  mode: 'self-hosted',
  isActive: true,
};

const deployed: GatewayDeployment = {
  id: 'dep-1',
  name: 'v1.0-prod',
  gatewayId: 'gw-1',
  status: 'DEPLOYED',
  createdAt: '2026-07-01T08:00:00Z',
  updatedAt: '2026-07-01T08:05:00Z',
};

const query = <T,>(overrides: Partial<UseQueryResult<T, Error>>) =>
  ({
    refetch: vi.fn(),
    isFetching: false,
    ...overrides,
  }) as unknown as UseQueryResult<T, Error>;

// Minimal mutation-result stub, cast to the specific hook's return type.
const mutation = <H extends (...args: never[]) => unknown>(mutate = vi.fn()) =>
  ({ mutate, isPending: false }) as unknown as ReturnType<H>;

function renderPage() {
  return renderWithProviders(
    <Routes>
      <Route
        path="/organizations/:orgHandle/projects/:projectHandler/apis/:apiHandler/deploy"
        element={<DeployPage />}
      />
    </Routes>,
    { route: ROUTE }
  );
}

describe('DeployPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useApi).mockReturnValue(query({ isLoading: false, data: api }));
    vi.mocked(useGateways).mockReturnValue(
      query({ isLoading: false, data: [gateway] })
    );
    vi.mocked(useGatewayDeployments).mockReturnValue(
      query({ isLoading: false, data: [deployed] })
    );
    vi.mocked(useDeployApi).mockReturnValue(mutation<typeof useDeployApi>());
    vi.mocked(useUndeployGatewayDeployment).mockReturnValue(
      mutation<typeof useUndeployGatewayDeployment>()
    );
    vi.mocked(useRestoreGatewayDeployment).mockReturnValue(
      mutation<typeof useRestoreGatewayDeployment>()
    );
    vi.mocked(useDeleteGatewayDeployment).mockReturnValue(
      mutation<typeof useDeleteGatewayDeployment>()
    );
  });

  it('shows the loading state', () => {
    vi.mocked(useApi).mockReturnValue(query({ isLoading: true }));
    renderPage();
    expect(screen.getByText('Loading deploy state')).toBeInTheDocument();
  });

  it('prompts to add a gateway when none exist', () => {
    vi.mocked(useGateways).mockReturnValue(
      query({ isLoading: false, data: [] })
    );
    vi.mocked(useGatewayDeployments).mockReturnValue(
      query({ isLoading: false, data: [] })
    );
    renderPage();
    expect(screen.getByText('No gateway added yet')).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Add Gateway' })
    ).toBeInTheDocument();
  });

  it('renders the gateway card expanded with status and history', () => {
    renderPage();
    // Header: gateway name, connection chip, current deployment chip.
    expect(screen.getByText('Production Gateway')).toBeInTheDocument();
    expect(screen.getAllByText('Active').length).toBeGreaterThan(0);
    expect(screen.getByText('Current Deployment:')).toBeInTheDocument();
    // First gateway is expanded: status panel + history are visible.
    expect(screen.getByText('Deployment Status')).toBeInTheDocument();
    expect(screen.getByText('API Deployment History')).toBeInTheDocument();
    expect(screen.getByText('Latest')).toBeInTheDocument();
  });

  it('deploys with an auto-generated {gateway}_{date}_{n} name', async () => {
    const mutate = vi.fn();
    vi.mocked(useDeployApi).mockReturnValue(
      mutation<typeof useDeployApi>(mutate)
    );
    const { user } = renderPage();

    await user.click(screen.getByRole('button', { name: 'Deploy' }));

    expect(mutate).toHaveBeenCalledWith(
      {
        api,
        input: {
          name: expect.stringMatching(/^prod-gw_\d{4}-\d{2}-\d{2}_1$/),
          gatewayId: 'gw-1',
          base: 'current',
        },
      },
      expect.any(Object)
    );
  });

  it('stops the active deployment from the status panel', async () => {
    const mutate = vi.fn();
    vi.mocked(useUndeployGatewayDeployment).mockReturnValue(
      mutation<typeof useUndeployGatewayDeployment>(mutate)
    );
    const { user } = renderPage();

    await user.click(screen.getByRole('button', { name: 'Stop' }));

    expect(mutate).toHaveBeenCalledWith(
      { api, deployment: deployed },
      expect.any(Object)
    );
  });

  it('redeploys a suspended deployment', async () => {
    const suspended: GatewayDeployment = { ...deployed, status: 'UNDEPLOYED' };
    vi.mocked(useGatewayDeployments).mockReturnValue(
      query({ isLoading: false, data: [suspended] })
    );
    const mutate = vi.fn();
    vi.mocked(useRestoreGatewayDeployment).mockReturnValue(
      mutation<typeof useRestoreGatewayDeployment>(mutate)
    );
    const { user } = renderPage();

    expect(screen.getByText('Suspended')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Redeploy' }));

    expect(mutate).toHaveBeenCalledWith(
      { api, deployment: suspended },
      expect.any(Object)
    );
  });

  it('filters gateways by search', async () => {
    const second: Gateway = {
      ...gateway,
      id: 'gw-2',
      name: 'edge-gw',
      displayName: 'Edge Gateway',
      isActive: false,
    };
    vi.mocked(useGateways).mockReturnValue(
      query({ isLoading: false, data: [gateway, second] })
    );
    const { user } = renderPage();

    expect(screen.getByText('Edge Gateway')).toBeInTheDocument();
    await user.type(screen.getByPlaceholderText('Search gateways'), 'prod');

    expect(screen.queryByText('Edge Gateway')).not.toBeInTheDocument();
    expect(screen.getByText('Production Gateway')).toBeInTheDocument();
  });
});
