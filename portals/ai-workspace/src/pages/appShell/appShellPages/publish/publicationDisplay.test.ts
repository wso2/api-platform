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

import { buildViewInPortalUrl, isListedOnPortal, publicationStatusMeta } from './publicationDisplay';

describe('isListedOnPortal', () => {
  it('counts a published listing and a deprecated one, which is still on the hub', () => {
    expect(isListedOnPortal('PUBLISHED')).toBe(true);
    expect(isListedOnPortal('DEPRECATED')).toBe(true);
  });

  it('does not count a proxy that was never published, or no status at all', () => {
    expect(isListedOnPortal('NOT_PUBLISHED')).toBe(false);
    expect(isListedOnPortal(undefined)).toBe(false);
  });
});

describe('publicationStatusMeta', () => {
  it.each([
    ['PUBLISHED', 'success', 'Published'],
    ['DEPRECATED', 'warning', 'Deprecated'],
    ['NOT_PUBLISHED', 'default', 'Not published'],
    [undefined, 'default', 'Not published'],
  ] as const)('shows %s as a %s chip reading %s', (status, color, label) => {
    const meta = publicationStatusMeta(status);

    expect(meta.color).toBe(color);
    expect(meta.label.defaultMessage).toBe(label);
  });
});

describe('buildViewInPortalUrl', () => {
  it('points at the proxy’s page in the hub’s default view', () => {
    expect(buildViewInPortalUrl('https://hub.example.com', 'acme', 'travel')).toBe(
      'https://hub.example.com/api-portal/acme/views/default/mcp/travel'
    );
  });

  it('keeps a path the hub is mounted under, without doubling its trailing slash', () => {
    expect(buildViewInPortalUrl('https://example.com/hub/', 'acme', 'travel')).toBe(
      'https://example.com/hub/api-portal/acme/views/default/mcp/travel'
    );
  });

  it('encodes the organization and proxy handles', () => {
    expect(buildViewInPortalUrl('https://hub.example.com', 'a b', 'x/y')).toBe(
      'https://hub.example.com/api-portal/a%20b/views/default/mcp/x%2Fy'
    );
  });

  it.each(['not a url', '', 'javascript:alert(1)', 'data:text/html,<b>x</b>', 'ftp://hub.example.com'])(
    'gives no link for %j, which is not an http(s) address',
    (address) => {
      expect(buildViewInPortalUrl(address, 'acme', 'travel')).toBeUndefined();
    }
  );

  it('accepts a plain http hub, as a local one often is', () => {
    expect(buildViewInPortalUrl('http://localhost:3000', 'acme', 'travel')).toBe(
      'http://localhost:3000/api-portal/acme/views/default/mcp/travel'
    );
  });
});
