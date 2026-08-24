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

import type { Api } from '../../types/domain';
import { getApiCapabilities } from './apiCapabilities';

const baseApi: Api = {
  id: 'component-1',
  projectId: 'project-1',
  name: 'orders-api',
  displayName: 'Orders API',
  handler: 'orders-api',
  kind: 'API_PROXY',
  status: 'ACTIVE',
  httpBased: true,
};

describe('getApiCapabilities', () => {
  it('enables cURL testing and deploy for API proxy components', () => {
    const capabilities = getApiCapabilities(baseApi);

    expect(capabilities.canDeploy).toBe(true);
    expect(capabilities.canTest).toBe(true);
    expect(capabilities.testMode).toBe('curl');
  });

  it('disables component scoped entries without component data', () => {
    const capabilities = getApiCapabilities();

    expect(capabilities.canDeploy).toBe(false);
    expect(capabilities.testMode).toBe('none');
  });
});
