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

import { mcpProxiesApis } from '../../../../apis/MCP/mcpProxiesApis';
import { usePublishProject } from '../../../../hooks/usePublishProject';
import { logger } from '../../../../utils/logger';
import { useHubPublishing } from './useHubPublishing';

const gate = vi.hoisted(() => ({ on: true }));

vi.mock('../../../../config.env', () => ({
  isFeatureEnabled: (feature: string) => feature === 'api-portal-integration' && gate.on,
}));
vi.mock('../../../../apis/MCP/mcpProxiesApis', () => ({ mcpProxiesApis: { listMcpProxyApiPortalPublications: vi.fn() } }));
vi.mock('../../../../hooks/usePublishProject', () => ({ usePublishProject: vi.fn() }));
vi.mock('../../../../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const list = vi.mocked(mcpProxiesApis.listMcpProxyApiPortalPublications);

const hubsAre = (...statuses: string[]) =>
  list.mockResolvedValue({
    list: statuses.map((status, index) => ({ apiPortalId: `hub-${index}`, status })),
    pagination: { total: statuses.length, offset: 0, limit: 100 },
  } as never);

const inOrganization = (organizationId: string) =>
  vi.mocked(usePublishProject).mockReturnValue({
    organizationId,
    publishPath: (suffix = '') => `/publish${suffix}`,
  } as never);

/** Lets the read settle, for the cases whose answer is the same as before it. */
const settle = () => act(async () => undefined);

beforeEach(() => {
  gate.on = true;
  list.mockReset();
  vi.mocked(logger.error).mockReset();
  inOrganization('org-1');
});

describe('useHubPublishing', () => {
  describe('where publishing to MCP Hubs is on', () => {
    it('is published once any hub carries the proxy as published', async () => {
      hubsAre('NOT_PUBLISHED', 'PUBLISHED');

      const { result } = renderHook(() => useHubPublishing('travel'));

      await waitFor(() => expect(result.current.isPublished).toBe(true));
      expect(list).toHaveBeenCalledWith('travel', expect.any(String));
    });

    it('counts a deprecated listing, which is still on the hub', async () => {
      hubsAre('DEPRECATED');

      const { result } = renderHook(() => useHubPublishing('travel'));

      await waitFor(() => expect(result.current.isPublished).toBe(true));
    });

    it('is not published when no hub carries it', async () => {
      hubsAre('NOT_PUBLISHED', 'NOT_PUBLISHED');

      const { result } = renderHook(() => useHubPublishing('travel'));
      await settle();

      expect(list).toHaveBeenCalled();
      expect(result.current.isPublished).toBe(false);
    });

    it('is not published in an organization with no hubs, which is not an error', async () => {
      hubsAre();

      const { result } = renderHook(() => useHubPublishing('travel'));
      await settle();

      expect(result.current.isPublished).toBe(false);
      expect(logger.error).not.toHaveBeenCalled();
    });

    it('stays not published, and logs, when the read fails', async () => {
      list.mockRejectedValue(new Error('forbidden'));

      const { result } = renderHook(() => useHubPublishing('travel'));
      await settle();

      expect(result.current.isPublished).toBe(false);
      expect(logger.error).toHaveBeenCalled();
    });

    it('forgets the previous proxy while the next one is read', async () => {
      hubsAre('PUBLISHED');
      const { rerender, result } = renderHook(({ id }) => useHubPublishing(id), { initialProps: { id: 'travel' } });
      await waitFor(() => expect(result.current.isPublished).toBe(true));

      list.mockReturnValue(new Promise(() => undefined));
      rerender({ id: 'weather' });

      await waitFor(() => expect(result.current.isPublished).toBe(false));
    });

    it('reads nothing before there is a proxy or an organization', () => {
      inOrganization('');
      const { rerender } = renderHook(({ id }) => useHubPublishing(id), { initialProps: { id: 'travel' as string | undefined } });
      rerender({ id: undefined });

      expect(list).not.toHaveBeenCalled();
    });

    it('starts the publish flow at the hub list for this proxy', () => {
      hubsAre();

      const { result } = renderHook(() => useHubPublishing('a b'));

      expect(result.current.publishUrl).toBe('/publish/a%20b');
    });
  });

  describe('where publishing to MCP Hubs is off', () => {
    beforeEach(() => {
      gate.on = false;
    });

    it('reads nothing at all, and reports nothing published', () => {
      const { result } = renderHook(() => useHubPublishing('travel'));

      expect(list).not.toHaveBeenCalled();
      expect(result.current.isPublished).toBe(false);
    });
  });
});
