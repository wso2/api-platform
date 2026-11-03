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

import { describe, expect, it } from 'vitest';

import { createDeployClient } from './deployApi';
import type { ApiFetch } from './hostPort';

describe('readApi', () => {
  it('returns the API name alongside its endpoint and read-only flag', async () => {
    const calls: unknown[][] = [];
    const apiFetch = (async (...args: unknown[]) => {
      calls.push(args);
      return { name: 'Pet Store', upstream: { main: { url: 'https://pets.example.com' } }, readOnly: true };
    }) as ApiFetch;
    const api = await createDeployClient(apiFetch, 'proj', 'pets').readApi();
    expect(api).toEqual({ name: 'Pet Store', endpointUrl: 'https://pets.example.com', readOnly: true });
    expect(calls[0]).toEqual(['GET', '/rest-apis/pets']);
  });

  it('leaves the name undefined when the API has none, and read-only false', async () => {
    const apiFetch = (async () => ({})) as ApiFetch;
    expect(await createDeployClient(apiFetch, 'proj', 'pets').readApi()).toEqual({
      name: undefined,
      endpointUrl: undefined,
      readOnly: false,
    });
  });

  it('reads only the display name for the other kinds, and never marks them read-only', async () => {
    const calls: unknown[][] = [];
    const apiFetch = (async (...args: unknown[]) => {
      calls.push(args);
      return { name: 'Weather MCP', readOnly: true, upstream: { main: { url: 'https://x' } } };
    }) as ApiFetch;
    const api = await createDeployClient(apiFetch, 'proj', 'weather', 'Mcp').readApi();
    expect(api).toEqual({ name: 'Weather MCP', readOnly: false });
    expect(calls[0]).toEqual(['GET', '/mcp-proxies/weather']);
  });

  it('leaves the name empty, rather than failing, when another kind cannot be read', async () => {
    const apiFetch = (async () => {
      throw new Error('boom');
    }) as ApiFetch;
    expect(await createDeployClient(apiFetch, 'proj', 'x', 'LlmProxy').readApi()).toEqual({ readOnly: false });
  });
});
