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
import { NO_RESOURCE_LIMITS, PortProvider, type CloudHostPort } from '@/hostPort';
import { aGateway, collection } from '@/test/msw';
import { makeAuthState } from '@/test/mockAuthState';
import { makeConsoleScope } from '@/test/mockScope';
import { server } from '@/test/server';
import { renderWithProviders, screen } from '@/test/utils';

import { ProjectWelcome } from './ProjectWelcome';

const ORG = 'acme';

const port: CloudHostPort = {
  orgHandle: ORG,
  navigate: () => {},
  notify: () => {},
  apiFetch: async () => undefined as never,
  billing: { organization: async () => null },
  resourceLimits: NO_RESOURCE_LIMITS,
  resources: { invalidate: () => {} },
  permissions: { canAny: () => true },
  trackFeature: () => undefined,
};

const portal = { id: 'devs', displayName: 'Devs' };

const serveCounts = (gateways: number, portals: number) =>
  server.use(
    collection(
      '/gateways',
      Array.from({ length: gateways }, (_, index) => aGateway({ id: `gw-${index}` })),
    ),
    collection(
      '/api-portals',
      Array.from({ length: portals }, (_, index) => ({ ...portal, id: `p-${index}` })),
    ),
  );

function renderWelcome(
  options: { onCreateApi?: () => void; scopes?: string[]; enforce?: boolean } = {},
) {
  return renderWithProviders(
    <PortProvider value={port}>
      <ApiScopeProvider orgId={ORG}>
        <ProjectWelcome
          onCreateApi={options.onCreateApi ?? (() => {})}
          orgDisplayName="Acme"
          orgHandle={ORG}
          projectName="Retail"
        />
      </ApiScopeProvider>
    </PortProvider>,
    {
      route: `/organizations/${ORG}/projects/retail`,
      scope: makeConsoleScope(),
      ...(options.enforce
        ? {
            authState: makeAuthState({
              user: { name: 'T', email: 't@example.com', scopes: options.scopes ?? [] },
            }),
            permissionMode: 'enforce' as const,
          }
        : {}),
    },
  );
}

vi.mock('@/hooks/useGreetingName', () => ({ useGreetingName: (fallback: string) => fallback }));

beforeEach(() => resetHttpClient());

describe('ProjectWelcome', () => {
  it('greets the user and names the project', async () => {
    serveCounts(0, 0);
    renderWelcome();

    expect(await screen.findByText('Acme')).toBeInTheDocument();
    expect(screen.getByText(/Hello,/)).toBeInTheDocument();
    expect(screen.getByText('Retail')).toBeInTheDocument();
    expect(screen.getByText('Expose and secure APIs')).toBeInTheDocument();
  });

  it('opens API creation from Create API', async () => {
    serveCounts(0, 0);
    const onCreateApi = vi.fn();
    const { user } = renderWelcome({ onCreateApi });

    await user.click(await screen.findByRole('button', { name: 'Create API' }));

    expect(onCreateApi).toHaveBeenCalledTimes(1);
  });

  it('leaves Create API out for a user who may not create APIs', async () => {
    serveCounts(0, 0);
    renderWelcome({ enforce: true, scopes: [] });

    await screen.findByText('Expose and secure APIs');
    expect(screen.queryByRole('button', { name: 'Create API' })).not.toBeInTheDocument();
  });

  describe('gateway and portal cards', () => {
    it('offer to connect a gateway and create a portal when there are none', async () => {
      serveCounts(0, 0);
      renderWelcome();

      expect(await screen.findByRole('button', { name: /Connect a gateway/ })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /Create a portal/ })).toBeInTheDocument();
      expect(screen.queryByText(/^\d+ gateways?$/)).not.toBeInTheDocument();
    });

    it('show how many exist, and offer to manage them', async () => {
      serveCounts(3, 1);
      renderWelcome();

      expect(await screen.findByText('3 gateways')).toBeInTheDocument();
      expect(await screen.findByText('1 portal')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /Manage gateways/ })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: /Manage portals/ })).toBeInTheDocument();
    });

    it('say "1 gateway" for a single gateway', async () => {
      serveCounts(1, 0);
      renderWelcome();

      expect(await screen.findByText('1 gateway')).toBeInTheDocument();
    });

    it('leave both actions out for a user who may create neither', async () => {
      serveCounts(0, 0);
      renderWelcome({ enforce: true, scopes: [] });

      await screen.findByText('Run on any gateway');
      expect(screen.queryByRole('button', { name: /Connect a gateway/ })).not.toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /Create a portal/ })).not.toBeInTheDocument();
    });
  });
});
