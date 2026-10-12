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

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import {
  ExtensionsProvider,
  PAGE_API_OBSERVABILITY_LOGS_SLOT,
  type ApiControlPlaneExtension,
} from '../extensions';
import { AppRoutes } from './AppRoutes';
import { routes } from './paths';
import { anOrganization, aProject, aRestApi, collection, resource } from '../test/msw';
import { authStatePresets } from '../test/mockAuthState';
import { server } from '../test/server';
import { renderWithProviders, screen } from '../test/utils';

// Covers the `page.apiObservabilityLogs` slot: a host can replace the built-in
// Logs page, and the built-in one is still what renders when no host registers
// an override.
describe('AppRoutes observability logs page override', () => {
  const org = anOrganization({
    id: 'api-platform-demo',
    displayName: 'API Platform Demo',
  });
  const project = aProject({ id: 'retail-apis', displayName: 'Retail APIs' });
  const api = aRestApi({ id: 'orders', displayName: 'Orders' });

  beforeEach(() => {
    vi.stubEnv('VITE_USE_MOCK_API', 'true');
    server.use(
      collection('/organizations', [org]),
      resource('/organizations/:organizationId', org),
      collection('/projects', [project]),
      resource('/projects/:projectId', project),
      collection('/rest-apis', [api]),
      resource('/rest-apis/:restApiId', api)
    );
  });
  afterEach(() => vi.unstubAllEnvs());

  const override: ApiControlPlaneExtension = {
    id: 'api-observability-logs',
    label: 'Logs',
    level: 'api',
    order: 0,
    render: () => <div>Overridden logs page</div>,
    routePath: 'observability/logs',
    slot: PAGE_API_OBSERVABILITY_LOGS_SLOT,
  };

  const renderAt = (
    route: string,
    extensions: ApiControlPlaneExtension[] = [override]
  ) =>
    renderWithProviders(
      <ExtensionsProvider extensions={extensions}>
        <AppRoutes extensions={extensions} />
      </ExtensionsProvider>,
      { authState: authStatePresets.authenticated(), route }
    );

  it('renders the override in place of the built-in page inside an API', async () => {
    renderAt(
      routes.apiObservabilityLogs('api-platform-demo', 'retail-apis', 'orders')
    );

    expect(
      await screen.findByText('Overridden logs page')
    ).toBeInTheDocument();
  });

  it('renders it at the scope-less alias too, where the built-in page would gate', async () => {
    // The override replaces the whole page, `ScopeGate` included.
    renderAt(routes.apiObservabilityLogs('api-platform-demo', null, null));

    expect(
      await screen.findByText('Overridden logs page')
    ).toBeInTheDocument();
  });

  it('keeps the built-in page when nothing is registered', async () => {
    renderAt(routes.apiObservabilityLogs('api-platform-demo', null, null), []);

    expect(
      await screen.findByText('Runtime logs are streamed per API.')
    ).toBeInTheDocument();
    expect(
      screen.queryByText('Overridden logs page')
    ).not.toBeInTheDocument();
  });
});
