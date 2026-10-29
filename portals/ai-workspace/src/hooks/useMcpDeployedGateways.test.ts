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

import { getGateways } from '../apis/gatewayApis';
import { getMCPServerDeployments } from '../apis/MCP/mcpServerDeployApis';
import { useMcpDeployedGateways } from './useMcpDeployedGateways';

vi.mock('../apis/gatewayApis', () => ({ getGateways: vi.fn() }));
vi.mock('../apis/MCP/mcpServerDeployApis', () => ({ getMCPServerDeployments: vi.fn() }));
vi.mock('../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const gateway = (id: string) => ({ id, name: id, displayName: id.toUpperCase() });
const deployment = (gatewayId: string, status: string, createdAt: string) => ({ gatewayId, status, createdAt });

/** Answers each gateway's deployment read from `byGateway`. */
const deploymentsByGateway = (byGateway: Record<string, unknown[] | Error>) =>
  vi.mocked(getMCPServerDeployments).mockImplementation(async (_proxyId, _baseUrl, gatewayId) => {
    const answer = byGateway[gatewayId ?? ''] ?? [];
    if (answer instanceof Error) throw answer;
    return { count: answer.length, list: answer } as never;
  });

beforeEach(() => {
  vi.mocked(getGateways).mockReset();
  vi.mocked(getMCPServerDeployments).mockReset();
});

describe('useMcpDeployedGateways', () => {
  it('reads nothing without an organization or an MCP proxy', async () => {
    const { result } = renderHook(() => useMcpDeployedGateways('', 'travel'));

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.gateways).toEqual([]);
    expect(getGateways).not.toHaveBeenCalled();
  });

  it('lists only the gateways the proxy is deployed to, newest deployment first', async () => {
    vi.mocked(getGateways).mockResolvedValue({ list: [gateway('old'), gateway('new'), gateway('never')] } as never);
    deploymentsByGateway({
      old: [deployment('old', 'DEPLOYED', '2026-01-01T00:00:00Z')],
      new: [deployment('new', 'DEPLOYED', '2026-06-01T00:00:00Z')],
      never: [deployment('never', 'UNDEPLOYED', '2026-07-01T00:00:00Z')],
    });

    const { result } = renderHook(() => useMcpDeployedGateways('org', 'travel'));

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.gateways.map(({ id }) => id)).toEqual(['new', 'old']);
  });

  it('orders a gateway by its latest deployment when it has several', async () => {
    vi.mocked(getGateways).mockResolvedValue({ list: [gateway('a'), gateway('b')] } as never);
    deploymentsByGateway({
      a: [deployment('a', 'DEPLOYED', '2026-01-01T00:00:00Z'), deployment('a', 'DEPLOYED', '2026-09-01T00:00:00Z')],
      b: [deployment('b', 'DEPLOYED', '2026-05-01T00:00:00Z')],
    });

    const { result } = renderHook(() => useMcpDeployedGateways('org', 'travel'));

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.gateways.map(({ id }) => id)).toEqual(['a', 'b']);
  });

  it('treats a gateway whose deployments cannot be read as having none', async () => {
    vi.mocked(getGateways).mockResolvedValue({ list: [gateway('ok'), gateway('broken')] } as never);
    deploymentsByGateway({
      ok: [deployment('ok', 'DEPLOYED', '2026-01-01T00:00:00Z')],
      broken: new Error('boom'),
    });

    const { result } = renderHook(() => useMcpDeployedGateways('org', 'travel'));

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.gateways.map(({ id }) => id)).toEqual(['ok']);
  });

  it('ends with no gateways, not an error, when the gateway list itself fails', async () => {
    vi.mocked(getGateways).mockRejectedValue(new Error('down'));

    const { result } = renderHook(() => useMcpDeployedGateways('org', 'travel'));

    await waitFor(() => expect(result.current.isLoading).toBe(false));
    expect(result.current.gateways).toEqual([]);
  });
});
