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

import { getApiCapabilities } from './apiCapabilities';
import { RestApi } from '../../../../../api/resources/restApis';

const baseApi: RestApi = {
  id: 'component-1',
  projectId: 'project-1',
  displayName: 'Orders API',
  // The spec's own kind and transport casing — `RestApi`, lowercase transports.
  // `API_PROXY` was the pre-migration domain name and matched nothing.
  kind: 'RestApi',
  transport: ['http', 'https'],
  description: 'API proxy for the Orders service',
  context: '/orders',
  version: '1.0.0',
  createdAt: '2023-01-01T00:00:00Z',
  updatedAt: '2023-01-02T00:00:00Z',
  upstream: {
    main: {
      url: 'https://api.example.com/orders',
    }
  },
};

describe('getApiCapabilities', () => {
  it('enables cURL testing and deploy for API proxy components', () => {
    const capabilities = getApiCapabilities(baseApi);

    expect(capabilities.canDeploy).toBe(true);
    expect(capabilities.canTest).toBe(true);
    expect(capabilities.testMode).toBe('curl');
  });

  // The regression that hid the entire Test menu: `transport` arrives lowercase
  // per the spec, and was being compared against upper-case constants, so
  // `canTest` came back false for every API.
  it.each([
    ['lowercase, as the spec documents', ['http', 'https']],
    ['upper-case', ['HTTP', 'HTTPS']],
    ['mixed', ['Https']],
  ])('treats %s transports as HTTP-reachable', (_name, transport) => {
    expect(getApiCapabilities({ ...baseApi, transport }).canTest).toBe(true);
  });

  it('is not HTTP-reachable when no HTTP transport is offered', () => {
    expect(getApiCapabilities({ ...baseApi, transport: ['ws'] }).canTest).toBe(
      false
    );
    expect(getApiCapabilities({ ...baseApi, transport: [] }).testMode).toBe(
      'none'
    );
  });

  it('disables component scoped entries without component data', () => {
    const capabilities = getApiCapabilities();

    expect(capabilities.canDeploy).toBe(false);
    expect(capabilities.testMode).toBe('none');
  });
});
