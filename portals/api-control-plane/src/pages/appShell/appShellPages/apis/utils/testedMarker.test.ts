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

import { afterEach, describe, expect, it, vi } from 'vitest';

import { markApiTested, wasApiTested } from './testedMarker';

afterEach(() => {
  vi.unstubAllGlobals();
  window.localStorage.clear();
});

describe('testedMarker', () => {
  it('remembers per API that its test command was copied', () => {
    expect(wasApiTested('orders')).toBe(false);

    markApiTested('orders');

    expect(wasApiTested('orders')).toBe(true);
    expect(wasApiTested('payments')).toBe(false);
  });

  it('degrades to "not tested" when storage is unavailable', () => {
    const blocked = () => {
      throw new Error('blocked');
    };
    vi.stubGlobal('localStorage', { getItem: blocked, setItem: blocked });

    expect(() => markApiTested('orders')).not.toThrow();
    expect(wasApiTested('orders')).toBe(false);
  });
});
