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

import { ExtensionsProvider, type ApiControlPlaneExtension } from '../extensions';
import { AppRoutes } from './AppRoutes';
import { anOrganization, aProject, collection, resource } from '../test/msw';
import { authStatePresets } from '../test/mockAuthState';
import { server } from '../test/server';
import { renderWithProviders, screen } from '../test/utils';

// A sidebar extension may carry `children`, each a page of its own. Without a
// route apiece the sidebar would offer a sub-item that leads nowhere, and
// without composing the parent's path a child could name a top-level route and
// shadow a built-in page.
describe('AppRoutes extension children', () => {
  const org = anOrganization({ id: 'api-platform-demo', displayName: 'API Platform Demo' });
  const project = aProject({ id: 'retail-apis', displayName: 'Retail APIs' });

  beforeEach(() => {
    vi.stubEnv('VITE_USE_MOCK_API', 'true');
    server.use(
      collection('/organizations', [org]),
      resource('/organizations/:organizationId', org),
      collection('/projects', [project]),
      resource('/projects/:projectId', project),
      collection('/rest-apis', [])
    );
  });
  afterEach(() => vi.unstubAllEnvs());

  const observability: ApiControlPlaneExtension = {
    id: 'observability',
    claims: 'observability',
    label: 'Observability',
    level: 'organization',
    order: 70,
    render: () => <div>Parent page</div>,
    routePath: 'observability',
    slot: 'sidebar.organization',
    children: [
      { id: 'observability-logs', label: 'Logs', render: () => <div>Logs page</div>, routePath: 'logs' },
      // A bare segment that also names a built-in page: it must mount under the
      // parent, not at the top level where it would shadow Gateways.
      {
        id: 'observability-gateways',
        label: 'Gateways',
        render: () => <div>Child gateways page</div>,
        routePath: 'gateways',
      },
    ],
  };

  const renderAt = (route: string, extensions: ApiControlPlaneExtension[] = [observability]) =>
    renderWithProviders(
      <ExtensionsProvider extensions={extensions}>
        <AppRoutes extensions={extensions} />
      </ExtensionsProvider>,
      { authState: authStatePresets.authenticated(), route }
    );

  // A parent may omit `render`; its own path then redirects to the first child
  // visible in the current scope, and `aliases` redirect to the parent.
  const parentWithoutRender: ApiControlPlaneExtension = {
    id: 'observability',
    claims: 'observability',
    label: 'Observability',
    level: 'organization',
    order: 70,
    routePath: 'observability',
    slot: 'sidebar.organization',
    aliases: ['old-logs'],
    children: [
      {
        id: 'observability-hidden',
        label: 'Hidden',
        render: () => <div>Hidden page</div>,
        routePath: 'hidden',
        isVisible: () => false,
      },
      { id: 'observability-logs', label: 'Logs', render: () => <div>Logs page</div>, routePath: 'logs' },
    ],
  };

  it('renders a child at the parent path, with the child’s own render', async () => {
    renderAt('/organizations/api-platform-demo/observability/logs');

    expect(await screen.findByText('Logs page')).toBeInTheDocument();
    expect(screen.queryByText('Parent page')).not.toBeInTheDocument();
  });

  it('still renders the parent at its own path', async () => {
    renderAt('/organizations/api-platform-demo/observability');

    expect(await screen.findByText('Parent page')).toBeInTheDocument();
  });

  it('mounts a child under the parent', async () => {
    renderAt('/organizations/api-platform-demo/observability/gateways');

    expect(await screen.findByText('Child gateways page')).toBeInTheDocument();
  });

  it('does not let a child claim the top-level path of the same name', async () => {
    renderAt('/organizations/api-platform-demo/gateways');

    // The built-in Gateways page answers here; the child does not.
    await screen.findByRole('main');
    expect(screen.queryByText('Child gateways page')).not.toBeInTheDocument();
  });

  it('redirects a parent without render to its first visible child', async () => {
    renderAt('/organizations/api-platform-demo/observability', [parentWithoutRender]);

    expect(await screen.findByText('Logs page')).toBeInTheDocument();
    expect(screen.queryByText('Hidden page')).not.toBeInTheDocument();
  });

  it('redirects an alias to the extension, and on to its first child', async () => {
    renderAt('/organizations/api-platform-demo/old-logs', [parentWithoutRender]);

    expect(await screen.findByText('Logs page')).toBeInTheDocument();
  });
});
