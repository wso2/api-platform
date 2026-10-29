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

import { renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { mcpProxiesApis } from '../../../../apis/MCP/mcpProxiesApis';
import { usePublishProject } from '../../../../hooks/usePublishProject';
import { useHubPublishing } from './useHubPublishing';

const gate = vi.hoisted(() => ({ on: true }));

vi.mock('../../../../config.env', () => ({
  get MULTI_HUB_PUBLISHING() {
    return gate.on;
  },
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

beforeEach(() => {
  gate.on = true;
  list.mockReset();
  inOrganization('org-1');
});

describe('useHubPublishing', () => {
  describe('where the deployment publishes to hubs', () => {
    it('is listed once any hub carries the proxy as published', async () => {
      hubsAre('NOT_PUBLISHED', 'PUBLISHED');

      const { result } = renderHook(() => useHubPublishing('travel'));

      await waitFor(() => expect(result.current.isListed).toBe(true));
      expect(result.current.hasPublication).toBe(true);
      expect(list).toHaveBeenCalledWith('travel', expect.any(String));
    });

    it('counts a deprecated listing, which is still on the hub', async () => {
      hubsAre('DEPRECATED');

      const { result } = renderHook(() => useHubPublishing('travel'));

      await waitFor(() => expect(result.current.isListed).toBe(true));
    });

    it('is not listed when no hub carries it', async () => {
      hubsAre('NOT_PUBLISHED', 'NOT_PUBLISHED');

      const { result } = renderHook(() => useHubPublishing('travel'));

      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(result.current.isListed).toBe(false);
      expect(result.current.hasPublication).toBe(false);
    });

    it('is not listed in an organization with no hubs, which is not an error', async () => {
      hubsAre();

      const { result } = renderHook(() => useHubPublishing('travel'));

      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(result.current.isListed).toBe(false);
    });

    it('is loading while it reads', async () => {
      hubsAre('PUBLISHED');

      const { result } = renderHook(() => useHubPublishing('travel'));

      expect(result.current.isLoading).toBe(true);
      await waitFor(() => expect(result.current.isLoading).toBe(false));
    });

    it('does not claim the proxy is unpublished when the read fails', async () => {
      list.mockRejectedValue(new Error('forbidden'));

      const { result } = renderHook(() => useHubPublishing('travel'));

      await waitFor(() => expect(result.current.isLoading).toBe(false));
      expect(result.current.isListed).toBeNull();
      expect(result.current.hasPublication).toBe(false);
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

  describe('where the deployment has a single hub', () => {
    beforeEach(() => {
      gate.on = false;
    });

    it('reads nothing at all', () => {
      renderHook(() => useHubPublishing('travel'));

      expect(list).not.toHaveBeenCalled();
    });

    it('tells the banner to show no publish step', () => {
      const { result } = renderHook(() => useHubPublishing('travel'));

      expect(result.current.hasPublication).toBeUndefined();
      expect(result.current.isListed).toBeNull();
      expect(result.current.isLoading).toBe(false);
    });
  });
});
