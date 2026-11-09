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

import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor } from '@testing-library/react';

import { mcpProxiesApis } from '../../apis/MCP/mcpProxiesApis';
import { MCPServersProvider, useMCPServers } from './MCPProxiesContext';

vi.mock('../../apis/MCP/mcpProxiesApis', () => ({
  mcpProxiesApis: {
    getMCPServers: vi.fn(),
    createMCPServer: vi.fn(),
    updateMCPServer: vi.fn(),
    deleteMCPServer: vi.fn(),
  },
}));
vi.mock('../../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const currentOrganization = { uuid: 'org-1' };
const currentProject = { id: 'proj-1' };
vi.mock('../AppShellContext', () => ({
  useAppShell: () => ({ currentOrganization, currentProject }),
}));

const wrapper = ({ children }: { children: React.ReactNode }) => (
  <MCPServersProvider>{children}</MCPServersProvider>
);

const renderServers = () => renderHook(() => useMCPServers(), { wrapper });

beforeEach(() => {
  vi.mocked(mcpProxiesApis.getMCPServers).mockReset();
});

describe('loading the MCP Proxies list', () => {
  it('does not truncate a project with more MCP proxies than the server default page size', async () => {
    // Same class of bug reported for App LLM Proxies: a project with more
    // than 20 MCP proxies only ever showed 20, because the list fetch never
    // passed an explicit limit and the server defaults an unpaged request to 20.
    const allServers = {
      count: 25,
      list: Array.from({ length: 25 }, (_, i) => ({ id: `mcp-${i}`, displayName: `mcp-${i}` })),
      pagination: { total: 25, offset: 0, limit: 100 },
    };
    vi.mocked(mcpProxiesApis.getMCPServers).mockResolvedValue(allServers as never);

    const { result } = renderServers();

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.mcpServersResponse.list).toHaveLength(25);
    expect(mcpProxiesApis.getMCPServers).toHaveBeenCalledWith(
      'proj-1',
      expect.any(String),
      100,
      0
    );
  });

  it('walks a second page when a project holds more than one full page of MCP proxies', async () => {
    const firstPage = {
      count: 100,
      list: Array.from({ length: 100 }, (_, i) => ({ id: `mcp-${i}`, displayName: `mcp-${i}` })),
      pagination: { total: 110, offset: 0, limit: 100 },
    };
    const secondPage = {
      count: 10,
      list: Array.from({ length: 10 }, (_, i) => ({ id: `mcp-${100 + i}`, displayName: `mcp-${100 + i}` })),
      pagination: { total: 110, offset: 100, limit: 100 },
    };
    vi.mocked(mcpProxiesApis.getMCPServers)
      .mockResolvedValueOnce(firstPage as never)
      .mockResolvedValueOnce(secondPage as never);

    const { result } = renderServers();

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.mcpServersResponse.list).toHaveLength(110);
    expect(mcpProxiesApis.getMCPServers).toHaveBeenCalledTimes(2);
    expect(mcpProxiesApis.getMCPServers).toHaveBeenNthCalledWith(
      2,
      'proj-1',
      expect.any(String),
      100,
      100
    );
  });
});
