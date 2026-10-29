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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import { beforeEach, describe, expect, it, vi } from 'vitest';

import { mcpProxiesApis } from '../../../../apis/MCP/mcpProxiesApis';
import { renderWithProviders, screen, waitFor } from '../../../../test/utils';
import type { PublicationSummaryItem } from '../../../../utils/types';
import McpPublishHubs from './McpPublishHubs';

vi.mock('../../../../apis/MCP/mcpProxiesApis', () => ({
  mcpProxiesApis: { getMCPServer: vi.fn(), listMcpProxyApiPortalPublications: vi.fn() },
}));
vi.mock('../../../../hooks/usePublishProject', () => ({
  usePublishProject: () => ({
    currentOrganization: { handle: 'acme' },
    organizationId: 'org-1',
    overviewPath: (id: string) => `/mcp-proxy/${id}`,
    publishPath: (suffix = '') => `/publish${suffix}`,
  }),
}));
vi.mock('../../../../Components/AILoader', () => ({ default: () => <div data-testid="loader" /> }));
vi.mock('../../../../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const api = vi.mocked(mcpProxiesApis);

const server = { id: 'travel', displayName: 'Travel MCP' };
const hub = (fields: Partial<PublicationSummaryItem>): PublicationSummaryItem => ({
  apiPortalId: 'main',
  apiPortalName: 'Main Hub',
  apiPortalDescription: 'The main hub.',
  apiPortalUrl: 'https://hub.example.com',
  status: 'NOT_PUBLISHED',
  ...fields,
});
const hubs = [
  hub({ apiPortalId: 'main', apiPortalName: 'Main Hub', status: 'PUBLISHED' }),
  hub({ apiPortalId: 'eu', apiPortalName: 'EU Hub', apiPortalDescription: 'For Europe.', apiPortalUrl: 'https://eu.example.com' }),
  hub({ apiPortalId: 'old', apiPortalName: 'Legacy Hub', apiPortalDescription: 'Being retired.', status: 'DEPRECATED' }),
];

const hubsAre = (list: PublicationSummaryItem[]) =>
  api.listMcpProxyApiPortalPublications.mockResolvedValue({
    list,
    pagination: { total: list.length, offset: 0, limit: 100 },
  });

const renderPage = () =>
  renderWithProviders(<McpPublishHubs />, { path: '/publish/:serverId', route: '/publish/travel' });

const waitForCards = () => screen.findByRole('heading', { name: 'Publish Travel MCP' });

beforeEach(() => {
  api.getMCPServer.mockReset();
  api.listMcpProxyApiPortalPublications.mockReset();
  api.getMCPServer.mockResolvedValue(server as never);
  hubsAre(hubs);
});

describe('McpPublishHubs', () => {
  it('shows a loader while it reads', async () => {
    renderPage();

    expect(screen.getByText('Loading MCP Hubs')).toBeInTheDocument();
    await waitForCards();
  });

  it('asks for this proxy’s status on every hub', async () => {
    renderPage();
    await waitForCards();

    expect(api.getMCPServer).toHaveBeenCalledWith('travel', expect.any(String));
    expect(api.listMcpProxyApiPortalPublications).toHaveBeenCalledWith('travel', expect.any(String));
  });

  it('shows a card for each hub, with the proxy’s own status on it', async () => {
    renderPage();
    await waitForCards();

    expect(screen.getByText('Main Hub')).toBeInTheDocument();
    expect(screen.getByText('EU Hub')).toBeInTheDocument();
    expect(screen.getByText('Legacy Hub')).toBeInTheDocument();
    expect(screen.getByText('Published')).toBeInTheDocument();
    expect(screen.getByText('Not published')).toBeInTheDocument();
    expect(screen.getByText('Deprecated')).toBeInTheDocument();
  });

  it('says how many hubs it is on, counting a deprecated listing, which is still there', async () => {
    renderPage();
    await waitForCards();

    expect(screen.getByText('Published to 2 of 3 MCP Hubs')).toBeInTheDocument();
  });

  it('says it in the singular for a single hub', async () => {
    hubsAre([hub({ status: 'PUBLISHED' })]);

    renderPage();
    await waitForCards();

    expect(screen.getByText('Published to 1 of 1 MCP Hub')).toBeInTheDocument();
  });

  it('opens each hub’s own publish page for this proxy', async () => {
    renderPage();
    await waitForCards();

    const links = screen.getAllByRole('link', { name: 'Go to publish' }).map((link) => link.getAttribute('href'));
    expect(links).toEqual(['/publish/travel/portals/main', '/publish/travel/portals/eu', '/publish/travel/portals/old']);
  });

  it('encodes ids that are not URL-safe in those links', async () => {
    hubsAre([hub({ apiPortalId: 'a b/c' })]);

    renderPage();
    await waitForCards();

    expect(screen.getByRole('link', { name: 'Go to publish' })).toHaveAttribute(
      'href',
      '/publish/travel/portals/a%20b%2Fc'
    );
  });

  it('goes back to the proxy’s own page', async () => {
    renderPage();
    await waitForCards();

    expect(screen.getByRole('link', { name: 'Back to MCP Proxy' })).toHaveAttribute('href', '/mcp-proxy/travel');
  });

  describe('searching', () => {
    it('narrows the cards by name', async () => {
      const { user } = renderPage();
      await waitForCards();

      await user.type(screen.getByPlaceholderText('Search MCP Hubs'), 'eu');

      expect(screen.getByText('EU Hub')).toBeInTheDocument();
      expect(screen.queryByText('Main Hub')).not.toBeInTheDocument();
    });

    it('also matches a hub’s description and address, in any case', async () => {
      const { user } = renderPage();
      await waitForCards();

      await user.type(screen.getByPlaceholderText('Search MCP Hubs'), 'RETIRED');
      expect(screen.getByText('Legacy Hub')).toBeInTheDocument();
      expect(screen.queryByText('EU Hub')).not.toBeInTheDocument();

      await user.clear(screen.getByPlaceholderText('Search MCP Hubs'));
      await user.type(screen.getByPlaceholderText('Search MCP Hubs'), 'eu.example');
      expect(screen.getByText('EU Hub')).toBeInTheDocument();
      expect(screen.queryByText('Legacy Hub')).not.toBeInTheDocument();
    });

    it('says so when nothing matches, and keeps the count of what exists', async () => {
      const { user } = renderPage();
      await waitForCards();

      await user.type(screen.getByPlaceholderText('Search MCP Hubs'), 'zzz');

      expect(screen.getByRole('heading', { name: 'No matching MCP Hubs' })).toBeInTheDocument();
      expect(screen.getByText('Published to 2 of 3 MCP Hubs')).toBeInTheDocument();
    });
  });

  describe('an organization with no hubs yet', () => {
    it('says so, as an empty state with nothing to search', async () => {
      hubsAre([]);

      renderPage();

      expect(await screen.findByRole('heading', { name: 'No MCP Hubs Available' })).toBeInTheDocument();
      expect(screen.getByText('Add an MCP Hub for this organization before publishing an MCP proxy to it.')).toBeInTheDocument();
      expect(screen.queryByPlaceholderText('Search MCP Hubs')).not.toBeInTheDocument();
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });

    it('still names the proxy and offers the way back', async () => {
      hubsAre([]);

      renderPage();
      await waitForCards();

      expect(screen.getByRole('link', { name: 'Back to MCP Proxy' })).toBeInTheDocument();
      expect(screen.getByText('Published to 0 of 0 MCP Hubs')).toBeInTheDocument();
    });
  });

  describe('when it cannot be read', () => {
    it('explains a missing permission in plain words, without a retry that cannot help', async () => {
      api.listMcpProxyApiPortalPublications.mockRejectedValue(Object.assign(new Error('forbidden'), { status: 403 }));

      renderPage();

      expect(await screen.findByText('You don’t have permission')).toBeInTheDocument();
      expect(
        screen.getByText(
          'You don’t have permission to publish MCP proxies to MCP Hubs. Contact your organization admin to request access.'
        )
      ).toBeInTheDocument();
      expect(screen.queryByRole('button', { name: /retry|try again/i })).not.toBeInTheDocument();
    });

    it('shows the error with a retry for anything else, and reads again on retry', async () => {
      api.listMcpProxyApiPortalPublications.mockRejectedValueOnce(Object.assign(new Error('boom'), { status: 500 }));

      const { user } = renderPage();
      await user.click(await screen.findByRole('button', { name: /retry|try again/i }));

      await waitFor(() => expect(api.listMcpProxyApiPortalPublications).toHaveBeenCalledTimes(2));
      expect(await screen.findByText('Main Hub')).toBeInTheDocument();
    });

    it('does not mistake a failed read for an organization with no hubs', async () => {
      api.listMcpProxyApiPortalPublications.mockRejectedValue(Object.assign(new Error('boom'), { status: 500 }));

      renderPage();

      expect(await screen.findByRole('alert')).toBeInTheDocument();
      expect(screen.queryByText('No MCP Hubs Available')).not.toBeInTheDocument();
    });
  });
});
