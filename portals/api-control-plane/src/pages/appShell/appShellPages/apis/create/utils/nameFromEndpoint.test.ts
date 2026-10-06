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

import { nameFromEndpoint, versionFromEndpoint } from './nameFromEndpoint';

describe('nameFromEndpoint', () => {
  it('names the API after the resource in the path, not the host', () => {
    expect(
      nameFromEndpoint('https://apis.bijira.dev/samples/reading-list-api-service/v1.0/books'),
    ).toBe('Books');
    expect(nameFromEndpoint('https://apis.bijira.dev/samples/reading-list-api-service/v1.0')).toBe(
      'Reading List API Service',
    );
    expect(nameFromEndpoint('https://api.example.com/orders/v2')).toBe('Orders');
  });

  it('skips versions, ids and generic routing words', () => {
    expect(nameFromEndpoint('https://example.com/api/v1/customers/42')).toBe('Customers');
    expect(nameFromEndpoint('https://example.com/rest/payments/2026-01-01')).toBe('Payments');
  });

  it('falls back to the host, minus generic prefixes', () => {
    expect(nameFromEndpoint('https://api.example.com/v1')).toBe('Example');
    expect(nameFromEndpoint('http://billing:8080/')).toBe('Billing');
  });

  it('offers nothing it cannot stand behind', () => {
    expect(nameFromEndpoint('not a url')).toBe('');
    expect(nameFromEndpoint('http://10.0.0.7:8080/')).toBe('');
    expect(nameFromEndpoint('http://[::1]:8080/')).toBe('');
    expect(nameFromEndpoint('https://x.io/a/')).toBe('');
  });
});

describe('versionFromEndpoint', () => {
  it('reads an explicit v<major>[.<minor>] segment', () => {
    expect(versionFromEndpoint('https://api.example.com/orders/v2')).toBe('2.0.0');
    expect(versionFromEndpoint('https://example.com/svc/v1.4/items')).toBe('1.4.0');
  });

  it('ignores bare numbers, dates and leading zeros', () => {
    expect(versionFromEndpoint('https://example.com/orders/2')).toBe('');
    expect(versionFromEndpoint('https://example.com/2026-01-01/orders')).toBe('');
    expect(versionFromEndpoint('https://example.com/v01/orders')).toBe('');
    expect(versionFromEndpoint('not a url')).toBe('');
  });
});
