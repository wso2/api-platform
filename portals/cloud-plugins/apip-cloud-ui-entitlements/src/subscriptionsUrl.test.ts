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

import { describe, expect, it } from 'vitest';

import { subscriptionsUrl } from './subscriptionsUrl';

describe('subscriptionsUrl', () => {
  it('opens the subscriptions on the configured Cloud Console', () => {
    expect(subscriptionsUrl('https://console.example.com/')).toBe(
      'https://console.example.com/billing?tab=subscriptions'
    );
  });

  it("falls back to this page's origin, which Cloud Console shares", () => {
    expect(subscriptionsUrl(undefined)).toBe(`${window.location.origin}/billing?tab=subscriptions`);
    expect(subscriptionsUrl('  ')).toBe(`${window.location.origin}/billing?tab=subscriptions`);
  });

  it('never links to another scheme', () => {
    expect(subscriptionsUrl('javascript:alert(1)')).toBe(
      `${window.location.origin}/billing?tab=subscriptions`
    );
    expect(subscriptionsUrl('not a url')).toBe(`${window.location.origin}/billing?tab=subscriptions`);
  });
});
