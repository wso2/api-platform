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

import { mcpProxiesApis } from '../apis/MCP/mcpProxiesApis';
import { useMcpProxyDocuments } from './useMcpProxyDocuments';

vi.mock('../apis/MCP/mcpProxiesApis', () => ({ mcpProxiesApis: { listMcpProxyDocuments: vi.fn() } }));
vi.mock('../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const setup = { id: 'setup', type: 'HowTo', displayName: 'Setup' };

beforeEach(() => {
  vi.mocked(mcpProxiesApis.listMcpProxyDocuments).mockReset();
});

describe('useMcpProxyDocuments', () => {
  it('reads nothing until it is enabled', () => {
    const { result } = renderHook(() => useMcpProxyDocuments('travel', false));

    expect(mcpProxiesApis.listMcpProxyDocuments).not.toHaveBeenCalled();
    expect(result.current).toEqual({ documents: undefined, failed: false, upToDate: false });
  });

  it('holds no documents until they are in, then all of them', async () => {
    vi.mocked(mcpProxiesApis.listMcpProxyDocuments).mockResolvedValue([setup]);

    const { result } = renderHook(() => useMcpProxyDocuments('travel', true));
    expect(result.current.documents).toBeUndefined();

    await waitFor(() => expect(result.current.documents).toEqual([setup]));
    expect(mcpProxiesApis.listMcpProxyDocuments).toHaveBeenCalledWith('travel', expect.any(String));
    expect(result.current.upToDate).toBe(true);
  });

  it('keeps the earlier list while it is read again, but not up to date until the new one is in', async () => {
    const faq = { id: 'faq', type: 'FAQ', displayName: 'FAQ' };
    vi.mocked(mcpProxiesApis.listMcpProxyDocuments).mockResolvedValueOnce([setup]);
    const { result, rerender } = renderHook(({ enabled }) => useMcpProxyDocuments('travel', enabled), {
      initialProps: { enabled: true },
    });
    await waitFor(() => expect(result.current.upToDate).toBe(true));

    rerender({ enabled: false });
    vi.mocked(mcpProxiesApis.listMcpProxyDocuments).mockResolvedValueOnce([setup, faq]);
    rerender({ enabled: true });

    expect(result.current.documents).toEqual([setup]);
    expect(result.current.upToDate).toBe(false);
    await waitFor(() => expect(result.current.documents).toEqual([setup, faq]));
    expect(result.current.upToDate).toBe(true);
  });

  it('reads when it becomes enabled', async () => {
    vi.mocked(mcpProxiesApis.listMcpProxyDocuments).mockResolvedValue([setup]);

    const { result, rerender } = renderHook(({ enabled }) => useMcpProxyDocuments('travel', enabled), {
      initialProps: { enabled: false },
    });
    rerender({ enabled: true });

    await waitFor(() => expect(result.current.documents).toEqual([setup]));
    expect(mcpProxiesApis.listMcpProxyDocuments).toHaveBeenCalledTimes(1);
  });

  it('reports a failed read instead of documents', async () => {
    vi.mocked(mcpProxiesApis.listMcpProxyDocuments).mockRejectedValue(new Error('forbidden'));

    const { result } = renderHook(() => useMcpProxyDocuments('travel', true));

    await waitFor(() => expect(result.current.failed).toBe(true));
    expect(result.current.documents).toBeUndefined();
  });
});
