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

import { describe, it, expect, vi, beforeEach } from 'vitest';

import { get } from '../../clients/choreoApiClient';
import { logger } from '../../utils/logger';
import { mcpProxyExists } from './mcpProxiesApis';

vi.mock('../../clients/choreoApiClient', () => ({
  get: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
  del: vi.fn(),
}));

vi.mock('../../utils/logger', () => ({
  logger: { error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn() },
}));

const BASE = 'https://control-plane.test';

beforeEach(() => {
  vi.mocked(get).mockReset();
  vi.mocked(logger.error).mockReset();
});

describe('mcpProxyExists', () => {
  it('is true when the MCP proxy can be fetched', async () => {
    vi.mocked(get).mockResolvedValue({ id: 'one' });

    await expect(mcpProxyExists('one', BASE)).resolves.toBe(true);
    expect(get).toHaveBeenCalledWith('/mcp-proxies/one', undefined, BASE);
  });

  it('is false on a 404 without logging it as an error', async () => {
    vi.mocked(get).mockRejectedValue({ status: 404 });

    await expect(mcpProxyExists('free-id', BASE)).resolves.toBe(false);
    expect(logger.error).not.toHaveBeenCalled();
  });

  it('rethrows and logs any other failure', async () => {
    const failure = { status: 500 };
    vi.mocked(get).mockRejectedValue(failure);

    await expect(mcpProxyExists('one', BASE)).rejects.toBe(failure);
    expect(logger.error).toHaveBeenCalled();
  });

  it('encodes the identifier into the path', async () => {
    vi.mocked(get).mockResolvedValue({});

    await mcpProxyExists('a/b', BASE);

    expect(get).toHaveBeenCalledWith('/mcp-proxies/a%2Fb', undefined, BASE);
  });
});
