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

import { act, renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { getApiPortal } from '../../../../apis/apiPortalApis';
import { mcpProxiesApis } from '../../../../apis/MCP/mcpProxiesApis';
import { useMcpDeployedGateways } from '../../../../hooks/useMcpDeployedGateways';
import { usePublishPageData } from './usePublishPageData';

vi.mock('../../../../apis/apiPortalApis', () => ({ getApiPortal: vi.fn() }));
vi.mock('../../../../apis/MCP/mcpProxiesApis', () => ({
  mcpProxiesApis: {
    getMCPServer: vi.fn(),
    getMcpProxyApiPortalDraft: vi.fn(),
    getMcpProxyApiPortalPublication: vi.fn(),
    getMcpProxyApiPortalDraftDefinition: vi.fn(),
    getMcpProxyApiPortalPublicationDefinition: vi.fn(),
    getMcpProxyApiPortalDraftLandingPage: vi.fn(),
    getMcpProxyApiPortalPublicationLandingPage: vi.fn(),
    getMcpProxyApiPortalDraftThumbnail: vi.fn(),
    getMcpProxyApiPortalPublicationThumbnail: vi.fn(),
  },
}));
vi.mock('../../../../hooks/useMcpDeployedGateways', () => ({ useMcpDeployedGateways: vi.fn() }));
vi.mock('../../../../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const api = vi.mocked(mcpProxiesApis);

const proxyCapabilities = { tools: [{ name: 'search' }], resources: [], prompts: [] };
const server = {
  id: 'travel',
  displayName: 'Travel MCP',
  version: '1.0.0',
  description: 'From the proxy',
  context: '/travel',
  capabilities: proxyCapabilities,
};
const hub = { id: 'hub', handle: 'hub', name: 'Main Hub', status: 'ACTIVE' };
const draft = { displayName: 'Draft name', version: '2.0.0', subscriptionPlanIds: ['gold'], docIds: [] };
const publication = {
  displayName: 'Live name',
  version: '1.0.0',
  status: 'PUBLISHED',
  subscriptionPlanIds: ['silver'],
  docIds: [],
};
const gateway = { id: 'gw1', name: 'prod', displayName: 'Prod', endpoints: ['https://gw.example.com'] };

const draftImage = new Blob(['draft'], { type: 'image/png' });
const liveImage = new Blob(['live'], { type: 'image/png' });

const notFound = () => Object.assign(new Error('not found'), { status: 404 });

/** Every read answers "nothing saved" unless a case says otherwise. */
const readsAre = (answers: Partial<Record<'draft' | 'publication' | 'draftDefinition' | 'liveDefinition', unknown>> = {}) => {
  const answer = (value: unknown) => (value === undefined ? Promise.reject(notFound()) : Promise.resolve(value));
  api.getMCPServer.mockResolvedValue(server as never);
  api.getMcpProxyApiPortalDraft.mockImplementation(() => answer(answers.draft) as never);
  api.getMcpProxyApiPortalPublication.mockImplementation(() => answer(answers.publication) as never);
  api.getMcpProxyApiPortalDraftDefinition.mockImplementation(() => answer(answers.draftDefinition) as never);
  api.getMcpProxyApiPortalPublicationDefinition.mockImplementation(() => answer(answers.liveDefinition) as never);
  api.getMcpProxyApiPortalDraftLandingPage.mockResolvedValue('# Draft page');
  api.getMcpProxyApiPortalPublicationLandingPage.mockResolvedValue('# Live page');
  api.getMcpProxyApiPortalDraftThumbnail.mockResolvedValue(draftImage);
  api.getMcpProxyApiPortalPublicationThumbnail.mockResolvedValue(liveImage);
  vi.mocked(getApiPortal).mockResolvedValue(hub as never);
};

const gatewaysAre = (gateways: unknown[], isLoading = false) =>
  vi.mocked(useMcpDeployedGateways).mockReturnValue({ gateways, isLoading } as never);

const renderData = (wantPublished = false) =>
  renderHook(({ want }) => usePublishPageData('hub', 'travel', 'org', want), { initialProps: { want: wantPublished } });

beforeEach(() => {
  Object.values(api).forEach((fn) => fn.mockReset());
  vi.mocked(getApiPortal).mockReset();
  vi.mocked(useMcpDeployedGateways).mockReset();
  gatewaysAre([]);
});

describe('usePublishPageData', () => {
  describe('what the editor opens with', () => {
    it('starts from the draft, and from the draft’s own definition', async () => {
      readsAre({ draft, publication, draftDefinition: { tools: [{ name: 'draft-tool' }] }, liveDefinition: { tools: [] } });

      const { result } = renderData();

      await waitFor(() => expect(result.current.seed).toBeDefined());
      expect(result.current.seed?.values).toMatchObject({ displayName: 'Draft name', subscriptionPlanIds: ['gold'] });
      expect(JSON.parse(result.current.seed?.definitionText ?? '{}').tools).toEqual([{ name: 'draft-tool' }]);
    });

    it('does not read the live definition when the draft has its own', async () => {
      readsAre({ draft, publication, draftDefinition: { tools: [] } });

      const { result } = renderData();

      await waitFor(() => expect(result.current.seed).toBeDefined());
      expect(api.getMcpProxyApiPortalPublicationDefinition).not.toHaveBeenCalled();
    });

    it('falls back to the live listing’s definition when the draft stored none', async () => {
      readsAre({ publication, liveDefinition: { tools: [{ name: 'live-tool' }] } });

      const { result } = renderData();

      await waitFor(() => expect(result.current.seed).toBeDefined());
      expect(result.current.seed?.values.displayName).toBe('Live name');
      expect(JSON.parse(result.current.seed?.definitionText ?? '{}').tools).toEqual([{ name: 'live-tool' }]);
    });

    it('falls back to what the proxy itself exposes when nothing was ever saved', async () => {
      readsAre();

      const { result } = renderData();

      await waitFor(() => expect(result.current.seed).toBeDefined());
      expect(result.current.seed?.values).toMatchObject({ displayName: 'Travel MCP', version: '1.0.0' });
      expect(JSON.parse(result.current.seed?.definitionText ?? '{}').tools).toEqual([{ name: 'search' }]);
    });

    it('defaults the production URL to the newest gateway the proxy is deployed to', async () => {
      readsAre();
      gatewaysAre([gateway, { ...gateway, id: 'gw2', endpoints: ['https://older.example.com'] }]);

      const { result } = renderData();

      await waitFor(() => expect(result.current.seed).toBeDefined());
      expect(result.current.seed?.values.productionUrl).toBe('https://gw.example.com/travel/mcp');
      expect(result.current.productionUrlOptions.map(({ url }) => url)).toEqual([
        'https://gw.example.com/travel/mcp',
        'https://older.example.com/travel/mcp',
      ]);
    });

    it('treats a missing draft, listing or definition as nothing saved, not as a failure', async () => {
      readsAre();

      const { result } = renderData();

      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(result.current.error).toBeUndefined();
      expect(result.current.draft).toBeUndefined();
      expect(result.current.publication).toBeUndefined();
    });
  });

  describe('loading', () => {
    it('stays loading until the gateways are read too', async () => {
      readsAre();
      gatewaysAre([], true);

      const { result, rerender } = renderData();
      await waitFor(() => expect(api.getMCPServer).toHaveBeenCalled());

      expect(result.current.isLoading).toBe(true);
      expect(result.current.seed).toBeUndefined();

      gatewaysAre([], false);
      rerender({ want: false });
      await waitFor(() => expect(result.current.isLoading).toBe(false));
    });

    it('reads nothing without an organization', () => {
      readsAre();

      renderHook(() => usePublishPageData('hub', 'travel', '', false));

      expect(api.getMCPServer).not.toHaveBeenCalled();
    });

    it('does not go back to loading when the page is read again after one of its own saves', async () => {
      readsAre({ draft });

      const { result } = renderData();
      await waitFor(() => expect(result.current.isLoading).toBe(false));

      act(() => result.current.refetch());

      expect(result.current.isLoading).toBe(false);
      await waitFor(() => expect(api.getMCPServer).toHaveBeenCalledTimes(2));
    });
  });

  describe('the landing page', () => {
    const withPage = { hasLandingPage: true };

    it('opens on the draft’s own page, which the draft already stores', async () => {
      readsAre({ draft: { ...draft, ...withPage }, publication: { ...publication, ...withPage } });

      const { result } = renderData();

      await waitFor(() => expect(result.current.seed).toBeDefined());
      expect(result.current.seed?.landingPage).toEqual({ text: '# Draft page', inDraft: true });
      expect(result.current.seed?.landingPageFailed).toBe(false);
    });

    it('falls back to the live listing’s page, which the draft does not store yet', async () => {
      readsAre({ draft, publication: { ...publication, ...withPage } });

      const { result } = renderData();

      await waitFor(() => expect(result.current.seed).toBeDefined());
      expect(result.current.seed?.landingPage).toEqual({ text: '# Live page', inDraft: false });
      expect(api.getMcpProxyApiPortalDraftLandingPage).not.toHaveBeenCalled();
    });

    it('has none, and reads none, when neither the draft nor the live listing says it has one', async () => {
      readsAre({ draft, publication });

      const { result } = renderData();

      await waitFor(() => expect(result.current.seed).toBeDefined());
      expect(result.current.seed?.landingPage).toBeUndefined();
      expect(result.current.seed?.landingPageFailed).toBe(false);
      expect(api.getMcpProxyApiPortalDraftLandingPage).not.toHaveBeenCalled();
      expect(api.getMcpProxyApiPortalPublicationLandingPage).not.toHaveBeenCalled();
    });

    it('does not fall through to the live page when the draft’s own cannot be read, so it is never written over', async () => {
      readsAre({ draft: { ...draft, ...withPage }, publication: { ...publication, ...withPage } });
      api.getMcpProxyApiPortalDraftLandingPage.mockRejectedValue(new Error('boom'));

      const { result } = renderData();

      await waitFor(() => expect(result.current.seed).toBeDefined());
      expect(result.current.seed?.landingPage).toBeUndefined();
      expect(result.current.seed?.landingPageFailed).toBe(true);
    });

    it('still opens when the live page cannot be read, and says so for the published view', async () => {
      readsAre({ draft, publication: { ...publication, ...withPage } });
      api.getMcpProxyApiPortalPublicationLandingPage.mockRejectedValue(new Error('boom'));

      const { result } = renderData();

      await waitFor(() => expect(result.current.seed).toBeDefined());
      expect(result.current.seed?.landingPageFailed).toBe(true);
      expect(result.current.publishedLandingPage).toEqual({ failed: true });
      expect(result.current.error).toBeUndefined();
    });

    it('shows the published view the live page whatever the draft holds', async () => {
      readsAre({ draft: { ...draft, ...withPage }, publication: { ...publication, ...withPage } });

      const { result } = renderData();

      await waitFor(() => expect(result.current.seed).toBeDefined());
      expect(result.current.publishedLandingPage).toEqual({ text: '# Live page', failed: false });
    });
  });

  describe('the thumbnail', () => {
    const withImage = { hasThumbnail: true };

    it('opens on the draft’s own image, which the draft already stores', async () => {
      readsAre({ draft: { ...draft, ...withImage }, publication: { ...publication, ...withImage } });

      const { result } = renderData();

      await waitFor(() => expect(result.current.seed).toBeDefined());
      expect(result.current.seed?.thumbnail).toEqual({ blob: draftImage, inDraft: true });
    });

    it('falls back to the live listing’s image, which the draft does not store yet', async () => {
      readsAre({ draft, publication: { ...publication, ...withImage } });

      const { result } = renderData();

      await waitFor(() => expect(result.current.seed).toBeDefined());
      expect(result.current.seed?.thumbnail).toEqual({ blob: liveImage, inDraft: false });
      expect(api.getMcpProxyApiPortalDraftThumbnail).not.toHaveBeenCalled();
    });

    it('has none, and reads none, when neither the draft nor the live listing says it has one', async () => {
      readsAre({ draft, publication });

      const { result } = renderData();

      await waitFor(() => expect(result.current.seed).toBeDefined());
      expect(result.current.seed?.thumbnail).toBeUndefined();
      expect(api.getMcpProxyApiPortalDraftThumbnail).not.toHaveBeenCalled();
      expect(api.getMcpProxyApiPortalPublicationThumbnail).not.toHaveBeenCalled();
    });

    it('does not fall through to the live image when the draft’s own cannot be read, so it is never written over', async () => {
      readsAre({ draft: { ...draft, ...withImage }, publication: { ...publication, ...withImage } });
      api.getMcpProxyApiPortalDraftThumbnail.mockRejectedValue(new Error('boom'));

      const { result } = renderData();

      await waitFor(() => expect(result.current.seed).toBeDefined());
      expect(result.current.seed?.thumbnail).toBeUndefined();
      expect(result.current.error).toBeUndefined();
    });

    it('still opens when the live image cannot be read', async () => {
      readsAre({ draft, publication: { ...publication, ...withImage } });
      api.getMcpProxyApiPortalPublicationThumbnail.mockRejectedValue(new Error('boom'));

      const { result } = renderData();

      await waitFor(() => expect(result.current.seed).toBeDefined());
      expect(result.current.seed?.thumbnail).toBeUndefined();
      expect(result.current.publishedThumbnail).toBeUndefined();
      expect(result.current.error).toBeUndefined();
    });

    it('shows the published view the live image whatever the draft holds', async () => {
      readsAre({ draft: { ...draft, ...withImage }, publication: { ...publication, ...withImage } });

      const { result } = renderData();

      await waitFor(() => expect(result.current.seed).toBeDefined());
      expect(result.current.publishedThumbnail).toBe(liveImage);
    });
  });

  describe('failures', () => {
    it('reports a failed read of the proxy, and stops loading', async () => {
      readsAre();
      api.getMCPServer.mockRejectedValue(Object.assign(new Error('forbidden'), { status: 403 }));

      const { result } = renderData();

      await waitFor(() => expect(result.current.error).toBeDefined());
      expect(result.current.error?.message).toBe('forbidden');
      expect(result.current.isLoading).toBe(false);
      expect(result.current.seed).toBeUndefined();
    });

    it('reports a failed read of the draft, which is not the same as there being none', async () => {
      readsAre();
      api.getMcpProxyApiPortalDraft.mockRejectedValue(Object.assign(new Error('boom'), { status: 500 }));

      const { result } = renderData();

      await waitFor(() => expect(result.current.error?.message).toBe('boom'));
    });

    it('still opens when the hub’s own details cannot be read', async () => {
      readsAre({ draft });
      vi.mocked(getApiPortal).mockRejectedValue(new Error('forbidden'));

      const { result } = renderData();

      await waitFor(() => expect(result.current.seed).toBeDefined());
      expect(result.current.portal).toBeUndefined();
      expect(result.current.error).toBeUndefined();
    });

    it('wraps a thrown value that is not an Error', async () => {
      readsAre();
      api.getMCPServer.mockRejectedValue('plain text');

      const { result } = renderData();

      await waitFor(() => expect(result.current.error?.message).toBe('plain text'));
    });
  });

  describe('the published definition', () => {
    it('is read only once it is wanted, and only for a live listing', async () => {
      readsAre({ draft, publication, draftDefinition: { tools: [] }, liveDefinition: { tools: [{ name: 'live' }] } });

      const { result, rerender } = renderData(false);
      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(api.getMcpProxyApiPortalPublicationDefinition).not.toHaveBeenCalled();

      rerender({ want: true });

      await waitFor(() =>
        expect(result.current.publishedDefinition).toEqual({
          definition: { tools: [{ name: 'live' }] },
          failed: false,
          isLoading: false,
        })
      );
    });

    it('is not read when there is no live listing to read it from', async () => {
      readsAre({ draft });

      const { result } = renderData(true);
      await waitFor(() => expect(result.current.isLoading).toBe(false));

      expect(api.getMcpProxyApiPortalPublicationDefinition).not.toHaveBeenCalled();
    });

    it('reads as none when the listing stored no definition', async () => {
      readsAre({ publication, draftDefinition: { tools: [] } });

      const { result } = renderData(true);

      await waitFor(() => expect(result.current.publishedDefinition.isLoading).toBe(false));
      expect(result.current.publishedDefinition).toEqual({ definition: undefined, failed: false, isLoading: false });
    });

    it('says it failed when it cannot be read', async () => {
      readsAre({ publication, draftDefinition: { tools: [] } });
      api.getMcpProxyApiPortalPublicationDefinition.mockRejectedValue(Object.assign(new Error('boom'), { status: 500 }));

      const { result } = renderData(true);

      await waitFor(() => expect(result.current.publishedDefinition.failed).toBe(true));
      expect(result.current.publishedDefinition.definition).toBeUndefined();
    });
  });
});
