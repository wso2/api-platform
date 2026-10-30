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

import {
  generateSharedKey,
  internalAuthConfig,
  isHttpsUrl,
  sharedKeyHash,
  toHandle,
} from './apiPortalSetup';

describe('toHandle', () => {
  it('lowercases and dashes a display name', () => {
    expect(toHandle('  Partner Portal (EU) ')).toBe('partner-portal-eu');
  });

  it('stays within the spec limit without a trailing dash', () => {
    const handle = toHandle(`${'a'.repeat(39)} b`);
    expect(handle).toBe('a'.repeat(39));
  });
});

describe('isHttpsUrl', () => {
  it('accepts https and rejects everything else', () => {
    expect(isHttpsUrl('https://portal.example.com')).toBe(true);
    expect(isHttpsUrl('http://portal.example.com')).toBe(false);
    expect(isHttpsUrl('portal.example.com')).toBe(false);
  });
});

describe('shared key', () => {
  it('generates 64 hex characters', () => {
    expect(generateSharedKey()).toMatch(/^[0-9a-f]{64}$/);
  });

  it('hashes the key text the way setup.sh does', async () => {
    // printf '%s' abc | openssl dgst -sha256
    expect(await sharedKeyHash('abc')).toBe(
      'ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad',
    );
  });

  it('renders the portal config entry', () => {
    expect(internalAuthConfig('ff')).toBe('[api_portal.internal_auth]\nhash = "ff"');
  });
});
