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

import { beforeEach, describe, expect, it } from 'vitest';

import { ApiScopeProvider } from '@/api/core/ApiScopeProvider';
import { resetHttpClient } from '@/api/core/http';
import {
  accepts,
  aDeployment,
  aGateway,
  aRestApi,
  collection,
  recorder,
  resource,
  type DeploymentFixture,
  type Recorder,
} from '@/test/msw';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';
import { DeployPage } from './DeployPage';

const ORG = 'api-platform-demo';
const PROJECT = 'retail-apis';
const API = 'orders-api';

const api = aRestApi({ displayName: 'Orders', id: API, projectId: PROJECT });

const gateway = aGateway({ displayName: 'Edge Gateway', id: 'edge-gateway', isActive: true });

/**
 * Settled on purpose: `useDeployments` polls while any deployment is
 * transitioning, and a DEPLOYING fixture would leave a timer running past the
 * assertions.
 */
const deployment: DeploymentFixture = aDeployment({
  createdAt: '2026-01-01T00:00:00Z',
  deploymentId: 'deployment-1',
  gatewayId: 'edge-gateway',
  name: 'Edge_Gateway_2026-01-01_1',
  status: 'DEPLOYED',
});

let requests: Recorder;

/**
 * The page's hooks read `ApiScopeContext` for the org, and the console scope
 * for the API handle — hence both providers. `isApiScope` has to be true or
 * `ScopeGate` renders its picker instead of the page.
 *
 * `params` is passed whole: `makeConsoleScope` spreads its overrides last, so a
 * partial `params` would replace the org and project handles rather than extend
 * them.
 */
function renderPage() {
  return renderWithProviders(
    <ApiScopeProvider orgId={ORG}>
      <DeployPage />
    </ApiScopeProvider>,
    {
      route: `/organizations/${ORG}/projects/${PROJECT}/apis/${API}/deploy`,
      scope: makeConsoleScope({
        isApiScope: true,
        params: { apiHandler: API, orgHandle: ORG, projectHandler: PROJECT },
      }),
    },
  );
}

/** The three reads the page makes before it can render a gateway card. */
function serveDeployState(deployments: DeploymentFixture[] = [deployment]) {
  server.use(
    resource('/rest-apis/:restApiId', api),
    collection('/gateways', [gateway]),
    collection('/rest-apis/:restApiId/deployments', deployments, { record: requests }),
  );
}

beforeEach(() => {
  requests = recorder();
  resetHttpClient();
});

describe('DeployPage', () => {
  it('reads deployments from the API-scoped sub-resource and renders the current one', async () => {
    serveDeployState();

    renderPage();

    expect(await screen.findByText('Edge Gateway')).toBeInTheDocument();
    // The name comes off the deployment record, so this fails if the list
    // envelope (`{ list }`) or the record's own fields are read wrongly.
    // `findAll`, because the first gateway auto-expands: the name renders in
    // the header's "Current Deployment" chip and again in the history row.
    expect(await screen.findAllByText('Edge_Gateway_2026-01-01_1')).not.toHaveLength(0);

    // Deployments hang off the API, not off a flat collection filtered by id.
    expect(requests.last()?.url.pathname).toContain(`/rest-apis/${API}/deployments`);
    expect(requests.last()?.headers.get('X-Org-Id')).toBe(ORG);
  });

  it('deploys to the API-scoped endpoint with the gateway handle in the body', async () => {
    serveDeployState([]);
    const deploys = recorder();
    server.use(
      accepts('post', '/rest-apis/:restApiId/deployments', deployment, { record: deploys }),
    );

    const { user } = renderPage();

    await user.click(await screen.findByRole('button', { name: 'Deploy' }));

    await expect.poll(() => deploys.count()).toBe(1);
    expect(deploys.last()?.url.pathname).toContain(`/rest-apis/${API}/deployments`);
    // `base: 'current'` is what makes this deploy the working copy rather than
    // a re-deploy of an existing artifact.
    expect(JSON.parse(deploys.last()?.body ?? '{}')).toMatchObject({
      base: 'current',
      gatewayId: 'edge-gateway',
    });
  });

  it('offers gateway creation when the organization has none', async () => {
    server.use(
      resource('/rest-apis/:restApiId', api),
      collection('/gateways', []),
      collection('/rest-apis/:restApiId/deployments', []),
    );

    renderPage();

    expect(await screen.findByText('No gateway added yet')).toBeInTheDocument();
  });
});
