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

import { beforeEach, describe, expect, it, vi } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import {
  aDeployment,
  aGateway,
  accepts,
  noContent,
  recorder,
  type DeploymentFixture,
  type Recorder,
} from '@/test/msw';
import { server } from '@/test/server';
import { renderWithProviders, screen, waitFor } from '@/test/utils';
import { GraphqlGatewayDeployCard } from './GraphqlGatewayDeployCard';

const ORG = 'api-platform-demo';
const API = 'countries-graphql-api';
const GATEWAY_ID = 'edge-gateway';

const gateway = aGateway({ displayName: 'Edge Gateway', id: GATEWAY_ID, isActive: true });

let requests: Recorder;

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

const renderCard = (deployments: DeploymentFixture[], onToggleExpand = vi.fn()) =>
  renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <GraphqlGatewayDeployCard
        deployments={deployments}
        gateway={gateway}
        graphqlApiId={API}
        isExpanded
        onRefresh={vi.fn()}
        onToggleExpand={onToggleExpand}
        refreshing={false}
      />
    </ApiScopeProvider>,
  );

describe('GraphqlGatewayDeployCard — a live deployment', () => {
  const live = aDeployment({
    createdAt: '2026-01-03T00:00:00Z',
    deploymentId: 'deployment-live',
    gatewayId: GATEWAY_ID,
    name: 'Edge_Gateway_2026-01-03_1',
    status: 'DEPLOYED',
  });

  it('stops the current deployment at the graphql-apis-scoped undeploy endpoint', async () => {
    server.use(
      accepts(
        'post',
        '/graphql-apis/:graphqlApiId/deployments/:deploymentId/undeploy',
        { ...live, status: 'UNDEPLOYING' },
        { record: requests },
      ),
    );
    const { user } = renderCard([live]);

    await user.click(screen.getByRole('button', { name: 'Stop' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(requests.last()?.url.pathname).toContain(
      `/graphql-apis/${API}/deployments/deployment-live/undeploy`,
    );
  });
});

describe('GraphqlGatewayDeployCard — a failed deployment', () => {
  const failed = aDeployment({
    createdAt: '2026-01-02T00:00:00Z',
    deploymentId: 'deployment-failed',
    gatewayId: GATEWAY_ID,
    name: 'Edge_Gateway_2026-01-02_1',
    status: 'FAILED',
    statusReason: 'DEPLOYMENT_TIMEOUT',
  });

  it('shows the failure reason and redeploys at the graphql-apis-scoped restore endpoint', async () => {
    server.use(
      accepts(
        'post',
        '/graphql-apis/:graphqlApiId/deployments/:deploymentId/restore',
        { ...failed, status: 'DEPLOYING' },
        { record: requests },
      ),
    );
    const { user } = renderCard([failed]);

    expect(screen.getByText('Deployment timed out. Gateway did not respond.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Redeploy' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(requests.last()?.url.pathname).toContain(
      `/graphql-apis/${API}/deployments/deployment-failed/restore`,
    );
  });
});

describe('GraphqlGatewayDeployCard — deployment history', () => {
  const deployments = [
    aDeployment({
      createdAt: '2026-01-04T00:00:00Z',
      deploymentId: 'd4',
      gatewayId: GATEWAY_ID,
      name: 'Edge_Gateway_2026-01-04_1',
      status: 'DEPLOYED',
    }),
    aDeployment({
      createdAt: '2026-01-03T00:00:00Z',
      deploymentId: 'd3',
      gatewayId: GATEWAY_ID,
      name: 'Edge_Gateway_2026-01-03_1',
      status: 'ARCHIVED',
    }),
    aDeployment({
      createdAt: '2026-01-02T00:00:00Z',
      deploymentId: 'd2',
      gatewayId: GATEWAY_ID,
      name: 'Edge_Gateway_2026-01-02_1',
      status: 'ARCHIVED',
    }),
    aDeployment({
      createdAt: '2026-01-01T00:00:00Z',
      deploymentId: 'd1',
      gatewayId: GATEWAY_ID,
      name: 'Edge_Gateway_2026-01-01_1',
      status: 'ARCHIVED',
    }),
  ];

  it('shows only the newest three inline, with the rest reachable via View More', async () => {
    const { user } = renderCard(deployments);

    // The newest one legitimately appears twice: once in the header's
    // "Current Deployment" chip, once as the history's first row.
    expect(screen.getAllByText('Edge_Gateway_2026-01-04_1').length).toBeGreaterThan(0);
    expect(screen.getByText('Edge_Gateway_2026-01-03_1')).toBeInTheDocument();
    expect(screen.getByText('Edge_Gateway_2026-01-02_1')).toBeInTheDocument();
    expect(screen.queryByText('Edge_Gateway_2026-01-01_1')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'View More' }));

    expect(await screen.findByText('Edge_Gateway_2026-01-01_1')).toBeInTheDocument();
  });

  it('deletes a settled deployment record from the full history drawer', async () => {
    server.use(
      noContent('delete', '/graphql-apis/:graphqlApiId/deployments/:deploymentId', {
        record: requests,
      }),
    );
    const { user } = renderCard(deployments);

    await user.click(screen.getByRole('button', { name: 'View More' }));
    await screen.findByText('Edge_Gateway_2026-01-01_1');

    // The oldest (archived, settled) row is safe to delete — the live one has no delete button at all.
    await user.click(screen.getByRole('button', { name: 'Delete Edge_Gateway_2026-01-01_1' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(requests.last()?.url.pathname).toContain(`/graphql-apis/${API}/deployments/d1`);
  });

  it('never offers to delete the currently-live deployment', () => {
    renderCard(deployments);

    expect(
      screen.queryByRole('button', { name: 'Delete Edge_Gateway_2026-01-04_1' }),
    ).not.toBeInTheDocument();
  });
});

describe('GraphqlGatewayDeployCard — restoring an older deployment', () => {
  const deployments = [
    aDeployment({
      createdAt: '2026-01-02T00:00:00Z',
      deploymentId: 'current',
      gatewayId: GATEWAY_ID,
      name: 'Current',
      status: 'DEPLOYED',
    }),
    aDeployment({
      createdAt: '2026-01-01T00:00:00Z',
      deploymentId: 'older',
      gatewayId: GATEWAY_ID,
      name: 'Older',
      status: 'ARCHIVED',
    }),
  ];

  it('opens the selector, picks a different deployment and restores it', async () => {
    server.use(
      accepts(
        'post',
        '/graphql-apis/:graphqlApiId/deployments/:deploymentId/restore',
        { ...deployments[1], status: 'DEPLOYING' },
        { record: requests },
      ),
    );
    const { user } = renderCard(deployments);

    await user.click(screen.getByRole('button', { name: 'Change deployment on Edge Gateway' }));
    expect(await screen.findByText('Select Deployment to Restore')).toBeInTheDocument();

    await user.click(screen.getByRole('radio', { name: /Older/ }));
    await user.click(screen.getByRole('button', { name: 'Restore' }));

    await waitFor(() => expect(requests.count()).toBe(1));
    expect(requests.last()?.url.pathname).toContain(`/graphql-apis/${API}/deployments/older/restore`);
  });

  it('never lets the currently-deployed entry itself be re-selected for restore', async () => {
    const { user } = renderCard(deployments);

    await user.click(screen.getByRole('button', { name: 'Change deployment on Edge Gateway' }));
    await screen.findByText('Select Deployment to Restore');

    await user.click(screen.getByRole('radio', { name: /Current/ }));

    expect(screen.getByRole('button', { name: 'Restore' })).toBeDisabled();
  });
});
