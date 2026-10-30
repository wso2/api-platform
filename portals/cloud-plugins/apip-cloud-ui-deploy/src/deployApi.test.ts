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

  it('reads nothing for kinds other than REST APIs', async () => {
    let called = false;
    const apiFetch = (async () => {
      called = true;
    }) as ApiFetch;
    const api = await createDeployClient(apiFetch, 'proj', 'x', 'LlmProvider' as never).readApi();
    expect(api).toEqual({ readOnly: false });
    expect(called).toBe(false);
  });
});
