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

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { SCOPES } from '../../../../auth/permissions';
import { useAppAuth } from '../../../../contexts/AppAuthContext';
import { renderWithProviders, screen } from '../../../../test/utils';
import type { MCPServer } from '../../../../utils/types';
import HubOverviewHeader from './HubOverviewHeader';

vi.mock('../../../../contexts/AppAuthContext', () => ({ useAppAuth: vi.fn() }));

const server = {
  id: 'travel',
  displayName: 'Travel MCP',
  version: '1.2.0',
  description: 'Plans trips.',
  context: '/travel',
  updatedAt: '2026-10-01T00:00:00Z',
} as MCPServer;

const ALL = [
  SCOPES.MCP_PROXY_DELETE,
  SCOPES.MCP_PROXY_DEPLOYMENT_CREATE,
  SCOPES.MCP_PROXY_DEPLOYMENT_READ,
  SCOPES.MCP_PROXY_API_PORTAL_MANAGE,
];

const youMay = (...scopes: string[]) =>
  vi.mocked(useAppAuth).mockReturnValue({ hasPermission: (scope: string) => scopes.includes(scope) } as never);

type Props = Partial<React.ComponentProps<typeof HubOverviewHeader>>;

const renderHeader = (props: Props = {}) => {
  const onDeleteClick = vi.fn();
  const onBlockedNavigation = vi.fn();
  const view = renderWithProviders(
    <HubOverviewHeader
      deleteBlockedReason={null}
      initials="TM"
      isReadOnlyServer={false}
      onBlockedNavigation={onBlockedNavigation}
      onDeleteClick={onDeleteClick}
      publishUrl="/publish/travel"
      server={server}
      {...props}
    />
  );
  return { ...view, onBlockedNavigation, onDeleteClick };
};

/** A link's anchor, found by its label: a disabled one has no `href`, so it is not a "link" role. */
const anchorOf = (label: string) => screen.getByText(label).closest('a') as HTMLAnchorElement;

beforeEach(() => {
  youMay(...ALL);
});

describe('HubOverviewHeader', () => {
  describe('the proxy’s identity', () => {
    it('shows its name, version, description and monogram', () => {
      renderHeader();

      expect(screen.getByRole('heading', { name: 'Travel MCP' })).toBeInTheDocument();
      expect(screen.getByText('1.2.0')).toBeInTheDocument();
      expect(screen.getByText('Plans trips.')).toBeInTheDocument();
      expect(screen.getByText('TM')).toBeInTheDocument();
    });

    it('shows its context, or the root for a proxy without one', () => {
      const { unmount } = renderHeader();
      expect(screen.getByText('/travel')).toBeInTheDocument();
      unmount();

      renderHeader({ server: { ...server, context: undefined } });
      expect(screen.getByText('/')).toBeInTheDocument();
    });

    it('says when it was last updated', () => {
      renderHeader();

      expect(screen.getByText('Last updated :')).toBeInTheDocument();
    });
  });

  describe('the description', () => {
    const LONG = 'Plans trips. '.repeat(60);
    const measuredAs = (scrollHeight: number, clientHeight: number) => {
      vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockReturnValue(scrollHeight);
      vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(clientHeight);
    };

    afterEach(() => {
      vi.restoreAllMocks();
    });

    it('has no show more toggle, however long it is', () => {
      measuredAs(200, 60);

      renderHeader({ server: { ...server, description: LONG } });

      expect(screen.queryByRole('button', { name: /show (more|less)/i })).not.toBeInTheDocument();
    });

    it('shows the full text in a tooltip when it is cut off', async () => {
      measuredAs(200, 60);
      const { user } = renderHeader({ server: { ...server, description: LONG } });

      await user.hover(screen.getAllByText(/Plans trips\./)[0]);

      expect(await screen.findByRole('tooltip')).toHaveTextContent(LONG.trim());
    });

    it('has no tooltip when it fits', async () => {
      measuredAs(60, 60);
      const { user } = renderHeader();

      await user.hover(screen.getByText('Plans trips.'));

      expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
    });
  });

  describe('editing', () => {
    it('opens the edit page, even for a gateway-created proxy', () => {
      renderHeader({ isReadOnlyServer: true });

      expect(screen.getByRole('link', { name: 'Edit MCP Proxy' })).toHaveAttribute('href', '/edit');
    });
  });

  describe('deleting', () => {
    it('asks the page to delete', async () => {
      const { onDeleteClick, user } = renderHeader();

      await user.click(screen.getByRole('button', { name: 'Delete Travel MCP' }));

      expect(onDeleteClick).toHaveBeenCalledTimes(1);
    });

    it('is off for a user without the delete scope', () => {
      youMay(...ALL.filter((scope) => scope !== SCOPES.MCP_PROXY_DELETE));

      renderHeader();

      expect(screen.getByRole('button', { name: 'Delete Travel MCP' })).toBeDisabled();
    });

    it('is off while the proxy is still deployed on a gateway it was created from', () => {
      renderHeader({ deleteBlockedReason: 'Undeploy it from the gateway first.' });

      expect(screen.getByRole('button', { name: 'Delete Travel MCP' })).toBeDisabled();
    });

    it('keeps the hook the end-to-end tests find it by', () => {
      renderHeader();

      expect(screen.getByRole('button', { name: 'Delete Travel MCP' })).toHaveAttribute(
        'data-cyid',
        'delete-mcp-proxy-button'
      );
    });
  });

  describe('deployment', () => {
    it('leads to the deploy page', () => {
      renderHeader();

      expect(screen.getByRole('link', { name: 'Deploy to Gateway' })).toHaveAttribute('href', '/deploy');
    });

    it('only lets a user without the deploy scope look, calling it View Deployments', () => {
      youMay(...ALL.filter((scope) => scope !== SCOPES.MCP_PROXY_DEPLOYMENT_CREATE));

      renderHeader();

      expect(screen.getByRole('link', { name: 'View Deployments' })).toBeInTheDocument();
    });

    it('only lets anyone look at a gateway-created proxy, whatever they may do', () => {
      renderHeader({ isReadOnlyServer: true });

      expect(screen.getByRole('link', { name: 'View Deployments' })).toBeInTheDocument();
    });

    it('is off without even the deployment-read scope, since there is nothing to see', () => {
      youMay(SCOPES.MCP_PROXY_DELETE);

      renderHeader();

      expect(anchorOf('View Deployments')).toHaveAttribute('aria-disabled', 'true');
    });

    it('is held back by unsaved changes on the page', async () => {
      const { onBlockedNavigation, user } = renderHeader();

      await user.click(screen.getByRole('link', { name: 'Deploy to Gateway' }));

      expect(onBlockedNavigation).toHaveBeenCalledTimes(1);
    });

    it('is not held back for a read-only proxy, which has nothing to lose', async () => {
      const { onBlockedNavigation, user } = renderHeader({ isReadOnlyServer: true });

      await user.click(screen.getByRole('link', { name: 'View Deployments' }));

      expect(onBlockedNavigation).not.toHaveBeenCalled();
    });
  });

  describe('publication', () => {
    it('leads to the hub list', () => {
      renderHeader();

      expect(screen.getByRole('link', { name: 'Publish to MCP Hub' })).toHaveAttribute('href', '/publish/travel');
    });

    it('opens for a user who may manage publications, since each action is done from that page', () => {
      youMay(SCOPES.MCP_PROXY_API_PORTAL_MANAGE);

      renderHeader();

      expect(screen.getByRole('link', { name: 'Publish to MCP Hub' })).toBeInTheDocument();
    });

    it('is off for a user without the manage scope', () => {
      youMay(SCOPES.MCP_PROXY_DELETE, SCOPES.MCP_PROXY_DEPLOYMENT_READ);

      renderHeader();

      expect(anchorOf('Publish to MCP Hub')).toHaveAttribute('aria-disabled', 'true');
    });

    it('is held back by unsaved changes on the page', async () => {
      const { onBlockedNavigation, user } = renderHeader();

      await user.click(screen.getByRole('link', { name: 'Publish to MCP Hub' }));

      expect(onBlockedNavigation).toHaveBeenCalledTimes(1);
    });
  });
});
