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

import { Route, Routes, useLocation } from 'react-router-dom';
import { describe, expect, it } from 'vitest';

import { renderWithProviders, screen } from '../../../../test/utils';
import type { PublicationSummaryItem } from '../../../../utils/types';
import { HubPublicationCard } from './HubPublicationCard';

const hub: PublicationSummaryItem = {
  apiPortalId: 'main',
  apiPortalName: 'Main Hub',
  apiPortalDescription: 'The organization’s MCP hub.',
  apiPortalUrl: 'https://hub.example.com',
  status: 'PUBLISHED',
  publicationUpdatedAt: '2026-10-02T00:00:00Z',
};

const publishPath = (apiPortalId: string) => `/publish/travel/portals/${apiPortalId}`;

const renderCard = (publication: Partial<PublicationSummaryItem> = {}) =>
  renderWithProviders(
    <HubPublicationCard
      mcpProxyId="travel"
      orgHandle="acme"
      publication={{ ...hub, ...publication }}
      publishPath={publishPath}
    />
  );

/** The anchor a label sits in: without an address it has no `href`, so it is not a "link" to look up by role. */
const anchorOf = (label: string) => screen.getByText(label).closest('a') as HTMLAnchorElement;

describe('HubPublicationCard', () => {
  it('shows the hub’s name and description', () => {
    renderCard();

    expect(screen.getByText('Main Hub')).toBeInTheDocument();
    expect(screen.getByText('The organization’s MCP hub.')).toBeInTheDocument();
  });

  it('falls back to the hub’s handle when it has no name', () => {
    renderCard({ apiPortalName: undefined });

    expect(screen.getByText('main')).toBeInTheDocument();
  });

  describe('its status', () => {
    it.each([
      ['PUBLISHED', 'Published'],
      ['DEPRECATED', 'Deprecated'],
      ['NOT_PUBLISHED', 'Not published'],
    ] as const)('reads %s as %s', (status, label) => {
      renderCard({ status });

      expect(screen.getByText(label)).toBeInTheDocument();
    });

    it('says when the listing last changed, only when it has one', () => {
      const { unmount } = renderCard();
      expect(screen.getByText(/ago|Just now|Unknown/)).toBeInTheDocument();
      unmount();

      renderCard({ status: 'NOT_PUBLISHED', publicationUpdatedAt: undefined });
      expect(screen.queryByText(/ago|Just now|Unknown/)).not.toBeInTheDocument();
    });
  });

  describe('Go to publish', () => {
    it('opens this MCP proxy’s publish page for the hub', () => {
      renderCard();

      expect(screen.getByRole('link', { name: 'Go to publish' })).toHaveAttribute('href', '/publish/travel/portals/main');
    });

    it('carries the hub’s name along, so the page can title itself before it has read the hub', async () => {
      function Landing() {
        return <p>arrived with {JSON.stringify(useLocation().state)}</p>;
      }
      const { user } = renderWithProviders(
        <Routes>
          <Route
            element={<HubPublicationCard mcpProxyId="travel" orgHandle="acme" publication={hub} publishPath={() => '/landing'} />}
            path="/"
          />
          <Route element={<Landing />} path="/landing" />
        </Routes>
      );

      await user.click(screen.getByRole('link', { name: 'Go to publish' }));

      expect(await screen.findByText('arrived with {"hubName":"Main Hub"}')).toBeInTheDocument();
    });

    it('is off for a hub without an id, since there is no flow to open', () => {
      renderCard({ apiPortalId: undefined });

      expect(anchorOf('Go to publish')).toHaveAttribute('aria-disabled', 'true');
    });
  });

  describe('View in MCP Hub', () => {
    it('links to the proxy’s page on a hub where it is published', () => {
      renderCard();

      const link = screen.getByRole('link', { name: 'View in MCP Hub' });
      expect(link).toHaveAttribute('href', 'https://hub.example.com/api-portal/acme/views/default/mcp/travel');
      expect(link).toHaveAttribute('target', '_blank');
      expect(link).toHaveAttribute('rel', 'noopener noreferrer');
    });

    it('still links for a deprecated listing, which remains on the hub', () => {
      renderCard({ status: 'DEPRECATED' });

      expect(screen.getByRole('link', { name: 'View in MCP Hub' })).toHaveAttribute('href');
    });

    it('has nowhere to link before the proxy was ever published', () => {
      renderCard({ status: 'NOT_PUBLISHED' });

      expect(anchorOf('View in MCP Hub')).toHaveAttribute('aria-disabled', 'true');
    });

    it('has nowhere to link for a hub with no address', () => {
      renderCard({ apiPortalUrl: undefined });

      expect(anchorOf('View in MCP Hub')).toHaveAttribute('aria-disabled', 'true');
    });

    it.each(['javascript:alert(1)', 'not a url'])('does not link to a hub address of %j', (apiPortalUrl) => {
      renderCard({ apiPortalUrl });

      const link = anchorOf('View in MCP Hub');
      expect(link).toHaveAttribute('aria-disabled', 'true');
      expect(link).not.toHaveAttribute('href');
    });
  });
});
