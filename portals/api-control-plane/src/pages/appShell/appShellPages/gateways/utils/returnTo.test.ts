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

import { readReturnTo, withReturnTo } from './returnTo';

const DEPLOY = '/organizations/acme/projects/shop/apis/orders/deploy';

describe('returnTo', () => {
  it('round-trips an in-app path through a query string', () => {
    const url = withReturnTo('/organizations/acme/gateways/new', DEPLOY);

    expect(readReturnTo(url.slice(url.indexOf('?')))).toBe(DEPLOY);
  });

  it('appends to an existing query string', () => {
    expect(withReturnTo('/a?tab=1', '/b')).toBe('/a?tab=1&returnTo=%2Fb');
  });

  it('leaves the path alone when there is nowhere to return to', () => {
    expect(withReturnTo('/a', undefined)).toBe('/a');
  });

  it('refuses anything that would leave the app', () => {
    for (const unsafe of ['https://evil.example', '//evil.example', '/\\evil.example', 'deploy']) {
      expect(readReturnTo(`?returnTo=${encodeURIComponent(unsafe)}`)).toBeUndefined();
      expect(withReturnTo('/a', unsafe)).toBe('/a');
    }
  });
});
