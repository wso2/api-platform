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

import { getGateways } from '../../../../apis/gatewayApis';
import { getMCPServerDeployments } from '../../../../apis/MCP/mcpServerDeployApis';
import { resolveApiPortalUrl } from '../../../../apis/apiPortalApis';
import { mcpProxiesApis } from '../../../../apis/MCP/mcpProxiesApis';
import { NO_PERMISSION_TOOLTIP, SCOPES } from '../../../../auth/permissions';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { useAppShell } from '../../../../contexts/AppShellContext';
import useAIWorkspaceSnackbar from '../../../../hooks/aiWorkspaceSnackbar';
import { renderWithProviders, screen, waitFor } from '../../../../test/utils';
import ExternalServersOverview from './ExternalServersOverview';

/** The two settings a deployment is configured with, switched per case. */
const settings = vi.hoisted(() => ({ apiPortal: false, multiHub: false }));

vi.mock('../../../../config.env', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../../../config.env')>()),
  get API_PORTAL_ENABLED() {
    return settings.apiPortal;
  },
  get MULTI_HUB_PUBLISHING() {
    return settings.apiPortal && settings.multiHub;
  },
}));
vi.mock('../../../../contexts/AppAuthContext', () => ({ useAppAuth: vi.fn() }));
vi.mock('../../../../contexts/AppShellContext', () => ({ useAppShell: vi.fn() }));
vi.mock('../../../../hooks/aiWorkspaceSnackbar', () => ({ default: vi.fn() }));
vi.mock('../../../../apis/gatewayApis', () => ({ getGateways: vi.fn() }));
vi.mock('../../../../apis/MCP/mcpServerDeployApis', () => ({ getMCPServerDeployments: vi.fn() }));
vi.mock('../../../../apis/MCP/mcpProxiesApis', () => ({
  mcpProxiesApis: {
    getMCPServer: vi.fn(),
    getMcpProxyApiPortalPublication: vi.fn(),
    getMcpProxyApiPortalDraft: vi.fn(),
    listMcpProxyApiPortalPublications: vi.fn(),
    publishMcpProxyToApiPortal: vi.fn(),
    unpublishMcpProxyFromApiPortal: vi.fn(),
  },
}));
vi.mock('../../../../apis/apiPortalApis', async (importActual) => ({
  ...(await importActual<typeof import('../../../../apis/apiPortalApis')>()),
  resolveApiPortalUrl: vi.fn().mockResolvedValue(undefined),
}));
vi.mock('../../../../apis/policyHubApis', () => ({ getGuardrails: vi.fn().mockResolvedValue([]) }));
vi.mock('../../../../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const navigate = vi.hoisted(() => vi.fn());
vi.mock('react-router-dom', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router-dom')>()),
  useNavigate: () => navigate,
}));

const api = vi.mocked(mcpProxiesApis);

const server = {
  id: 'travel',
  displayName: 'Travel MCP',
  version: '1.0.0',
  description: 'Plans trips.',
  context: '/travel',
  updatedAt: '2026-10-01T00:00:00Z',
  policies: [],
  capabilities: { tools: [], resources: [], prompts: [] },
};
const gateway = { id: 'gw1', name: 'prod', displayName: 'Prod', endpoints: ['https://gw.example.com'] };

const notFound = () => Object.assign(new Error('not found'), { status: 404 });

/** The proxy is deployed to one gateway, or to none. */
const deployed = (isDeployed: boolean) => {
  vi.mocked(getGateways).mockResolvedValue({ list: [gateway] } as never);
  vi.mocked(getMCPServerDeployments).mockResolvedValue({
    count: isDeployed ? 1 : 0,
    list: isDeployed ? [{ gatewayId: 'gw1', status: 'DEPLOYED', createdAt: '2026-10-01T00:00:00Z' }] : [],
  } as never);
};

const hubs = (...statuses: string[]) =>
  api.listMcpProxyApiPortalPublications.mockResolvedValue({
    list: statuses.map((status, index) => ({ apiPortalId: `hub-${index}`, status })),
    pagination: { total: statuses.length, offset: 0, limit: 100 },
  } as never);

const renderOverview = async () => {
  const view = renderWithProviders(<ExternalServersOverview />, {
    path: '/organizations/acme/projects/alpha/mcp-proxy/:serverId',
    route: '/organizations/acme/projects/alpha/mcp-proxy/travel',
  });
  await screen.findByRole('heading', { name: 'Travel MCP' });
  return view;
};

beforeEach(() => {
  settings.apiPortal = false;
  settings.multiHub = false;
  navigate.mockReset();
  Object.values(api).forEach((fn) => fn.mockReset());
  api.getMCPServer.mockResolvedValue(server as never);
  api.getMcpProxyApiPortalPublication.mockRejectedValue(notFound());
  api.getMcpProxyApiPortalDraft.mockRejectedValue(notFound());
  hubs();
  deployed(true);
  vi.mocked(useAIWorkspaceSnackbar).mockReturnValue(vi.fn() as never);
  vi.mocked(useAppShell).mockReturnValue({
    currentOrganization: { uuid: 'org-1', handle: 'acme', name: 'Acme' },
    currentProject: { id: 'alpha', displayName: 'Alpha' },
    projectsForCurrentOrganization: [{ id: 'alpha', displayName: 'Alpha' }],
  } as never);
  vi.mocked(useAppAuth).mockReturnValue({ hasPermission: () => true } as never);
});

describe('ExternalServersOverview publishing', () => {
  describe('on a deployment that publishes nowhere', () => {
    it('shows the header it always had, with no publishing controls', async () => {
      await renderOverview();

      expect(screen.getByText('Deploy to Gateway')).toBeInTheDocument();
      expect(screen.queryByText('Publish to API Portal')).not.toBeInTheDocument();
      expect(screen.queryByText('Publish to MCP Hub')).not.toBeInTheDocument();
      expect(api.listMcpProxyApiPortalPublications).not.toHaveBeenCalled();
    });
  });

  describe('on a deployment with a single hub, as old cloud runs', () => {
    beforeEach(() => {
      settings.apiPortal = true;
      settings.multiHub = false;
    });

    it('keeps the single-hub publish button in the header it always had', async () => {
      await renderOverview();

      expect(await screen.findByText('Publish to API Portal')).toBeInTheDocument();
    });

    it('has none of the hub pages’ controls, and never asks about hubs', async () => {
      await renderOverview();
      await screen.findByText('Publish to API Portal');

      expect(screen.queryByText('Publish to MCP Hub')).not.toBeInTheDocument();
      expect(api.listMcpProxyApiPortalPublications).not.toHaveBeenCalled();
    });

    it('gives Deploy and Publish the same pill shape and one width, with Unpublish in red and a link to the portal once published', async () => {
      api.getMcpProxyApiPortalPublication.mockResolvedValue({} as never);
      vi.mocked(resolveApiPortalUrl).mockResolvedValue('https://portal.example.com');

      await renderOverview();

      const unpublish = await screen.findByRole('button', { name: 'Unpublish from API Portal' });
      const deploy = screen.getByRole('link', { name: 'Deploy to Gateway' });
      expect(unpublish).toHaveClass('MuiButton-colorError');
      expect(deploy).toHaveStyle({ width: '240px' });
      expect(unpublish).toHaveStyle({ width: '240px' });
      expect(await screen.findByRole('link', { name: /View in API Portal/ })).toHaveAttribute(
        'href',
        expect.stringMatching(/^https:\/\/portal\.example\.com\/acme\/views\/[^/]+\/mcp\/.+/)
      );
    });

    it('is not offered the single-hub button before the proxy is deployed anywhere', async () => {
      deployed(false);

      await renderOverview();

      await waitFor(() => expect(getMCPServerDeployments).toHaveBeenCalled());
      expect(screen.queryByText('Publish to API Portal')).not.toBeInTheDocument();
    });
  });

  describe('on a deployment with several hubs', () => {
    beforeEach(() => {
      settings.apiPortal = true;
      settings.multiHub = true;
    });

    it('swaps the header for the one that publishes to the hub list', async () => {
      await renderOverview();

      expect(await screen.findByRole('link', { name: 'Publish to MCP Hub' })).toHaveAttribute(
        'href',
        '/organizations/acme/projects/alpha/publish/travel'
      );
    });

    it('drops the single-hub publish button, which would publish to just one of them', async () => {
      await renderOverview();
      await screen.findByRole('link', { name: 'Publish to MCP Hub' });

      expect(screen.queryByText('Publish to API Portal')).not.toBeInTheDocument();
      expect(screen.queryByText('Unpublish from API Portal')).not.toBeInTheDocument();
      expect(api.getMcpProxyApiPortalPublication).not.toHaveBeenCalled();
    });

    it('asks which hubs carry the proxy, and counts the publish step done in the setup banner when one does', async () => {
      hubs('NOT_PUBLISHED', 'PUBLISHED');

      await renderOverview();

      expect(await screen.findByText('3/4')).toBeInTheDocument();
      expect(screen.getByRole('link', { name: 'Publish to MCP Hub' })).toBeInTheDocument();
      expect(api.listMcpProxyApiPortalPublications).toHaveBeenCalledWith('travel', expect.any(String));
    });

    it('adds the publish step to the setup banner, and goes to the hub list from it', async () => {
      const { user } = await renderOverview();

      await user.click(await screen.findByRole('button', { name: 'Publish to MCP Hub' }));

      expect(navigate).toHaveBeenCalledWith('/organizations/acme/projects/alpha/publish/travel');
    });

    it('keeps the publish step from leaving the page without the permission the header button needs', async () => {
      const showSnackbar = vi.fn();
      vi.mocked(useAIWorkspaceSnackbar).mockReturnValue(showSnackbar as never);
      vi.mocked(useAppAuth).mockReturnValue({
        hasPermission: (scope: string) => scope !== SCOPES.MCP_PROXY_API_PORTAL_MANAGE,
      } as never);
      const { user } = await renderOverview();

      await user.click(await screen.findByRole('button', { name: 'Publish to MCP Hub' }));

      expect(navigate).not.toHaveBeenCalled();
      expect(showSnackbar).toHaveBeenCalledWith(NO_PERMISSION_TOOLTIP, 'error');
    });

    it('still opens the delete confirmation, which the page owns', async () => {
      const { user } = await renderOverview();

      await user.click(screen.getByRole('button', { name: 'Delete Travel MCP' }));

      expect(await screen.findByRole('dialog')).toBeInTheDocument();
    });

    it('still offers publishing, with the publish step not counted, when the status cannot be read', async () => {
      api.listMcpProxyApiPortalPublications.mockRejectedValue(Object.assign(new Error('boom'), { status: 500 }));

      await renderOverview();

      await waitFor(() => expect(api.listMcpProxyApiPortalPublications).toHaveBeenCalled());
      expect(await screen.findByRole('link', { name: 'Publish to MCP Hub' })).toBeInTheDocument();
      expect(screen.getByText('2/4')).toBeInTheDocument();
    });
  });
});
